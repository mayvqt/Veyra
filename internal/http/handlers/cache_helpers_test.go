package handlers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
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

func TestCacheLoadJSONCoalescesConcurrentMisses(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	db.SetMaxOpenConns(1)
	var calls atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan x, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := cacheLoadJSON(h, context.Background(), "shared", time.Minute, func() (x, error) {
				calls.Add(1)
				<-start
				return x{A: 42}, nil
			})
			if err != nil {
				t.Errorf("cache load failed: %v", err)
				return
			}
			results <- value
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	if calls.Load() != 1 {
		t.Fatalf("loader called %d times, want 1", calls.Load())
	}
	for result := range results {
		if result.A != 42 {
			t.Fatalf("unexpected cached result: %+v", result)
		}
	}
}

func TestCacheLoadJSONUsesBoundedStaleValueOnFailure(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	if err := cacheSetJSON(context.Background(), db, "stale", x{A: 7}, -time.Second); err != nil {
		t.Fatal(err)
	}
	value, stale, err := cacheLoadJSONWithStale(h, context.Background(), "stale", time.Minute, 5*time.Minute, func() (x, error) {
		return x{}, errors.New("upstream unavailable")
	})
	if err != nil || !stale || value.A != 7 {
		t.Fatalf("value=%+v stale=%v err=%v", value, stale, err)
	}
}

func TestCacheLoadJSONRejectsOverageStaleValue(t *testing.T) {
	h, db := mkHandlers(t)
	defer db.Close()
	if err := cacheSetJSON(context.Background(), db, "too-old", x{A: 7}, -time.Hour); err != nil {
		t.Fatal(err)
	}
	_, stale, err := cacheLoadJSONWithStale(h, context.Background(), "too-old", time.Minute, 5*time.Minute, func() (x, error) {
		return x{}, errors.New("upstream unavailable")
	})
	if err == nil || stale {
		t.Fatalf("stale=%v err=%v; want loader failure", stale, err)
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
