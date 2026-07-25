package handlers

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	"github.com/mayvqt/veyra/internal/config"
)

func TestConfigWarnings(t *testing.T) {
	h := &Handlers{cfg: config.Config{CookieSecure: false}}
	warnings := h.configWarnings()
	if len(warnings) < 5 {
		t.Fatalf("expected multiple warnings, got %d", len(warnings))
	}
}

func TestConfigWarningsNoWarningsWhenConfigured(t *testing.T) {
	h := &Handlers{cfg: config.Config{
		AppBaseURL:           "https://arr.example",
		MediaServerURL:       "http://mediaserver:8096",
		MediaServerPublicURL: "https://jf.example",
		MediaServerAPIKey:    "media-key",
		SeerrURL:             "http://seerr:5055",
		SeerrPublicURL:       "https://se.example",
		SeerrAPIKey:          "k",
		SonarrURL:            "http://sonarr:8989",
		SonarrAPIKey:         "k",
		RadarrURL:            "http://radarr:7878",
		RadarrAPIKey:         "k",
		ProwlarrURL:          "http://prowlarr:9696",
		ProwlarrAPIKey:       "k",
		CookieSecure:         true,
	}}
	warnings := h.configWarnings()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestConfigWarningsIgnoreAbsentOptionalConnectors(t *testing.T) {
	h := &Handlers{cfg: config.Config{
		AppBaseURL:           "https://arr.example",
		MediaServerURL:       "http://mediaserver:8096",
		MediaServerPublicURL: "https://jf.example",
		MediaServerAPIKey:    "media-key",
		CookieSecure:         true,
	}}
	warnings := h.configWarnings()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for absent optional connectors, got %v", warnings)
	}
}

func TestConfigWarningsReportPartialOptionalConnectors(t *testing.T) {
	h := &Handlers{cfg: config.Config{
		AppBaseURL:           "https://arr.example",
		MediaServerURL:       "http://mediaserver:8096",
		MediaServerPublicURL: "https://jf.example",
		MediaServerAPIKey:    "media-key",
		SeerrURL:             "http://seerr:5055",
		SonarrAPIKey:         "sonarr-key",
		CookieSecure:         true,
	}}
	warnings := h.configWarnings()
	want := []string{
		"Seerr is partially configured: SEERR_PUBLIC_URL is missing",
		"Seerr is partially configured: SEERR_API_KEY is missing",
		"Sonarr is partially configured: SONARR_URL is missing",
	}
	if len(warnings) != len(want) {
		t.Fatalf("expected %d warnings, got %d: %v", len(want), len(warnings), warnings)
	}
	for i := range want {
		if warnings[i] != want[i] {
			t.Fatalf("warning %d: want %q got %q", i, want[i], warnings[i])
		}
	}
}

func TestDatabaseStatus(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	h := &Handlers{db: db}
	r := httptest.NewRequest("GET", "/", nil)
	if got := h.databaseStatus(r); got != "Healthy" {
		t.Fatalf("expected healthy got %s", got)
	}
	_ = db.Close()
	if got := h.databaseStatus(r); got != "Degraded" {
		t.Fatalf("expected degraded got %s", got)
	}
}

func TestAdminActionItemsPrioritizeOperationalWarnings(t *testing.T) {
	items := adminActionItems("Healthy", []serviceHealthSummary{
		{Name: "Jellyfin", Status: adminServiceOnline, Configured: true, Required: true},
		{Name: "Seerr", Status: adminServiceOffline, Configured: true},
		{Name: "Sonarr", Status: adminServiceNotConfigured},
	}, []string{"COOKIE_SECURE is disabled"}, 2, []adminOperationalWarning{{Title: "Sonarr reports health warnings", Detail: "1 upstream warning needs review.", Href: "/admin/integrations"}})

	wantTitles := []string{"Seerr is offline", "Sonarr reports health warnings", "Configuration warnings", "Recent warning events"}
	if len(items) != len(wantTitles) {
		t.Fatalf("expected %d action items, got %d: %+v", len(wantTitles), len(items), items)
	}
	for i, want := range wantTitles {
		if items[i].Title != want {
			t.Fatalf("item %d title: want %q got %q", i, want, items[i].Title)
		}
		if items[i].Severity != "warn" {
			t.Fatalf("item %d should be warn, got %q", i, items[i].Severity)
		}
	}
}

func TestAdminActionItemsShowsAllClearWhenHealthy(t *testing.T) {
	items := adminActionItems("Healthy", []serviceHealthSummary{
		{Name: "Jellyfin", Status: adminServiceOnline, Configured: true, Required: true},
		{Name: "Seerr", Status: adminServiceNotConfigured},
	}, nil, 0, nil)
	if len(items) != 1 {
		t.Fatalf("expected one all-clear item, got %d", len(items))
	}
	if items[0].Title != "All clear" || items[0].Severity != "ok" {
		t.Fatalf("unexpected all-clear item: %+v", items[0])
	}
}

func TestAdminOverviewSummaryCountsOperationalSignals(t *testing.T) {
	view := ViewData{
		TotalUsers:       4,
		AdminUsers:       1,
		ConfigWarnings:   []string{"COOKIE_SECURE is disabled"},
		RecentLoginCount: 2,
		WarningLogCount:  1,
		AuditLogs:        []AuditLogView{{CreatedAt: "2026-06-07T01:02:03Z"}},
	}
	summary := adminOverviewSummary(view, []serviceHealthSummary{
		{Name: "Jellyfin", Status: adminServiceOnline, Configured: true, Required: true},
		{Name: "Seerr", Status: adminServiceOffline, Configured: true},
		{Name: "Sonarr", Status: adminServiceNotConfigured, Configured: false},
	})

	if summary.OnlineServices != 1 || summary.TotalServices != 2 {
		t.Fatalf("unexpected health counts: %+v", summary)
	}
	if summary.ConfiguredServices != 2 || summary.TotalConnectors != 3 {
		t.Fatalf("unexpected connector counts: %+v", summary)
	}
	if summary.AdminShare != "25%" {
		t.Fatalf("expected 25%% admin share, got %q", summary.AdminShare)
	}
	if summary.LatestActivityAt != "2026-06-07T01:02:03Z" {
		t.Fatalf("unexpected latest activity: %q", summary.LatestActivityAt)
	}
	if summary.ConfigurationStatus != "1 setting(s) need review" {
		t.Fatalf("unexpected config status: %q", summary.ConfigurationStatus)
	}
}

func TestAdminOverviewSummaryHandlesNoUsersOrActivity(t *testing.T) {
	summary := adminOverviewSummary(ViewData{}, []serviceHealthSummary{
		{Name: "Jellyfin", Status: adminServiceOnline, Configured: true, Required: true},
		{Name: "Seerr", Status: adminServiceNotConfigured},
	})

	if summary.AdminShare != "0%" {
		t.Fatalf("expected empty admin share, got %q", summary.AdminShare)
	}
	if summary.LatestActivityAt != "No recent activity" {
		t.Fatalf("unexpected latest activity: %q", summary.LatestActivityAt)
	}
	if summary.HealthSummary != "All monitored services are online" {
		t.Fatalf("unexpected health summary: %q", summary.HealthSummary)
	}
	if summary.ConfigurationStatus != "No warnings detected" {
		t.Fatalf("unexpected config status: %q", summary.ConfigurationStatus)
	}
}

func TestAdminStatusRowsIncludeServicesDatabaseAndVersion(t *testing.T) {
	rows := adminStatusRows([]serviceHealthSummary{
		{Name: "Jellyfin", Status: adminServiceOnline, Configured: true, Required: true},
		{Name: "Seerr", Status: adminServiceOffline, Configured: true},
		{Name: "Sonarr", Status: adminServiceNotConfigured},
	}, "Healthy", "dev")

	want := []AdminStatusRow{
		{Label: "Jellyfin", Value: "Online", Severity: "ok"},
		{Label: "Seerr", Value: "Offline", Severity: "bad"},
		{Label: "Sonarr", Value: "Not configured", Severity: "neutral"},
		{Label: "Database", Value: "Healthy", Severity: "ok"},
		{Label: "Version", Value: "dev"},
	}
	if len(rows) != len(want) {
		t.Fatalf("expected %d rows, got %d: %+v", len(want), len(rows), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row %d: want %+v got %+v", i, want[i], rows[i])
		}
	}
}
