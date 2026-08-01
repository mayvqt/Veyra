package logging

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactingHandlerRemovesConfiguredSecretsAndSensitiveErrors(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	log := slog.New(newRedactingHandler(base, []string{"configured-api-key"}))
	log.With("config", slog.Group("connector", "key", "configured-api-key")).Error(
		"request with configured-api-key failed",
		"err", errors.New("upstream rejected Authorization: Bearer other-secret"),
		"url", "https://user:pass@example.test/api?token=query-secret",
		"api_key", "unconfigured-secret",
	)

	got := buf.String()
	for _, secret := range []string{"configured-api-key", "other-secret", "query-secret", "unconfigured-secret", "user", "pass"} {
		if strings.Contains(got, secret) {
			t.Fatalf("log leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "redacted sensitive error") {
		t.Fatalf("expected sanitized error marker, got %s", got)
	}
}
