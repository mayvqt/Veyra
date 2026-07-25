package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/auth"
	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func (h *Handlers) AdminSettingsGet(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	view := ViewData{AppName: h.appName(r), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), User: u}
	h.populateSettingsView(r, &view)
	if r.URL.Query().Get("saved") == "1" {
		view.Success = "Settings saved."
	}
	if r.URL.Query().Get("saved") == "restart" {
		view.Success = "Settings saved. Veyra is restarting so service connection changes can be loaded."
	}
	h.render(w, "admin_settings.html", view)
}

func (h *Handlers) AdminSettingsPost(w http.ResponseWriter, r *http.Request) {
	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	u, _ := middleware.UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	pairs := settingsPairsFromForm(r)
	setupIn, setupChanged, err := h.setupInputFromSettingsForm(r)
	if err != nil {
		http.Error(w, "failed to read setup settings", http.StatusInternalServerError)
		return
	}
	if errMsg := validateSettingsInput(pairs); errMsg != "" {
		h.renderSettingsError(w, r, u, pairs, setupIn, errMsg)
		return
	}
	if setupChanged {
		if errMsg := validateSetupInput(setupIn); errMsg != "" {
			h.renderSettingsError(w, r, u, pairs, setupIn, errMsg)
			return
		}
	}
	for k, v := range pairs {
		if err := store.UpsertSetting(r.Context(), h.db, k, v); err != nil {
			http.Error(w, "failed to save settings", http.StatusInternalServerError)
			return
		}
	}
	if setupChanged {
		if err := config.SaveSetup(r.Context(), h.db, security.NewCrypto(h.cfg.EncryptionKey), setupIn); err != nil {
			http.Error(w, "failed to save setup settings", http.StatusInternalServerError)
			return
		}
	}
	audit := map[string]string{"widgets": enabledWidgetSummary(pairs)}
	if setupChanged {
		audit["setup_config"] = "updated"
	}
	_ = store.InsertAuditLog(r.Context(), h.db, &u.ID, "admin.settings.updated", "settings", auditMetadata(audit), middleware.ClientIP(r))
	if setupChanged {
		http.Redirect(w, r, "/admin/settings?saved=restart", http.StatusFound)
		if h.restart != nil {
			go h.restart()
		}
		return
	}
	http.Redirect(w, r, "/admin/settings?saved=1", http.StatusFound)
}

func (h *Handlers) renderSettingsError(w http.ResponseWriter, r *http.Request, u auth.User, pairs map[string]string, setupIn config.SetupInput, errMsg string) {
	view := ViewData{AppName: h.appName(r), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), User: u, Error: errMsg, MediaServerName: h.cfg.MediaServerType.Label()}
	applySettingsPairsToView(&view, pairs)
	h.populateSetupConfigView(r, &view, setupIn)
	h.render(w, "admin_settings.html", view)
}

func (h *Handlers) populateSettingsView(r *http.Request, view *ViewData) {
	settings := readSettings(r, h.db,
		settingAppName,
		settingAppLogoURL,
		settingAppAccentColor,
		settingMediaServerPublicURL,
		settingSeerrPublicURL,
		settingWidgetRequestBot,
		settingWidgetRecentMedia,
		settingWidgetRecentRequests,
		settingWidgetRequestQuota,
		settingWidgetDownloadQueue,
		settingWidgetCalendar,
		settingDashboardMessage,
	)
	view.BrandAppName = withDefault(readSettingFromMap(settings, settingAppName), h.cfg.AppName)
	view.MediaServerName = h.cfg.MediaServerType.Label()
	view.BrandLogoURL = readSettingFromMap(settings, settingAppLogoURL)
	view.BrandAccent = withDefault(readSettingFromMap(settings, settingAppAccentColor), "#d43f24")
	view.SettingsMediaServerPublic = withDefault(readSettingFromMap(settings, settingMediaServerPublicURL), h.cfg.MediaServerPublicURL)
	view.SettingsSeerrPublic = withDefault(readSettingFromMap(settings, settingSeerrPublicURL), h.cfg.SeerrPublicURL)
	view.SettingsShowRequestBot = readBoolSettingFromMap(settings, settingWidgetRequestBot, true)
	view.SettingsShowRecentMedia = readBoolSettingFromMap(settings, settingWidgetRecentMedia, true)
	view.SettingsShowRecentReqs = readBoolSettingFromMap(settings, settingWidgetRecentRequests, true)
	view.SettingsShowQuota = readBoolSettingFromMap(settings, settingWidgetRequestQuota, true)
	view.SettingsShowQueue = readBoolSettingFromMap(settings, settingWidgetDownloadQueue, true)
	view.SettingsShowCalendar = readBoolSettingFromMap(settings, settingWidgetCalendar, true)
	view.DashboardMessage = readSettingFromMap(settings, settingDashboardMessage)
	h.populateSetupConfigView(r, view, config.SetupInput{})
}

func settingsPairsFromForm(r *http.Request) map[string]string {
	return map[string]string{
		settingAppName:              strings.TrimSpace(r.FormValue("app_name")),
		settingAppLogoURL:           strings.TrimSpace(r.FormValue("logo_url")),
		settingAppAccentColor:       strings.TrimSpace(r.FormValue("accent_color")),
		settingMediaServerPublicURL: strings.TrimSpace(r.FormValue("media_server_public_url")),
		settingSeerrPublicURL:       strings.TrimSpace(r.FormValue("seerr_public_url")),
		settingWidgetRequestBot:     fmt.Sprintf("%t", r.FormValue("show_request_bot") == "on"),
		settingWidgetRecentMedia:    fmt.Sprintf("%t", r.FormValue("show_recent_media") == "on"),
		settingWidgetRecentRequests: fmt.Sprintf("%t", r.FormValue("show_recent_requests") == "on"),
		settingWidgetRequestQuota:   fmt.Sprintf("%t", r.FormValue("show_request_quota") == "on"),
		settingWidgetDownloadQueue:  fmt.Sprintf("%t", r.FormValue("show_download_queue") == "on"),
		settingWidgetCalendar:       fmt.Sprintf("%t", r.FormValue("show_upcoming_calendar") == "on"),
		settingDashboardMessage:     strings.TrimSpace(r.FormValue("dashboard_message")),
	}
}

func applySettingsPairsToView(view *ViewData, pairs map[string]string) {
	view.BrandAppName = pairs[settingAppName]
	view.BrandLogoURL = pairs[settingAppLogoURL]
	view.BrandAccent = pairs[settingAppAccentColor]
	view.SettingsMediaServerPublic = pairs[settingMediaServerPublicURL]
	view.SettingsSeerrPublic = pairs[settingSeerrPublicURL]
	view.SettingsShowRequestBot = pairs[settingWidgetRequestBot] == "true"
	view.SettingsShowRecentMedia = pairs[settingWidgetRecentMedia] == "true"
	view.SettingsShowRecentReqs = pairs[settingWidgetRecentRequests] == "true"
	view.SettingsShowQuota = pairs[settingWidgetRequestQuota] == "true"
	view.SettingsShowQueue = pairs[settingWidgetDownloadQueue] == "true"
	view.SettingsShowCalendar = pairs[settingWidgetCalendar] == "true"
	view.DashboardMessage = pairs[settingDashboardMessage]
}
