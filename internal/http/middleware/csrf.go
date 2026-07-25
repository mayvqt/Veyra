package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/mayvqt/veyra/internal/auth"
)

const csrfCookie = "veyra_csrf"

func EnsureCSRFToken(w http.ResponseWriter, r *http.Request, secure bool) string {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return c.Value
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	tok := hex.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: tok, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int(auth.DefaultSessionDuration.Seconds())})
	return tok
}

func ValidateCSRF(r *http.Request) bool {
	c, err := r.Cookie(csrfCookie)
	if err != nil || c.Value == "" {
		return false
	}
	if err := r.ParseForm(); err != nil {
		return false
	}
	formToken := r.FormValue("csrf_token")
	if formToken == "" || len(formToken) != len(c.Value) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(formToken), []byte(c.Value)) == 1
}
