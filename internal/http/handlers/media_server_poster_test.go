package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/store"

	"github.com/go-chi/chi/v5"
)

func TestMediaServerPosterReturnsBadGatewayWhenAPIKeyMissing(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-poster", Username: "u", DisplayName: "U", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := h.authSvc.CreateSession(context.Background(), auth.User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: urow.IsAdmin}, "token", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}

	wrapped := middleware.RequireAuth(h.authSvc)(http.HandlerFunc(h.MediaServerPoster))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/media/server/poster/abc", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("itemID", "abc")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
	r.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: raw})
	wrapped.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502 got %d", w.Code)
	}
}
