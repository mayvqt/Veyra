package store

import (
	"context"
	"database/sql"
	"time"
)

type SessionRow struct {
	IDHash                          string
	UserID                          int64
	MediaServerAccessTokenEncrypted string
	ExpiresAt                       time.Time
	CreatedAt                       time.Time
	LastSeenAt                      time.Time
	AbsoluteExpiresAt               time.Time
	AdminCheckedAt                  time.Time
	IPAddress                       string
	UserAgent                       string
}

func InsertSession(ctx context.Context, db *sql.DB, s SessionRow) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO sessions (id_hash, user_id, media_server_access_token_encrypted, expires_at, created_at, last_seen_at, absolute_expires_at, admin_checked_at, ip_address, user_agent)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, s.IDHash, s.UserID, s.MediaServerAccessTokenEncrypted, s.ExpiresAt.UTC(), s.CreatedAt.UTC(), s.LastSeenAt.UTC(), s.AbsoluteExpiresAt.UTC(), s.AdminCheckedAt.UTC(), s.IPAddress, s.UserAgent)
	return err
}

func GetSession(ctx context.Context, db *sql.DB, idHash string) (SessionRow, error) {
	var s SessionRow
	var absoluteExp sql.NullTime
	var adminChecked sql.NullTime
	err := db.QueryRowContext(ctx, `
SELECT id_hash, user_id, COALESCE(media_server_access_token_encrypted, ''), expires_at, created_at, last_seen_at, absolute_expires_at, admin_checked_at, COALESCE(ip_address, ''), COALESCE(user_agent, '')
FROM sessions WHERE id_hash = ?
`, idHash).Scan(&s.IDHash, &s.UserID, &s.MediaServerAccessTokenEncrypted, &s.ExpiresAt, &s.CreatedAt, &s.LastSeenAt, &absoluteExp, &adminChecked, &s.IPAddress, &s.UserAgent)
	if err != nil {
		return SessionRow{}, err
	}
	if absoluteExp.Valid {
		s.AbsoluteExpiresAt = absoluteExp.Time
	}
	if adminChecked.Valid {
		s.AdminCheckedAt = adminChecked.Time
	}
	now := time.Now().UTC()
	if !s.ExpiresAt.After(now) || (!s.AbsoluteExpiresAt.IsZero() && !s.AbsoluteExpiresAt.After(now)) {
		return SessionRow{}, sql.ErrNoRows
	}
	return s, nil
}

func TouchSession(ctx context.Context, db *sql.DB, idHash string, t time.Time) error {
	_, err := db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id_hash = ?`, t.UTC(), idHash)
	return err
}

func UpdateSessionAdminCheckedAt(ctx context.Context, db *sql.DB, idHash string, t time.Time) error {
	_, err := db.ExecContext(ctx, `UPDATE sessions SET admin_checked_at = ? WHERE id_hash = ?`, t.UTC(), idHash)
	return err
}

func DeleteSession(ctx context.Context, db *sql.DB, idHash string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE id_hash = ?`, idHash)
	return err
}
