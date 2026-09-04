package handlers

import (
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/dashboard"
)

func TestRenderTemplateFailureReturnsOnlyServerError(t *testing.T) {
	h := &Handlers{
		tmpl: template.Must(template.New("bad.html").Parse(`{{ define "bad.html" }}{{ .Missing.Field }}{{ end }}`)),
		log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	w := httptest.NewRecorder()
	h.render(w, "bad.html", ViewData{})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Fatalf("expected internal server error body, got %q", body)
	}
	if strings.Contains(body, "Missing") || strings.Contains(body, "Field") {
		t.Fatalf("unexpected template internals leaked in body: %q", body)
	}
}

func TestDashboardTemplateLabelsRequestPanel(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", ViewData{AppName: "Veyra", SeerrStatus: "Online", SettingsShowRequestBot: true, StaticVersion: "test-version"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, ">Request media<") {
		t.Fatal("expected dashboard request panel to be labeled Request media")
	}
	if strings.Contains(body, "Request Bot") {
		t.Fatal("dashboard should not use old Request Bot label")
	}
	if !strings.Contains(body, `/static/dashboard.js?v=test-version`) {
		t.Fatal("expected dashboard script to be versioned")
	}
	if !strings.Contains(body, `class="nav-link active" href="/dashboard" aria-current="page"`) {
		t.Fatal("expected dashboard navigation to identify the current page")
	}
	if !strings.Contains(body, `data-seerr-message role="status" aria-live="polite" aria-atomic="true"`) {
		t.Fatal("expected request status messages to be announced to assistive technology")
	}
	if strings.Contains(body, "Services connected") {
		t.Fatal("dashboard should not claim all services are connected unconditionally")
	}
}

func TestDashboardTemplateMarksRefreshableWidgets(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", ViewData{
		AppName:                 "Veyra",
		SettingsShowRequestBot:  true,
		SettingsShowQuota:       true,
		SettingsShowCalendar:    true,
		SettingsShowRecentMedia: true,
		SettingsShowRecentReqs:  true,
		SettingsShowQueue:       true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, key := range []string{"quota", "calendar", "recent-media", "recent-requests", "queue"} {
		if strings.Count(body, `data-dashboard-refresh="`+key+`"`) != 1 {
			t.Fatalf("expected one refresh marker for %q", key)
		}
	}
	if strings.Contains(body, `data-dashboard-refresh="request-bot"`) || strings.Contains(body, `data-dashboard-refresh="container"`) {
		t.Fatal("request bot and dashboard container must not be refresh widgets")
	}
}

func TestDashboardTemplateDisclosesStaleData(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", ViewData{AppName: "Veyra", DashboardDataStale: true})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Some dashboard data is temporarily cached.") {
		t.Fatal("expected stale data notice")
	}
}

func TestDashboardTemplateRendersServiceLinksInRail(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	view := ViewData{
		AppName: "Veyra",
		DashboardServices: dashboardServices(
			"Jellyfin",
			"https://media.example.com",
			"Online",
			"https://requests.example.com",
			"Offline",
		),
		SettingsShowRequestBot: true,
		StaticVersion:          "test-version",
	}

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", view)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`/static/app.css?v=test-version`,
		`class="rail-links" aria-labelledby="media-services-title"`,
		`id="media-services-title" class="rail-label">Quick Links`,
		`aria-label="Jellyfin Online"`,
		`aria-label="Seerr Offline"`,
		`class="rail-service-link" href="https://media.example.com" target="_blank" rel="noopener noreferrer"`,
		`class="rail-service-link" href="https://requests.example.com" target="_blank" rel="noopener noreferrer"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected dashboard service rail to contain %q", want)
		}
	}
	for _, unwanted := range []string{"Stream your library", "Discover and request"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("expected service rail to omit redundant copy %q", unwanted)
		}
	}
	if strings.Index(body, `class="rail-links"`) > strings.Index(body, `data-seerr-request-bot`) {
		t.Fatal("expected service links to render in the navigation rail")
	}
}

func TestDashboardTemplateHidesServiceLinksWithoutServices(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", ViewData{AppName: "Veyra"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), `class="rail-links"`) {
		t.Fatal("expected service links to be hidden without configured services")
	}
}

func TestDashboardTemplateDoesNotRenderObsoleteServiceSummary(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	view := ViewData{
		AppName:           "Veyra",
		MediaServerStatus: "Online",
		SeerrStatus:       "Offline",
		QuotaValue:        "Movies: 3/5 remaining",
	}

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", view)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, unwanted := range []string{
		`class="dashboard-hero"`,
		`class="dashboard-quick-actions"`,
		`aria-label="Service summary"`,
		`href="/admin/settings">Settings`,
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("dashboard should not render duplicate action markup %q", unwanted)
		}
	}
}

func TestDashboardTemplateCanHideRequestPanel(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", ViewData{AppName: "Veyra", SeerrStatus: "Online", SettingsShowRequestBot: false})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), `data-seerr-request-bot`) {
		t.Fatal("expected request panel to be hidden")
	}
}

func TestAuthTemplatesDoNotRenderNavigation(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	for _, tc := range []struct {
		name     string
		template string
		want     string
	}{
		{name: "login", template: "login.html", want: `class="auth-page login-page"`},
		{name: "setup", template: "setup.html", want: `class="auth-page setup-page"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.render(w, tc.template, ViewData{AppName: "Veyra"})
			if w.Code != http.StatusOK {
				t.Fatalf("want 200 got %d", w.Code)
			}
			body := w.Body.String()
			if !strings.Contains(body, tc.want) {
				t.Fatalf("expected %s to use auth shell class %q", tc.template, tc.want)
			}
			if strings.Contains(body, `class="topbar"`) || strings.Contains(body, `class="topnav"`) {
				t.Fatalf("%s should not render app navigation", tc.template)
			}
			if strings.Contains(body, `class="login-visual-copy"`) || strings.Contains(body, ">Watch<") || strings.Contains(body, ">Request<") || strings.Contains(body, ">Manage<") {
				t.Fatalf("%s should not render fake navigation pills", tc.template)
			}
		})
	}
}

func TestAdminSettingsTemplateUsesSharedPanelLayout(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	view := ViewData{
		AppName: "Veyra",
		SetupConfigGroups: []SetupConfigGroup{
			{Name: "Jellyfin", Fields: []SetupConfigField{{Label: "Internal URL", Name: "setup_media_server_url"}}},
		},
	}

	w := httptest.NewRecorder()
	h.render(w, "admin_settings.html", view)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`class="grid settings-grid"`,
		`class="panel half settings-panel" id="settings-dashboard"`,
		`class="panel half settings-panel" id="settings-widgets"`,
		`class="panel half settings-panel" id="settings-branding"`,
		`class="panel settings-panel" id="settings-connections"`,
		`class="stack-item settings-service-card"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected settings template to contain %q", want)
		}
	}
	for _, unwanted := range []string{`class="settings-nav"`, `class="settings-section-index"`} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("expected settings template to reuse shared components instead of %q", unwanted)
		}
	}
}

func TestDashboardTemplateRendersRequestLifecycle(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	view := ViewData{
		AppName:                "Veyra",
		SettingsShowRecentReqs: true,
		RecentRequests: []dashboard.RequestItem{
			{
				Title:     "Test Movie",
				Status:    "Downloading",
				StatusKey: "downloading",
				Media:     "Movie",
				User:      "admin",
				Lifecycle: []dashboard.RequestLifecycleStep{
					{Label: "Submitted", Key: "submitted", State: "done"},
					{Label: "Approved", Key: "approved", State: "done"},
					{Label: "Queued", Key: "queued", State: "done"},
					{Label: "Downloading", Key: "downloading", State: "current"},
					{Label: "Available", Key: "available", State: "todo"},
				},
			},
		},
	}

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", view)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`class="request-head"`,
		`class="request-progress progress-downloading"`,
		`class="request-lifecycle"`,
		`class="request-lifecycle-step step-downloading is-current">Downloading`,
		`Movie · Requested by admin`,
		`class="pill status-pill status-downloading">Downloading`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected dashboard request lifecycle to contain %q", want)
		}
	}
}

func TestDashboardTemplateCompactsAvailableRequests(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	view := ViewData{
		AppName:                "Veyra",
		SettingsShowRecentReqs: true,
		RecentRequests:         []dashboard.RequestItem{{Title: "Ready Movie", Status: "Available", StatusKey: "available", Media: "Movie"}},
	}

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", view)
	body := w.Body.String()
	if !strings.Contains(body, `class="stack-item request-row request-complete"`) {
		t.Fatal("expected available request to render as a compact completed row")
	}
	if strings.Contains(body, `progress-available`) || strings.Contains(body, `class="request-lifecycle"`) {
		t.Fatal("available request should not repeat completed progress details")
	}
}

func TestDashboardTemplateRendersQuotaMeters(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	view := ViewData{
		AppName:           "Veyra",
		SeerrStatus:       "Online",
		SettingsShowQuota: true,
		QuotaValue:        "Movies: 3/5 remaining · Series: unlimited",
		QuotaMeters: []QuotaMeter{
			{Label: "Movies", Value: "3 remaining", Detail: "2 used of 5", Class: "warn", Percent: 60},
			{Label: "Series", Value: "Unlimited", Detail: "No request limit", Class: "ok", Unlimited: true},
		},
	}

	w := httptest.NewRecorder()
	h.render(w, "dashboard.html", view)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`class="panel quota-panel"`,
		`class="quota-meter quota-meter-warn"`,
		`style="width: 60%"`,
		`class="quota-meter quota-meter-ok"`,
		`No request limit`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected dashboard quota meters to contain %q", want)
		}
	}
	if strings.Contains(body, `style="width: 100%"`) {
		t.Fatal("unlimited quota should not render a misleading progress meter")
	}
}
