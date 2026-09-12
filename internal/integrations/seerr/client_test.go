package seerr

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/testutil"
)

func TestHealthAndRecentRequests(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/status" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		}
		if r.URL.Path == "/api/v1/request" {
			body := `{"results":[{"status":2,"createdAt":"2025-01-01T00:00:00Z","media":{"mediaType":"movie","title":"Test Movie"}}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	hs := c.Health(context.Background())
	if !hs.OK {
		t.Fatal("expected healthy")
	}
	reqs, err := c.RecentRequests(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Status != "Approved" {
		t.Fatal("unexpected requests payload")
	}
}

func TestSearchRejectsOversizedJSONResponse(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := strings.Repeat(" ", seerrJSONLimit) + `{"results":[]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	if _, err := c.Search(context.Background(), "query", 5); err == nil {
		t.Fatal("expected oversized response to be rejected")
	}
}

func TestNotConfiguredReturnsErrorsWithoutRequests(t *testing.T) {
	c := NewClient("", "", "")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request to %s", r.URL.String())
		return nil, nil
	})}

	if hs := c.Health(context.Background()); hs.OK || hs.Message != "not configured" {
		t.Fatalf("unexpected health status: %+v", hs)
	}
	if _, err := c.Search(context.Background(), "arrival", 5); err == nil {
		t.Fatal("expected search configuration error")
	}
	if _, err := c.RecentRequests(context.Background(), 5); err == nil {
		t.Fatal("expected recent requests configuration error")
	}
	if _, err := c.CreateRequest(context.Background(), CreateRequestInput{MediaID: 1, MediaType: "movie"}); err == nil {
		t.Fatal("expected create request configuration error")
	}
	if _, err := c.UserQuotaForUser(context.Background(), UserIdentity{ID: 7}); err == nil {
		t.Fatal("expected quota configuration error")
	}
	if _, err := c.ResolveUserByMediaServerID(context.Background(), "media-user"); err == nil {
		t.Fatal("expected user resolve configuration error")
	}
	if _, err := c.ResolveUserByMediaServerID(context.Background(), "jf-123"); err == nil {
		t.Fatal("expected mediaserver user resolve configuration error")
	}
}

func TestMalformedBaseURLReturnsErrorsWithoutRequests(t *testing.T) {
	c := NewClient("://bad-url", "", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request to %s", r.URL.String())
		return nil, nil
	})}

	if hs := c.Health(context.Background()); hs.OK || hs.Message == "not configured" || hs.Message == "" {
		t.Fatalf("unexpected health status: %+v", hs)
	}
	if _, err := c.Search(context.Background(), "arrival", 5); err == nil {
		t.Fatal("expected search URL error")
	}
	if _, err := c.RecentRequests(context.Background(), 5); err == nil {
		t.Fatal("expected recent requests URL error")
	}
}

func TestUserQuotaPrimaryEndpoint(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/user/7/quota" {
			body := `{"requestLimit":10,"requestCount":3}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	q, err := c.UserQuotaForUser(context.Background(), UserIdentity{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if q.Remaining != 7 {
		t.Fatalf("expected remaining 7 got %d", q.Remaining)
	}
}

func TestUserQuotaMovieSeriesFields(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/user/7/quota" {
			body := `{"movieRequestLimit":4,"movieRequestCount":1,"seriesRequestLimit":6,"seriesRequestCount":2}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	q, err := c.UserQuotaForUser(context.Background(), UserIdentity{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if q.MovieRemaining != 3 || q.SeriesRemaining != 4 {
		t.Fatalf("unexpected movie/series quota remaining: %+v", q)
	}
}

func TestRecentRequestsForUserResolvesTitles(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v1/request":
			if got := r.URL.Query().Get("requestedBy"); got != "7" {
				t.Fatalf("expected requestedBy=7, got %q", got)
			}
			body := `{"results":[{"status":5,"createdAt":"2025-01-01T00:00:00Z","media":{"mediaType":"tv","tmdbId":100088},"requestedBy":{"username":"admin"}}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v1/tv/100088":
			body := `{"name":"The Last of Us"}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	reqs, err := c.RecentRequestsForUser(context.Background(), UserIdentity{ID: 7, Username: "admin"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Title != "The Last of Us" || reqs[0].Status != "Available" {
		t.Fatalf("unexpected user requests payload: %+v", reqs)
	}
}

func TestUnlinkedUserNeverLoadsPrivateData(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("unlinked identity must not make an upstream request")
		return nil, nil
	})}
	user := UserIdentity{Username: "admin"}
	if _, err := c.RecentRequestsForUser(context.Background(), user, 0); err != ErrUserNotLinked {
		t.Fatalf("history error: %v", err)
	}
	if _, err := c.UserQuotaForUser(context.Background(), user); err != ErrUserNotLinked {
		t.Fatalf("quota error: %v", err)
	}
}

func TestRecentRequestsIncludesLifecycle(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/request" {
			body := `{"results":[{"status":2,"createdAt":"2025-01-01T00:00:00Z","media":{"mediaType":"movie","title":"Test Movie","status":3},"requestedBy":{"username":"admin"}}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	reqs, err := c.RecentRequests(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected one request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Status != "Downloading" || req.StatusKey != "downloading" {
		t.Fatalf("expected downloading status, got %+v", req)
	}
	want := []dashboard.RequestLifecycleStep{
		{Label: "Submitted", Key: "submitted", State: "done"},
		{Label: "Approved", Key: "approved", State: "done"},
		{Label: "Downloading", Key: "downloading", State: "current"},
		{Label: "Partial", Key: "partial", State: "todo"},
		{Label: "Available", Key: "available", State: "todo"},
	}
	if len(req.Lifecycle) != len(want) {
		t.Fatalf("unexpected lifecycle length: %+v", req.Lifecycle)
	}
	for i := range want {
		if req.Lifecycle[i] != want[i] {
			t.Fatalf("lifecycle step %d: got %+v want %+v", i, req.Lifecycle[i], want[i])
		}
	}
}

func TestRequestLifecycleStatusMapping(t *testing.T) {
	tests := []struct {
		name          string
		requestStatus requestStatusCode
		mediaStatus   int
		wantStatus    string
		wantKey       string
		wantCurrent   string
	}{
		{name: "submitted", requestStatus: seerrRequestStatusPending, wantStatus: "Submitted", wantKey: "submitted", wantCurrent: "Submitted"},
		{name: "approved", requestStatus: seerrRequestStatusApproved, wantStatus: "Approved", wantKey: "approved", wantCurrent: "Approved"},
		{name: "pending availability remains approved", requestStatus: seerrRequestStatusApproved, mediaStatus: seerrMediaStatusPending, wantStatus: "Approved", wantKey: "approved", wantCurrent: "Approved"},
		{name: "downloading", requestStatus: seerrRequestStatusApproved, mediaStatus: seerrMediaStatusProcessing, wantStatus: "Downloading", wantKey: "downloading", wantCurrent: "Downloading"},
		{name: "partial", requestStatus: seerrRequestStatusApproved, mediaStatus: seerrMediaStatusPartiallyAvailable, wantStatus: "Partially available", wantKey: "partial", wantCurrent: "Partial"},
		{name: "available", requestStatus: seerrRequestStatusApproved, mediaStatus: seerrMediaStatusAvailable, wantStatus: "Available", wantKey: "available", wantCurrent: "Available"},
		{name: "declined", requestStatus: seerrRequestStatusDeclined, mediaStatus: seerrMediaStatusProcessing, wantStatus: "Declined", wantKey: "declined", wantCurrent: "Submitted"},
		{name: "failed", requestStatus: seerrRequestStatusFailed, mediaStatus: seerrMediaStatusProcessing, wantStatus: "Failed", wantKey: "failed", wantCurrent: "Submitted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := lifecycleStatus(tt.requestStatus, tt.mediaStatus)
			if status.Label != tt.wantStatus || status.Key != tt.wantKey {
				t.Fatalf("status = %+v, want label %q key %q", status, tt.wantStatus, tt.wantKey)
			}
			steps := requestLifecycle(tt.requestStatus, tt.mediaStatus)
			var current string
			for _, step := range steps {
				if step.State == requestLifecycleStateCurrent {
					current = step.Label
					break
				}
			}
			if current != tt.wantCurrent {
				t.Fatalf("current step = %q, want %q; steps=%+v", current, tt.wantCurrent, steps)
			}
		})
	}
}

func TestRecentRequestsPendingAvailabilityStaysApproved(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/request" {
			body := `{"results":[{"status":2,"createdAt":"2025-01-01T00:00:00Z","media":{"mediaType":"movie","title":"Future Movie","status":2,"releaseDate":"2999-01-01"},"requestedBy":{"username":"admin"}}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	reqs, err := c.RecentRequests(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected one request, got %d", len(reqs))
	}
	if reqs[0].Status != "Approved" || reqs[0].StatusKey != "approved" {
		t.Fatalf("expected approved pending availability, got %+v", reqs[0])
	}
	if reqs[0].Lifecycle[1].State != requestLifecycleStateCurrent {
		t.Fatalf("expected approved lifecycle to be current, got %+v", reqs[0].Lifecycle)
	}
}

func TestResolveUserByMediaServerID(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/user/jellyfin/jf-123" {
			body := `{"id":9,"jellyfinUsername":"user","jellyfinUserId":"jf-123"}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	user, err := c.ResolveUserByMediaServerID(context.Background(), "jf-123")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 9 || user.Username != "user" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestResolveUserByMediaServerIDUsesSeerrSharedJellyfinRouteForEmby(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/user/jellyfin/emby-user" {
			t.Fatalf("unexpected user lookup path %q", r.URL.Path)
		}
		body := `{"id":12,"jellyfinUsername":"embyuser","jellyfinUserId":"emby-user"}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	user, err := c.ResolveUserByMediaServerID(context.Background(), "emby-user")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 12 || user.Username != "embyuser" {
		t.Fatalf("unexpected Emby-backed Seerr user: %+v", user)
	}
}

func TestSearchReturnsRequestableMedia(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v1/search":
			if got := r.URL.Query().Get("query"); got != "slow horses" {
				t.Fatalf("unexpected query %q", got)
			}
			if !strings.Contains(r.URL.RawQuery, "slow%20horses") {
				t.Fatalf("expected spaces encoded as %%20, got raw query %q", r.URL.RawQuery)
			}
			body := `{"results":[{"id":1,"mediaType":"tv","name":"Slow Horses","firstAirDate":"2022-04-01","overview":"Spies.","poster_path":"/poster.jpg","mediaInfo":{"status":1}},{"id":2,"mediaType":"person","name":"Actor"}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v1/tv/1":
			body := `{"seasons":[{"seasonNumber":0,"name":"Specials"},{"seasonNumber":1,"name":"Season 1","episodeCount":6},{"seasonNumber":2,"name":"Season 2","episodeCount":6}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	results, err := c.Search(context.Background(), " slow   horses ", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].MediaType != "tv" || !results[0].CanRequest || results[0].OpenURL != "https://seerr.example/tv/1" {
		t.Fatalf("unexpected search results: %+v", results)
	}
	if results[0].PosterPath != "/poster.jpg" || results[0].PosterURL != "https://image.tmdb.org/t/p/w342/poster.jpg" {
		t.Fatalf("unexpected poster fields: %+v", results[0])
	}
	if len(results[0].Seasons) != 2 || results[0].Seasons[0].Number != 1 {
		t.Fatalf("unexpected seasons: %+v", results[0].Seasons)
	}
}

func TestSearchFetchesTVSeasonsConcurrently(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	var mu sync.Mutex
	activeSeasonRequests := 0
	maxActiveSeasonRequests := 0
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v1/search":
			body := `{"results":[
				{"id":1,"mediaType":"tv","name":"One","mediaInfo":{"status":1}},
				{"id":2,"mediaType":"tv","name":"Two","mediaInfo":{"status":1}},
				{"id":3,"mediaType":"movie","title":"Movie","mediaInfo":{"status":1}}
			]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case "/api/v1/tv/1", "/api/v1/tv/2":
			mu.Lock()
			activeSeasonRequests++
			if activeSeasonRequests > maxActiveSeasonRequests {
				maxActiveSeasonRequests = activeSeasonRequests
			}
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			activeSeasonRequests--
			mu.Unlock()
			body := `{"seasons":[{"seasonNumber":1,"name":"Season 1","episodeCount":6}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		default:
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
	})}

	results, err := c.Search(context.Background(), "shows", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d (%+v)", len(results), results)
	}
	if len(results[0].Seasons) != 1 || len(results[1].Seasons) != 1 {
		t.Fatalf("expected seasons on both TV results, got %+v", results)
	}
	if len(results[2].Seasons) != 0 {
		t.Fatalf("did not expect movie season lookup, got %+v", results[2])
	}
	if maxActiveSeasonRequests < 2 {
		t.Fatalf("expected concurrent season lookups, max active was %d", maxActiveSeasonRequests)
	}
}

func TestCreateRequestSendsSeerrPayload(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/request" || r.Method != http.MethodPost {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		if got := r.Header.Get("X-Api-Key"); got != "k" {
			t.Fatalf("missing api key header: %q", got)
		}
		if got := r.Header.Get("X-Api-User"); got != "7" {
			t.Fatalf("missing user header: %q", got)
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		body := string(bodyBytes)
		for _, want := range []string{`"mediaId":100`, `"mediaType":"tv"`, `"seasons":[2,3]`} {
			if !strings.Contains(body, want) {
				t.Fatalf("request body %q missing %s", body, want)
			}
		}
		if strings.Contains(body, `"userId"`) {
			t.Fatalf("request body must rely on X-Api-User instead of userId: %q", body)
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{"id":44,"status":1}`)), Header: make(http.Header)}, nil
	})}

	created, err := c.CreateRequest(context.Background(), CreateRequestInput{MediaID: 100, MediaType: "tv", UserID: 7, Seasons: []int{2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 44 || created.Status != "Pending" {
		t.Fatalf("unexpected created request: %+v", created)
	}
}

func TestCreateRequestRequiresTVSeason(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request to %s", r.URL.String())
		return nil, nil
	})}

	if _, err := c.CreateRequest(context.Background(), CreateRequestInput{MediaID: 100, MediaType: "tv", UserID: 7}); err == nil {
		t.Fatal("expected missing season error")
	}
}

func TestCreateRequestReportsNonJSONResponseWithoutHTMLBody(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Content-Type", "text/html")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<!DOCTYPE html><title>Login</title>`)), Header: header}, nil
	})}

	_, err := c.CreateRequest(context.Background(), CreateRequestInput{MediaID: 100, MediaType: "movie", UserID: 7})
	if err == nil {
		t.Fatal("expected non-JSON response error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "non-JSON") {
		t.Fatalf("expected non-JSON error, got %q", msg)
	}
	if strings.Contains(msg, "<!DOCTYPE") || strings.Contains(msg, "<title>") {
		t.Fatalf("unexpected HTML body leaked into error: %q", msg)
	}
}

func TestUserQuotaNestedEndpoint(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/user/7/quota" {
			body := `{"movie":{"limit":4,"used":1,"remaining":3},"tv":{"limit":6,"used":2,"remaining":4}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	q, err := c.UserQuotaForUser(context.Background(), UserIdentity{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if q.MovieRemaining != 3 || q.SeriesRemaining != 4 {
		t.Fatalf("unexpected nested quota: %+v", q)
	}
}

func TestUserQuotaUnlimitedNestedEndpoint(t *testing.T) {
	c := NewClient("http://seerr.local", "https://seerr.example", "k")
	c.http = &http.Client{Transport: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/user/7/quota" {
			body := `{"movie":{"limit":0,"used":0},"tv":{"limit":0,"used":0}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	q, err := c.UserQuotaForUser(context.Background(), UserIdentity{ID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !q.MovieUnlimited || !q.SeriesUnlimited {
		t.Fatalf("expected unlimited nested quota: %+v", q)
	}
}
