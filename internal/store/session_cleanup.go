package store

import (
	"context"
	"database/sql"
	"time"
)

func DeleteExpiredSessions(ctx context.Context, db *sql.DB, now time.Time) (int64, error) {
	result, err := db.ExecContext(ctx, `
DELETE FROM sessions
WHERE expires_at < ?
   OR (absolute_expires_at IS NOT NULL AND absolute_expires_at < ?)
`, now.UTC(), now.UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
