package mediaserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/testutil"
)

type recentFixture struct {
	movies     []latestItem
	tv         []latestItem
	movieCalls int
	tvLimits   []int
}

func (f *recentFixture) client(t *testing.T, serverType config.MediaServerType, publicURL string) *Client {
	t.Helper()
	c := newTestClient(t, serverType, "http://media.invalid", publicURL, "key")
	c.http.Transport = testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if serverType == config.MediaServerJellyfin {
			if r.URL.Path != "/Items/Latest" || r.URL.Query().Get("userId") != "member" {
				t.Errorf("recent media lost its user scope: %s", r.URL)
			}
		} else if r.URL.Path != "/Users/member/Items/Latest" {
			t.Errorf("recent media lost its user scope: %s", r.URL)
		}
		query := make(map[string]string)
		for key, values := range r.URL.Query() {
			query[strings.ToLower(key)] = values[0]
		}
		if query["groupitems"] != "false" {
			t.Error("native grouping can discard series identity and newest episode dates")
		}
		limit, err := strconv.Atoi(query["limit"])
		if err != nil || limit <= 0 {
			t.Errorf("invalid latest limit: %q", query["limit"])
		}
		rows := f.movies
		if query["includeitemtypes"] != "Movie" && query["includeitemtypes"] != "Episode" {
			t.Error("TV recency must use episode dates, not series folder creation dates")
		}
		if query["includeitemtypes"] == recentTVTypes {
			f.tvLimits = append(f.tvLimits, limit)
			rows = f.tv
		} else {
			f.movieCalls++
		}
		body, err := json.Marshal(rows[:min(len(rows), limit)])
		if err != nil {
			t.Error(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})
	return c
}

func recentEpisode(id, seriesID string, addedAt time.Time) latestItem {
	return latestItem{ID: id, Name: "Episode", Type: "Episode", SeriesID: seriesID, SeriesName: seriesID, DateCreated: addedAt.Format(time.RFC3339)}
}

func TestRecentlyAddedGroupsBySeriesIdentity(t *testing.T) {
	newest := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	season, episode := 1, 5
	for _, serverType := range []config.MediaServerType{config.MediaServerJellyfin, config.MediaServerEmby} {
		for _, publicURL := range []string{"", "https://watch.example"} {
			t.Run(string(serverType)+"/"+publicURL, func(t *testing.T) {
				f := &recentFixture{
					movies: []latestItem{
						{ID: "m1", Type: "Movie", Name: "Shared title", ProductionYear: 2021, DateCreated: newest.Add(-15 * time.Minute).Format(time.RFC3339)},
						{ID: "m2", Type: "Movie", Name: "Shared title", ProductionYear: 2022, DateCreated: newest.Add(-20 * time.Minute).Format(time.RFC3339)},
					},
					tv: []latestItem{
						{ID: "s1", Type: "Series", Name: "Shared title", DateCreated: "2020-01-01T00:00:00Z"},
						recentEpisode("older", "s1", newest.Add(-10*time.Minute)),
						recentEpisode("newest", "s1", newest),
						recentEpisode("another", "s2", newest.Add(-5*time.Minute)),
						recentEpisode("orphan1", "", newest.Add(-30*time.Minute)),
						recentEpisode("orphan2", "", newest.Add(-35*time.Minute)),
					},
				}
				for i := range f.tv {
					f.tv[i].SeriesName = "Shared title [tmdbid-123]"
				}
				f.tv[2].Name, f.tv[2].SeriesPrimaryImageTag = "New episode", "series-poster"
				f.tv[2].ParentIndexNumber, f.tv[2].IndexNumber = &season, &episode
				c := f.client(t, serverType, publicURL)
				items, err := c.RecentlyAdded(context.Background(), "member", "", 12)
				if err != nil {
					t.Fatal(err)
				}
				if len(items) != 6 {
					t.Fatalf("want six distinct media identities, got %+v", items)
				}
				for i, id := range []string{"s1", "s2", "m1", "m2", "orphan1", "orphan2"} {
					item := items[i]
					if item.Title != "Shared title" || !strings.HasPrefix(item.ImageURL, "/media/server/poster/"+id+"?") {
						t.Errorf("wrong identity or title at %d: %+v", i, item)
					}
					wantURL := ""
					if publicURL != "" {
						wantURL = c.provider.ItemURL(publicURL, id)
					}
					if item.OpenURL != wantURL {
						t.Errorf("wrong destination at %d: got %q, want %q", i, item.OpenURL, wantURL)
					}
				}
				if !items[0].AddedAt.Equal(newest) || items[0].Type != "TV" || items[0].Subtitle != "S01E05 · New episode" || items[0].ImageURL != "/media/server/poster/s1?tag=series-poster" {
					t.Fatalf("show did not retain its newest episode: %+v", items[0])
				}
				if items[2].Type != "Movie" || items[2].Year != 2021 || items[3].Year != 2022 {
					t.Fatalf("movie presentation changed: %+v", items[2:4])
				}
			})
		}
	}
}

func TestRecentlyAddedExpandsPastBulkTVImport(t *testing.T) {
	newest := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	for _, serverType := range []config.MediaServerType{config.MediaServerJellyfin, config.MediaServerEmby} {
		t.Run(string(serverType), func(t *testing.T) {
			f := &recentFixture{movies: []latestItem{{ID: "movie", Name: "Movie", Type: "Movie", DateCreated: newest.Add(-235 * time.Minute).Format(time.RFC3339)}}}
			for i := range 230 {
				f.tv = append(f.tv, recentEpisode(fmt.Sprintf("a-%d", i), "Show A", newest.Add(-time.Duration(i)*time.Minute)))
			}
			f.tv = append(f.tv, recentEpisode("b", "Show B", newest.Add(-230*time.Minute)), recentEpisode("c", "Show C", newest.Add(-240*time.Minute)))
			items, err := f.client(t, serverType, "").RecentlyAdded(context.Background(), "member", "", 3)
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 3 || items[0].Title != "Show A" || items[1].Title != "Show B" || items[2].Title != "Movie" {
				t.Fatalf("bulk import crowded out newer titles: %+v", items)
			}
			if !reflect.DeepEqual(f.tvLimits, []int{100, 200, 400}) || f.movieCalls != 1 {
				t.Fatalf("unexpected scan: TV=%v movies=%d", f.tvLimits, f.movieCalls)
			}
		})
	}
}

func TestRecentlyAddedStopsAtCombinedMovieCutoff(t *testing.T) {
	newest := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	f := &recentFixture{movies: []latestItem{{ID: "movie", Name: "Movie", Type: "Movie", DateCreated: newest.Add(-5 * time.Minute).Format(time.RFC3339)}}}
	for i := range 500 {
		f.tv = append(f.tv, recentEpisode(fmt.Sprintf("a-%d", i), "Show A", newest.Add(-time.Duration(i)*time.Minute)))
	}
	items, err := f.client(t, config.MediaServerJellyfin, "").RecentlyAdded(context.Background(), "member", "", 2)
	if err != nil || len(items) != 2 || !reflect.DeepEqual(f.tvLimits, []int{100}) {
		t.Fatalf("scanned TV history that cannot enter the list: items=%+v limits=%v err=%v", items, f.tvLimits, err)
	}
}

func TestRecentlyAddedReplacesExpandedSnapshot(t *testing.T) {
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	f := &recentFixture{}
	for i := range 100 {
		f.tv = append(f.tv, recentEpisode(fmt.Sprintf("a-%d", i), "Removed show", now.Add(-time.Duration(i)*time.Minute)))
	}
	c := f.client(t, config.MediaServerJellyfin, "")
	original := c.http.Transport
	c.http.Transport = testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("limit") == "200" {
			f.tv = []latestItem{recentEpisode("b", "Show B", now), recentEpisode("c", "Show C", now.Add(-time.Minute))}
		}
		return original.RoundTrip(r)
	})
	items, err := c.RecentlyAdded(context.Background(), "member", "", 2)
	if err != nil || len(items) != 2 || items[0].Title != "Show B" || items[1].Title != "Show C" {
		t.Fatalf("an old snapshot was merged into current media: %+v, %v", items, err)
	}
}

func TestRecentlyAddedRejectsIncompleteExpansion(t *testing.T) {
	for _, mode := range []string{"upstream error", "cancellation", "row ceiling"} {
		t.Run(mode, func(t *testing.T) {
			f := &recentFixture{}
			for i := range recentTVMaxLimit + 1 {
				f.tv = append(f.tv, recentEpisode(fmt.Sprintf("a-%d", i), "Show A", time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)))
			}
			c := f.client(t, config.MediaServerJellyfin, "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := c.http.Transport
			c.http.Transport = testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) > 10*time.Second {
					t.Error("recently added traversal has no overall deadline")
				}
				if r.URL.Query().Get("limit") == "200" {
					switch mode {
					case "upstream error":
						return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
					case "cancellation":
						cancel()
						return nil, r.Context().Err()
					}
				}
				return original.RoundTrip(r)
			})
			items, err := c.RecentlyAdded(ctx, "member", "", 2)
			if err == nil || items != nil {
				t.Fatalf("incomplete media became a successful list: %+v, %v", items, err)
			}
			if mode == "cancellation" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation was lost: %v", err)
			}
			if mode == "row ceiling" && f.tvLimits[len(f.tvLimits)-1] != recentTVMaxLimit {
				t.Fatalf("TV scan did not respect its row ceiling: %v", f.tvLimits)
			}
		})
	}
}

func TestRecentlyAddedRejectsMissingIdentity(t *testing.T) {
	f := &recentFixture{tv: []latestItem{{Type: "Episode", Name: "Missing ID"}}}
	items, err := f.client(t, config.MediaServerJellyfin, "").RecentlyAdded(context.Background(), "member", "", 12)
	if err == nil || items != nil {
		t.Fatalf("media without a stable identity became a successful list: %+v, %v", items, err)
	}
}
