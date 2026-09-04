package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/store"
)

func TestSetupGetRendersWhenRequired(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.MediaServerAPIKey = "change-me!!"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/setup", nil)
	h.SetupGet(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "Set up Veyra") || !strings.Contains(body, "Prowlarr API key") {
		t.Fatalf("expected setup form, got %s", body)
	}
}

func TestSetupGetDoesNotPrefillSecrets(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.MediaServerAPIKey = "jf-real-secret"
	h.cfg.SeerrAPIKey = "seerr-real-secret"
	h.cfg.ProwlarrAPIKey = "change-me!!"

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/setup", nil)
	h.SetupGet(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
	body := w.Body.String()
	for _, secret := range []string{"jf-real-secret", "seerr-real-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("setup form leaked secret %q", secret)
		}
	}
}

func TestSetupPostRejectsUnsafeURLSchemes(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.MediaServerAPIKey = "change-me!!"

	form := url.Values{}
	form.Set("csrf_token", "token")
	form.Set("app_base_url", "javascript://veyra")
	form.Set("media_server_type", config.MediaServerJellyfin.String())
	form.Set("media_server_url", "http://mediaserver:8096")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "token"})
	h.SetupPost(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want validation render 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "App base URL must be a valid http or https URL.") {
		t.Fatalf("expected unsafe scheme validation error, got %s", w.Body.String())
	}
}

func TestSetupValidationRejectsCredentialsInConnectorURLs(t *testing.T) {
	in := config.SetupInput{
		AppBaseURL:        "https://veyra.home.test",
		MediaServerType:   config.MediaServerJellyfin.String(),
		MediaServerURL:    "http://mediaserver:8096",
		MediaServerAPIKey: "jf-secret",
		SeerrURL:          "https://user:password@seerr.home.test?api_key=leaked",
		SeerrAPIKey:       "seerr-secret",
	}
	if msg := validateSetupInput(in); !strings.Contains(msg, "Seerr internal URL") {
		t.Fatalf("expected credential-bearing connector URL to fail, got %q", msg)
	}
}

func TestSetupPostStoresEncryptedSettingsAndRestarts(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.EncryptionKey = "12345678901234567890123456789012"
	h.cfg.MediaServerAPIKey = "change-me!!"
	restarted := make(chan struct{}, 1)
	h.restart = func() { restarted <- struct{}{} }

	form := url.Values{}
	form.Set("csrf_token", "token")
	form.Set("app_base_url", "https://veyra.home.test")
	form.Set("media_server_type", config.MediaServerJellyfin.String())
	form.Set("media_server_url", "http://mediaserver:8096")
	form.Set("media_server_public_url", "https://mediaserver.home.test")
	form.Set("media_server_api_key", "jf-secret")
	form.Set("seerr_url", "http://seerr:5055")
	form.Set("seerr_api_key", "seerr-secret")
	form.Set("sonarr_url", "http://sonarr:8989")
	form.Set("sonarr_api_key", "sonarr-secret")
	form.Set("radarr_url", "http://radarr:7878")
	form.Set("radarr_api_key", "radarr-secret")
	form.Set("prowlarr_url", "http://prowlarr:9696")
	form.Set("prowlarr_api_key", "prowlarr-secret")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "token"})
	h.SetupPost(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	select {
	case <-restarted:
	case <-time.After(time.Second):
		t.Fatal("expected setup save to request restart")
	}
	stored, err := store.GetSetting(context.Background(), db, "setup.media_server.api_key")
	if err != nil {
		t.Fatal(err)
	}
	if stored == "" || strings.Contains(stored, "jf-secret") {
		t.Fatalf("expected encrypted mediaserver api key, got %q", stored)
	}
	loaded, err := config.ApplyStoredSetup(context.Background(), db, h.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MediaServerAPIKey != "jf-secret" || loaded.ProwlarrAPIKey != "prowlarr-secret" {
		t.Fatalf("stored setup did not apply: %+v", loaded)
	}
}

func TestSetupPostRejectsPlaceholderSecrets(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	h.cfg.MediaServerAPIKey = "change-me!!"

	form := url.Values{}
	form.Set("csrf_token", "token")
	form.Set("app_base_url", "https://veyra.home.test")
	form.Set("media_server_type", config.MediaServerJellyfin.String())
	form.Set("media_server_url", "http://mediaserver:8096")
	form.Set("media_server_api_key", "change-me!!")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "veyra_csrf", Value: "token"})
	h.SetupPost(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want validation render 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Jellyfin API key still contains a placeholder value.") {
		t.Fatalf("expected placeholder validation error, got %s", w.Body.String())
	}
}
