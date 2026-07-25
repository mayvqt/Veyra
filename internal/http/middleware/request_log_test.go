package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/auth"
)

func TestRequestLogIncludesAuthenticatedUsername(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := RequestLog(log)(RequireAuth(resolverStub{user: auth.User{Username: "ada"}})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "raw"})
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if !strings.Contains(buf.String(), `"username":"ada"`) {
		t.Fatalf("expected username in request log, got %s", buf.String())
	}
}

func TestRequestLogUsesPlaceholderUsernameWhenUnauthenticated(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := RequestLog(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if !strings.Contains(buf.String(), `"username":"-"`) {
		t.Fatalf("expected placeholder username in request log, got %s", buf.String())
	}
}

func TestRequestLogKeepsFirstStatusCode(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := RequestLog(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/requests", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("want recorder status 202 got %d", w.Code)
	}
	if !strings.Contains(buf.String(), `"status":202`) {
		t.Fatalf("expected first status in request log, got %s", buf.String())
	}
}
