package store

import (
	"context"
	"database/sql"
	"time"
)

type CacheRow struct {
	Key       string
	ValueJSON string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func UpsertCache(ctx context.Context, db *sql.DB, key, valueJSON string, ttl time.Duration) error {
	now := time.Now().UTC()
	exp := now.Add(ttl)
	_, err := db.ExecContext(ctx, `
INSERT INTO cache_entries (key, value_json, expires_at, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, expires_at=excluded.expires_at, created_at=excluded.created_at
`, key, valueJSON, exp, now)
	return err
}

func GetCache(ctx context.Context, db *sql.DB, key string) (string, bool, error) {
	row, ok, err := GetCacheEntry(ctx, db, key)
	if err != nil || !ok {
		return "", false, err
	}
	if !row.ExpiresAt.After(time.Now().UTC()) {
		return "", false, nil
	}
	return row.ValueJSON, true, nil
}

func GetCacheEntry(ctx context.Context, db *sql.DB, key string) (CacheRow, bool, error) {
	var row CacheRow
	err := db.QueryRowContext(ctx, `SELECT key, value_json, expires_at, created_at FROM cache_entries WHERE key = ?`, key).Scan(&row.Key, &row.ValueJSON, &row.ExpiresAt, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return CacheRow{}, false, nil
	}
	if err != nil {
		return CacheRow{}, false, err
	}
	return row, true, nil
}

func DeleteExpiredCache(ctx context.Context, db *sql.DB, now time.Time) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM cache_entries WHERE expires_at <= ?`, now.UTC())
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}
