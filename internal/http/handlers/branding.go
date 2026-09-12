package handlers

import (
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/store"
)

func (h *Handlers) applyBranding(w http.ResponseWriter, data *ViewData) {
	if h.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	settings, err := store.GetSettings(ctx, h.db, []string{settingAppName, settingAppLogoURL, settingAppAccentColor})
	if err != nil {
		return
	}
	data.AppName = appNameFromSettings(settings, data.AppName)
	if accent := settings[settingAppAccentColor]; hexColorRe.MatchString(accent) {
		// Only the validated six-digit hexadecimal color enters CSS.
		data.AccentStyle = template.CSS(":root{--accent:" + accent + ";--accent-strong:color-mix(in srgb," + accent + ",white 70%);--accent-soft:color-mix(in srgb," + accent + " 13%,transparent)}")
	}
	logo := settings[settingAppLogoURL]
	if logo == "" || !isValidURL(logo) {
		return
	}
	u, err := url.Parse(logo)
	if err != nil || strings.ContainsAny(u.Host, ";'\" \\<>\t\r\n") {
		return
	}
	data.LogoURL = logo
	if csp := w.Header().Get("Content-Security-Policy"); csp != "" {
		origin := u.Scheme + "://" + u.Host
		w.Header().Set("Content-Security-Policy", strings.Replace(csp, "img-src 'self'", "img-src 'self' "+origin, 1))
	}
}
