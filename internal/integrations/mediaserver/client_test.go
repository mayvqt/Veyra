package mediaserver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/testutil"
)

func newTestClient(t *testing.T, serverType config.MediaServerType, baseURL, publicURL, apiKey string) *Client {
	t.Helper()
	client, err := NewClient(serverType, baseURL, publicURL, apiKey)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestHealthAndRecentlyAdded(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/System/Ping" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
		}
		if r.URL.Path == "/Items/Latest" {
			if got := r.URL.Query().Get("userId"); got != "u1" {
				t.Fatalf("expected Jellyfin latest userId=u1, got %q", got)
			}
			types := r.URL.Query().Get("IncludeItemTypes")
			if types == "" {
				types = r.URL.Query().Get("includeItemTypes")
			}
			body := `[]`
			if types == "Movie" {
				body = `[{"Id":"m1","Name":"Movie1","Type":"Movie","ProductionYear":2024,"DateCreated":"2025-01-01T00:00:00Z","ImageTags":{"Primary":"tag1"}}]`
			}
			if types == "Episode,Series" {
				body = `[{"Id":"e1","Name":"The Beginning","Type":"Episode","ProductionYear":2025,"DateCreated":"2025-01-02T00:00:00Z","SeriesId":"s1","SeriesName":"Show1","SeriesPrimaryImageTag":"series-tag","ParentIndexNumber":1,"IndexNumber":2,"ImageTags":{"Primary":"episode-tag"}}]`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	hs := c.Health(context.Background())
	if !hs.OK {
		t.Fatal("expected healthy")
	}
	items, err := c.RecentlyAdded(context.Background(), "u1", "token", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Title != "Show1" || items[1].Title != "Movie1" {
		t.Fatal("unexpected recently added payload")
	}
	if items[0].Type != "TV" || items[0].Subtitle != "S01E02 · The Beginning" {
		t.Fatalf("unexpected episode presentation: %+v", items[0])
	}
	if items[0].ImageURL != "/media/server/poster/s1?tag=series-tag" {
		t.Fatalf("expected series poster URL, got %q", items[0].ImageURL)
	}
}

func TestHealthHandlesMalformedBaseURL(t *testing.T) {
	if _, err := NewClient(config.MediaServerJellyfin, "http://[::1", "https://jf.example", ""); err == nil {
		t.Fatal("expected malformed base URL to be rejected at construction")
	}
}

func TestRecentlyAddedReturnsMalformedBaseURLError(t *testing.T) {
	if _, err := NewClient(config.MediaServerJellyfin, "javascript://mediaserver", "https://jf.example", ""); err == nil {
		t.Fatal("expected unsafe media server URL scheme to be rejected")
	}
}

func TestNewClientRejectsUnsupportedProvider(t *testing.T) {
	if _, err := NewClient(config.MediaServerType("plex"), "http://mediaserver.local", "", ""); err == nil {
		t.Fatal("expected unsupported provider to be rejected")
	}
}

func TestRecentlyAddedCleansTitleAndKeepsPosterURLWithoutTag(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/Items/Latest" {
			types := r.URL.Query().Get("includeItemTypes")
			body := `[]`
			if types == "Movie" {
				body = `[{"Id":"m2","Name":"Greenland 2 Migration (2026) [tmdbid-840464]","Type":"Movie","ProductionYear":2026,"DateCreated":"2026-01-01T00:00:00Z","ImageTags":{"Primary":""}}]`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	items, err := c.RecentlyAdded(context.Background(), "u2", "token", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Greenland 2 Migration (2026)" {
		t.Fatalf("expected cleaned title, got %q", items[0].Title)
	}
	if items[0].ImageURL == "" {
		t.Fatal("expected image URL even when primary tag is missing")
	}
}

func TestRecentlyAddedUsesTypeSpecificRequests(t *testing.T) {
	calls := map[string]int{}
	var mu sync.Mutex
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/Items/Latest" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		types := r.URL.Query().Get("includeItemTypes")
		mu.Lock()
		calls[types]++
		mu.Unlock()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header)}, nil
	})}

	_, err := c.RecentlyAdded(context.Background(), "u3", "token", 12)
	if err != nil {
		t.Fatal(err)
	}
	if calls["Movie"] != 1 {
		t.Fatalf("expected one Movie call, got %d", calls["Movie"])
	}
	if calls["Episode,Series"] != 1 {
		t.Fatalf("expected one Episode,Series call, got %d", calls["Episode,Series"])
	}
}

func TestRecentlyAddedFetchesMovieAndTVConcurrently(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "")
	started := make(chan string, 2)
	release := make(chan struct{})
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/Items/Latest" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		types := r.URL.Query().Get("includeItemTypes")
		started <- types
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header)}, nil
	})}

	type result struct {
		items []dashboard.MediaItem
		err   error
	}
	done := make(chan result, 1)
	go func() {
		items, err := c.RecentlyAdded(context.Background(), "u4", "token", 5)
		done <- result{items: items, err: err}
	}()

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "Movie", "Episode,Series")
	close(release)
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if len(got.items) != 0 {
		t.Fatalf("expected empty merged result, got %+v", got.items)
	}
}

func TestRecentlyAddedUsesAPIKeyWhenTokenEmpty(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Emby-Token"); got != "server-key" {
			t.Fatalf("expected X-Emby-Token header to use api key, got %q", got)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header)}, nil
	})}

	if _, err := c.RecentlyAdded(context.Background(), "u1", "", 5); err != nil {
		t.Fatal(err)
	}
}

func TestRecentlyAddedUsesEmbyUserLatestEndpoint(t *testing.T) {
	c := newTestClient(t, config.MediaServerEmby, "http://emby.local", "https://emby.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/Users/emby-user/Items/Latest" {
			t.Fatalf("unexpected Emby latest path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("IncludeItemTypes"); got == "" {
			t.Fatal("expected Emby PascalCase latest query parameters")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header)}, nil
	})}

	if _, err := c.RecentlyAdded(context.Background(), "emby-user", "", 5); err != nil {
		t.Fatal(err)
	}
}

func TestPrimaryImageUsesDocumentedImageRoute(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/Items/item-1/Images/Primary" {
			t.Fatalf("unexpected image path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("tag"); got != "tag-1" {
			t.Fatalf("expected tag query, got %q", got)
		}
		if got := r.URL.Query().Get("maxWidth"); got != "360" {
			t.Fatalf("expected maxWidth=360, got %q", got)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("image")), Header: make(http.Header)}, nil
	})}

	img, err := c.PrimaryImage(context.Background(), "item-1", "tag-1", "", 360)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Body.Close()
}

func TestPrimaryImageRejectsActiveContent(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("<script>alert(1)</script>")),
			Header:     http.Header{"Content-Type": []string{"text/html"}},
		}, nil
	})}

	if _, err := c.PrimaryImage(context.Background(), "item-1", "", "", 360); err == nil {
		t.Fatal("expected active content to be rejected")
	}
}

func TestAdminSummaryCollectsServerStats(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/System/Info":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ServerName":"Library","Version":"10.9.0","OperatingSystem":"Linux"}`)), Header: make(http.Header)}, nil
		case "/Users":
			if got := r.Header.Get("X-Emby-Token"); got != "server-key" {
				t.Fatalf("expected server api key, got %q", got)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"Name":"A"},{"Name":"B"}]`)), Header: make(http.Header)}, nil
		case "/Items/Counts":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"MovieCount":10,"SeriesCount":3,"EpisodeCount":40}`)), Header: make(http.Header)}, nil
		case "/Library/VirtualFolders":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"Items":[{"Name":"Movies"},{"Name":"Shows"}]}`)), Header: make(http.Header)}, nil
		case "/Sessions":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[
				{"UserName":"Angel","Client":"Jellyfin Web","DeviceName":"Firefox","NowPlayingItem":{"Name":"Heat","Type":"Movie"}},
				{"UserName":"Idle","Client":"Android TV","DeviceName":"Shield"}
			]`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.ServerName != "Library" || summary.UserCount != 2 || summary.MovieCount != 10 || summary.LibraryCount != 2 {
		t.Fatalf("unexpected admin summary: %+v", summary)
	}
	if len(summary.ActivePlaybacks) != 1 || summary.ActivePlaybacks[0].Title != "Heat" || summary.ActivePlaybacks[0].User != "Angel" {
		t.Fatalf("unexpected active playback rows: %+v", summary.ActivePlaybacks)
	}
}

func TestAdminSummaryUsesPublicSystemInfoWithoutAPIKey(t *testing.T) {
	c := newTestClient(t, config.MediaServerEmby, "http://emby.local", "https://emby.example", "")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/System/Info/Public" {
			t.Fatalf("unexpected unauthenticated admin summary path %q", r.URL.Path)
		}
		if got := r.Header.Get("X-Emby-Token"); got != "" {
			t.Fatalf("unexpected token on public system info request %q", got)
		}
		return response(http.StatusOK, `{"ServerName":"Emby","Version":"4.9.0"}`), nil
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.ServerName != "Emby" || summary.Version != "4.9.0" {
		t.Fatalf("unexpected public system summary: %+v", summary)
	}
}

func TestAdminSummaryAcceptsArrayLibraryPayload(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/System/Info":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ServerName":"Library","Version":"10.9.0"}`)), Header: make(http.Header)}, nil
		case "/Users", "/Sessions":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[]`)), Header: make(http.Header)}, nil
		case "/Items/Counts":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		case "/Library/VirtualFolders":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"Name":"Movies"},{"Name":"Shows"}]`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.LibraryCount != 2 {
		t.Fatalf("expected two libraries from array payload, got %+v", summary)
	}
	if strings.Contains(strings.Join(summary.Warnings, "\n"), "Libraries unavailable") {
		t.Fatalf("expected no library warning, got %+v", summary.Warnings)
	}
}

func TestEmbyAdminSummaryUsesVirtualFoldersQuery(t *testing.T) {
	c := newTestClient(t, config.MediaServerEmby, "http://emby.local", "https://emby.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/System/Info":
			if got := r.Header.Get("X-Emby-Token"); got != "server-key" {
				t.Fatalf("expected system info request to use API key, got %q", got)
			}
			return response(http.StatusOK, `{"ServerName":"Emby","OperatingSystem":"Linux"}`), nil
		case "/Sessions":
			return response(http.StatusOK, `[]`), nil
		case "/Users/Query":
			return response(http.StatusOK, `{"Items":[{"Id":"one","Name":"One"},{"Id":"two","Name":"Two"}],"TotalRecordCount":2}`), nil
		case "/Items/Counts":
			return response(http.StatusOK, `{}`), nil
		case "/Library/VirtualFolders/Query":
			return response(http.StatusOK, `{"Items":[{"Name":"Movies"},{"Name":"Shows"}],"TotalRecordCount":2}`), nil
		default:
			return response(http.StatusNotFound, ""), nil
		}
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.LibraryCount != 2 || summary.UserCount != 2 || summary.OperatingSystem != "Linux" {
		t.Fatalf("expected two Emby libraries and users, got %+v", summary)
	}
	if strings.Contains(strings.Join(summary.Warnings, "\n"), "Libraries unavailable") {
		t.Fatalf("expected no Emby library warning, got %+v", summary.Warnings)
	}
}

func TestAdminSummaryFetchesIndependentDetailsConcurrently(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	started := make(chan string, 4)
	release := make(chan struct{})
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/System/Info":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ServerName":"Library","Version":"10.9.0","OperatingSystem":"Linux"}`)), Header: make(http.Header)}, nil
		case "/Users":
			started <- r.URL.Path
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"Name":"A"}]`)), Header: make(http.Header)}, nil
		case "/Items/Counts":
			started <- r.URL.Path
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"MovieCount":10,"SeriesCount":3,"EpisodeCount":40}`)), Header: make(http.Header)}, nil
		case "/Library/VirtualFolders":
			started <- r.URL.Path
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"Items":[{"Name":"Movies"}]}`)), Header: make(http.Header)}, nil
		case "/Sessions":
			started <- r.URL.Path
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"UserName":"Angel","NowPlayingItem":{"Name":"Heat","Type":"Movie"}}]`)), Header: make(http.Header)}, nil
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

	testutil.WaitForSignals(t, started, 300*time.Millisecond, "/Users", "/Items/Counts", "/Library/VirtualFolders", "/Sessions")
	close(release)
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.summary.UserCount != 1 || got.summary.MovieCount != 10 || got.summary.LibraryCount != 1 || len(got.summary.ActivePlaybacks) != 1 {
		t.Fatalf("expected concurrent detail payloads, got %+v", got.summary)
	}
}

func TestAdminSummaryKeepsPartialStatsWarnings(t *testing.T) {
	c := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "https://jf.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/System/Info":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ServerName":"Library","Version":"10.9.0"}`)), Header: make(http.Header)}, nil
		case "/Users":
			return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`forbidden`)), Header: make(http.Header)}, nil
		case "/Items/Counts", "/Library/VirtualFolders":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		case "/Sessions":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[]`)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Warnings) != 1 || !strings.Contains(strings.Join(summary.Warnings, "\n"), "Users unavailable") {
		t.Fatalf("expected users warning, got %+v", summary.Warnings)
	}
}

func TestAdminSummarySortsConcurrentWarnings(t *testing.T) {
	c := newTestClient(t, config.MediaServerEmby, "http://emby.local", "https://emby.example", "server-key")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/System/Info" {
			return response(http.StatusOK, `{"ServerName":"Emby"}`), nil
		}
		return response(http.StatusServiceUnavailable, ""), nil
	})}

	summary, err := c.AdminSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantPrefixes := []string{"Item counts unavailable", "Libraries unavailable", "Sessions unavailable", "Users unavailable"}
	if len(summary.Warnings) != len(wantPrefixes) {
		t.Fatalf("warnings = %+v", summary.Warnings)
	}
	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(summary.Warnings[i], prefix) {
			t.Fatalf("warning %d = %q, want prefix %q", i, summary.Warnings[i], prefix)
		}
	}
}
