package handlers

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/mayvqt/veyra/internal/store"
)

type x struct{ A int }

func TestCacheJSONHelpers(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	cacheSetJSON(context.Background(), db, "k", x{A: 9}, time.Second)
	var out x
	if !cacheGetJSON(context.Background(), db, "k", &out) {
		t.Fatal("expected cache hit")
	}
	if out.A != 9 {
		t.Fatalf("expected 9 got %d", out.A)
	}

	cacheSetJSON(context.Background(), db, "k2", x{A: 3}, -1*time.Second)
	var out2 x
	if cacheGetJSON(context.Background(), db, "k2", &out2) {
		t.Fatal("expected expired miss")
	}
}

func TestCacheSetJSONReturnsMarshalError(t *testing.T) {
	err := cacheSetJSON(context.Background(), nil, "bad", make(chan int), time.Second)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal cache value") {
		t.Fatalf("expected marshal context, got %v", err)
	}
}

func TestCacheHelpersTreatNilDBAsDisabledCache(t *testing.T) {
	if cacheGetJSON(context.Background(), nil, "missing", &x{}) {
		t.Fatal("expected nil db cache miss")
	}
	if err := cacheSetJSON(context.Background(), nil, "missing", x{A: 1}, time.Second); err != nil {
		t.Fatalf("expected nil db cache write to be ignored, got %v", err)
	}
}

func TestCacheSetJSONReturnsStoreError(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	err = cacheSetJSON(context.Background(), db, "k", x{A: 1}, time.Second)
	if err == nil {
		t.Fatal("expected store error")
	}
	if !strings.Contains(err.Error(), "upsert cache value") {
		t.Fatalf("expected store context, got %v", err)
	}
}
