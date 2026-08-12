package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitRequestBodyRejectsKnownOversizedBody(t *testing.T) {
	handler := LimitRequestBody(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("oversized request reached handler")
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345")))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", w.Code)
	}
}

func TestLimitRequestBodyBoundsUnknownLengthBody(t *testing.T) {
	handler := LimitRequestBody(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected bounded reader error")
		}
	}))
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	r.ContentLength = -1
	handler.ServeHTTP(httptest.NewRecorder(), r)
}
