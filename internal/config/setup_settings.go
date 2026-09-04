package config

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

const (
	settingAppBaseURL           = "setup.app_base_url"
	settingMediaServerType      = "setup.media_server.type"
	settingMediaServerURL       = "setup.media_server.url"
	settingMediaServerPublicURL = "setup.media_server.public_url"
	settingMediaServerAPIKey    = "setup.media_server.api_key"
	settingSeerrURL             = "setup.seerr.url"
	settingSeerrPublicURL       = "setup.seerr.public_url"
	settingSeerrAPIKey          = "setup.seerr.api_key"
	settingSonarrURL            = "setup.sonarr.url"
	settingSonarrAPIKey         = "setup.sonarr.api_key"
	settingRadarrURL            = "setup.radarr.url"
	settingRadarrAPIKey         = "setup.radarr.api_key"
	settingProwlarrURL          = "setup.prowlarr.url"
	settingProwlarrAPIKey       = "setup.prowlarr.api_key"
)

var setupSettingEnvNames = map[string]string{
	settingAppBaseURL:           "APP_BASE_URL",
	settingMediaServerType:      "MEDIA_SERVER_TYPE",
	settingMediaServerURL:       "MEDIA_SERVER_URL",
	settingMediaServerPublicURL: "MEDIA_SERVER_PUBLIC_URL",
	settingMediaServerAPIKey:    "MEDIA_SERVER_API_KEY",
	settingSeerrURL:             "SEERR_URL",
	settingSeerrPublicURL:       "SEERR_PUBLIC_URL",
	settingSeerrAPIKey:          "SEERR_API_KEY",
	settingSonarrURL:            "SONARR_URL",
	settingSonarrAPIKey:         "SONARR_API_KEY",
	settingRadarrURL:            "RADARR_URL",
	settingRadarrAPIKey:         "RADARR_API_KEY",
	settingProwlarrURL:          "PROWLARR_URL",
	settingProwlarrAPIKey:       "PROWLARR_API_KEY",
}

var encryptedSetupSettings = []string{
	settingAppBaseURL,
	settingMediaServerType,
	settingMediaServerURL,
	settingMediaServerPublicURL,
	settingMediaServerAPIKey,
	settingSeerrURL,
	settingSeerrPublicURL,
	settingSeerrAPIKey,
	settingSonarrURL,
	settingSonarrAPIKey,
	settingRadarrURL,
	settingRadarrAPIKey,
	settingProwlarrURL,
	settingProwlarrAPIKey,
}

type SetupInput struct {
	AppBaseURL           string
	MediaServerType      string
	MediaServerURL       string
	MediaServerPublicURL string
	MediaServerAPIKey    string
	SeerrURL             string
	SeerrPublicURL       string
	SeerrAPIKey          string
	SonarrURL            string
	SonarrAPIKey         string
	RadarrURL            string
	RadarrAPIKey         string
	ProwlarrURL          string
	ProwlarrAPIKey       string
}

func ApplyStoredSetup(ctx context.Context, db *sql.DB, cfg Config) (Config, error) {
	if db == nil || cfg.EncryptionKey == "" {
		return cfg, nil
	}
	values, _, err := decryptSetupSettings(ctx, db, security.NewCrypto(cfg.EncryptionKey))
	if err != nil {
		return Config{}, fmt.Errorf("read stored setup: %w", err)
	}
	if len(values) == 0 {
		return cfg, nil
	}
	applyStoredString(&cfg.AppBaseURL, values[settingAppBaseURL], settingAppBaseURL)
	if os.Getenv(setupSettingEnvNames[settingMediaServerType]) == "" && values[settingMediaServerType] != "" {
		serverType, err := ParseMediaServerType(values[settingMediaServerType])
		if err != nil {
			return Config{}, fmt.Errorf("stored media server type: %w", err)
		}
		cfg.MediaServerType = serverType
	}
	applyStoredString(&cfg.MediaServerURL, values[settingMediaServerURL], settingMediaServerURL)
	applyStoredString(&cfg.MediaServerPublicURL, values[settingMediaServerPublicURL], settingMediaServerPublicURL)
	applyStoredString(&cfg.MediaServerAPIKey, values[settingMediaServerAPIKey], settingMediaServerAPIKey)
	applyStoredString(&cfg.SeerrURL, values[settingSeerrURL], settingSeerrURL)
	applyStoredString(&cfg.SeerrPublicURL, values[settingSeerrPublicURL], settingSeerrPublicURL)
	applyStoredString(&cfg.SeerrAPIKey, values[settingSeerrAPIKey], settingSeerrAPIKey)
	applyStoredString(&cfg.SonarrURL, values[settingSonarrURL], settingSonarrURL)
	applyStoredString(&cfg.SonarrAPIKey, values[settingSonarrAPIKey], settingSonarrAPIKey)
	applyStoredString(&cfg.RadarrURL, values[settingRadarrURL], settingRadarrURL)
	applyStoredString(&cfg.RadarrAPIKey, values[settingRadarrAPIKey], settingRadarrAPIKey)
	applyStoredString(&cfg.ProwlarrURL, values[settingProwlarrURL], settingProwlarrURL)
	applyStoredString(&cfg.ProwlarrAPIKey, values[settingProwlarrAPIKey], settingProwlarrAPIKey)
	cfg = NormalizeURLs(cfg)
	if err := ValidateURLs(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func StoredSetup(ctx context.Context, db *sql.DB, crypto *security.Crypto) (SetupInput, map[string]bool, error) {
	values, stored, err := decryptSetupSettings(ctx, db, crypto)
	if err != nil {
		return SetupInput{}, nil, err
	}
	return setupInputFromValues(values), stored, nil
}

func decryptSetupSettings(ctx context.Context, db *sql.DB, crypto *security.Crypto) (map[string]string, map[string]bool, error) {
	settings, err := store.GetSettings(ctx, db, encryptedSetupSettings)
	if err != nil {
		return nil, nil, err
	}
	values := make(map[string]string, len(settings))
	stored := make(map[string]bool, len(settings))
	for key, raw := range settings {
		enc := strings.TrimSpace(raw)
		if enc == "" {
			continue
		}
		stored[key] = true
		plain, err := crypto.Decrypt(enc)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt setting %q: %w", key, err)
		}
		values[key] = strings.TrimSpace(plain)
	}
	return values, stored, nil
}

func setupInputFromValues(values map[string]string) SetupInput {
	return SetupInput{
		AppBaseURL:           values[settingAppBaseURL],
		MediaServerType:      normalizeMediaServerTypeInput(values[settingMediaServerType]),
		MediaServerURL:       values[settingMediaServerURL],
		MediaServerPublicURL: values[settingMediaServerPublicURL],
		MediaServerAPIKey:    values[settingMediaServerAPIKey],
		SeerrURL:             values[settingSeerrURL],
		SeerrPublicURL:       values[settingSeerrPublicURL],
		SeerrAPIKey:          values[settingSeerrAPIKey],
		SonarrURL:            values[settingSonarrURL],
		SonarrAPIKey:         values[settingSonarrAPIKey],
		RadarrURL:            values[settingRadarrURL],
		RadarrAPIKey:         values[settingRadarrAPIKey],
		ProwlarrURL:          values[settingProwlarrURL],
		ProwlarrAPIKey:       values[settingProwlarrAPIKey],
	}
}

func SaveSetup(ctx context.Context, db *sql.DB, crypto *security.Crypto, in SetupInput) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin setup update: %w", err)
	}
	defer tx.Rollback()
	if err := SaveSetupTx(ctx, tx, crypto, in); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit setup update: %w", err)
	}
	return nil
}

// SaveSetupTx writes setup settings using an existing transaction. The caller
// owns the transaction and decides whether to commit or roll it back.
func SaveSetupTx(ctx context.Context, tx *sql.Tx, crypto *security.Crypto, in SetupInput) error {
	in = normalizeSetupInput(in)
	if _, err := ParseMediaServerType(in.MediaServerType); err != nil {
		return err
	}
	encrypted := make(map[string]string, len(encryptedSetupSettings))
	for key, value := range setupInputValues(in) {
		enc, err := crypto.Encrypt(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("encrypt setting %q: %w", key, err)
		}
		encrypted[key] = enc
	}
	return store.UpsertSettingsTx(ctx, tx, encrypted)
}

func setupInputValues(in SetupInput) map[string]string {
	return map[string]string{
		settingAppBaseURL:           in.AppBaseURL,
		settingMediaServerType:      normalizeMediaServerTypeInput(in.MediaServerType),
		settingMediaServerURL:       in.MediaServerURL,
		settingMediaServerPublicURL: in.MediaServerPublicURL,
		settingMediaServerAPIKey:    in.MediaServerAPIKey,
		settingSeerrURL:             in.SeerrURL,
		settingSeerrPublicURL:       in.SeerrPublicURL,
		settingSeerrAPIKey:          in.SeerrAPIKey,
		settingSonarrURL:            in.SonarrURL,
		settingSonarrAPIKey:         in.SonarrAPIKey,
		settingRadarrURL:            in.RadarrURL,
		settingRadarrAPIKey:         in.RadarrAPIKey,
		settingProwlarrURL:          in.ProwlarrURL,
		settingProwlarrAPIKey:       in.ProwlarrAPIKey,
	}
}

func normalizeSetupInput(in SetupInput) SetupInput {
	in.AppBaseURL = normalizeHTTPURL(in.AppBaseURL)
	in.MediaServerType = normalizeMediaServerTypeInput(in.MediaServerType)
	in.MediaServerURL = normalizeHTTPURL(in.MediaServerURL)
	in.MediaServerPublicURL = normalizeHTTPURL(in.MediaServerPublicURL)
	in.SeerrURL = normalizeHTTPURL(in.SeerrURL)
	in.SeerrPublicURL = normalizeHTTPURL(in.SeerrPublicURL)
	in.SonarrURL = normalizeHTTPURL(in.SonarrURL)
	in.RadarrURL = normalizeHTTPURL(in.RadarrURL)
	in.ProwlarrURL = normalizeHTTPURL(in.ProwlarrURL)
	return in
}

func normalizeMediaServerTypeInput(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func SetupRequired(cfg Config) bool {
	return !cfg.MediaServerType.Valid() ||
		cfg.MediaServerURL == "" ||
		(cfg.SeerrURL != "" && cfg.SeerrAPIKey == "") ||
		(cfg.SonarrURL != "" && cfg.SonarrAPIKey == "") ||
		(cfg.RadarrURL != "" && cfg.RadarrAPIKey == "") ||
		(cfg.ProwlarrURL != "" && cfg.ProwlarrAPIKey == "") ||
		IsPlaceholderValue(cfg.MediaServerPublicURL) ||
		IsPlaceholderValue(cfg.MediaServerAPIKey) ||
		IsPlaceholderValue(cfg.SeerrAPIKey) ||
		IsPlaceholderValue(cfg.SonarrAPIKey) ||
		IsPlaceholderValue(cfg.RadarrAPIKey) ||
		IsPlaceholderValue(cfg.ProwlarrAPIKey)
}

func applyString(dst *string, value string) {
	if value != "" {
		*dst = value
	}
}

func applyStoredString(dst *string, value, settingKey string) {
	if os.Getenv(setupSettingEnvNames[settingKey]) != "" {
		return
	}
	applyString(dst, value)
}

func IsPlaceholderValue(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value != "" &&
		(strings.Contains(value, "change-me") ||
			strings.Contains(value, "replace-me") ||
			strings.Contains(value, "example.com"))
}
