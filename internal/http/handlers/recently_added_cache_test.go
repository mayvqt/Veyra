package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/store"
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
