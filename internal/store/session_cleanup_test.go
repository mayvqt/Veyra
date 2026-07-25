package store

import (
	"context"
	"testing"
	"time"
)

func TestDeleteExpiredSessions(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	ctx := context.Background()
	user, err := UpsertUserByMediaServerID(ctx, db, UserRow{MediaServerUserID: "cleanup-user", Username: "cleanup"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, session := range []SessionRow{
		{IDHash: "expired", UserID: user.ID, ExpiresAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Hour), LastSeenAt: now.Add(-time.Hour), AbsoluteExpiresAt: now.Add(time.Hour)},
		{IDHash: "absolute-expired", UserID: user.ID, ExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Hour), LastSeenAt: now.Add(-time.Hour), AbsoluteExpiresAt: now.Add(-time.Minute)},
		{IDHash: "active", UserID: user.ID, ExpiresAt: now.Add(time.Hour), CreatedAt: now, LastSeenAt: now, AbsoluteExpiresAt: now.Add(time.Hour)},
	} {
		if err := InsertSession(ctx, db, session); err != nil {
			t.Fatal(err)
		}
	}
	deleted, err := DeleteExpiredSessions(ctx, db, now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted sessions = %d, want 2", deleted)
	}
	if _, err := GetSession(ctx, db, "active"); err != nil {
		t.Fatalf("active session was removed: %v", err)
	}
}
