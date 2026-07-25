package handlers

import (
	"net/http"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/store"
)

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	if config.SetupRequired(h.cfg) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if cookie, err := r.Cookie(middleware.SessionCookieName); err == nil && cookie.Value != "" {
		if _, user, err := h.authSvc.ResolveSession(r.Context(), cookie.Value); err == nil {
			if user.IsAdmin {
				http.Redirect(w, r, "/admin", http.StatusFound)
				return
			}
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handlers) Guide(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	settings := readSettings(r, h.db, settingAppName, settingSeerrPublicURL, settingMediaServerPublicURL)
	view := ViewData{
		AppName:              appNameFromSettings(settings, h.cfg.AppName),
		CSRFToken:            middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
		Now:                  time.Now(),
		User:                 u,
		SeerrPublicURL:       withDefault(readSettingFromMap(settings, settingSeerrPublicURL), h.cfg.SeerrPublicURL),
		MediaServerPublicURL: withDefault(readSettingFromMap(settings, settingMediaServerPublicURL), h.cfg.MediaServerPublicURL),
		MediaServerName:      h.cfg.MediaServerType.Label(),
	}
	h.render(w, "guide.html", view)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if config.SetupRequired(h.cfg) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	csrf := middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure)
	h.render(w, "login.html", ViewData{AppName: h.appName(r), CSRFToken: csrf, Now: time.Now(), MediaServerName: h.cfg.MediaServerType.Label()})
}

func (h *Handlers) LoginPost(w http.ResponseWriter, r *http.Request) {
	if config.SetupRequired(h.cfg) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if !h.loginLimiter.Allow(middleware.ClientIP(r), username) {
		http.Error(w, "too many login attempts", http.StatusTooManyRequests)
		return
	}

	userIn, token, err := h.mediaserver.Authenticate(r.Context(), username, password)
	if err != nil {
		_ = store.InsertAuditLog(r.Context(), h.db, nil, "login.failure", username, auditMetadata(map[string]string{"reason": "invalid_credentials"}), middleware.ClientIP(r))
		h.render(w, "login.html", ViewData{AppName: h.appName(r), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), Error: "Invalid credentials", MediaServerName: h.cfg.MediaServerType.Label()})
		return
	}
	userRow, err := store.UpsertUserByMediaServerID(r.Context(), h.db, store.UserRow{MediaServerUserID: userIn.ID, Username: userIn.Username, DisplayName: userIn.Username, IsAdmin: userIn.IsAdmin})
	if err != nil {
		http.Error(w, "failed to persist user", http.StatusInternalServerError)
		return
	}
	user := auth.User{ID: userRow.ID, MediaServerUserID: userRow.MediaServerUserID, Username: userRow.Username, DisplayName: userRow.DisplayName, IsAdmin: userRow.IsAdmin}
	sessionDuration := auth.DefaultSessionDuration
	if r.FormValue("remember_me") == "on" {
		sessionDuration = auth.RememberSessionDuration
	}
	sessionID, err := h.authSvc.CreateSessionWithDuration(r.Context(), user, token, r, sessionDuration)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	_ = store.InsertAuditLog(r.Context(), h.db, &user.ID, "login.success", user.Username, auditMetadata(map[string]string{"display_name": user.DisplayName}), middleware.ClientIP(r))

	h.setSessionCookie(w, sessionID, sessionDuration)
	if user.IsAdmin {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Handlers) LogoutPost(w http.ResponseWriter, r *http.Request) {
	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err == nil && cookie.Value != "" {
		session, user, _ := h.authSvc.ResolveSession(r.Context(), cookie.Value)
		if token, err := h.authSvc.DecryptSessionToken(session.MediaServerAccessTokenEncrypted); err == nil && token != "" {
			if err := h.mediaserver.Logout(r.Context(), token); err != nil {
				h.log.Warn("media server logout failed", "provider", h.mediaserver.Name(), "err", err)
			}
		}
		_ = h.authSvc.DestroySession(r.Context(), cookie.Value)
		if user.ID > 0 {
			_ = store.InsertAuditLog(r.Context(), h.db, &user.ID, "logout", "session", "{}", middleware.ClientIP(r))
		}
	}
	h.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handlers) setSessionCookie(w http.ResponseWriter, sessionID string, duration time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(duration.Seconds()),
		Expires:  time.Now().Add(duration),
	})
}

func (h *Handlers) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
