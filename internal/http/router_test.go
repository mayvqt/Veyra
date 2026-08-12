package http

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/store"
)

func TestNewRouterServesEmbeddedAssetsOutsideProjectDirectory(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	cfg := config.Config{
		AppName:         "Veyra",
		EncryptionKey:   "12345678901234567890123456789012",
		SessionSecret:   "session-secret-with-at-least-32-characters",
		MediaServerType: config.MediaServerJellyfin,
		MediaServerURL:  "http://media.local",
	}
	router, err := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/healthz", "/static/dashboard.js", "/login"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(stdhttp.MethodGet, path, nil))
		if w.Code != stdhttp.StatusOK {
			t.Fatalf("GET %s returned %d", path, w.Code)
		}
	}
}
