package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/security"
)

func (h *Handlers) SetupGet(w http.ResponseWriter, r *http.Request) {
	if !config.SetupRequired(h.cfg) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	h.render(w, "setup.html", ViewData{
		AppName:         h.appName(r),
		CSRFToken:       middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
		Now:             time.Now(),
		Setup:           setupInputFromConfig(h.cfg),
		MediaServerName: h.cfg.MediaServerType.Label(),
	})
}

func (h *Handlers) SetupPost(w http.ResponseWriter, r *http.Request) {
	if !config.SetupRequired(h.cfg) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if !middleware.ValidateCSRF(r) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	in := setupInputFromForm(r)
	if errMsg := validateSetupInput(in); errMsg != "" {
		h.render(w, "setup.html", ViewData{
			AppName:         h.appName(r),
			CSRFToken:       middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
			Now:             time.Now(),
			Error:           errMsg,
			Setup:           in,
			MediaServerName: mediaServerInputLabel(in.MediaServerType),
		})
		return
	}
	if err := config.SaveSetup(r.Context(), h.db, security.NewCrypto(h.cfg.EncryptionKey), in); err != nil {
		http.Error(w, "failed to save setup", http.StatusInternalServerError)
		return
	}
	h.render(w, "setup.html", ViewData{
		AppName:         h.appName(r),
		CSRFToken:       middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
		Now:             time.Now(),
		SetupComplete:   true,
		MediaServerName: mediaServerInputLabel(in.MediaServerType),
	})
	if h.restart != nil {
		go h.restart()
	}
}

func setupInputFromConfig(cfg config.Config) config.SetupInput {
	return config.SetupInput{
		AppBaseURL:           cfg.AppBaseURL,
		MediaServerType:      cfg.MediaServerType.String(),
		MediaServerURL:       cfg.MediaServerURL,
		MediaServerPublicURL: cfg.MediaServerPublicURL,
		MediaServerAPIKey:    "",
		SeerrURL:             cfg.SeerrURL,
		SeerrPublicURL:       cfg.SeerrPublicURL,
		SeerrAPIKey:          "",
		SonarrURL:            cfg.SonarrURL,
		SonarrAPIKey:         "",
		RadarrURL:            cfg.RadarrURL,
		RadarrAPIKey:         "",
		ProwlarrURL:          cfg.ProwlarrURL,
		ProwlarrAPIKey:       "",
	}
}

func setupInputFromForm(r *http.Request) config.SetupInput {
	return config.SetupInput{
		AppBaseURL:           strings.TrimSpace(r.FormValue("app_base_url")),
		MediaServerType:      strings.TrimSpace(r.FormValue("media_server_type")),
		MediaServerURL:       strings.TrimSpace(r.FormValue("media_server_url")),
		MediaServerPublicURL: strings.TrimSpace(r.FormValue("media_server_public_url")),
		MediaServerAPIKey:    strings.TrimSpace(r.FormValue("media_server_api_key")),
		SeerrURL:             strings.TrimSpace(r.FormValue("seerr_url")),
		SeerrPublicURL:       strings.TrimSpace(r.FormValue("seerr_public_url")),
		SeerrAPIKey:          strings.TrimSpace(r.FormValue("seerr_api_key")),
		SonarrURL:            strings.TrimSpace(r.FormValue("sonarr_url")),
		SonarrAPIKey:         strings.TrimSpace(r.FormValue("sonarr_api_key")),
		RadarrURL:            strings.TrimSpace(r.FormValue("radarr_url")),
		RadarrAPIKey:         strings.TrimSpace(r.FormValue("radarr_api_key")),
		ProwlarrURL:          strings.TrimSpace(r.FormValue("prowlarr_url")),
		ProwlarrAPIKey:       strings.TrimSpace(r.FormValue("prowlarr_api_key")),
	}
}

func validateSetupInput(in config.SetupInput) string {
	in = normalizeSetupInputForValidation(in)
	serverType, err := config.ParseMediaServerType(in.MediaServerType)
	if err != nil {
		return "Media server must be Jellyfin or Emby."
	}
	label := serverType.Label()
	if in.MediaServerURL == "" {
		return label + " internal URL is required."
	}
	if in.AppBaseURL == "" {
		return "App base URL is required."
	}
	if !validSetupURL(in.AppBaseURL) {
		return "App base URL must be a valid http or https URL."
	}
	for _, candidate := range []struct {
		label string
		value string
	}{
		{label + " internal URL", in.MediaServerURL},
		{label + " public URL", in.MediaServerPublicURL},
		{"Seerr internal URL", in.SeerrURL},
		{"Seerr public URL", in.SeerrPublicURL},
		{"Sonarr URL", in.SonarrURL},
		{"Radarr URL", in.RadarrURL},
		{"Prowlarr URL", in.ProwlarrURL},
	} {
		if candidate.value != "" && !validSetupURL(candidate.value) {
			return candidate.label + " must be a valid http or https URL."
		}
	}
	if in.SeerrURL != "" && in.SeerrAPIKey == "" {
		return "Seerr API key is required when Seerr URL is set."
	}
	if in.SonarrURL != "" && in.SonarrAPIKey == "" {
		return "Sonarr API key is required when Sonarr URL is set."
	}
	if in.RadarrURL != "" && in.RadarrAPIKey == "" {
		return "Radarr API key is required when Radarr URL is set."
	}
	if in.ProwlarrURL != "" && in.ProwlarrAPIKey == "" {
		return "Prowlarr API key is required when Prowlarr URL is set."
	}
	for _, candidate := range []struct {
		label string
		value string
	}{
		{"App base URL", in.AppBaseURL},
		{label + " public URL", in.MediaServerPublicURL},
		{label + " API key", in.MediaServerAPIKey},
		{"Seerr public URL", in.SeerrPublicURL},
		{"Seerr API key", in.SeerrAPIKey},
		{"Sonarr API key", in.SonarrAPIKey},
		{"Radarr API key", in.RadarrAPIKey},
		{"Prowlarr API key", in.ProwlarrAPIKey},
	} {
		if config.IsPlaceholderValue(candidate.value) {
			return candidate.label + " still contains a placeholder value."
		}
	}
	return ""
}

func normalizeSetupInputForValidation(in config.SetupInput) config.SetupInput {
	in.AppBaseURL = config.NormalizeHTTPURL(in.AppBaseURL)
	if serverType, err := config.ParseMediaServerType(in.MediaServerType); err == nil {
		in.MediaServerType = serverType.String()
	}
	in.MediaServerURL = config.NormalizeHTTPURL(in.MediaServerURL)
	in.MediaServerPublicURL = config.NormalizeHTTPURL(in.MediaServerPublicURL)
	in.SeerrURL = config.NormalizeHTTPURL(in.SeerrURL)
	in.SeerrPublicURL = config.NormalizeHTTPURL(in.SeerrPublicURL)
	in.SonarrURL = config.NormalizeHTTPURL(in.SonarrURL)
	in.RadarrURL = config.NormalizeHTTPURL(in.RadarrURL)
	in.ProwlarrURL = config.NormalizeHTTPURL(in.ProwlarrURL)
	return in
}

func mediaServerInputLabel(value string) string {
	serverType, err := config.ParseMediaServerType(value)
	if err != nil {
		return "Media server"
	}
	return serverType.Label()
}

func validSetupURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	value = config.NormalizeHTTPURL(value)
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	return u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}
