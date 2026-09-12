package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthenticationOriginRevokesSessionsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "veyra.db")
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	user, err := UpsertUserByMediaServerID(ctx, db, UserRow{MediaServerUserID: "admin", Username: "admin", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	insert := func(id string) {
		t.Helper()
		if err := InsertSession(ctx, db, SessionRow{IDHash: id, UserID: user.ID, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(time.Hour), CreatedAt: now, LastSeenAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	insert("legacy")
	if err := EnsureAuthOrigin(ctx, db, "server-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSession(ctx, db, "legacy"); err == nil {
		t.Fatal("unknown legacy session survived")
	}
	insert("current")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := EnsureAuthOrigin(ctx, db, "server-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSession(ctx, db, "current"); err != nil {
		t.Fatalf("same-origin session removed: %v", err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_origin BEFORE UPDATE ON settings BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAuthOrigin(ctx, db, "server-b"); err == nil {
		t.Fatal("expected transaction failure")
	}
	if _, err := GetSession(ctx, db, "current"); err != nil {
		t.Fatal("failed origin update lost sessions")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_origin`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAuthOrigin(ctx, db, "server-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetSession(ctx, db, "current"); err == nil {
		t.Fatal("previous server session survived")
	}
	if _, err := GetUserByID(ctx, db, user.ID); err != nil {
		t.Fatal("history user removed")
	}
}
