package auth

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

const testSessionSecret = "session-secret-with-at-least-32-characters"

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

type checkerStub struct {
	isAdmin bool
	err     error
	calls   int
}

func (c *checkerStub) FetchUserAdminStatus(ctx context.Context, userID, token string) (bool, error) {
	c.calls++
	return c.isAdmin, c.err
}

func TestCreateResolveDestroySession(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-1", Username: "u", DisplayName: "U", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, nil)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"

	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: urow.IsAdmin}, "tok", r)
	if err != nil {
		t.Fatal(err)
	}
	sess, user, err := svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != urow.ID || sess.IDHash == "" {
		t.Fatal("session resolve mismatch")
	}
	pt, err := svc.DecryptSessionToken(sess.MediaServerAccessTokenEncrypted)
	if err != nil || pt != "tok" {
		t.Fatal("token decrypt mismatch")
	}
	if err := svc.DestroySession(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSessionWithDurationSetsMatchingExpiries(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-remember", Username: "ur", DisplayName: "UR", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, nil)
	r := httptest.NewRequest("GET", "/", nil)

	raw, err := svc.CreateSessionWithDuration(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", r, RememberSessionDuration)
	if err != nil {
		t.Fatal(err)
	}
	sess, _, err := svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	lifetime := sess.ExpiresAt.Sub(sess.CreatedAt)
	if lifetime < RememberSessionDuration-time.Minute || lifetime > RememberSessionDuration+time.Minute {
		t.Fatalf("expected remembered lifetime near %s, got %s", RememberSessionDuration, lifetime)
	}
	if !sess.AbsoluteExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("expected absolute expiry to match session expiry, got %s and %s", sess.AbsoluteExpiresAt, sess.ExpiresAt)
	}
}

func TestResolveSessionRefreshesAdminEvery15Min(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-2", Username: "u2", DisplayName: "U2", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, &checkerStub{isAdmin: false})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: true}, "tok", r)
	if err != nil {
		t.Fatal(err)
	}
	idHash := security.HashSessionID(testSessionSecret, raw)
	if err := store.UpdateSessionAdminCheckedAt(context.Background(), db, idHash, time.Now().Add(-16*time.Minute)); err != nil {
		t.Fatal(err)
	}

	_, user, err := svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if user.IsAdmin {
		t.Fatal("expected admin revoked after refresh")
	}
}

func TestResolveSessionPermissionChangeWritesAuditLog(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-3", Username: "u3", DisplayName: "U3", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, &checkerStub{isAdmin: false})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: true}, "tok", r)
	if err != nil {
		t.Fatal(err)
	}
	idHash := security.HashSessionID(testSessionSecret, raw)
	if err := store.UpdateSessionAdminCheckedAt(context.Background(), db, idHash, time.Now().Add(-16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := store.ListRecentAuditLogsByActionTarget(context.Background(), db, "permission.change_detected", "jf-3", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Fatal("expected permission.change_detected log")
	}
}

func TestResolveSessionFailsWhenAbsoluteLifetimeExpired(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-abs", Username: "ua", DisplayName: "UA", IsAdmin: false})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, nil)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", r)
	if err != nil {
		t.Fatal(err)
	}
	idHash := security.HashSessionID(testSessionSecret, raw)
	_, err = db.ExecContext(context.Background(), `UPDATE sessions SET absolute_expires_at = ? WHERE id_hash = ?`, time.Now().Add(-time.Hour), idHash)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.ResolveSession(context.Background(), raw)
	if err == nil {
		t.Fatal("expected resolve failure when absolute lifetime expired")
	}
}
