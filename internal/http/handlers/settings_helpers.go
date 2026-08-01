package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func enabledWidgetSummary(pairs map[string]string) string {
	enabled := make([]string, 0, 6)
	if pairs[settingWidgetRequestBot] == "true" {
		enabled = append(enabled, "requests")
	}
	if pairs[settingWidgetRecentMedia] == "true" {
		enabled = append(enabled, "recent media")
	}
	if pairs[settingWidgetCalendar] == "true" {
		enabled = append(enabled, "calendar")
	}
	if pairs[settingWidgetRecentRequests] == "true" {
		enabled = append(enabled, "recent requests")
	}
	if pairs[settingWidgetRequestQuota] == "true" {
		enabled = append(enabled, "quota")
	}
	if pairs[settingWidgetDownloadQueue] == "true" {
		enabled = append(enabled, "download queue")
	}
	if len(enabled) == 0 {
		return "none"
	}
	return strings.Join(enabled, ", ")
}

func combineIntegrationNotes(configured string, recent []string) []string {
	out := []string{configured}
	if len(recent) == 0 {
		out = append(out, "Recent Failures: none")
		return out
	}
	out = append(out, "Recent Failures:")
	out = append(out, recent...)
	return out
}

func (h *Handlers) appName(r *http.Request) string {
	return withDefault(readSetting(r, h.db, settingAppName), h.cfg.AppName)
}

func appNameFromSettings(settings map[string]string, fallback string) string {
	return withDefault(readSettingFromMap(settings, settingAppName), fallback)
}

func readSetting(r *http.Request, db *sql.DB, key string) string {
	v, _ := store.GetSetting(r.Context(), db, key)
	return v
}

func readSettings(r *http.Request, db *sql.DB, keys ...string) map[string]string {
	v, err := store.GetSettings(r.Context(), db, keys)
	if err != nil {
		return map[string]string{}
	}
	return v
}

func readSettingFromMap(settings map[string]string, key string) string {
	if settings == nil {
		return ""
	}
	return settings[key]
}

func readBoolSettingFromMap(settings map[string]string, key string, fallback bool) bool {
	v := readSettingFromMap(settings, key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return parsed
}

func readBoolSetting(r *http.Request, db *sql.DB, key string, fallback bool) bool {
	v := readSetting(r, db, key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return parsed
}

func withDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func boolToStatus(ok bool) string {
	if ok {
		return "Online"
	}
	return "Offline"
}

func boolYesNo(ok bool) string {
	if ok {
		return "Yes"
	}
	return "No"
}

func healthErr(h integrations.HealthStatus) string {
	switch h.Message {
	case "Online", "not configured":
		return ""
	}
	return security.RedactErr(errors.New(h.Message))
}

func (h *Handlers) databaseStatus(r *http.Request) string {
	if err := h.db.PingContext(r.Context()); err != nil {
		return "Degraded"
	}
	return "Healthy"
}

func (h *Handlers) configWarnings() []string {
	w := make([]string, 0, 4)
	w = appendMissingSettings(w,
		configField{Name: "APP_BASE_URL", Value: h.cfg.AppBaseURL},
		configField{Name: "MEDIA_SERVER_URL", Value: h.cfg.MediaServerURL},
		configField{Name: "MEDIA_SERVER_PUBLIC_URL", Value: h.cfg.MediaServerPublicURL},
		configField{Name: "MEDIA_SERVER_API_KEY", Value: h.cfg.MediaServerAPIKey},
	)
	w = appendPartialConnectorWarnings(w, "Seerr",
		configField{Name: "SEERR_URL", Value: h.cfg.SeerrURL},
		configField{Name: "SEERR_PUBLIC_URL", Value: h.cfg.SeerrPublicURL},
		configField{Name: "SEERR_API_KEY", Value: h.cfg.SeerrAPIKey},
	)
	w = appendPartialConnectorWarnings(w, "Sonarr",
		configField{Name: "SONARR_URL", Value: h.cfg.SonarrURL},
		configField{Name: "SONARR_API_KEY", Value: h.cfg.SonarrAPIKey},
	)
	w = appendPartialConnectorWarnings(w, "Radarr",
		configField{Name: "RADARR_URL", Value: h.cfg.RadarrURL},
		configField{Name: "RADARR_API_KEY", Value: h.cfg.RadarrAPIKey},
	)
	w = appendPartialConnectorWarnings(w, "Prowlarr",
		configField{Name: "PROWLARR_URL", Value: h.cfg.ProwlarrURL},
		configField{Name: "PROWLARR_API_KEY", Value: h.cfg.ProwlarrAPIKey},
	)
	if !h.cfg.CookieSecure {
		w = append(w, "COOKIE_SECURE is disabled")
	}
	return w
}

type configField struct {
	Name  string
	Value string
}

func appendMissingSettings(warnings []string, fields ...configField) []string {
	for _, field := range fields {
		if field.Value == "" {
			warnings = append(warnings, field.Name+" is not configured")
		}
	}
	return warnings
}

func appendPartialConnectorWarnings(warnings []string, connector string, fields ...configField) []string {
	hasAny := false
	for _, field := range fields {
		if field.Value != "" {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return warnings
	}
	for _, field := range fields {
		if field.Value == "" {
			warnings = append(warnings, connector+" is partially configured: "+field.Name+" is missing")
		}
	}
	return warnings
}

func (h *Handlers) serviceErrorHistory(r *http.Request, service string, limit int) []string {
	logs, err := store.ListRecentAuditLogsByActionTarget(r.Context(), h.db, "service.health.failure", service, limit)
	if err != nil || len(logs) == 0 {
		return nil
	}
	out := make([]string, 0, len(logs))
	for _, row := range logs {
		msg := strings.TrimSpace(row.Metadata.String)
		if msg == "" {
			msg = "unknown error"
		}
		out = append(out, row.CreatedAt.Format(time.RFC3339)+" - "+security.RedactErr(errors.New(msg)))
	}
	return out
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateSettingsInput(pairs map[string]string) string {
	if strings.TrimSpace(pairs[settingAppName]) == "" {
		return "App name is required"
	}
	if len(pairs[settingAppName]) > 60 {
		return "App name must be 60 characters or fewer"
	}
	if pairs[settingAppLogoURL] != "" && !isValidURL(pairs[settingAppLogoURL]) {
		return "Logo URL must be a valid http or https URL"
	}
	if !hexColorRe.MatchString(pairs[settingAppAccentColor]) {
		return "Accent color must be a valid hex color like #d43f24"
	}
	if !isValidURL(pairs[settingMediaServerPublicURL]) {
		return "Media server public URL must be a valid http or https URL"
	}
	if !isValidURL(pairs[settingSeerrPublicURL]) {
		return "Seerr public URL must be a valid http or https URL"
	}
	if len(pairs[settingDashboardMessage]) > 280 {
		return "Dashboard message must be 280 characters or fewer"
	}
	return ""
}

func isValidURL(s string) bool {
	return config.ValidateHTTPURL(s, false) == nil
}

func formatSessionTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

func formatQuotaValue(q seerr.Quota) string {
	if q.MovieLimit > 0 || q.SeriesLimit > 0 || q.MovieUnlimited || q.SeriesUnlimited {
		movie := "unlimited"
		series := "unlimited"
		if !q.MovieUnlimited && q.MovieLimit > 0 {
			movie = fmt.Sprintf("%d/%d remaining", q.MovieRemaining, q.MovieLimit)
		}
		if !q.SeriesUnlimited && q.SeriesLimit > 0 {
			series = fmt.Sprintf("%d/%d remaining", q.SeriesRemaining, q.SeriesLimit)
		}
		return fmt.Sprintf("Movies: %s · Series: %s", movie, series)
	}
	if q.Unlimited {
		return "Requests unlimited"
	}
	return fmt.Sprintf("Requests remaining: %d/%d", q.Remaining, q.Limit)
}

type quotaDisplay struct {
	Value  string
	Meters []QuotaMeter
}

func quotaMeters(q seerr.Quota) []QuotaMeter {
	meters := make([]QuotaMeter, 0, 2)
	if q.MovieUnlimited {
		meters = append(meters, QuotaMeter{Label: "Movies", Value: "Unlimited", Detail: "No limit", Class: "ok", Percent: 100})
	} else if q.MovieLimit > 0 {
		meters = append(meters, quotaMeter("Movies", q.MovieRemaining, q.MovieLimit, q.MovieUsed))
	}

	if q.SeriesUnlimited {
		meters = append(meters, QuotaMeter{Label: "Series", Value: "Unlimited", Detail: "No limit", Class: "ok", Percent: 100})
	} else if q.SeriesLimit > 0 {
		meters = append(meters, quotaMeter("Series", q.SeriesRemaining, q.SeriesLimit, q.SeriesUsed))
	}
	return meters
}

func quotaMeter(label string, remaining, limit, used int) QuotaMeter {
	meter := QuotaMeter{
		Label:   label,
		Value:   fmt.Sprintf("%d/%d left", remaining, limit),
		Detail:  fmt.Sprintf("%d used", used),
		Class:   "ok",
		Percent: quotaRemainingPercent(remaining, limit),
	}
	if remaining == 0 {
		meter.Class = "bad"
	} else if remaining <= 2 {
		meter.Class = "warn"
	}
	return meter
}

func quotaRemainingPercent(remaining, limit int) int {
	if limit <= 0 || remaining <= 0 {
		return 0
	}
	if remaining >= limit {
		return 100
	}
	return remaining * 100 / limit
}

func quotaViewValues(q seerr.Quota) quotaDisplay {
	return quotaDisplay{
		Value:  formatQuotaValue(q),
		Meters: quotaMeters(q),
	}
}
