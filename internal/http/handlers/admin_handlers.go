package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/buildinfo"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/integrations/arr"
	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
	"github.com/mayvqt/veyra/internal/store"
)

func (h *Handlers) Admin(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	mediaName := h.cfg.MediaServerType.Label()
	mediaStat := h.cachedHealth(r, "admin:health:media_server", h.mediaserver)
	sStat := h.cachedHealth(r, "admin:health:seerr", h.seerr)
	arrServices := h.arrAdminServices()
	arrHealth := h.arrHealthStatuses(r, arrServices)
	mediaStatus := boolToStatus(mediaStat.OK)
	sStatus := boolToStatus(sStat.OK)
	view := ViewData{
		AppName:           h.appName(r),
		CSRFToken:         middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
		Now:               time.Now(),
		User:              u,
		MediaServerName:   mediaName,
		MediaServerStatus: mediaStatus,
		SeerrStatus:       sStatus,
		SonarrStatus:      boolToStatus(arrHealth["Sonarr"].OK),
		RadarrStatus:      boolToStatus(arrHealth["Radarr"].OK),
		ProwlarrStatus:    boolToStatus(arrHealth["Prowlarr"].OK),
		AppVersion:        buildinfo.Version,
		DatabaseStatus:    h.databaseStatus(r),
		ConfigWarnings:    h.configWarnings(),
	}
	if counts, err := store.CountUsers(r.Context(), h.db); err == nil {
		view.TotalUsers = counts.Total
		view.AdminUsers = counts.Admin
		view.StandardUsers = view.TotalUsers - view.AdminUsers
	}
	if rows, err := store.ListRecentAuditLogs(r.Context(), h.db, 8); err == nil {
		view.AuditLogs = auditLogViews(rows)
		stats := summarizeAuditLogs(rows)
		view.RecentLoginCount = stats.RecentLoginCount
		view.WarningLogCount = stats.WarningLogCount
	}
	services := []serviceHealthSummary{
		serviceHealth(mediaName, true, h.mediaServerConfigured(), mediaStat.OK),
		serviceHealth("Seerr", false, h.seerrConfigured(), sStat.OK),
	}
	for _, svc := range arrServices {
		services = append(services, serviceHealth(svc.Name, false, svc.Configured, arrHealth[svc.Name].OK))
	}
	view.AdminOverviewSummary = adminOverviewSummary(view, services)
	view.AdminStatusRows = adminStatusRows(services, view.DatabaseStatus, view.AppVersion)
	view.AdminActionItems = adminActionItems(view.DatabaseStatus, services, view.ConfigWarnings, view.WarningLogCount, h.adminOperationalWarnings(r, arrServices))
	h.render(w, "admin_overview.html", view)
}

func (h *Handlers) AdminUsers(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	rows, _ := store.ListUsers(r.Context(), h.db, 200)
	view := ViewData{AppName: h.appName(r), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), User: u, MediaServerName: h.cfg.MediaServerType.Label()}
	view.TotalUsers = len(rows)
	view.Users = make([]AdminUserView, 0, len(rows))
	for _, row := range rows {
		if row.IsAdmin {
			view.AdminUsers++
		}
		lastLogin := "-"
		if row.LastLoginAt.Valid {
			lastLogin = row.LastLoginAt.Time.Format(time.RFC3339)
		}
		view.Users = append(view.Users, AdminUserView{Username: row.Username, DisplayName: row.DisplayName, MediaServerUserID: row.MediaServerUserID, IsAdmin: row.IsAdmin, CreatedAt: row.CreatedAt.Format(time.RFC3339), LastLoginAt: lastLogin})
	}
	view.StandardUsers = view.TotalUsers - view.AdminUsers
	h.render(w, "admin_users.html", view)
}

func (h *Handlers) AdminLogs(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	rows, _ := store.ListRecentAuditLogs(r.Context(), h.db, 200)
	view := ViewData{AppName: h.appName(r), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), User: u}
	view.AuditLogs = auditLogViews(rows)
	h.render(w, "admin_logs.html", view)
}

func (h *Handlers) AdminPlayback(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	view := ViewData{
		AppName:         h.appName(r),
		CSRFToken:       middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure),
		Now:             time.Now(),
		User:            u,
		MediaServerName: h.cfg.MediaServerType.Label(),
	}
	if summary, ok := h.cachedMediaServerAdminSummary(r); ok {
		view.Playback = playbackSessionViews(summary.ActivePlaybacks)
	}
	h.render(w, "admin_playback.html", view)
}

func (h *Handlers) AdminIntegrations(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	now := time.Now().UTC()
	settings := readSettings(r, h.db, settingAppName, settingMediaServerPublicURL, settingSeerrPublicURL)
	j := h.cachedHealth(r, "admin:health:media_server", h.mediaserver)
	s := h.cachedHealth(r, "admin:health:seerr", h.seerr)
	seerrConfigured := h.seerrConfigured()
	arrServices := h.arrAdminServices()
	arrHealth := h.arrHealthStatuses(r, arrServices)

	view := ViewData{AppName: appNameFromSettings(settings, h.cfg.AppName), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), User: u, MediaServerName: h.cfg.MediaServerType.Label()}
	view.ServiceStatuses = []ServiceStatus{
		h.mediaserverServiceStatus(r, now, withDefault(readSettingFromMap(settings, settingMediaServerPublicURL), h.cfg.MediaServerPublicURL), j),
		{Name: "Seerr", Internal: h.cfg.SeerrURL, Public: withDefault(readSettingFromMap(settings, settingSeerrPublicURL), h.cfg.SeerrPublicURL), Health: integrationHealthStatus(seerrConfigured, s.OK), Configured: seerrConfigured, LastChecked: now.Format(time.RFC3339), LastError: healthErr(s), RecentErrors: combineIntegrationNotes("Configured: "+boolYesNo(seerrConfigured), h.serviceErrorHistory(r, "Seerr", 3))},
	}
	for _, svc := range arrServices {
		view.ServiceStatuses = append(view.ServiceStatuses, h.arrServiceStatus(r, now, svc, arrHealth[svc.Name]))
	}
	h.render(w, "admin_integrations.html", view)
}

type arrAdminService struct {
	Name       string
	Internal   string
	APIKey     string
	CacheKey   string
	Client     *arr.Client
	Configured bool
}

func (h *Handlers) arrAdminServices() []arrAdminService {
	return []arrAdminService{
		{Name: "Sonarr", Internal: h.cfg.SonarrURL, APIKey: h.cfg.SonarrAPIKey, CacheKey: "admin:health:sonarr", Client: h.sonarr, Configured: h.cfg.SonarrURL != "" && h.cfg.SonarrAPIKey != ""},
		{Name: "Radarr", Internal: h.cfg.RadarrURL, APIKey: h.cfg.RadarrAPIKey, CacheKey: "admin:health:radarr", Client: h.radarr, Configured: h.cfg.RadarrURL != "" && h.cfg.RadarrAPIKey != ""},
		{Name: "Prowlarr", Internal: h.cfg.ProwlarrURL, APIKey: h.cfg.ProwlarrAPIKey, CacheKey: "admin:health:prowlarr", Client: h.prowlarr, Configured: h.cfg.ProwlarrURL != "" && h.cfg.ProwlarrAPIKey != ""},
	}
}

func (h *Handlers) mediaServerConfigured() bool {
	return h.cfg.MediaServerURL != "" && h.cfg.MediaServerPublicURL != "" && h.cfg.MediaServerAPIKey != ""
}

func (h *Handlers) seerrConfigured() bool {
	return h.cfg.SeerrURL != "" && h.cfg.SeerrPublicURL != "" && h.cfg.SeerrAPIKey != ""
}

func (h *Handlers) arrHealthStatuses(r *http.Request, services []arrAdminService) map[string]integrations.HealthStatus {
	out := make(map[string]integrations.HealthStatus, len(services))
	for _, svc := range services {
		out[svc.Name] = h.cachedHealthIfConfigured(r, svc)
	}
	return out
}

func (h *Handlers) cachedHealthIfConfigured(r *http.Request, svc arrAdminService) integrations.HealthStatus {
	if !svc.Configured || svc.Client == nil {
		return integrations.HealthStatus{OK: false, Message: "not configured"}
	}
	return h.cachedHealth(r, svc.CacheKey, svc.Client)
}

func (h *Handlers) mediaserverServiceStatus(r *http.Request, now time.Time, publicURL string, health integrations.HealthStatus) ServiceStatus {
	name := h.cfg.MediaServerType.Label()
	configured := h.mediaServerConfigured()
	status := ServiceStatus{
		Name:         name,
		Internal:     h.cfg.MediaServerURL,
		Public:       publicURL,
		Health:       integrationHealthStatus(configured, health.OK),
		Configured:   configured,
		Required:     true,
		LastChecked:  now.Format(time.RFC3339),
		LastError:    healthErr(health),
		RecentErrors: combineIntegrationNotes("Configured: "+boolYesNo(configured), h.serviceErrorHistory(r, name, 3)),
	}
	summary, ok := h.cachedMediaServerAdminSummary(r)
	if !ok {
		return status
	}
	status.Version = summary.Version
	status.Runtime = summary.OperatingSystem
	status.Stats = []AdminStat{
		{Label: "Server", Value: withDefault(summary.ServerName, name)},
		{Label: "Users", Value: fmt.Sprintf("%d", summary.UserCount)},
		{Label: "Libraries", Value: fmt.Sprintf("%d", summary.LibraryCount)},
		{Label: "Movies", Value: fmt.Sprintf("%d", summary.MovieCount)},
		{Label: "Series", Value: fmt.Sprintf("%d", summary.SeriesCount)},
		{Label: "Episodes", Value: fmt.Sprintf("%d", summary.EpisodeCount)},
		{Label: "Active Playback", Value: fmt.Sprintf("%d", len(summary.ActivePlaybacks))},
	}
	status.Issues = append(status.Issues, summary.Warnings...)
	return status
}

func (h *Handlers) arrServiceStatus(r *http.Request, now time.Time, svc arrAdminService, health integrations.HealthStatus) ServiceStatus {
	status := ServiceStatus{
		Name:         svc.Name,
		Internal:     svc.Internal,
		Health:       integrationHealthStatus(svc.Configured, health.OK),
		Configured:   svc.Configured,
		Required:     false,
		LastChecked:  now.Format(time.RFC3339),
		LastError:    healthErr(health),
		RecentErrors: combineIntegrationNotes("Configured: "+boolYesNo(svc.Configured), h.serviceErrorHistory(r, svc.Name, 3)),
	}
	summary, ok := h.cachedArrAdminSummary(r, svc)
	if !ok {
		return status
	}
	status.Version = summary.Version
	status.Runtime = summary.Runtime
	status.Stats = []AdminStat{
		{Label: "App", Value: withDefault(summary.AppName, svc.Name)},
		{Label: "Branch", Value: withDefault(summary.Branch, "-")},
	}
	status.Issues = healthIssueMessages(summary.HealthIssues)
	status.DiskWarnings = summary.DiskWarnings
	return status
}

func (h *Handlers) adminOperationalWarnings(r *http.Request, services []arrAdminService) []adminOperationalWarning {
	warnings := make([]adminOperationalWarning, 0)
	for _, svc := range services {
		summary, ok := h.cachedArrAdminSummary(r, svc)
		if !ok {
			continue
		}
		if len(summary.HealthIssues) > 0 {
			warnings = append(warnings, adminOperationalWarning{
				Title:  svc.Name + " reports health warnings",
				Detail: fmt.Sprintf("%d upstream warning(s) need review.", len(summary.HealthIssues)),
				Href:   "/admin/integrations",
			})
		}
		if len(summary.DiskWarnings) > 0 {
			warnings = append(warnings, adminOperationalWarning{
				Title:  svc.Name + " storage is low",
				Detail: summary.DiskWarnings[0],
				Href:   "/admin/integrations",
			})
		}
	}
	return warnings
}

func (h *Handlers) cachedArrAdminSummary(r *http.Request, svc arrAdminService) (arr.AdminSummary, bool) {
	if !svc.Configured || svc.Client == nil {
		return arr.AdminSummary{}, false
	}
	key := "admin:summary:" + strings.ToLower(svc.Name)
	var summary arr.AdminSummary
	if cacheGetJSON(r.Context(), h.db, key, &summary) {
		return summary, true
	}
	summary, err := svc.Client.AdminSummary(r.Context())
	if err != nil {
		return arr.AdminSummary{}, false
	}
	h.cacheSetJSON(r.Context(), key, summary, cacheTTLHealth)
	return summary, true
}

func (h *Handlers) cachedMediaServerAdminSummary(r *http.Request) (mediaserver.AdminSummary, bool) {
	key := "admin:summary:media_server"
	var summary mediaserver.AdminSummary
	if cacheGetJSON(r.Context(), h.db, key, &summary) {
		return summary, true
	}
	summary, err := h.mediaserver.AdminSummary(r.Context())
	if err != nil {
		return mediaserver.AdminSummary{}, false
	}
	h.cacheSetJSON(r.Context(), key, summary, cacheTTLHealth)
	return summary, true
}

func healthIssueMessages(issues []arr.HealthIssue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		msg := strings.TrimSpace(issue.Message)
		if msg == "" {
			msg = strings.TrimSpace(issue.Source)
		}
		if msg == "" {
			msg = "Health warning"
		}
		out = append(out, msg)
	}
	return out
}

func playbackSessionViews(rows []mediaserver.PlaybackSession) []AdminPlaybackSession {
	out := make([]AdminPlaybackSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminPlaybackSession{
			User:       withDefault(row.User, "Unknown user"),
			Title:      withDefault(row.Title, "Unknown title"),
			Client:     row.Client,
			DeviceName: row.DeviceName,
			MediaType:  row.MediaType,
		})
	}
	return out
}
