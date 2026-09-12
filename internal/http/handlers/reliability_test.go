package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations/arr"
	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
	"github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/store"
	"github.com/mayvqt/veyra/internal/testutil"
)

func TestCacheOutageCoalescesFailuresAndAllowsCancellation(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cacheSetJSON(context.Background(), "outage", x{7}, -time.Second)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	load := func() (x, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return x{}, errors.New("offline")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, stale, err := cacheLoadJSONWithStale(h, context.Background(), "outage", time.Minute, time.Minute, load)
			if err != nil || !stale || v.A != 7 {
				t.Errorf("value=%v stale=%v error=%v", v, stale, err)
			}
		}()
	}
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := cacheLoadJSONWithStale(h, ctx, "outage", time.Minute, time.Minute, load)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("canceled waiter: %v", err)
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("outage made %d provider calls; want one", calls.Load())
	}
}

func TestIntegrationCacheSurvivesRestartOnlyForSameOrigin(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	u := auth.User{MediaServerUserID: "member", Username: "7"}
	h.cacheSetJSON(context.Background(), "seerr:user:media_server:member", seerr.UserIdentity{ID: 7}, time.Minute)
	for _, change := range []string{"same", "seerr-url", "seerr-key", "media-url", "media-provider"} {
		t.Run(change, func(t *testing.T) {
			cfg := h.cfg
			switch change {
			case "seerr-url":
				cfg.SeerrURL = "http://replacement.invalid"
			case "seerr-key":
				cfg.SeerrAPIKey = "replacement-key"
			case "media-url":
				cfg.MediaServerURL = "http://replacement.invalid"
			case "media-provider":
				cfg.MediaServerType = "emby"
			}
			fresh := &Handlers{cfg: cfg, db: db, cacheNamespace: integrationCacheNamespace(cfg), seerr: &fakeSeerr{resolveByIDErr: seerr.ErrUserNotLinked}}
			got, err := fresh.resolveSeerrUser(httptest.NewRequest("GET", "/", nil), u)
			if change == "same" {
				if err != nil || got.ID != 7 {
					t.Fatalf("same-origin cache lost: %v %v", got, err)
				}
				return
			}
			if err == nil || got.ID != 0 {
				t.Fatalf("previous identity reused: %v %v", got, err)
			}
		})
	}
}

func TestNumericUsernameCannotReadLinkedUserCache(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{resolveByIDErr: seerr.ErrUserNotLinked}
	for _, key := range []string{"requests:recent:v2:7", "requests:recent:user:7"} {
		h.cacheSetJSON(context.Background(), key, []dashboard.RequestItem{{Title: "PRIVATE HISTORY"}}, time.Minute)
	}
	for _, key := range []string{"requests:quota:7", "requests:quota:user:7"} {
		h.cacheSetJSON(context.Background(), key, seerr.Quota{Limit: 314159, Remaining: 314159}, time.Minute)
	}
	user, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "unlinked", Username: "7"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: user.ID}, "token", "", "")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/dashboard", nil)
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	w := httptest.NewRecorder()
	middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.Dashboard)).ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != 200 || strings.Contains(body, "PRIVATE HISTORY") || strings.Contains(body, "314159") || !strings.Contains(body, "Link your Jellyfin account") {
		t.Fatalf("unsafe dashboard: status %d body %s", w.Code, body)
	}
}

func TestOptionalDashboardLinksAndBranding(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	settings := map[string]string{settingAppName: "My Media", settingAppAccentColor: "#abcdef", settingAppLogoURL: "https://assets.example/logo.svg"}
	if msg := validateSettingsInput(settings); msg != "" {
		t.Fatal(msg)
	}
	if err := store.UpsertSettings(context.Background(), db, settings); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	middleware.SecurityHeaders(http.HandlerFunc(h.Login)).ServeHTTP(w, httptest.NewRequest("GET", "/login", nil))
	for _, expected := range []string{"My Media", "https://assets.example/logo.svg", "--accent:#abcdef"} {
		if !strings.Contains(w.Body.String(), expected) {
			t.Errorf("missing branding %q", expected)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "img-src 'self' https://assets.example") {
		t.Fatal("logo origin not permitted")
	}
}

func TestAdminLoadsUnavailableServicesOnceAndConcurrently(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{}
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	var calls atomic.Int32
	allEntered := make(chan struct{})
	http.DefaultTransport = testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 4 {
			close(allEntered)
		}
		select {
		case <-allEntered:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	snapshot := h.loadAdminSnapshot(httptest.NewRequest("GET", "/admin", nil).WithContext(ctx), h.arrAdminServices())
	if ctx.Err() != nil || calls.Load() != 4 || snapshot.mediaHealth.OK {
		t.Fatalf("serial/repeated health loads: calls=%d context=%v", calls.Load(), ctx.Err())
	}
}

func TestPlaybackFailureAndMissingGuideLinksAreExplicit(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	http.DefaultTransport = testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/Sessions" {
			t.Errorf("unnecessary playback lookup: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	var err error
	h.mediaserver, err = mediaserver.NewClient(h.cfg.MediaServerType, "http://media.invalid", "", "key")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.AdminPlayback(w, httptest.NewRequest("GET", "/admin/playback", nil))
	body := w.Body.String()
	if !strings.Contains(body, "Playback sessions are unavailable") || strings.Contains(body, "0 active sessions") || strings.Contains(body, "No active playback") {
		t.Fatalf("misleading playback: %s", body)
	}
	h.cfg.MediaServerPublicURL, h.cfg.SeerrPublicURL = "", ""
	w = httptest.NewRecorder()
	h.Guide(w, httptest.NewRequest("GET", "/guide", nil))
	if strings.Contains(w.Body.String(), `href=""`) || !strings.Contains(w.Body.String(), "Ask your administrator") {
		t.Fatal("guide exposes missing links")
	}
}

func TestRequestConflictDoesNotAuditSuccess(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{resolvedUser: &seerr.UserIdentity{ID: 7}, createErr: seerr.ErrRequestConflict}
	user, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "member", Username: "member"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: user.ID}, "token", "", "")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"csrf_token": {"csrf"}, "media_id": {"42"}, "media_type": {"tv"}, "seasons": {"2"}}
	r := httptest.NewRequest("POST", "/api/seerr/request", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "csrf"})
	w := httptest.NewRecorder()
	middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.SeerrRequestPost)).ServeHTTP(w, r)
	logs, err := store.ListRecentAuditLogsByActionTarget(context.Background(), db, "request.created", "42", 10)
	if w.Code != http.StatusConflict || err != nil || len(logs) != 0 {
		t.Fatalf("no-op status=%d audit=%v error=%v", w.Code, logs, err)
	}
}

func TestHealthyAndUnconfiguredAdminServicesHaveNoErrorFooter(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.seerr = &fakeSeerr{}
	h.cacheSetJSON(context.Background(), "admin:summary:media_server", mediaserver.AdminSummary{ServerName: "Test"}, time.Minute)
	for _, svc := range h.arrAdminServices() {
		h.cacheSetJSON(context.Background(), "admin:summary:"+strings.ToLower(svc.Name), arr.AdminSummary{AppName: svc.Name}, time.Minute)
	}
	h.cfg.SonarrURL = ""
	w := httptest.NewRecorder()
	h.AdminIntegrations(w, httptest.NewRequest("GET", "/admin/integrations", nil))
	if strings.Contains(w.Body.String(), "Service details unavailable") {
		t.Fatal("healthy/unconfigured service rendered as failed")
	}
}
