package store

import (
	"context"
	"database/sql"
	"time"
)

type UserRow struct {
	ID                int64
	MediaServerUserID string
	Username          string
	DisplayName       string
	IsAdmin           bool
}

func UpsertUserByMediaServerID(ctx context.Context, db *sql.DB, u UserRow) (UserRow, error) {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx, `
INSERT INTO users (media_server_user_id, username, display_name, is_admin, created_at, updated_at, last_login_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(media_server_user_id) DO UPDATE SET
  username=excluded.username,
  display_name=excluded.display_name,
  is_admin=excluded.is_admin,
  updated_at=excluded.updated_at,
  last_login_at=excluded.last_login_at
`, u.MediaServerUserID, u.Username, u.DisplayName, u.IsAdmin, now, now, now)
	if err != nil {
		return UserRow{}, err
	}
	return GetUserByMediaServerID(ctx, db, u.MediaServerUserID)
}

func UpdateUserAdmin(ctx context.Context, db *sql.DB, id int64, isAdmin bool) error {
	_, err := db.ExecContext(ctx, `UPDATE users SET is_admin = ?, updated_at = ? WHERE id = ?`, isAdmin, time.Now().UTC(), id)
	return err
}

func GetUserByMediaServerID(ctx context.Context, db *sql.DB, mediaserverUserID string) (UserRow, error) {
	var u UserRow
	err := db.QueryRowContext(ctx, `SELECT id, media_server_user_id, username, COALESCE(display_name, ''), is_admin FROM users WHERE media_server_user_id = ?`, mediaserverUserID).
		Scan(&u.ID, &u.MediaServerUserID, &u.Username, &u.DisplayName, &u.IsAdmin)
	return u, err
}

func GetUserByID(ctx context.Context, db *sql.DB, id int64) (UserRow, error) {
	var u UserRow
	err := db.QueryRowContext(ctx, `SELECT id, media_server_user_id, username, COALESCE(display_name, ''), is_admin FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.MediaServerUserID, &u.Username, &u.DisplayName, &u.IsAdmin)
	return u, err
}
