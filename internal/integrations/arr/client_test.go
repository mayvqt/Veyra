package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/testutil"
)

func TestHealthAndQueue(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Api-Key"); got != "k" {
			t.Fatalf("expected api key header, got %q", got)
		}
		switch r.URL.Path {
		case "/api/v3/system/status":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		case "/api/v3/queue":
			body := `{"records":[{"series":{"title":"Slow Horses"},"episode":{"seasonNumber":4,"episodeNumber":1,"title":"Identity Theft"},"status":"downloading","trackedDownloadStatus":"ok","size":1000,"sizeleft":250,"timeleft":"00:10:00","downloadClient":"qBittorrent","protocol":"torrent"}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	if hs := c.Health(context.Background()); !hs.OK {
		t.Fatalf("expected healthy, got %+v", hs)
	}
	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queue item, got %d", len(items))
	}
	if items[0].Title != "Slow Horses" || items[0].Progress != 75 || items[0].Source != "Sonarr" || items[0].Status != "Downloading" || items[0].Kind != "TV" {
		t.Fatalf("unexpected queue item: %+v", items[0])
	}
}

func TestQueueRejectsOversizedJSONResponse(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := strings.Repeat(" ", arrJSONLimit) + `{"records":[]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	if _, err := c.Queue(context.Background(), 5); err == nil {
		t.Fatal("expected oversized response to be rejected")
	}
}

func TestHealthHandlesNilClient(t *testing.T) {
	var c *Client
	hs := c.Health(context.Background())
	if hs.OK || hs.Message != "not configured" {
		t.Fatalf("expected nil client to be not configured, got %+v", hs)
	}
}

func TestHealthHandlesZeroValueClient(t *testing.T) {
	c := &Client{}
	hs := c.Health(context.Background())
	if hs.OK || hs.Message != "not configured" {
		t.Fatalf("expected zero-value client to be not configured, got %+v", hs)
	}
}

func TestHealthHandlesInvalidURL(t *testing.T) {
	c := NewClient("Sonarr", "://sonarr", "k")
	hs := c.Health(context.Background())
	if hs.OK || hs.Message == "" {
		t.Fatalf("expected invalid URL health failure, got %+v", hs)
	}
}

func TestQueueHandlesInvalidURL(t *testing.T) {
	c := NewClient("Sonarr", "://sonarr", "k")
	if _, err := c.Queue(context.Background(), 5); err == nil {
		t.Fatal("expected invalid URL queue error")
	}
}

func TestUpcomingHandlesInvalidURL(t *testing.T) {
	c := NewClient("Sonarr", "://sonarr", "k")
	if _, err := c.Upcoming(context.Background(), 7, 5); err == nil {
		t.Fatal("expected invalid URL calendar error")
	}
}

func TestQueueMoviePayload(t *testing.T) {
	c := NewClient("Radarr", "http://radarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"records":[{"movie":{"title":"Heat"},"status":"queued","size":1073741824,"sizeleft":1073741824,"estimatedCompletionTime":"2026-01-01T01:00:00Z"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected queued movie payload to be visible, got: %+v", items)
	}
	if items[0].Title != "Heat" || items[0].Kind != "Movie" {
		t.Fatalf("unexpected movie queue item: %+v", items[0])
	}
}

func TestQueueRadarrQueuedWithoutMovieObjectStillVisible(t *testing.T) {
	c := NewClient("Radarr", "http://radarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"records":[{"title":"Some Movie 2026 1080p WEB-DL x265","status":"queued","size":1000,"sizeleft":400}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected queued radarr row without movie object to be visible, got: %+v", items)
	}
	if items[0].Kind != "Movie" {
		t.Fatalf("expected movie kind for radarr queue row, got %+v", items[0])
	}
}

func TestQueueFiltersNonActiveAndKeepsActive(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"records":[{"series":{"title":"Severance"},"status":"queued","size":1000,"sizeleft":1000},{"series":{"title":"Andor"},"status":"downloading","size":1000,"sizeleft":200}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one active download, got %d (%+v)", len(items), items)
	}
	if items[0].Title != "Andor" || items[0].Progress != 80 || items[0].Kind != "TV" {
		t.Fatalf("unexpected active queue item: %+v", items[0])
	}
}

func TestQueueSubtitleFallsBackToRootSeasonEpisodeFields(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"records":[{"series":{"title":"Andor"},"title":"One Way Out","seasonNumber":1,"episodeNumber":10,"status":"downloading","size":1000,"sizeleft":200}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one active download, got %d (%+v)", len(items), items)
	}
	if items[0].Subtitle != "S01E10 One Way Out" {
		t.Fatalf("unexpected subtitle fallback behavior: %+v", items[0])
	}
}

func TestQueueFetchesAllPagesWhenLimitIsZero(t *testing.T) {
	c := NewClient("Radarr", "http://radarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			records := make([]map[string]any, 0, 250)
			records = append(records, map[string]any{"movie": map[string]any{"title": "Movie A"}, "status": "downloading", "size": 1000, "sizeleft": 400})
			for i := 1; i < 250; i++ {
				records = append(records, map[string]any{"movie": map[string]any{"title": "Queued"}, "status": "queued", "size": 1000, "sizeleft": 900})
			}
			body, _ := json.Marshal(map[string]any{"totalRecords": 251, "records": records})
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
		case "2":
			body := `{"totalRecords":251,"records":[{"movie":{"title":"Movie C"},"status":"downloading","size":1000,"sizeleft":100}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"totalRecords":251,"records":[]}`)), Header: make(http.Header)}, nil
		}
	})}

	items, err := c.Queue(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 visible rows across pages, got %d (%+v)", len(items), items)
	}
	if items[0].Title != "Movie A" || items[1].Title != "Queued" || items[2].Title != "Movie C" || items[0].Kind != "Movie" || items[1].Kind != "Movie" || items[2].Kind != "Movie" {
		t.Fatalf("unexpected page-merged results: %+v", items)
	}
}

func TestQueueDeduplicatesRepeatedRowsAcrossPages(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		page := r.URL.Query().Get("page")
		if page == "1" {
			body := `{"totalRecords":500,"records":[{"id":"dup-1","series":{"title":"Severance"},"episode":{"seasonNumber":2,"episodeNumber":3,"title":"Hello, Ms. Cobel"},"status":"downloading","size":1000,"sizeleft":200}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		if page == "2" {
			body := `{"totalRecords":500,"records":[{"id":"dup-1","series":{"title":"Severance"},"episode":{"seasonNumber":2,"episodeNumber":3,"title":"Hello, Ms. Cobel"},"status":"downloading","size":1000,"sizeleft":150}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"totalRecords":500,"records":[]}`)), Header: make(http.Header)}, nil
	})}

	items, err := c.Queue(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected deduplicated queue item count 1, got %d (%+v)", len(items), items)
	}
	if items[0].Title != "Severance" {
		t.Fatalf("unexpected deduplicated item: %+v", items[0])
	}
}

func TestQueueResolvesCleanSeriesTitleFromSeriesID(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/queue":
			body := `{"records":[{"seriesId":42,"title":"Show.Name.S01E01.1080p.WEB-DL","status":"downloading","size":1000,"sizeleft":500}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v3/series/42":
			body := `{"title":"Show Name"}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queue item, got %d", len(items))
	}
	if items[0].Title != "Show Name" {
		t.Fatalf("expected clean resolved title, got %+v", items[0])
	}
}

func TestQueueCachesResolvedSeriesTitleByID(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	seriesLookups := 0
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/queue":
			body := `{"records":[
				{"seriesId":42,"title":"Show.Name.S01E01.1080p.WEB-DL","status":"downloading","size":1000,"sizeleft":500},
				{"seriesId":42,"title":"Show.Name.S01E02.1080p.WEB-DL","status":"downloading","size":1000,"sizeleft":400}
			]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v3/series/42":
			seriesLookups++
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"title":"Show Name"}`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 queue items, got %d (%+v)", len(items), items)
	}
	if items[0].Title != "Show Name" || items[1].Title != "Show Name" {
		t.Fatalf("expected cached clean titles, got %+v", items)
	}
	if seriesLookups != 1 {
		t.Fatalf("expected one series lookup, got %d", seriesLookups)
	}
}

func TestQueueCachesMissingSeriesTitleByID(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	seriesLookups := 0
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/queue":
			body := `{"records":[
				{"seriesId":42,"title":"Missing.Show.S01E01.1080p.WEB-DL","status":"downloading","size":1000,"sizeleft":500},
				{"seriesId":42,"title":"Missing.Show.S01E02.1080p.WEB-DL","status":"downloading","size":1000,"sizeleft":400}
			]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v3/series/42":
			seriesLookups++
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 queue items, got %d (%+v)", len(items), items)
	}
	if items[0].Title != "Missing.Show.S01E01.1080p.WEB-DL" || items[1].Title != "Missing.Show.S01E02.1080p.WEB-DL" {
		t.Fatalf("expected release title fallback after cached miss, got %+v", items)
	}
	if seriesLookups != 1 {
		t.Fatalf("expected one failed series lookup, got %d", seriesLookups)
	}
}

func TestQueueResolvesCleanMovieTitleFromMovieID(t *testing.T) {
	c := NewClient("Radarr", "http://radarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/queue":
			body := `{"records":[{"movieId":77,"title":"Greenland 2 Migration 2026 1080p WEB-DL x265","status":"downloading","size":1000,"sizeleft":300}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v3/movie/77":
			body := `{"title":"Greenland 2: Migration"}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	items, err := c.Queue(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queue item, got %d", len(items))
	}
	if items[0].Title != "Greenland 2: Migration" {
		t.Fatalf("expected clean movie title, got %+v", items[0])
	}
}

func TestQueueDeduplicatesSameReleaseNameWithDifferentIDs(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"records":[
			{"id":"a1","title":"Foundation (2021) S01 S01 (1080p ATVP WEB-DL x265 HEVC 10bi)","series":{"title":"Foundation"},"episode":{"seasonNumber":1,"episodeNumber":1,"title":"The Emperor's Peace"},"status":"downloading","size":1000,"sizeleft":400},
			{"id":"a2","title":"Foundation (2021) S01 S01 (1080p ATVP WEB-DL x265 HEVC 10bi)","series":{"title":"Foundation"},"episode":{"seasonNumber":1,"episodeNumber":1,"title":"The Emperor's Peace"},"status":"downloading","size":1000,"sizeleft":380}
		]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Queue(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected duplicate release rows to collapse to 1, got %d (%+v)", len(items), items)
	}
}

func TestUpcomingCalendarForShowAndMovie(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `[{"series":{"title":"The Last of Us"},"title":"Future Days","seasonNumber":2,"episodeNumber":1,"airDateUtc":"2026-06-01T03:00:00Z","hasFile":true},{"movie":{"title":"Dune: Messiah"},"inCinemas":"2026-06-15T00:00:00Z","hasFile":false}]`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Upcoming(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 upcoming items, got %d", len(items))
	}
	if items[0].Title != "The Last of Us" || !strings.Contains(items[0].Subtitle, "S02E01") {
		t.Fatalf("unexpected show upcoming item: %+v", items[0])
	}
	if items[0].Availability != "available" {
		t.Fatalf("expected available show item, got %+v", items[0])
	}
	if items[1].Title != "Dune: Messiah" || items[1].Kind != "Movie" {
		t.Fatalf("unexpected movie upcoming item: %+v", items[1])
	}
	if items[1].Availability != "upcoming" && items[1].Availability != "missing" {
		t.Fatalf("expected upcoming/missing movie availability, got %+v", items[1])
	}
}

func TestUpcomingCalendarUsesSeriesTitleWhenSeriesObjectMissing(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `[{"seriesTitle":"Blood and Bone","title":"Bone Debt","seasonNumber":5,"episodeNumber":8,"airDateUtc":"2026-06-01T07:00:00Z"}]`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Upcoming(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 upcoming item, got %d", len(items))
	}
	if items[0].Title != "Blood and Bone" {
		t.Fatalf("expected show title from seriesTitle, got %+v", items[0])
	}
	if items[0].Subtitle != "S05E08 Bone Debt" {
		t.Fatalf("expected normalized subtitle, got %+v", items[0])
	}
}

func TestUpcomingSkipsInvalidDates(t *testing.T) {
	c := NewClient("Radarr", "http://radarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `[{"movie":{"title":"Bad Date"},"inCinemas":"not-a-date"}]`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	items, err := c.Upcoming(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected invalid-date rows to be skipped, got %+v", items)
	}
}

func TestUpcomingCalendarUsesDigitalReleaseFallback(t *testing.T) {
	c := NewClient("Radarr", "http://radarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `[{"movie":{"title":"Avatar 3"},"digitalRelease":"2026-12-20T00:00:00Z"}]`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	items, err := c.Upcoming(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].AirsAt.IsZero() || !items[0].AirsAt.Equal(time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected fallback date: %+v", items[0])
	}
}

func TestUpcomingCalendarRequestsSeriesData(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("includeSeries"); got != "true" {
			t.Fatalf("expected includeSeries=true, got %q", got)
		}
		body := `[{"series":{"title":"The Bear"},"title":"Review","seasonNumber":1,"episodeNumber":1,"airDateUtc":"2026-06-01T03:00:00Z"}]`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	items, err := c.Upcoming(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "The Bear" {
		t.Fatalf("unexpected upcoming result: %+v", items)
	}
}

func TestAdminSummaryCollectsHealthAndDiskWarnings(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Api-Key"); got != "k" {
			t.Fatalf("expected api key header, got %q", got)
		}
		switch r.URL.Path {
		case "/api/v3/system/status":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"appName":"Sonarr","version":"4.0.0","branch":"main"}`)), Header: make(http.Header)}, nil
		case "/api/v3/health":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"source":"Indexer","type":"warning","message":"Indexer unavailable"}]`)), Header: make(http.Header)}, nil
		case "/api/v3/diskspace":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"path":"/media","label":"Media","freeSpace":1073741824,"totalSpace":107374182400}]`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.Version != "4.0.0" || len(summary.HealthIssues) != 1 || len(summary.DiskWarnings) != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if !strings.Contains(summary.DiskWarnings[0], "Media") {
		t.Fatalf("expected disk warning label, got %+v", summary.DiskWarnings)
	}
}

func TestAdminSummaryFetchesIndependentDetailsConcurrently(t *testing.T) {
	c := NewClient("Sonarr", "http://sonarr.local", "k")
	started := make(chan string, 2)
	release := make(chan struct{})
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/system/status":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"appName":"Sonarr","version":"4.0.0"}`)), Header: make(http.Header)}, nil
		case "/api/v3/health":
			started <- r.URL.Path
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"source":"indexer","message":"Indexer unavailable"}]`)), Header: make(http.Header)}, nil
		case "/api/v3/diskspace":
			started <- r.URL.Path
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"path":"/media","label":"Media","freeSpace":1,"totalSpace":100}]`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	type result struct {
		summary AdminSummary
		err     error
	}
	done := make(chan result, 1)
	go func() {
		summary, err := c.AdminSummary(context.Background())
		done <- result{summary: summary, err: err}
	}()

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "/api/v3/health", "/api/v3/diskspace")
	close(release)
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if len(got.summary.HealthIssues) != 1 || len(got.summary.DiskWarnings) != 1 {
		t.Fatalf("expected concurrent detail payloads, got %+v", got.summary)
	}
}

func TestAdminSummaryHandlesNilClient(t *testing.T) {
	var c *Client
	if _, err := c.AdminSummary(context.Background()); err == nil {
		t.Fatal("expected nil client admin summary error")
	}
}

func TestAdminSummaryHandlesInvalidURL(t *testing.T) {
	c := NewClient("Sonarr", "://sonarr", "k")
	if _, err := c.AdminSummary(context.Background()); err == nil {
		t.Fatal("expected invalid URL admin summary error")
	}
}

func TestProwlarrUsesV1AdminAPI(t *testing.T) {
	c := NewProwlarrClient("http://prowlarr.local", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			t.Fatalf("expected prowlarr v1 path, got %s", r.URL.Path)
		}
		if r.URL.Path == "/api/v1/system/status" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"appName":"Prowlarr","version":"1.0.0"}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[]`)), Header: make(http.Header)}, nil
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.AppName != "Prowlarr" || summary.Version != "1.0.0" {
		t.Fatalf("unexpected prowlarr summary: %+v", summary)
	}
}

func TestProwlarrNormalizesHostPortBaseURL(t *testing.T) {
	c := NewProwlarrClient("192.168.1.4:9696", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "http" || r.URL.Host != "192.168.1.4:9696" || r.URL.Path != "/api/v1/system/status" {
			t.Fatalf("unexpected normalized request URL: %s", r.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"appName":"Prowlarr","version":"1.0.0"}`)), Header: make(http.Header)}, nil
	})}

	hs := c.Health(context.Background())
	if !hs.OK {
		t.Fatalf("expected normalized prowlarr health success, got %+v", hs)
	}
}
