package handlers

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/store"
)

func TestAuditLogViewIncludesActorAndReadableRequestDetails(t *testing.T) {
	row := store.AuditLogRow{
		UserID:    sql.NullInt64{Int64: 7, Valid: true},
		Username:  sql.NullString{String: "ada", Valid: true},
		Display:   sql.NullString{String: "Ada Lovelace", Valid: true},
		Action:    "request.created",
		Target:    sql.NullString{String: "123", Valid: true},
		Metadata:  sql.NullString{String: auditMetadata(map[string]string{"media_type": "tv", "seasons": "2,3", "seerr_request_id": "44"}), Valid: true},
		IPAddress: sql.NullString{String: "1.2.3.4", Valid: true},
		CreatedAt: time.Date(2026, 6, 7, 1, 2, 3, 0, time.UTC),
	}

	view := auditLogView(row)
	if view.Actor != "Ada Lovelace (ada)" {
		t.Fatalf("unexpected actor: %+v", view)
	}
	if view.Message != "Request created" {
		t.Fatalf("unexpected message: %+v", view)
	}
	if !strings.Contains(view.Detail, "Ada Lovelace") || !strings.Contains(view.Detail, "tv 123") || !strings.Contains(view.Detail, "seasons 2,3") || !strings.Contains(view.Detail, "#44") {
		t.Fatalf("unexpected request detail: %+v", view)
	}
	if strings.Contains(view.Metadata, "{") {
		t.Fatalf("expected readable metadata summary, got %q", view.Metadata)
	}
}

func TestAuditLogViewShowsAttemptedUsernameForLoginFailure(t *testing.T) {
	row := store.AuditLogRow{
		Action:    "login.failure",
		Target:    sql.NullString{String: "baduser", Valid: true},
		Metadata:  sql.NullString{String: auditMetadata(map[string]string{"reason": "invalid_credentials"}), Valid: true},
		IPAddress: sql.NullString{String: "1.2.3.4", Valid: true},
		CreatedAt: time.Date(2026, 6, 7, 1, 2, 3, 0, time.UTC),
	}

	view := auditLogView(row)
	if view.Actor != "baduser (attempted)" {
		t.Fatalf("unexpected actor: %+v", view)
	}
	if !strings.Contains(view.Detail, "baduser") || !strings.Contains(view.Detail, "1.2.3.4") {
		t.Fatalf("unexpected login failure detail: %+v", view)
	}
}
