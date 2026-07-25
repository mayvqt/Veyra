package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUserSessionSettingsAuditFlows(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	defer db.Close()

	u, err := UpsertUserByMediaServerID(ctx, db, UserRow{MediaServerUserID: "jf1", Username: "angel", DisplayName: "Angel", IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == 0 {
		t.Fatal("expected user id")
	}

	now := time.Now().UTC()
	err = InsertSession(ctx, db, SessionRow{IDHash: "hash1", UserID: u.ID, MediaServerAccessTokenEncrypted: "enc", ExpiresAt: now.Add(time.Hour), CreatedAt: now, LastSeenAt: now, IPAddress: "1.1.1.1", UserAgent: "ua"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := GetSession(ctx, db, "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != u.ID {
		t.Fatal("session user mismatch")
	}

	if err := UpsertSetting(ctx, db, "app.name", "VeyraX"); err != nil {
		t.Fatal(err)
	}
	v, err := GetSetting(ctx, db, "app.name")
	if err != nil || v != "VeyraX" {
		t.Fatalf("unexpected setting: %v %s", err, v)
	}
	if err := UpsertSetting(ctx, db, "mediaserver.public_url", "https://watch.example"); err != nil {
		t.Fatal(err)
	}
	settings, err := GetSettings(ctx, db, []string{"app.name", "mediaserver.public_url", "app.name", "missing.setting"})
	if err != nil {
		t.Fatal(err)
	}
	if settings["app.name"] != "VeyraX" {
		t.Fatalf("unexpected app.name: %q", settings["app.name"])
	}
	if settings["mediaserver.public_url"] != "https://watch.example" {
		t.Fatalf("unexpected mediaserver.public_url: %q", settings["mediaserver.public_url"])
	}
	if _, ok := settings["missing.setting"]; ok {
		t.Fatal("did not expect missing setting to be present")
	}

	uid := u.ID
	if err := InsertAuditLog(ctx, db, &uid, "login.success", "session", "{}", "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	logs, err := ListRecentAuditLogs(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Fatal("expected audit logs")
	}
	if logs[0].Username.String != "angel" || logs[0].Display.String != "Angel" {
		t.Fatalf("expected audit log to include actor fields, got %+v", logs[0])
	}

	serviceLogs, err := ListRecentAuditLogsByActionTarget(ctx, db, "login.success", "session", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(serviceLogs) == 0 {
		t.Fatal("expected filtered audit logs")
	}
	if serviceLogs[0].Username.String != "angel" {
		t.Fatalf("expected filtered audit log username, got %+v", serviceLogs[0])
	}

	users, err := ListUsers(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatal("expected one user")
	}
	if _, err := UpsertUserByMediaServerID(ctx, db, UserRow{MediaServerUserID: "jf2", Username: "casey", DisplayName: "Casey", IsAdmin: false}); err != nil {
		t.Fatal(err)
	}
	counts, err := CountUsers(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Total != 2 || counts.Admin != 1 {
		t.Fatalf("unexpected user counts: %+v", counts)
	}
}
