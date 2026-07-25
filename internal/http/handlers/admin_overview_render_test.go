package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/store"
)

func TestAdminOverviewRendersRecentLogs(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	_ = store.InsertAuditLog(context.Background(), db, nil, "service.health.failure", "Seerr", "timeout", "1.1.1.1")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	h.Admin(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Recent Activity") || !strings.Contains(body, "Service health check failed") {
		t.Fatal("expected recent activity section with inserted log")
	}
}

func TestAdminOverviewRendersActionCenter(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg = config.Config{AppName: "Veyra", CookieSecure: false}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin", nil)
	h.Admin(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"Action Center", "Service Health", "Connectors Ready", "Configuration Posture", "Review", "Standard Users"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected overview to contain %q", want)
		}
	}
	if !strings.Contains(body, `class="status-dot `) {
		t.Fatal("expected overview service status rows to render health dots")
	}
	if strings.Contains(body, "Quick Actions") {
		t.Fatal("overview should not render quick actions")
	}
}

func TestAdminUsersRendersSummaryAndEmptyState(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	h.AdminUsers(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"User Summary", "Standard Users", `class="panel users-panel"`, "No Jellyfin users have signed in yet."} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected users page to contain %q", want)
		}
	}
}
