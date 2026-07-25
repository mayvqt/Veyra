package mediaserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/integrations"
)

type Client struct {
	provider  provider
	baseURL   string
	publicURL string
	apiKey    string
	http      *http.Client
}

type ImageResponse struct {
	Body        io.ReadCloser
	ContentType string
}

const mediaServerJSONLimit = 2 << 20

func NewClient(serverType config.MediaServerType, baseURL, publicURL, apiKey string) (*Client, error) {
	provider, err := newProvider(serverType)
	if err != nil {
		return nil, err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if err := validateServerURL("internal URL", baseURL); err != nil {
		return nil, err
	}
	if err := validateServerURL("public URL", publicURL); err != nil {
		return nil, err
	}
	return &Client{
		provider:  provider,
		baseURL:   baseURL,
		publicURL: publicURL,
		apiKey:    strings.TrimSpace(apiKey),
		http:      &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func validateServerURL(label, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid media server %s: %w", label, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid media server %s: must be an http or https URL", label)
	}
	if parsed.User != nil {
		return fmt.Errorf("invalid media server %s: credentials are not allowed", label)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid media server %s: query parameters and fragments are not allowed", label)
	}
	return nil
}

func (c *Client) ID() string   { return "media_server" }
func (c *Client) Name() string { return c.provider.Name() }
func (c *Client) Kind() string { return "media" }
func (c *Client) HasAPIKey() bool {
	return c.apiKey != ""
}

func (c *Client) setAuthHeader(req *http.Request, token string) {
	key := strings.TrimSpace(token)
	if key == "" {
		key = c.apiKey
	}
	if key == "" {
		return
	}
	c.provider.Authorize(req, key)
}

func (c *Client) Health(ctx context.Context) integrations.HealthStatus {
	req, err := c.newRequest(ctx, http.MethodGet, "/System/Ping", "", nil)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: err.Error()}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return integrations.HealthStatus{OK: true, Message: "Online"}
	}
	return integrations.HealthStatus{OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

type latestItem struct {
	ID             string `json:"Id"`
	Name           string `json:"Name"`
	Type           string `json:"Type"`
	ProductionYear int    `json:"ProductionYear"`
	DateCreated    string `json:"DateCreated"`
	ImageTags      struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

var tmdbSuffixRE = regexp.MustCompile(`\s*\[tmdbid-\d+\]\s*$`)

func (c *Client) RecentlyAdded(ctx context.Context, userID, token string, limit int) ([]dashboard.MediaItem, error) {
	if limit <= 0 {
		return []dashboard.MediaItem{}, nil
	}
	// Pull recent movies and recent TV separately, then merge. This avoids one
	// category crowding out the other when the media server applies Latest limits.
	var (
		movies   []dashboard.MediaItem
		tv       []dashboard.MediaItem
		movieErr error
		tvErr    error
		wg       sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		movies, movieErr = c.fetchLatestByTypes(ctx, userID, token, limit, "Movie")
	}()
	go func() {
		defer wg.Done()
		tv, tvErr = c.fetchLatestByTypes(ctx, userID, token, limit, "Episode,Series")
	}()
	wg.Wait()
	if movieErr != nil {
		return nil, movieErr
	}
	if tvErr != nil {
		return nil, tvErr
	}
	merged := append(movies, tv...)
	seen := make(map[string]struct{}, len(merged))
	out := make([]dashboard.MediaItem, 0, len(merged))
	for _, item := range merged {
		key := item.OpenURL + "|" + item.Title
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].AddedAt.After(out[j].AddedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (c *Client) fetchLatestByTypes(ctx context.Context, userID, token string, limit int, types string) ([]dashboard.MediaItem, error) {
	path := c.provider.LatestItemsPath(userID, types, limit)
	req, err := c.newRequest(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s latest failed: %d", c.Name(), resp.StatusCode)
	}

	var rows []latestItem
	if err := decodeMediaServerJSON(resp.Body, &rows); err != nil {
		return nil, err
	}
	out := make([]dashboard.MediaItem, 0, len(rows))
	for _, r := range rows {
		addedAt, _ := time.Parse(time.RFC3339, r.DateCreated)
		item := dashboard.MediaItem{Title: cleanDisplayTitle(r.Name), Type: r.Type, Year: r.ProductionYear, AddedAt: addedAt}
		if r.ID != "" && c.publicURL != "" {
			item.OpenURL = c.provider.ItemURL(c.publicURL, r.ID)
		}
		if r.ID != "" {
			img := url.Values{}
			if r.ImageTags.Primary != "" {
				img.Set("tag", r.ImageTags.Primary)
			}
			item.ImageURL = fmt.Sprintf("/media/server/poster/%s?%s", url.PathEscape(r.ID), img.Encode())
		}
		out = append(out, item)
	}
	return out, nil
}

func cleanDisplayTitle(name string) string {
	name = strings.TrimSpace(name)
	name = tmdbSuffixRE.ReplaceAllString(name, "")
	if name == "" {
		return "Untitled"
	}
	return name
}

func (c *Client) PrimaryImage(ctx context.Context, itemID, tag, token string, maxWidth int) (*ImageResponse, error) {
	q := url.Values{}
	if maxWidth > 0 {
		q.Set("maxWidth", fmt.Sprintf("%d", maxWidth))
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	path := fmt.Sprintf("/Items/%s/Images/Primary?%s", url.PathEscape(itemID), q.Encode())
	req, err := c.newRequest(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s image failed: %d", c.Name(), resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return &ImageResponse{Body: resp.Body, ContentType: contentType}, nil
}

func (c *Client) newRequest(ctx context.Context, method, path, token string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, fmt.Errorf("%s is not configured", c.Name())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req, token)
	return req, nil
}

func decodeMediaServerJSON(body io.Reader, out any) error {
	return json.NewDecoder(io.LimitReader(body, mediaServerJSONLimit)).Decode(out)
}
