package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/mayvqt/veyra/internal/buildinfo"
	"github.com/mayvqt/veyra/internal/config"
	httpx "github.com/mayvqt/veyra/internal/http"
	"github.com/mayvqt/veyra/internal/logging"
	"github.com/mayvqt/veyra/internal/store"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := store.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if err := store.CheckIntegrity(ctx, db); err != nil {
		return fmt.Errorf("check db integrity: %w", err)
	}

	if err := store.InitSchema(ctx, db); err != nil {
		return fmt.Errorf("initialize db schema: %w", err)
	}
	cfg, err = config.ApplyStoredSetup(ctx, db, cfg)
	if err != nil {
		return fmt.Errorf("apply stored setup: %w", err)
	}
	log := logging.New(cfg.LogLevel,
		cfg.SessionSecret,
		cfg.EncryptionKey,
		cfg.MediaServerAPIKey,
		cfg.SeerrAPIKey,
		cfg.SonarrAPIKey,
		cfg.RadarrAPIKey,
		cfg.ProwlarrAPIKey,
	)
	cleanupCtx, stopCleanup := context.WithCancel(ctx)
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		runMaintenance(cleanupCtx, log, db)
	}()
	defer func() {
		stopCleanup()
		<-cleanupDone
	}()

	r, err := httpx.NewRouter(cfg, log, db)
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	srv := newHTTPServer(cfg.AppBindAddr, r)

	errCh := make(chan error, 1)
	go func() {
		log.Info("veyra starting", "addr", cfg.AppBindAddr, "version", buildinfo.Version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

func runMaintenance(ctx context.Context, log *slog.Logger, db *sql.DB) {
	const cleanupInterval = 5 * time.Minute
	const expiredCacheRetention = 30 * time.Minute
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC()
			sessions, err := store.DeleteExpiredSessions(ctx, db, now)
			if err != nil {
				log.Warn("session cleanup failed", "err", err)
			} else if sessions > 0 {
				log.Debug("session cleanup removed expired entries", "count", sessions)
			}
			n, err := store.DeleteExpiredCache(ctx, db, now.Add(-expiredCacheRetention))
			if err != nil {
				log.Warn("cache cleanup failed", "err", err)
			} else if n > 0 {
				log.Debug("cache cleanup removed expired entries", "count", n)
			}
			checkpoint, err := store.CheckpointWAL(ctx, db)
			if err != nil {
				log.Warn("database checkpoint failed", "err", err)
			} else if checkpoint.Busy {
				log.Debug("database checkpoint deferred", "log_frames", checkpoint.LogFrames, "checkpointed_frames", checkpoint.CheckpointedFrames)
			}
		}
	}
}

func Backup(ctx context.Context, destination string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	source, err := filepath.Abs(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	target, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve backup path: %w", err)
	}
	if source == target {
		return fmt.Errorf("backup destination must differ from database path")
	}
	db, err := store.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if err := store.CheckIntegrity(ctx, db); err != nil {
		return fmt.Errorf("check db integrity: %w", err)
	}
	return store.BackupSQLite(ctx, db, destination)
}
