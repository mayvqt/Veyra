package handlers

import "fmt"

const (
	adminServiceOnline        = "Online"
	adminServiceOffline       = "Offline"
	adminServiceNotConfigured = "Not configured"
)

type serviceHealthSummary struct {
	Name       string
	Status     string
	Configured bool
	Required   bool
}

type adminOperationalWarning struct {
	Title  string
	Detail string
	Href   string
}

func adminActionItems(databaseStatus string, services []serviceHealthSummary, configWarnings []string, warningLogCount int, operationalWarnings []adminOperationalWarning) []AdminActionItem {
	items := []AdminActionItem{}
	if databaseStatus != "Healthy" {
		items = append(items, AdminActionItem{
			Title:    "Database needs attention",
			Detail:   "The app could not confirm database health.",
			Href:     "/admin/logs",
			Severity: "warn",
		})
	}
	for _, service := range services {
		if service.Status == adminServiceOffline {
			items = append(items, AdminActionItem{
				Title:    service.Name + " is offline",
				Detail:   serviceActionDetail(service),
				Href:     "/admin/integrations",
				Severity: "warn",
			})
		}
	}
	for _, warning := range operationalWarnings {
		items = append(items, AdminActionItem{
			Title:    warning.Title,
			Detail:   warning.Detail,
			Href:     withDefault(warning.Href, "/admin/integrations"),
			Severity: "warn",
		})
	}
	if len(configWarnings) > 0 {
		items = append(items, AdminActionItem{
			Title:    "Configuration warnings",
			Detail:   fmt.Sprintf("%d setting(s) should be reviewed.", len(configWarnings)),
			Href:     "/admin/settings",
			Severity: "warn",
		})
	}
	if warningLogCount > 0 {
		items = append(items, AdminActionItem{
			Title:    "Recent warning events",
			Detail:   fmt.Sprintf("%d warning event(s) in recent activity.", warningLogCount),
			Href:     "/admin/logs",
			Severity: "warn",
		})
	}
	if len(items) == 0 {
		items = append(items, AdminActionItem{
			Title:    "All clear",
			Detail:   "Core services, database health, and recent activity look normal.",
			Href:     "/admin/integrations",
			Severity: "ok",
		})
	}
	return items
}

func adminOverviewSummary(view ViewData, services []serviceHealthSummary) AdminOverviewSummary {
	summary := AdminOverviewSummary{
		TotalConnectors:     len(services),
		AdminShare:          adminShare(view.AdminUsers, view.TotalUsers),
		LatestActivityAt:    "No recent activity",
		HealthSummary:       "All monitored services are online",
		ConfigurationStatus: "No warnings detected",
	}
	for _, service := range services {
		if service.Configured {
			summary.TotalServices++
			summary.ConfiguredServices++
		}
		if service.Status == adminServiceOnline {
			summary.OnlineServices++
		}
	}
	if summary.TotalServices == 0 {
		summary.HealthSummary = "No configured services to monitor"
	} else if summary.OnlineServices != summary.TotalServices {
		summary.HealthSummary = fmt.Sprintf("%d of %d monitored services are online", summary.OnlineServices, summary.TotalServices)
	}
	if len(view.ConfigWarnings) > 0 {
		summary.ConfigurationStatus = fmt.Sprintf("%d setting(s) need review", len(view.ConfigWarnings))
	}
	if len(view.AuditLogs) > 0 {
		summary.LatestActivityAt = view.AuditLogs[0].CreatedAt
	}
	return summary
}

func adminStatusRows(services []serviceHealthSummary, databaseStatus, appVersion string) []AdminStatusRow {
	rows := make([]AdminStatusRow, 0, len(services)+2)
	for _, service := range services {
		rows = append(rows, AdminStatusRow{
			Label:    service.Name,
			Value:    service.Status,
			Severity: serviceStatusSeverity(service),
		})
	}
	rows = append(rows,
		AdminStatusRow{Label: "Database", Value: databaseStatus, Severity: statusSeverity(databaseStatus == "Healthy")},
		AdminStatusRow{Label: "Version", Value: appVersion},
	)
	return rows
}

func serviceHealth(name string, required, configured, online bool) serviceHealthSummary {
	return serviceHealthSummary{Name: name, Status: integrationStatus(configured, online), Required: required, Configured: configured}
}

func integrationHealthStatus(configured, online bool) string {
	return integrationStatus(configured, online)
}

func integrationStatus(configured, online bool) string {
	if !configured {
		return adminServiceNotConfigured
	}
	if online {
		return adminServiceOnline
	}
	return adminServiceOffline
}

func serviceActionDetail(service serviceHealthSummary) string {
	if service.Required {
		return "Check service health, URL, and credentials."
	}
	return "This optional connector is configured but not reachable. Check URL and API credentials."
}

func serviceStatusSeverity(service serviceHealthSummary) string {
	switch service.Status {
	case adminServiceOnline:
		return "ok"
	case adminServiceOffline:
		return "bad"
	default:
		return "neutral"
	}
}

func statusSeverity(ok bool) string {
	if ok {
		return "ok"
	}
	return "bad"
}

func adminShare(admins, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", admins*100/total)
}
