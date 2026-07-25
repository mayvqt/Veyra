package store

import (
	"context"
	"database/sql"
	"time"
)

type AuditLogRow struct {
	ID        int64
	UserID    sql.NullInt64
	Username  sql.NullString
	Display   sql.NullString
	Action    string
	Target    sql.NullString
	Metadata  sql.NullString
	IPAddress sql.NullString
	CreatedAt time.Time
}

func InsertAuditLog(ctx context.Context, db *sql.DB, userID *int64, action, target, metadata, ipAddress string) error {
	var uid any
	if userID != nil {
		uid = *userID
	}
	_, err := db.ExecContext(ctx, `
INSERT INTO audit_logs (user_id, action, target, metadata_json, ip_address, created_at)
VALUES (?, ?, ?, ?, ?, ?)
`, uid, action, target, metadata, ipAddress, time.Now().UTC())
	return err
}

func ListRecentAuditLogs(ctx context.Context, db *sql.DB, limit int) ([]AuditLogRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT audit_logs.id, audit_logs.user_id, users.username, users.display_name, audit_logs.action, audit_logs.target, audit_logs.metadata_json, audit_logs.ip_address, audit_logs.created_at
FROM audit_logs
LEFT JOIN users ON users.id = audit_logs.user_id
ORDER BY audit_logs.created_at DESC
LIMIT ?
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AuditLogRow{}
	for rows.Next() {
		var r AuditLogRow
		if err := scanAuditLogRow(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func ListRecentAuditLogsByActionTarget(ctx context.Context, db *sql.DB, action, target string, limit int) ([]AuditLogRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT audit_logs.id, audit_logs.user_id, users.username, users.display_name, audit_logs.action, audit_logs.target, audit_logs.metadata_json, audit_logs.ip_address, audit_logs.created_at
FROM audit_logs
LEFT JOIN users ON users.id = audit_logs.user_id
WHERE audit_logs.action = ? AND audit_logs.target = ?
ORDER BY audit_logs.created_at DESC
LIMIT ?
`, action, target, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AuditLogRow{}
	for rows.Next() {
		var r AuditLogRow
		if err := scanAuditLogRow(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanAuditLogRow(rows *sql.Rows, r *AuditLogRow) error {
	return rows.Scan(&r.ID, &r.UserID, &r.Username, &r.Display, &r.Action, &r.Target, &r.Metadata, &r.IPAddress, &r.CreatedAt)
}
