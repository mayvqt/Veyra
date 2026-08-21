package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mayvqt/veyra/internal/auth"
)

type ctxKey string

const (
	SessionCookieName = "veyra_session"
	userKey           = ctxKey("auth_user")
	sessionKey        = ctxKey("auth_session")
)

type SessionResolver interface {
	ResolveSession(ctx context.Context, rawID string) (auth.Session, auth.User, error)
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

func RequireAuth(svc SessionResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				requireAuthFailed(w, r)
				return
			}
			session, user, err := svc.ResolveSession(r.Context(), cookie.Value)
			if err != nil {
				requireAuthFailed(w, r)
				return
			}
			setRequestLogUsername(r.Context(), user.Username)
			ctx := context.WithValue(r.Context(), userKey, user)
			ctx = context.WithValue(ctx, sessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok || !u.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireAuthFailed(w http.ResponseWriter, r *http.Request) {
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(apiErrorResponse{Error: "Your session expired. Log in again."})
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func UserFromContext(ctx context.Context) (auth.User, bool) {
	u, ok := ctx.Value(userKey).(auth.User)
	return u, ok
}

func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	session, ok := ctx.Value(sessionKey).(auth.Session)
	return session, ok
}
