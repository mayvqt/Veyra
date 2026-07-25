package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppName              string
	AppBaseURL           string
	AppBindAddr          string
	DatabasePath         string
	SessionSecret        string
	EncryptionKey        string
	MediaServerType      MediaServerType
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
	TrustedProxyCIDRs    []string
	CookieSecure         bool
	LogLevel             string
}

func Load() (Config, error) {
	cfg := Config{
		AppName:              getEnv("APP_NAME", "Veyra"),
		AppBaseURL:           getEnv("APP_BASE_URL", "http://localhost:3767"),
		AppBindAddr:          getEnv("APP_BIND_ADDR", "0.0.0.0:3767"),
		DatabasePath:         getEnv("DATABASE_PATH", "./veyra.db"),
		SessionSecret:        os.Getenv("SESSION_SECRET"),
		EncryptionKey:        os.Getenv("ENCRYPTION_KEY"),
		MediaServerURL:       os.Getenv("MEDIA_SERVER_URL"),
		MediaServerPublicURL: os.Getenv("MEDIA_SERVER_PUBLIC_URL"),
		MediaServerAPIKey:    os.Getenv("MEDIA_SERVER_API_KEY"),
		SeerrURL:             os.Getenv("SEERR_URL"),
		SeerrPublicURL:       os.Getenv("SEERR_PUBLIC_URL"),
		SeerrAPIKey:          os.Getenv("SEERR_API_KEY"),
		SonarrURL:            os.Getenv("SONARR_URL"),
		SonarrAPIKey:         os.Getenv("SONARR_API_KEY"),
		RadarrURL:            os.Getenv("RADARR_URL"),
		RadarrAPIKey:         os.Getenv("RADARR_API_KEY"),
		ProwlarrURL:          os.Getenv("PROWLARR_URL"),
		ProwlarrAPIKey:       os.Getenv("PROWLARR_API_KEY"),
		TrustedProxyCIDRs:    splitCSV(getEnv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32")),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
	}
	mediaServerType, err := ParseMediaServerType(getEnv("MEDIA_SERVER_TYPE", string(MediaServerJellyfin)))
	if err != nil {
		return Config{}, err
	}
	cfg.MediaServerType = mediaServerType

	cookieSecure, err := getEnvBool("COOKIE_SECURE", true)
	if err != nil {
		return Config{}, err
	}
	cfg.CookieSecure = cookieSecure
	if len(cfg.SessionSecret) < 32 {
		return Config{}, fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}
	if isPlaceholderSessionSecret(cfg.SessionSecret) {
		return Config{}, fmt.Errorf("SESSION_SECRET must be replaced with a cryptographically random value")
	}
	if len(cfg.EncryptionKey) != 32 {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 characters")
	}
	for _, value := range cfg.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return Config{}, fmt.Errorf("invalid trusted proxy CIDR %q: %w", value, err)
		}
	}
	if cfg.LogLevel != "debug" && cfg.LogLevel != "info" && cfg.LogLevel != "warn" && cfg.LogLevel != "error" {
		return Config{}, fmt.Errorf("invalid LOG_LEVEL %q", cfg.LogLevel)
	}
	return NormalizeURLs(cfg), nil
}

type MediaServerType string

const (
	MediaServerJellyfin MediaServerType = "jellyfin"
	MediaServerEmby     MediaServerType = "emby"
)

func ParseMediaServerType(value string) (MediaServerType, error) {
	normalized := MediaServerType(strings.TrimSpace(strings.ToLower(value)))
	switch normalized {
	case MediaServerJellyfin, MediaServerEmby:
		return normalized, nil
	default:
		return "", fmt.Errorf("MEDIA_SERVER_TYPE must be %q or %q", MediaServerJellyfin, MediaServerEmby)
	}
}

func (t MediaServerType) String() string {
	return string(t)
}

func (t MediaServerType) Valid() bool {
	return t == MediaServerJellyfin || t == MediaServerEmby
}

func (t MediaServerType) Label() string {
	switch t {
	case MediaServerEmby:
		return "Emby"
	case MediaServerJellyfin:
		return "Jellyfin"
	default:
		return "Media server"
	}
}

func isPlaceholderSessionSecret(value string) bool {
	switch strings.TrimSpace(value) {
	case "change-this-to-a-long-random-secret!!", "replace-with-a-long-random-secret":
		return true
	default:
		return false
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func NormalizeURLs(cfg Config) Config {
	cfg.AppBaseURL = normalizeHTTPURL(cfg.AppBaseURL)
	cfg.MediaServerURL = normalizeHTTPURL(cfg.MediaServerURL)
	cfg.MediaServerPublicURL = normalizeHTTPURL(cfg.MediaServerPublicURL)
	cfg.SeerrURL = normalizeHTTPURL(cfg.SeerrURL)
	cfg.SeerrPublicURL = normalizeHTTPURL(cfg.SeerrPublicURL)
	cfg.SonarrURL = normalizeHTTPURL(cfg.SonarrURL)
	cfg.RadarrURL = normalizeHTTPURL(cfg.RadarrURL)
	cfg.ProwlarrURL = normalizeHTTPURL(cfg.ProwlarrURL)
	return cfg
}

func NormalizeHTTPURL(value string) string {
	return normalizeHTTPURL(value)
}

func normalizeHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "://") {
		return value
	}
	return "http://" + value
}
