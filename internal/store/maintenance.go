package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CheckpointResult struct {
	Busy               bool
	LogFrames          int
	CheckpointedFrames int
}

func CheckIntegrity(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA quick_check`)
	if err != nil {
		return fmt.Errorf("run SQLite integrity check: %w", err)
	}
	defer rows.Close()
	checked := false
	for rows.Next() {
		checked = true
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("read SQLite integrity result: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(result), "ok") {
			return fmt.Errorf("SQLite integrity check failed: %s", result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read SQLite integrity results: %w", err)
	}
	if !checked {
		return fmt.Errorf("SQLite integrity check returned no result")
	}
	return nil
}

func CheckpointWAL(ctx context.Context, db *sql.DB) (CheckpointResult, error) {
	var busy int
	var result CheckpointResult
	if err := db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &result.LogFrames, &result.CheckpointedFrames); err != nil {
		return CheckpointResult{}, fmt.Errorf("checkpoint SQLite WAL: %w", err)
	}
	result.Busy = busy != 0
	return result, nil
}

// BackupSQLite creates a transactionally consistent snapshot using SQLite's
// online VACUUM INTO operation. Existing destinations are never overwritten.
func BackupSQLite(ctx context.Context, db *sql.DB, destination string) error {
	destination = filepath.Clean(strings.TrimSpace(destination))
	if destination == "." || destination == "" {
		return fmt.Errorf("backup destination is required")
	}
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("backup destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect backup destination: %w", err)
	}
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".veyra-backup-*.db")
	if err != nil {
		return fmt.Errorf("reserve backup path: %w", err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("close backup placeholder: %w", err)
	}
	if err := os.Remove(tempPath); err != nil {
		return fmt.Errorf("prepare backup path: %w", err)
	}
	defer os.Remove(tempPath)
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, tempPath); err != nil {
		return fmt.Errorf("create SQLite backup: %w", err)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("restrict backup permissions: %w", err)
	}
	// Linking within the destination directory atomically refuses an existing
	// name, including one created by another backup after the initial check.
	if err := os.Link(tempPath, destination); err != nil {
		return fmt.Errorf("publish SQLite backup: %w", err)
	}
	return nil
}
