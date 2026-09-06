package config

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func TestSaveInitialSetupRejectsExistingState(t *testing.T) {
	for _, state := range []string{"empty stored setting", "existing user", "read error"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			db, err := store.OpenSQLite(t.TempDir() + "/veyra.db")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := store.InitSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			switch state {
			case "empty stored setting":
				err = store.UpsertSetting(ctx, db, settingMediaServerURL, "")
			case "existing user":
				_, err = store.UpsertUserByMediaServerID(ctx, db, store.UserRow{MediaServerUserID: "user", Username: "user"})
			case "read error":
				_, err = db.ExecContext(ctx, "DROP TABLE settings")
			}
			if err != nil {
				t.Fatal(err)
			}
			err = SaveInitialSetup(ctx, db, security.NewCrypto("12345678901234567890123456789012"), SetupInput{MediaServerType: "jellyfin", MediaServerURL: "http://replacement.invalid"})
			if err == nil || (state != "read error" && !errors.Is(err, ErrSetupAlreadyComplete)) {
				t.Fatalf("initial setup accepted existing or unreadable state: %v", err)
			}
			if state != "read error" {
				value, err := store.GetSetting(ctx, db, settingMediaServerURL)
				if err != nil || value != "" {
					t.Fatalf("rejected setup wrote settings: err=%v", err)
				}
			}
		})
	}
}

func TestSaveInitialSetupConcurrentClaimsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/veyra.db"
	dbs := make([]*sql.DB, 2)
	for i := range dbs {
		db, err := store.OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		dbs[i] = db
	}
	if err := store.InitSchema(ctx, dbs[0]); err != nil {
		t.Fatal(err)
	}
	crypto := security.NewCrypto("12345678901234567890123456789012")
	inputs := []SetupInput{
		{MediaServerType: "jellyfin", MediaServerURL: "http://first.invalid", MediaServerAPIKey: "first-key"},
		{MediaServerType: "emby", MediaServerURL: "http://second.invalid", MediaServerAPIKey: "second-key"},
	}
	start := make(chan struct{})
	results := make(chan int, 2)
	for i := range inputs {
		go func(i int) {
			<-start
			if err := SaveInitialSetup(ctx, dbs[i], crypto, inputs[i]); err != nil {
				results <- -1
				return
			}
			results <- i
		}(i)
	}
	close(start)
	winner := -1
	for range inputs {
		if result := <-results; result >= 0 {
			if winner >= 0 {
				t.Fatal("both initial setup requests succeeded")
			}
			winner = result
		}
	}
	if winner < 0 {
		t.Fatal("neither initial setup request succeeded")
	}
	stored, _, err := StoredSetup(ctx, dbs[0], crypto)
	if err != nil || stored != inputs[winner] {
		t.Fatalf("initial setup did not persist one complete request: err=%v", err)
	}
	if err := SaveInitialSetup(ctx, dbs[1], crypto, inputs[1-winner]); !errors.Is(err, ErrSetupAlreadyComplete) {
		t.Fatalf("later request could replace initial setup: %v", err)
	}
}

func TestExampleEnvironmentAllowsWizardSetupAfterRestart(t *testing.T) {
	content, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatal("invalid example environment assignment")
		}
		t.Setenv(key, value)
	}
	t.Setenv("SESSION_SECRET", "synthetic-session-secret-for-bootstrap-test")
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !SetupRequired(cfg) {
		t.Fatal("fresh example environment must open the wizard")
	}
	db, err := store.OpenSQLite(t.TempDir() + "/veyra.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.InitSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	in := SetupInput{AppBaseURL: "https://portal.example.test", MediaServerType: "emby", MediaServerURL: "http://emby:8096"}
	if err := SaveSetup(ctx, db, security.NewCrypto(cfg.EncryptionKey), in); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ApplyStoredSetup(ctx, db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if SetupRequired(reloaded) || reloaded.MediaServerType != MediaServerEmby || reloaded.MediaServerURL != in.MediaServerURL || reloaded.AppBaseURL != in.AppBaseURL {
		t.Fatal("example environment overrides completed wizard settings")
	}
}

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
