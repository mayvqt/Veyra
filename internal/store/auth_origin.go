package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// EnsureAuthOrigin revokes sessions from a previous (or unknown legacy) media
// server before a new router can accept them. User/audit history is preserved.
func EnsureAuthOrigin(ctx context.Context, db *sql.DB, origin string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'auth.origin'`).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if previous != origin {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions`); err != nil {
			return fmt.Errorf("revoke previous server sessions: %w", err)
		}
		if err := UpsertSettingsTx(ctx, tx, map[string]string{"auth.origin": origin}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
