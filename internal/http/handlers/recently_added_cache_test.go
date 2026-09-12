package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
	"github.com/mayvqt/veyra/internal/store"
	"github.com/mayvqt/veyra/internal/testutil"
)

func TestDashboardShowsRecentlyAddedUnavailableWhenAPIKeyMissing(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-cache", Username: "u", DisplayName: "U", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: urow.IsAdmin}, "token", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}

	wrapped := middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.Dashboard))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	wrapped.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Recently added media is unavailable right now.") {
		t.Fatal("expected unavailable message when recently added fetch fails")
	}
}

func TestDashboardDoesNotReuseUngroupedRecentlyAddedCache(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	for _, test := range []struct {
		name string
		ttl  time.Duration
	}{
		{name: "fresh", ttl: time.Minute},
		{name: "stale during outage", ttl: -time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, db := mkHandlers(t)
			defer db.Close()
			h.seerr = &fakeSeerr{}
			h.cfg.SeerrURL, h.cfg.SonarrURL, h.cfg.RadarrURL = "", "", ""
			var err error
			h.mediaserver, err = mediaserver.NewClient(h.cfg.MediaServerType, "http://media.invalid", "", "key")
			if err != nil {
				t.Fatal(err)
			}
			h.cacheSetJSON(context.Background(), "media:recent:member", []dashboard.MediaItem{{Title: "OLD EPISODE CARD"}}, test.ttl)
			user, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "member", Username: "member"})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: user.ID}, "token", "", "")
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
			r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
			w := httptest.NewRecorder()
			middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.Dashboard)).ServeHTTP(w, r)
			if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "OLD EPISODE CARD") || !strings.Contains(w.Body.String(), "Recently added media is unavailable right now.") {
				t.Fatalf("ungrouped cache survived representation change: status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
