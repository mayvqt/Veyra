package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestCacheSetGetExpiry(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	if err := UpsertCache(context.Background(), db, "k", `{"v":1}`, time.Second); err != nil {
		t.Fatal(err)
	}
	v, ok, err := GetCache(context.Background(), db, "k")
	if err != nil || !ok || v == "" {
		t.Fatalf("expected cache hit, got ok=%v err=%v v=%q", ok, err, v)
	}

	if err := UpsertCache(context.Background(), db, "k2", `{"v":2}`, -1*time.Second); err != nil {
		t.Fatal(err)
	}
	_, ok, err = GetCache(context.Background(), db, "k2")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected expired cache miss")
	}

	var remaining int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM cache_entries WHERE key = ?`, "k2").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected expired row to remain until cleanup, got %d", remaining)
	}
}

func TestDeleteExpiredCache(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	if err := UpsertCache(context.Background(), db, "fresh", `{"v":1}`, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := UpsertCache(context.Background(), db, "expired", `{"v":2}`, -time.Hour); err != nil {
		t.Fatal(err)
	}

	n, err := DeleteExpiredCache(context.Background(), db, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected to delete 1 expired row, deleted %d", n)
	}

	var remaining int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM cache_entries`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 row to remain, got %d", remaining)
	}
}
