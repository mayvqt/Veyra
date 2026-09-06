package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/integrations"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func TestResolveSessionRejectsRevokedUpstreamSession(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		for _, isAdmin := range []bool{false, true} {
			t.Run(fmt.Sprintf("status_%d_admin_%t", status, isAdmin), func(t *testing.T) {
				ctx := context.Background()
				db := testDB(t)
				defer db.Close()
				row, err := store.UpsertUserByMediaServerID(ctx, db, store.UserRow{MediaServerUserID: "revoked-user", Username: "user", IsAdmin: isAdmin})
				if err != nil {
					t.Fatal(err)
				}
				checker := &checkerStub{err: fmt.Errorf("fetch media user: %w", integrations.NewHTTPStatusError("media server", "user", status))}
				svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, checker)
				raw, err := svc.CreateSession(ctx, User{ID: row.ID}, "token", "127.0.0.1", "test-agent")
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := svc.ResolveSession(ctx, raw); err != nil || checker.calls != 0 {
					t.Fatalf("fresh session should retain the refresh interval: err=%v calls=%d", err, checker.calls)
				}
				idHash := security.HashSessionID(testSessionSecret, raw)
				if err := store.UpdateSessionAdminCheckedAt(ctx, db, idHash, time.Now().Add(-16*time.Minute)); err != nil {
					t.Fatal(err)
				}
				if _, _, err := svc.ResolveSession(ctx, raw); err == nil {
					t.Fatal("revoked upstream session retained member access")
				}
				if _, err := store.GetSession(ctx, db, idHash); !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("revoked session was not deleted: %v", err)
				}
				if _, _, err := svc.ResolveSession(ctx, raw); err == nil || checker.calls != 1 {
					t.Fatalf("deleted session resolved again: err=%v calls=%d", err, checker.calls)
				}
				stored, err := store.GetUserByID(ctx, db, row.ID)
				if err != nil || stored.IsAdmin != isAdmin {
					t.Fatalf("session rejection changed stored user permissions: err=%v admin=%t", err, stored.IsAdmin)
				}
			})
		}
	}
}

func TestResolveSessionFailsClosedWhenAdminRefreshFails(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-fail-closed", Username: "admin", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	checker := &checkerStub{err: errors.New("media server unavailable")}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, checker)
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, IsAdmin: true}, "tok", "127.0.0.1", "test-agent")
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
		t.Fatal("cached administrator access remained active after refresh failure")
	}
	if checker.calls != 1 {
		t.Fatalf("admin checker calls = %d, want 1", checker.calls)
	}
	storedRow, err := store.GetUserByID(context.Background(), db, urow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !storedRow.IsAdmin {
		t.Fatal("transient refresh failure permanently demoted the stored administrator")
	}
	_, storedUser, err := svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if storedUser.IsAdmin {
		t.Fatal("administrator access was not denied during retry backoff")
	}
	if checker.calls != 1 {
		t.Fatalf("admin checker calls = %d, want throttled at 1", checker.calls)
	}
	checker.err = nil
	checker.isAdmin = true
	svc.setAdminRetry(idHash, time.Now().Add(-time.Second))
	_, recoveredUser, err := svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if !recoveredUser.IsAdmin || checker.calls != 2 {
		t.Fatalf("administrator verification did not recover: user=%+v calls=%d", recoveredUser, checker.calls)
	}
}

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
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: urow.IsAdmin}, "tok", "1.2.3.4", "test-agent")
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
	if sess.IPAddress != "1.2.3.4" || sess.UserAgent != "test-agent" {
		t.Fatalf("session metadata mismatch: ip=%q user-agent=%q", sess.IPAddress, sess.UserAgent)
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
	raw, err := svc.CreateSessionWithDuration(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", "1.2.3.4", "test-agent", RememberSessionDuration)
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
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: true}, "tok", "1.2.3.4", "test-agent")
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

func TestResolveSessionRefreshesAdminWhenCheckTimestampIsInFuture(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	urow, err := store.UpsertUserByMediaServerID(context.Background(), db, store.UserRow{MediaServerUserID: "jf-clock-skew", Username: "admin", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	checker := &checkerStub{isAdmin: false}
	svc := NewService(db, security.NewCrypto("12345678901234567890123456789012"), testSessionSecret, checker)
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, IsAdmin: true}, "tok", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	idHash := security.HashSessionID(testSessionSecret, raw)
	if err := store.UpdateSessionAdminCheckedAt(context.Background(), db, idHash, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	_, user, err := svc.ResolveSession(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if user.IsAdmin || checker.calls != 1 {
		t.Fatalf("future check timestamp bypassed permission refresh: user=%+v calls=%d", user, checker.calls)
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
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: true}, "tok", "1.2.3.4", "test-agent")
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
	raw, err := svc.CreateSession(context.Background(), User{ID: urow.ID, MediaServerUserID: urow.MediaServerUserID, Username: urow.Username, DisplayName: urow.DisplayName, IsAdmin: false}, "tok", "1.2.3.4", "test-agent")
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
