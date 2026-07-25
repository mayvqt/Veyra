package config

import "testing"

func TestLoadRequiresStrongSessionSecret(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("SESSION_SECRET", "too-short")
	if _, err := Load(); err == nil {
		t.Fatal("expected short SESSION_SECRET to fail")
	}
}

func TestLoadRejectsExampleSessionSecrets(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	for _, value := range []string{"change-this-to-a-long-random-secret!!", "replace-with-a-long-random-secret"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SESSION_SECRET", value)
			if _, err := Load(); err == nil {
				t.Fatal("expected example SESSION_SECRET to fail")
			}
		})
	}
}

func TestLoadRejectsInvalidCookieSecure(t *testing.T) {
	t.Setenv("SESSION_SECRET", "session-secret-with-at-least-32-characters")
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("COOKIE_SECURE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid COOKIE_SECURE to fail")
	}
}

func TestLoadRejectsInvalidProxyAndLogConfiguration(t *testing.T) {
	t.Setenv("SESSION_SECRET", "session-secret-with-at-least-32-characters")
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-network")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid proxy CIDR to fail")
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	t.Setenv("LOG_LEVEL", "chatty")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid log level to fail")
	}
}

func TestLoadRejectsUnknownMediaServerType(t *testing.T) {
	t.Setenv("SESSION_SECRET", "session-secret-with-at-least-32-characters")
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	t.Setenv("MEDIA_SERVER_TYPE", "plex")
	if _, err := Load(); err == nil {
		t.Fatal("expected unknown media server type to fail")
	}
}
