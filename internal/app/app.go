package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
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

	srv := &http.Server{
		Addr:              cfg.AppBindAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

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

func runMaintenance(ctx context.Context, log *slog.Logger, db *sql.DB) {
	const cleanupInterval = 5 * time.Minute
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
			n, err := store.DeleteExpiredCache(ctx, db, now)
			if err != nil {
				log.Warn("cache cleanup failed", "err", err)
				continue
			}
			if n > 0 {
				log.Debug("cache cleanup removed expired entries", "count", n)
			}
		}
	}
}
