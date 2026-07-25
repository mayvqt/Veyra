package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/integrations"
)

type Client struct {
	name    string
	baseURL string
	apiKey  string
	apiBase string
	http    *http.Client
}

func NewClient(name, baseURL, apiKey string) *Client {
	return newClient(name, baseURL, apiKey, "/api/v3")
}

func NewProwlarrClient(baseURL, apiKey string) *Client {
	return newClient("Prowlarr", baseURL, apiKey, "/api/v1")
}

func newClient(name, baseURL, apiKey, apiBase string) *Client {
	return &Client{name: name, baseURL: strings.TrimRight(normalizeBaseURL(baseURL), "/"), apiKey: apiKey, apiBase: apiBase, http: &http.Client{Timeout: 10 * time.Second}}
}

func normalizeBaseURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "://") {
		return value
	}
	return "http://" + value
}

func (c *Client) ID() string   { return strings.ToLower(c.name) }
func (c *Client) Name() string { return c.name }
func (c *Client) Kind() string { return "downloads" }

func (c *Client) Health(ctx context.Context) integrations.HealthStatus {
	if c == nil {
		return integrations.HealthStatus{OK: false, Message: "not configured"}
	}
	if c.baseURL == "" || c.apiKey == "" {
		return integrations.HealthStatus{OK: false, Message: "not configured"}
	}
	req, err := c.newRequest(ctx, http.MethodGet, c.apiPath("/system/status"), nil)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: err.Error()}
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return integrations.HealthStatus{OK: true, Message: "Online"}
	}
	return integrations.HealthStatus{OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func (c *Client) Queue(ctx context.Context, limit int) ([]dashboard.QueueItem, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return nil, fmt.Errorf("%s is not configured", c.name)
	}
	pageSize := limit
	if pageSize <= 0 {
		pageSize = 250
	}
	out := make([]dashboard.QueueItem, 0, max(limit, 8))
	seen := make(map[string]struct{}, max(limit, 16))
	seenRelease := make(map[string]struct{}, max(limit, 16))
	titles := newQueueTitleResolver()
	for page := 1; ; page++ {
		rows, total, err := c.queuePage(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for i, row := range rows {
			if !isActiveDownload(row, c.name) {
				continue
			}
			key := queueDedupKey(row)
			if _, exists := seen[key]; exists {
				continue
			}
			releaseKey := queueReleaseDedupKey(row)
			if releaseKey != "" {
				if _, exists := seenRelease[releaseKey]; exists {
					continue
				}
			}
			item := dashboard.QueueItem{
				Title:     c.queueTitleResolved(ctx, row, titles),
				Subtitle:  queueSubtitle(row),
				Source:    c.name,
				Kind:      queueKind(row, c.name),
				Status:    queueStatus(row),
				Progress:  queueProgress(row),
				TimeLeft:  queueTimeLeft(row),
				Client:    firstString(row, "downloadClient", "downloadClientName"),
				Protocol:  titleWords(firstString(row, "protocol")),
				SizeLeft:  formatBytes(firstNumber(row, "sizeleft", "sizeLeft")),
				SortTime:  firstTime(row, "estimatedCompletionTime", "added"),
				SortIndex: ((page - 1) * pageSize) + i,
			}
			item.StatusKey = statusKey(item.Status)
			if item.Title == "" {
				item.Title = firstString(row, "title")
			}
			if item.Title == "" {
				item.Title = "Unknown download"
			}
			seen[key] = struct{}{}
			if releaseKey != "" {
				seenRelease[releaseKey] = struct{}{}
			}
			out = append(out, item)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		if limit > 0 || page*pageSize >= total || len(rows) < pageSize {
			break
		}
	}
	return out, nil
}

type queueTitleResolver struct {
	seriesByID  map[int]string
	movieByID   map[int]string
	episodeByID map[int]string
	misses      map[string]struct{}
	lookups     int
	maxLookups  int
}

func newQueueTitleResolver() *queueTitleResolver {
	return &queueTitleResolver{
		seriesByID:  make(map[int]string, 32),
		movieByID:   make(map[int]string, 32),
		episodeByID: make(map[int]string, 64),
		misses:      make(map[string]struct{}, 32),
		maxLookups:  8,
	}
}

func (c *Client) queueTitleResolved(ctx context.Context, row map[string]any, cache *queueTitleResolver) string {
	fallback := queueTitle(row)
	if fallback != "" && !looksLikeReleaseTitle(fallback) {
		return fallback
	}
	if cache != nil {
		if n := intPath(row, "seriesId"); n > 0 {
			if title := cache.resolve("series", n, cache.seriesByID, func() string {
				return c.fetchSeriesTitle(ctx, n)
			}); title != "" {
				return title
			}
		}
		if n := intPath(row, "movieId"); n > 0 {
			if title := cache.resolve("movie", n, cache.movieByID, func() string {
				return c.fetchMovieTitle(ctx, n)
			}); title != "" {
				return title
			}
		}
		if n := intPath(row, "episodeId"); n > 0 {
			if title := cache.resolve("episode", n, cache.episodeByID, func() string {
				return c.fetchEpisodeSeriesTitle(ctx, n)
			}); title != "" {
				return title
			}
		}
	}
	return fallback
}

func (r *queueTitleResolver) resolve(kind string, id int, cache map[int]string, fetch func() string) string {
	if title := cache[id]; title != "" {
		return title
	}
	missKey := fmt.Sprintf("%s:%d", kind, id)
	if _, ok := r.misses[missKey]; ok {
		return ""
	}
	if r.lookups >= r.maxLookups {
		return ""
	}
	r.lookups++
	title := fetch()
	if title == "" {
		r.misses[missKey] = struct{}{}
		return ""
	}
	cache[id] = title
	return title
}

var releaseTitleHintRE = regexp.MustCompile(`(?i)(\b(19|20)\d{2}\b|\b(480p|720p|1080p|2160p|webrip|web[- .]?dl|bluray|x264|x265|hevc)\b)`)

func looksLikeReleaseTitle(title string) bool {
	s := strings.TrimSpace(title)
	if s == "" {
		return false
	}
	if releaseTitleHintRE.MatchString(s) {
		return true
	}
	return strings.Count(s, ".") >= 3 || strings.Count(s, "_") >= 3 || strings.Count(s, "-") >= 3
}

func (c *Client) fetchSeriesTitle(ctx context.Context, id int) string {
	return c.fetchEntityTitle(ctx, fmt.Sprintf("/api/v3/series/%d", id), []string{"title"})
}

func (c *Client) fetchMovieTitle(ctx context.Context, id int) string {
	return c.fetchEntityTitle(ctx, fmt.Sprintf("/api/v3/movie/%d", id), []string{"title"})
}

func (c *Client) fetchEpisodeSeriesTitle(ctx context.Context, id int) string {
	return c.fetchEntityTitle(ctx, fmt.Sprintf("/api/v3/episode/%d", id), []string{"series", "title"}, []string{"seriesTitle"}, []string{"title"})
}

func (c *Client) fetchEntityTitle(ctx context.Context, path string, titlePaths ...[]string) string {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return ""
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	var row map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&row); err != nil {
		return ""
	}
	for _, p := range titlePaths {
		if s := stringPath(row, p...); s != "" {
			return s
		}
	}
	return ""
}

func queueDedupKey(row map[string]any) string {
	for _, f := range []string{"id", "queueId", "downloadId", "trackedDownloadId"} {
		if s := firstString(row, f); s != "" {
			return "id:" + strings.ToLower(strings.TrimSpace(s))
		}
	}
	if n := intPath(row, "episodeId"); n > 0 {
		return fmt.Sprintf("episode:%d", n)
	}
	if n := intPath(row, "movieId"); n > 0 {
		return fmt.Sprintf("movie:%d", n)
	}
	title := strings.ToLower(strings.TrimSpace(queueTitle(row)))
	if title != "" {
		season := intPath(row, "episode", "seasonNumber")
		episode := intPath(row, "episode", "episodeNumber")
		return fmt.Sprintf("fallback:%s:%d:%d", title, season, episode)
	}
	return fmt.Sprintf("fallback:%s:%s", strings.ToLower(firstString(row, "downloadClient", "downloadClientName")), strings.ToLower(firstString(row, "status", "trackedDownloadStatus")))
}

func queueReleaseDedupKey(row map[string]any) string {
	rel := strings.ToLower(strings.TrimSpace(firstString(row, "title")))
	if rel == "" {
		return ""
	}
	return rel
}

func (c *Client) Upcoming(ctx context.Context, days int, limit int) ([]dashboard.CalendarItem, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return nil, fmt.Errorf("%s is not configured", c.name)
	}
	if days <= 0 {
		days = 7
	}
	start := time.Now().UTC()
	end := start.Add(time.Duration(days) * 24 * time.Hour)
	return c.UpcomingWindow(ctx, start, end, limit)
}

func (c *Client) UpcomingWindow(ctx context.Context, start, end time.Time, limit int) ([]dashboard.CalendarItem, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return nil, fmt.Errorf("%s is not configured", c.name)
	}
	if end.IsZero() || !end.After(start) {
		return nil, nil
	}
	q := url.Values{}
	q.Set("start", start.Format(time.RFC3339))
	q.Set("end", end.Format(time.RFC3339))
	q.Set("includeSeries", "true")
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v3/calendar?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s calendar failed: %d", c.name, resp.StatusCode)
	}
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	out := make([]dashboard.CalendarItem, 0, len(rows))
	for _, row := range rows {
		airsAt := calendarTime(row)
		item := dashboard.CalendarItem{
			Title:        calendarTitle(row),
			Subtitle:     calendarSubtitle(row),
			Kind:         queueKind(row, c.name),
			Source:       c.name,
			AirsAt:       airsAt,
			Availability: calendarAvailability(row, airsAt),
		}
		if item.Title == "" || item.AirsAt.IsZero() {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func calendarTitle(row map[string]any) string {
	if s := stringPath(row, "series", "title"); s != "" {
		return s
	}
	for _, path := range [][]string{
		{"seriesTitle"},
		{"series", "sortTitle"},
		{"movie", "title"},
		{"title"},
	} {
		if s := stringPath(row, path...); s != "" {
			return s
		}
	}
	return ""
}

func calendarSubtitle(row map[string]any) string {
	season := intPath(row, "seasonNumber")
	episode := intPath(row, "episodeNumber")
	episodeTitle := stringPath(row, "title")
	if season > 0 && episode > 0 {
		sub := fmt.Sprintf("S%02dE%02d", season, episode)
		if episodeTitle != "" {
			sub += " " + episodeTitle
		}
		return sub
	}
	return firstString(row, "title")
}

func calendarTime(row map[string]any) time.Time {
	for _, field := range []string{"airDateUtc", "inCinemas", "digitalRelease", "physicalRelease"} {
		if s := firstString(row, field); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func calendarAvailability(row map[string]any, airsAt time.Time) string {
	if boolPath(row, "hasFile") {
		return "available"
	}
	if !airsAt.IsZero() && airsAt.After(time.Now().UTC()) {
		return "upcoming"
	}
	return "missing"
}

func (c *Client) queuePage(ctx context.Context, page, pageSize int) ([]map[string]any, int, error) {
	q := url.Values{}
	q.Set("page", fmt.Sprintf("%d", max(page, 1)))
	q.Set("pageSize", fmt.Sprintf("%d", max(pageSize, 1)))
	q.Set("sortKey", "timeleft")
	q.Set("sortDirection", "ascending")
	q.Set("includeUnknownMovieItems", "true")
	q.Set("includeUnknownSeriesItems", "true")
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v3/queue?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, 0, fmt.Errorf("%s queue failed: %d", c.name, resp.StatusCode)
	}
	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, err
	}
	rows := queueRows(payload)
	total := queueTotal(payload)
	if total <= 0 {
		total = len(rows)
	}
	return rows, total, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	return req, nil
}

func (c *Client) httpClient() *http.Client {
	if c == nil || c.http == nil {
		return http.DefaultClient
	}
	return c.http
}

func isActiveDownload(row map[string]any, source string) bool {
	status := strings.ToLower(firstString(row, "status"))
	tracked := strings.ToLower(firstString(row, "trackedDownloadStatus"))
	source = strings.ToLower(strings.TrimSpace(source))
	for _, s := range []string{status, tracked} {
		if strings.Contains(s, "downloading") ||
			strings.Contains(s, "downloadclientavailable") ||
			strings.Contains(s, "importpending") {
			return true
		}
		// Radarr often surfaces movie downloads as queued/pending while still active in client.
		if strings.Contains(s, "queued") || strings.Contains(s, "pending") {
			if rowHasMovie(row) || source == "radarr" {
				return true
			}
		}
		// If remaining size indicates active transfer, keep visible.
		size := firstNumber(row, "size")
		left := firstNumber(row, "sizeleft", "sizeLeft")
		if size > 0 && left > 0 && left < size {
			return true
		}
	}
	return false
}

func rowHasMovie(row map[string]any) bool {
	if _, ok := row["movie"].(map[string]any); ok {
		return true
	}
	return intPath(row, "movieId") > 0
}

func queueRows(payload any) []map[string]any {
	switch v := payload.(type) {
	case []any:
		return mapsFromAny(v)
	case map[string]any:
		if records, ok := v["records"].([]any); ok {
			return mapsFromAny(records)
		}
		if items, ok := v["items"].([]any); ok {
			return mapsFromAny(items)
		}
	}
	return nil
}

func queueTotal(payload any) int {
	root, ok := payload.(map[string]any)
	if !ok {
		return 0
	}
	for _, key := range []string{"totalRecords", "total"} {
		if v, ok := root[key].(float64); ok {
			return int(v)
		}
	}
	return 0
}

func mapsFromAny(rows []any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func queueTitle(row map[string]any) string {
	for _, path := range [][]string{
		{"movie", "title"},
		{"series", "title"},
		{"episode", "title"},
		{"title"},
	} {
		if s := stringPath(row, path...); s != "" {
			return s
		}
	}
	return ""
}

func queueKind(row map[string]any, source string) string {
	if _, ok := row["movie"].(map[string]any); ok {
		return "Movie"
	}
	if _, ok := row["series"].(map[string]any); ok {
		return "TV"
	}
	src := strings.ToLower(strings.TrimSpace(source))
	if src == "radarr" {
		return "Movie"
	}
	if src == "sonarr" {
		return "TV"
	}
	return "Media"
}

func queueSubtitle(row map[string]any) string {
	season := intPath(row, "episode", "seasonNumber")
	episode := intPath(row, "episode", "episodeNumber")
	if season == 0 {
		season = intPath(row, "seasonNumber")
	}
	if episode == 0 {
		episode = intPath(row, "episodeNumber")
	}
	episodeTitle := stringPath(row, "episode", "title")
	if episodeTitle == "" {
		episodeTitle = firstString(row, "title")
	}
	if season > 0 && episode > 0 {
		return fmt.Sprintf("S%02dE%02d %s", season, episode, episodeTitle)
	}
	quality := stringPath(row, "quality", "quality", "name")
	if quality == "" {
		quality = stringPath(row, "quality", "name")
	}
	return quality
}

func queueStatus(row map[string]any) string {
	status := firstString(row, "status", "trackedDownloadStatus")
	if status == "" {
		return "Queued"
	}
	status = strings.ReplaceAll(status, "_", " ")
	status = strings.ReplaceAll(status, "-", " ")
	return titleWords(status)
}

func queueProgress(row map[string]any) int {
	size := firstNumber(row, "size")
	left := firstNumber(row, "sizeleft", "sizeLeft")
	if size <= 0 || left < 0 {
		return 0
	}
	progress := int(math.Round((1 - (left / size)) * 100))
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func queueTimeLeft(row map[string]any) string {
	if s := firstString(row, "timeleft", "timeLeft"); s != "" {
		return s
	}
	eta := firstTime(row, "estimatedCompletionTime")
	if eta.IsZero() {
		return ""
	}
	d := time.Until(eta).Round(time.Minute)
	if d <= 0 {
		return "soon"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func firstString(row map[string]any, fields ...string) string {
	for _, field := range fields {
		if s := stringPath(row, field); s != "" {
			return s
		}
	}
	return ""
}

func firstNumber(row map[string]any, fields ...string) float64 {
	for _, field := range fields {
		if v, ok := row[field]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case int:
				return float64(n)
			}
		}
	}
	return 0
}

func firstTime(row map[string]any, fields ...string) time.Time {
	for _, field := range fields {
		if s := stringPath(row, field); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func stringPath(row map[string]any, path ...string) string {
	var cur any = row
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[key]
	}
	if s, ok := cur.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func intPath(row map[string]any, path ...string) int {
	var cur any = row
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur = m[key]
	}
	if n, ok := cur.(float64); ok {
		return int(n)
	}
	return 0
}

func boolPath(row map[string]any, path ...string) bool {
	var cur any = row
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		cur = m[key]
	}
	if b, ok := cur.(bool); ok {
		return b
	}
	return false
}

func formatBytes(n float64) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for i := 0; i < len(units)-1; i++ {
		if n < 1024 {
			return fmt.Sprintf("%.0f %s", n, units[i])
		}
		n /= 1024
	}
	return fmt.Sprintf("%.1f TB", n)
}

func titleWords(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func statusKey(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
