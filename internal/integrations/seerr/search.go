package seerr

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

const tmdbPosterBaseURL = "https://image.tmdb.org/t/p/w342"

func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = cleanSearchQuery(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	if limit <= 0 {
		limit = 8
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/search?query="+escapeSearchQuery(query), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("seerr search failed: %s", seerrHTTPError(resp))
	}
	var payload requestResp
	if err := decodeLimitedSeerrJSON(resp.Body, &payload); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, min(limit, len(payload.Results)))
	for _, row := range payload.Results {
		result := c.searchResult(row)
		if result.ID <= 0 || result.MediaType == "" {
			continue
		}
		out = append(out, result)
		if len(out) >= limit {
			break
		}
	}
	c.hydrateTVSeasons(ctx, out)
	return out, nil
}

func (c *Client) hydrateTVSeasons(ctx context.Context, results []SearchResult) {
	const maxSeasonLookups = 4
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxSeasonLookups)
	for i := range results {
		if results[i].MediaType != "tv" || !results[i].CanRequest {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			results[i].Seasons = c.tvSeasonOptions(ctx, results[i].ID)
		}(i)
	}
	wg.Wait()
}

func cleanSearchQuery(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

func escapeSearchQuery(query string) string {
	return strings.ReplaceAll(url.QueryEscape(query), "+", "%20")
}

func (c *Client) searchResult(row map[string]any) SearchResult {
	mediaType := normalizeSeerrMediaType(firstString(row, "mediaType"))
	if mediaType == "" {
		return SearchResult{}
	}
	id, _ := directInt(row, "id")
	title := firstString(row, "title", "name", "originalTitle", "originalName")
	if title == "" {
		title = "Untitled"
	}
	statusCode, _ := intPath(row, "mediaInfo", "status")
	hasRequest := activeRequestExists(row)
	status := mediaAvailabilityLabel(statusCode, hasRequest)
	path := posterPath(row)
	return SearchResult{
		ID:         id,
		MediaType:  mediaType,
		Title:      title,
		Year:       mediaYear(row),
		Overview:   firstString(row, "overview"),
		PosterPath: path,
		PosterURL:  posterURL(path),
		Status:     status,
		CanRequest: statusCode != 5 && !hasRequest,
		OpenURL:    c.publicMediaURL(mediaType, id),
	}
}

func posterPath(row map[string]any) string {
	return firstString(row, "posterPath", "poster_path", "poster", "image")
}

func posterURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "//") {
		return "https:" + path
	}
	if strings.HasPrefix(path, "/") {
		return tmdbPosterBaseURL + path
	}
	return ""
}

func normalizeSeerrMediaType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "movie":
		return "movie"
	case "tv", "series":
		return "tv"
	default:
		return ""
	}
}

func mediaYear(row map[string]any) string {
	for _, field := range []string{"releaseDate", "firstAirDate"} {
		raw := firstString(row, field)
		if len(raw) >= 4 {
			return raw[:4]
		}
	}
	return ""
}

func mediaAvailabilityLabel(statusCode int, hasRequest bool) string {
	if hasRequest {
		return "Requested"
	}
	switch statusCode {
	case 2:
		return "Pending"
	case 3:
		return "Processing"
	case 4:
		return "Partial"
	case 5:
		return "Available"
	default:
		return "Requestable"
	}
}

func activeRequestExists(row map[string]any) bool {
	requests, ok := anySlicePath(row, "mediaInfo", "requests")
	if !ok {
		return false
	}
	for _, item := range requests {
		req, ok := item.(map[string]any)
		if !ok {
			continue
		}
		status, _ := directInt(req, "status")
		if status == 1 || status == 2 {
			return true
		}
	}
	return false
}

func (c *Client) publicMediaURL(mediaType string, id int) string {
	if c.publicURL == "" || id <= 0 {
		return ""
	}
	return fmt.Sprintf("%s/%s/%d", c.publicURL, mediaType, id)
}

func (c *Client) tvSeasonOptions(ctx context.Context, tvID int) []SeasonOption {
	if tvID <= 0 {
		return nil
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/tv/"+strconv.Itoa(tvID), nil)
	if err != nil {
		return nil
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil
	}
	var payload map[string]any
	if err := decodeLimitedSeerrJSON(resp.Body, &payload); err != nil {
		return nil
	}
	rows, ok := anySlicePath(payload, "seasons")
	if !ok {
		return nil
	}
	out := make([]SeasonOption, 0, len(rows))
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		number, _ := directInt(row, "seasonNumber")
		if number <= 0 {
			continue
		}
		episodes, _ := directInt(row, "episodeCount")
		name := firstString(row, "name")
		if name == "" {
			name = fmt.Sprintf("Season %d", number)
		}
		out = append(out, SeasonOption{Number: number, Name: name, EpisodeCount: episodes})
	}
	return out
}
