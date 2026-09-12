package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/buildinfo"
	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/integrations/arr"
	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func (h *Handlers) Admin(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	mediaName := h.cfg.MediaServerType.Label()
	arrServices := h.arrAdminServices()
	snapshot := h.loadAdminSnapshot(r, arrServices)
	mediaStat, sStat, arrHealth := snapshot.mediaHealth, snapshot.seerrHealth, snapshot.arrHealth
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
	view.AdminActionItems = adminActionItems(view.DatabaseStatus, services, view.ConfigWarnings, view.WarningLogCount, adminOperationalWarnings(arrServices, snapshot.arrSummaries))
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
		lastLoginTime := AdminTimestamp{Label: "-"}
		if row.LastLoginAt.Valid {
			lastLoginTime = adminTimestamp(row.LastLoginAt.Time)
		}
		view.Users = append(view.Users, AdminUserView{Username: row.Username, DisplayName: row.DisplayName, MediaServerUserID: row.MediaServerUserID, MediaServerURL: mediaServerUserURL(h.cfg.MediaServerPublicURL, row.MediaServerUserID), IsAdmin: row.IsAdmin, CreatedAtTime: adminTimestamp(row.CreatedAt), LastLoginTime: lastLoginTime})
	}
	view.StandardUsers = view.TotalUsers - view.AdminUsers
	h.render(w, "admin_users.html", view)
}

func mediaServerUserURL(baseURL, userID string) string {
	if baseURL == "" || userID == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/web/index.html#!/users/user?userId=" + url.QueryEscape(userID)
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
	sessions, err := cacheLoadJSON(h, r.Context(), "admin:playback", cacheTTLHealth, func() ([]mediaserver.PlaybackSession, error) {
		return h.mediaserver.ActivePlayback(r.Context())
	})
	view.PlaybackUnavailable = err != nil
	view.Playback = playbackSessionViews(sessions)
	h.render(w, "admin_playback.html", view)
}

func (h *Handlers) AdminIntegrations(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	settings := readSettings(r, h.db, settingAppName, settingMediaServerPublicURL, settingSeerrPublicURL)
	seerrConfigured := h.seerrConfigured()
	arrServices := h.arrAdminServices()
	snapshot := h.loadAdminSnapshot(r, arrServices)
	j, s, arrHealth := snapshot.mediaHealth, snapshot.seerrHealth, snapshot.arrHealth

	view := ViewData{AppName: appNameFromSettings(settings, h.cfg.AppName), CSRFToken: middleware.EnsureCSRFToken(w, r, h.cfg.CookieSecure), Now: time.Now(), User: u, MediaServerName: h.cfg.MediaServerType.Label()}
	view.ServiceStatuses = []ServiceStatus{
		h.mediaserverServiceStatus(r, withDefault(readSettingFromMap(settings, settingMediaServerPublicURL), h.cfg.MediaServerPublicURL), j, snapshot.mediaSummary),
		{Name: "Seerr", Internal: security.RedactURL(h.cfg.SeerrURL), Public: security.RedactURL(withDefault(readSettingFromMap(settings, settingSeerrPublicURL), h.cfg.SeerrPublicURL)), Health: integrationHealthStatus(seerrConfigured, s.OK), Configured: seerrConfigured, LastError: healthErr(s), RecentErrors: h.serviceErrorHistory(r, "Seerr", 3)},
	}
	for _, svc := range arrServices {
		view.ServiceStatuses = append(view.ServiceStatuses, h.arrServiceStatus(r, svc, arrHealth[svc.Name], snapshot.arrSummaries[svc.Name]))
	}
	for _, service := range view.ServiceStatuses {
		if !service.Configured {
			view.HasUnconfiguredServices = true
			break
		}
	}
	h.render(w, "admin_integrations.html", view)
}

type arrAdminService struct {
	Name       string
	Internal   string
	Client     *arr.Client
	Configured bool
}

func (h *Handlers) arrAdminServices() []arrAdminService {
	return []arrAdminService{
		{Name: "Sonarr", Internal: h.cfg.SonarrURL, Client: h.sonarr, Configured: h.cfg.SonarrURL != "" && h.cfg.SonarrAPIKey != ""},
		{Name: "Radarr", Internal: h.cfg.RadarrURL, Client: h.radarr, Configured: h.cfg.RadarrURL != "" && h.cfg.RadarrAPIKey != ""},
		{Name: "Prowlarr", Internal: h.cfg.ProwlarrURL, Client: h.prowlarr, Configured: h.cfg.ProwlarrURL != "" && h.cfg.ProwlarrAPIKey != ""},
	}
}

func (h *Handlers) mediaServerConfigured() bool {
	return h.cfg.MediaServerURL != "" && h.cfg.MediaServerAPIKey != ""
}

func (h *Handlers) seerrConfigured() bool {
	return h.cfg.SeerrURL != "" && h.cfg.SeerrAPIKey != ""
}

func (h *Handlers) mediaserverServiceStatus(r *http.Request, publicURL string, health integrations.HealthStatus, summary *mediaserver.AdminSummary) ServiceStatus {
	name := h.cfg.MediaServerType.Label()
	configured := h.mediaServerConfigured()
	status := ServiceStatus{
		Name:         name,
		Internal:     security.RedactURL(h.cfg.MediaServerURL),
		Public:       security.RedactURL(publicURL),
		Health:       integrationHealthStatus(configured, health.OK),
		Configured:   configured,
		Required:     true,
		LastError:    healthErr(health),
		RecentErrors: h.serviceErrorHistory(r, name, 3),
	}
	if summary == nil {
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

func (h *Handlers) arrServiceStatus(r *http.Request, svc arrAdminService, health integrations.HealthStatus, summary *arr.AdminSummary) ServiceStatus {
	status := ServiceStatus{
		Name:         svc.Name,
		Internal:     security.RedactURL(svc.Internal),
		Health:       integrationHealthStatus(svc.Configured, health.OK),
		Configured:   svc.Configured,
		Required:     false,
		LastError:    healthErr(health),
		RecentErrors: h.serviceErrorHistory(r, svc.Name, 3),
	}
	if summary == nil {
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

func adminOperationalWarnings(services []arrAdminService, summaries map[string]*arr.AdminSummary) []adminOperationalWarning {
	warnings := make([]adminOperationalWarning, 0)
	for _, svc := range services {
		summary := summaries[svc.Name]
		if summary == nil {
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
	summary, err := cacheLoadJSON(h, r.Context(), key, cacheTTLHealth, func() (arr.AdminSummary, error) {
		return svc.Client.AdminSummary(r.Context())
	})
	if err != nil {
		return arr.AdminSummary{}, false
	}
	return summary, true
}

func (h *Handlers) cachedMediaServerAdminSummary(r *http.Request) (mediaserver.AdminSummary, bool) {
	key := "admin:summary:media_server"
	summary, err := cacheLoadJSON(h, r.Context(), key, cacheTTLHealth, func() (mediaserver.AdminSummary, error) {
		return h.mediaserver.AdminSummary(r.Context())
	})
	if err != nil {
		return mediaserver.AdminSummary{}, false
	}
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

// One bounded load per admin page. The summary's system-info request is also
// its health check, so unavailable services are never probed twice in sequence.
type adminSnapshot struct {
	mediaHealth, seerrHealth integrations.HealthStatus
	mediaSummary             *mediaserver.AdminSummary
	arrHealth                map[string]integrations.HealthStatus
	arrSummaries             map[string]*arr.AdminSummary
}

func (h *Handlers) loadAdminSnapshot(r *http.Request, services []arrAdminService) adminSnapshot {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	out := adminSnapshot{arrHealth: make(map[string]integrations.HealthStatus), arrSummaries: make(map[string]*arr.AdminSummary)}
	summaries := make([]*arr.AdminSummary, len(services))
	tasks := []func(){
		func() {
			summary, ok := h.cachedMediaServerAdminSummary(r)
			out.mediaHealth = integrations.HealthStatus{OK: ok}
			if ok {
				out.mediaSummary = &summary
			} else {
				out.mediaHealth.Message = "Service details unavailable"
			}
		},
		func() { out.seerrHealth = h.cachedHealth(r, "health:seerr", h.seerr) },
	}
	for i, svc := range services {
		tasks = append(tasks, func() {
			if summary, ok := h.cachedArrAdminSummary(r, svc); ok {
				summaries[i] = &summary
			}
		})
	}
	runConcurrently(tasks...)
	for i, svc := range services {
		out.arrSummaries[svc.Name] = summaries[i]
		message := "Service details unavailable"
		if !svc.Configured {
			message = "not configured"
		} else if summaries[i] != nil {
			message = "Online"
		}
		out.arrHealth[svc.Name] = integrations.HealthStatus{OK: summaries[i] != nil, Message: message}
	}
	return out
}
