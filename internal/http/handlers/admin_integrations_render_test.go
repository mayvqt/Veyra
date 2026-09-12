package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/integrations/mediaserver"
)

func TestAdminIntegrationsRendersUsefulStatusCards(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/integrations", nil)
	h.AdminIntegrations(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, s := range []string{"Jellyfin", "Seerr", "Sonarr", "Radarr", "Prowlarr", "Configured services"} {
		if !strings.Contains(body, s) {
			t.Fatalf("expected integrations page to contain %q", s)
		}
	}
	if strings.Contains(body, "Page rendered") || strings.Contains(body, "Status notes") || strings.Contains(body, "Recent Failures") {
		t.Fatal("healthy integrations should not render placeholder status notes")
	}
	if !strings.Contains(body, `class="status-dot integration-health`) {
		t.Fatal("expected integrations page to render health as status dots")
	}
	if strings.Contains(body, "integration-pill") {
		t.Fatal("expected integrations page not to render old health pills")
	}
}

func TestAdminIntegrationsHidesEmptyUnconfiguredSection(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.SeerrURL = "seerr"
	h.cfg.SeerrPublicURL = "https://seerr.example"
	h.cfg.SeerrAPIKey = "key"
	h.cfg.SonarrURL = "sonarr"
	h.cfg.SonarrAPIKey = "key"
	h.cfg.RadarrURL = "radarr"
	h.cfg.RadarrAPIKey = "key"
	h.cfg.ProwlarrURL = "prowlarr"
	h.cfg.ProwlarrAPIKey = "key"

	w := httptest.NewRecorder()
	h.AdminIntegrations(w, httptest.NewRequest(http.MethodGet, "/admin/integrations", nil))
	if strings.Contains(w.Body.String(), ">Not configured<") {
		t.Fatal("not configured section should be omitted when every service is configured")
	}
}

func TestAdminIntegrationsRendersUnconfiguredOptionalConnectorsCompactly(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.SonarrURL = ""
	h.cfg.SonarrAPIKey = ""
	h.sonarr = nil

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/integrations", nil)
	h.AdminIntegrations(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"Not configured", "Optional connector not configured", "integration-neutral"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected integrations page to contain %q", want)
		}
	}
	if strings.Contains(body, `aria-label="Sonarr Offline"`) {
		t.Fatal("unconfigured optional connector should not render as offline")
	}
}

func TestAdminIntegrationsDoesNotRenderActivePlaybackDetails(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	cacheSetJSON(context.Background(), db, h.cacheKey("admin:summary:media_server"), mediaserver.AdminSummary{
		ServerName:      "Library",
		Version:         "10.9.0",
		OperatingSystem: "Linux",
		ActivePlaybacks: []mediaserver.PlaybackSession{
			{User: "Angel", Title: "Heat", Client: "Jellyfin Web", DeviceName: "Firefox", MediaType: "Movie"},
		},
	}, time.Minute)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/integrations", nil)
	h.AdminIntegrations(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, s := range []string{"Heat", "Jellyfin Web on Firefox"} {
		if strings.Contains(body, s) {
			t.Fatalf("expected integrations page not to contain playback detail %q", s)
		}
	}
	if !strings.Contains(body, `href="/admin/playback">Playback`) {
		t.Fatal("expected integrations nav to link playback page")
	}
}

func TestAdminPlaybackRendersJellyfinActivePlayback(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	cacheSetJSON(context.Background(), db, h.cacheKey("admin:playback"), []mediaserver.PlaybackSession{
		{User: "Angel", Title: "Heat", Client: "Jellyfin Web", DeviceName: "Firefox", MediaType: "Movie"},
	}, time.Minute)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/playback", nil)
	h.AdminPlayback(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, s := range []string{"Active playback", "Heat", "Angel", "Jellyfin Web on Firefox"} {
		if !strings.Contains(body, s) {
			t.Fatalf("expected playback page to contain %q", s)
		}
	}
}
