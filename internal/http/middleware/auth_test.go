package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mayvqt/veyra/internal/auth"
)

type resolverStub struct {
	session auth.Session
	user    auth.User
	err     error
}

func (r resolverStub) ResolveSession(ctx context.Context, rawID string) (auth.Session, auth.User, error) {
	if r.err != nil {
		return auth.Session{}, auth.User{}, r.err
	}
	return r.session, r.user, nil
}

func TestRequireAuthAddsResolvedIdentityToContext(t *testing.T) {
	wantSession := auth.Session{IDHash: "session-hash"}
	wantUser := auth.User{Username: "angel"}
	h := RequireAuth(resolverStub{session: wantSession, user: wantUser})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSession, sessionOK := SessionFromContext(r.Context())
		gotUser, userOK := UserFromContext(r.Context())
		if !sessionOK || gotSession.IDHash != wantSession.IDHash {
			t.Fatalf("session context = %+v, %v", gotSession, sessionOK)
		}
		if !userOK || gotUser.Username != wantUser.Username {
			t.Fatalf("user context = %+v, %v", gotUser, userOK)
		}
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "raw"})
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}

func TestRequireAuthRedirectsWithoutSession(t *testing.T) {
	mw := RequireAuth(resolverStub{})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302 got %d", w.Code)
	}
}

func TestRequireAuthRedirectsOnResolverError(t *testing.T) {
	mw := RequireAuth(resolverStub{err: errors.New("bad")})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "raw"})
	h.ServeHTTP(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302 got %d", w.Code)
	}
}

func TestRequireAuthReturnsJSONForAPIWithoutSession(t *testing.T) {
	mw := RequireAuth(resolverStub{})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/seerr/request", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}
	var body apiErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestRequireAuthReturnsJSONForAPIResolverError(t *testing.T) {
	mw := RequireAuth(resolverStub{err: errors.New("bad")})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/seerr/search", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "raw"})
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", w.Code)
	}
	var body apiErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestRequireAdminBlocksNonAdmin(t *testing.T) {
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(r.Context(), userKey, auth.User{IsAdmin: false})
	h.ServeHTTP(w, r.WithContext(ctx))
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d", w.Code)
	}
}

func TestRequireAdminAllowsAdmin(t *testing.T) {
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(r.Context(), userKey, auth.User{IsAdmin: true})
	h.ServeHTTP(w, r.WithContext(ctx))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}
