package store

import (
	"context"
	"database/sql"
	"time"
)

type UserListRow struct {
	ID                int64
	MediaServerUserID string
	Username          string
	DisplayName       string
	IsAdmin           bool
	CreatedAt         time.Time
	LastLoginAt       sql.NullTime
}

type UserCounts struct {
	Total int
	Admin int
}

func CountUsers(ctx context.Context, db *sql.DB) (UserCounts, error) {
	var counts UserCounts
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_admin THEN 1 ELSE 0 END), 0)
FROM users
`).Scan(&counts.Total, &counts.Admin)
	return counts, err
}

func ListUsers(ctx context.Context, db *sql.DB, limit int) ([]UserListRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, media_server_user_id, username, COALESCE(display_name, ''), is_admin, created_at, last_login_at
FROM users ORDER BY created_at DESC LIMIT ?
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []UserListRow{}
	for rows.Next() {
		var u UserListRow
		if err := rows.Scan(&u.ID, &u.MediaServerUserID, &u.Username, &u.DisplayName, &u.IsAdmin, &u.CreatedAt, &u.LastLoginAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
