package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SettingRow struct {
	Key   string
	Value string
}

func UpsertSetting(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
`, key, value, time.Now().UTC())
	return err
}

func UpsertSettings(ctx context.Context, db *sql.DB, values map[string]string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin settings update: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
`, key, value, now); err != nil {
			return fmt.Errorf("upsert setting %q: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit settings update: %w", err)
	}
	return nil
}

func GetSetting(ctx context.Context, db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func GetSettings(ctx context.Context, db *sql.DB, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	seen := make(map[string]struct{}, len(keys))
	unique := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	if len(unique) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(unique)), ",")
	query := fmt.Sprintf(`SELECT key, value FROM settings WHERE key IN (%s)`, placeholders)
	args := make([]any, 0, len(unique))
	for _, k := range unique {
		args = append(args, k)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return out, err
		}
		out[key] = value
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}
