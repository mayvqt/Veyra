package middleware

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEnsureAndValidateCSRF(t *testing.T) {
	r := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	tok := EnsureCSRFToken(w, r, false)
	if tok == "" {
		t.Fatal("expected token")
	}
	res := w.Result()
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie")
	}

	form := url.Values{}
	form.Set("csrf_token", tok)
	r2 := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2.AddCookie(cookies[0])
	if !ValidateCSRF(r2) {
		t.Fatal("expected csrf validation success")
	}
}

func TestValidateCSRFFailsOnMismatch(t *testing.T) {
	r := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	_ = EnsureCSRFToken(w, r, false)
	res := w.Result()
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected csrf cookie")
	}

	form := url.Values{}
	form.Set("csrf_token", "wrong")
	r2 := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2.AddCookie(cookies[0])
	if ValidateCSRF(r2) {
		t.Fatal("expected csrf validation to fail")
	}
}
