package mediaserver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/integrations"
)

const (
	recentTVTypes        = "Episode"
	recentTVInitialLimit = 100
	recentTVMaxLimit     = 4096
)

type latestItem struct {
	ID                    string `json:"Id"`
	Name                  string `json:"Name"`
	Type                  string `json:"Type"`
	ProductionYear        int    `json:"ProductionYear"`
	DateCreated           string `json:"DateCreated"`
	SeriesID              string `json:"SeriesId"`
	SeriesName            string `json:"SeriesName"`
	SeriesPrimaryImageTag string `json:"SeriesPrimaryImageTag"`
	ParentIndexNumber     *int   `json:"ParentIndexNumber"`
	IndexNumber           *int   `json:"IndexNumber"`
	ImageTags             struct {
		Primary string `json:"Primary"`
	} `json:"ImageTags"`
}

var tmdbSuffixRE = regexp.MustCompile(`\s*\[tmdbid-\d+\]\s*$`)

func (c *Client) RecentlyAdded(ctx context.Context, userID, token string, limit int) ([]dashboard.MediaItem, error) {
	if limit <= 0 {
		return []dashboard.MediaItem{}, nil
	}
	// Bound the whole traversal, not just each upstream request. A failed or
	// incomplete traversal must not replace a complete cached list.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tvLimit := min(max(limit, recentTVInitialLimit), recentTVMaxLimit)
	var (
		movies   []latestItem
		tv       []latestItem
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
		tv, tvErr = c.fetchLatestByTypes(ctx, userID, token, tvLimit, recentTVTypes)
	}()
	wg.Wait()
	if movieErr != nil {
		return nil, movieErr
	}
	for {
		if tvErr != nil {
			return nil, tvErr
		}
		items, err := c.mergeLatestItems(movies, tv, limit)
		if err != nil {
			return nil, err
		}
		if len(tv) < tvLimit || recentWindowComplete(items, tv, limit) {
			return items, nil
		}
		if tvLimit == recentTVMaxLimit {
			return nil, fmt.Errorf("%s recently added TV exceeded the grouping limit", c.Name())
		}
		// Latest has no shared paging contract. Expand its prefix geometrically
		// to retain user preferences and actual episode dates. Native grouping
		// can use names as identity or replace dates with old container dates.
		tvLimit = min(tvLimit*2, recentTVMaxLimit)
		tv, tvErr = c.fetchLatestByTypes(ctx, userID, token, tvLimit, recentTVTypes)
	}
}

func recentWindowComplete(items []dashboard.MediaItem, tv []latestItem, limit int) bool {
	if len(items) < limit || len(tv) == 0 {
		return false
	}
	// Latest is ordered by DateCreated descending. Once the oldest fetched TV
	// item is older than the visible cutoff, unseen episodes cannot enter it.
	oldest, err := time.Parse(time.RFC3339, tv[len(tv)-1].DateCreated)
	return err == nil && !oldest.IsZero() && !oldest.After(items[len(items)-1].AddedAt)
}

func (c *Client) fetchLatestByTypes(ctx context.Context, userID, token string, limit int, types string) ([]latestItem, error) {
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
		return nil, integrations.NewHTTPStatusError(c.Name(), "latest items", resp.StatusCode)
	}
	var rows []latestItem
	if err := decodeMediaServerJSON(resp.Body, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) mergeLatestItems(movies, tv []latestItem, limit int) ([]dashboard.MediaItem, error) {
	indices := make(map[string]int, len(movies)+len(tv))
	out := make([]dashboard.MediaItem, 0, len(movies)+len(tv))
	for _, rows := range [][]latestItem{movies, tv} {
		for _, row := range rows {
			key, item, err := c.mediaItemFromLatest(row)
			if err != nil {
				return nil, err
			}
			if index, ok := indices[key]; ok {
				if item.AddedAt.After(out[index].AddedAt) {
					out[index] = item
				}
				continue
			}
			indices[key] = len(out)
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AddedAt.After(out[j].AddedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (c *Client) mediaItemFromLatest(row latestItem) (string, dashboard.MediaItem, error) {
	addedAt, _ := time.Parse(time.RFC3339, row.DateCreated)
	item := dashboard.MediaItem{Title: cleanDisplayTitle(row.Name), Type: row.Type, Year: row.ProductionYear, AddedAt: addedAt}
	id := strings.TrimSpace(row.ID)
	if id == "" {
		return "", item, fmt.Errorf("%s latest items returned an item without an ID", c.Name())
	}
	key := strings.ToLower(row.Type) + ":" + id
	imageTag := row.ImageTags.Primary
	switch {
	case strings.EqualFold(row.Type, "Series"):
		key, item.Type = "tv:"+id, "TV"
	case strings.EqualFold(row.Type, "Episode"):
		item.Type = "TV"
		if strings.TrimSpace(row.SeriesName) != "" {
			item.Title = cleanDisplayTitle(row.SeriesName)
		}
		item.Subtitle = episodeLabel(row.ParentIndexNumber, row.IndexNumber, cleanDisplayTitle(row.Name))
		if seriesID := strings.TrimSpace(row.SeriesID); seriesID != "" {
			id, imageTag, key = seriesID, row.SeriesPrimaryImageTag, "tv:"+seriesID
		}
	}
	if c.publicURL != "" {
		item.OpenURL = c.provider.ItemURL(c.publicURL, id)
	}
	img := url.Values{}
	if imageTag != "" {
		img.Set("tag", imageTag)
	}
	item.ImageURL = fmt.Sprintf("/media/server/poster/%s?%s", url.PathEscape(id), img.Encode())
	return key, item, nil
}

func episodeLabel(season, episode *int, title string) string {
	if season == nil || episode == nil {
		return title
	}
	return fmt.Sprintf("S%02dE%02d · %s", *season, *episode, title)
}

func cleanDisplayTitle(name string) string {
	name = strings.TrimSpace(name)
	name = tmdbSuffixRE.ReplaceAllString(name, "")
	if name == "" {
		return "Untitled"
	}
	return name
}
