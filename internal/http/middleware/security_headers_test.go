package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersIncludeCSPHardening(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(w, r)

	csp := w.Header().Get("Content-Security-Policy")
	for _, token := range []string{"default-src 'self'", "script-src 'self'", "img-src 'self' data: https://image.tmdb.org", "object-src 'none'", "frame-ancestors 'none'", "form-action 'self'"} {
		if !strings.Contains(csp, token) {
			t.Fatalf("expected CSP to contain %q, got %q", token, csp)
		}
	}
}

func TestSecurityHeadersLimitsRequestBodies(t *testing.T) {
	var readErr error
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
	h.ServeHTTP(httptest.NewRecorder(), r)
	if readErr == nil {
		t.Fatal("expected oversized request body to be rejected")
	}
}
