package config

import (
	"context"
	"database/sql"
	"testing"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func TestSetupRequiredAllowsEmptyOptionalKeys(t *testing.T) {
	cfg := Config{
		MediaServerType: MediaServerJellyfin,
		MediaServerURL:  "http://mediaserver:8096",
	}
	if SetupRequired(cfg) {
		t.Fatal("empty optional keys should not force setup")
	}
	cfg.MediaServerAPIKey = "change-me!!"
	if !SetupRequired(cfg) {
		t.Fatal("placeholder keys should force setup")
	}
}

func TestSetupRequiredSkipsWhenUnraidEnvVarsAreConfigured(t *testing.T) {
	cfg := Config{
		AppBaseURL:           "http://192.168.1.10:3767",
		MediaServerType:      MediaServerJellyfin,
		MediaServerURL:       "http://mediaserver:8096",
		MediaServerPublicURL: "http://192.168.1.20:8096",
		MediaServerAPIKey:    "jf-key",
		SeerrURL:             "http://seerr:5055",
		SeerrPublicURL:       "http://192.168.1.30:5055",
		SeerrAPIKey:          "seerr-key",
		SonarrURL:            "http://sonarr:8989",
		SonarrAPIKey:         "sonarr-key",
		RadarrURL:            "http://radarr:7878",
		RadarrAPIKey:         "radarr-key",
		ProwlarrURL:          "http://prowlarr:9696",
		ProwlarrAPIKey:       "prowlarr-key",
	}
	if SetupRequired(cfg) {
		t.Fatal("fully configured environment variables should bypass setup")
	}
}

func TestSetupRequiredShowsWizardForBootstrapOnlyConfig(t *testing.T) {
	cfg := Config{
		AppBaseURL:    "https://veyra.example.com",
		SessionSecret: "session-secret-with-at-least-32-characters",
		EncryptionKey: "12345678901234567890123456789012",
		DatabasePath:  "/config/veyra.db",
	}
	if !SetupRequired(cfg) {
		t.Fatal("bootstrap-only config should show setup")
	}
}

func TestNormalizeURLsAddsHTTPForHostPortValues(t *testing.T) {
	cfg := NormalizeURLs(Config{
		AppBaseURL:           "192.168.1.10:3767",
		MediaServerURL:       "mediaserver:8096",
		MediaServerPublicURL: "https://mediaserver.example.test",
		SeerrURL:             "192.168.1.11:5055",
		SonarrURL:            "sonarr:8989",
		RadarrURL:            "radarr:7878",
		ProwlarrURL:          "192.168.1.4:9696",
	})
	if cfg.AppBaseURL != "http://192.168.1.10:3767" ||
		cfg.MediaServerURL != "http://mediaserver:8096" ||
		cfg.MediaServerPublicURL != "https://mediaserver.example.test" ||
		cfg.SeerrURL != "http://192.168.1.11:5055" ||
		cfg.SonarrURL != "http://sonarr:8989" ||
		cfg.RadarrURL != "http://radarr:7878" ||
		cfg.ProwlarrURL != "http://192.168.1.4:9696" {
		t.Fatalf("unexpected normalized config: %+v", cfg)
	}
}

func TestApplyStoredSetupDoesNotOverrideEnvValues(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.InitSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	crypto := security.NewCrypto("12345678901234567890123456789012")
	if err := SaveSetup(ctx, db, crypto, SetupInput{
		MediaServerType:   MediaServerJellyfin.String(),
		MediaServerURL:    "http://setup-mediaserver:8096",
		MediaServerAPIKey: "setup-key",
		SeerrAPIKey:       "setup-seerr-key",
	}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MEDIA_SERVER_URL", "http://env-mediaserver:8096")
	cfg, err := ApplyStoredSetup(ctx, db, Config{
		EncryptionKey:     "12345678901234567890123456789012",
		MediaServerURL:    "http://env-mediaserver:8096",
		MediaServerAPIKey: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MediaServerURL != "http://env-mediaserver:8096" {
		t.Fatalf("expected env media server URL to win, got %q", cfg.MediaServerURL)
	}
	if cfg.MediaServerAPIKey != "setup-key" {
		t.Fatalf("expected setup API key to apply when env is absent, got %q", cfg.MediaServerAPIKey)
	}
}

func TestApplyStoredSetupRejectsCorruptEncryptedSettings(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.InitSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSetting(ctx, db, settingMediaServerType, "not-valid-ciphertext"); err != nil {
		t.Fatal(err)
	}

	_, err = ApplyStoredSetup(ctx, db, Config{EncryptionKey: "12345678901234567890123456789012"})
	if err == nil {
		t.Fatal("expected corrupt encrypted setup to fail")
	}
}

func TestSetupRequiredRejectsConfiguredIntegrationWithoutAPIKey(t *testing.T) {
	cfg := Config{
		MediaServerType: MediaServerJellyfin,
		MediaServerURL:  "http://mediaserver:8096",
		SeerrURL:        "http://seerr:5055",
	}
	if !SetupRequired(cfg) {
		t.Fatal("expected incomplete Seerr configuration to require setup")
	}
}
