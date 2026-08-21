package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

func cacheGetJSON[T any](ctx context.Context, db *sql.DB, key string, out *T) bool {
	if db == nil {
		return false
	}
	blob, ok, err := store.GetCache(ctx, db, key)
	if err != nil || !ok {
		return false
	}
	return json.Unmarshal([]byte(blob), out) == nil
}

func cacheGetStaleJSON[T any](ctx context.Context, db *sql.DB, key string, maxStale time.Duration, out *T) bool {
	if db == nil || maxStale <= 0 {
		return false
	}
	row, ok, err := store.GetCacheEntry(ctx, db, key)
	if err != nil || !ok {
		return false
	}
	now := time.Now().UTC()
	if row.ExpiresAt.After(now) || !now.Before(row.ExpiresAt.Add(maxStale)) {
		return false
	}
	return json.Unmarshal([]byte(row.ValueJSON), out) == nil
}

func cacheSetJSON(ctx context.Context, db *sql.DB, key string, v any, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	if db == nil {
		return nil
	}
	if err := store.UpsertCache(ctx, db, key, string(b), ttl); err != nil {
		return fmt.Errorf("upsert cache value: %w", err)
	}
	return nil
}

func (h *Handlers) cacheSetJSON(ctx context.Context, key string, v any, ttl time.Duration) {
	if err := cacheSetJSON(ctx, h.db, key, v, ttl); err != nil {
		h.debug("cache write failed", "key", key, "err", security.RedactErr(err))
	}
}

func cacheLoadJSON[T any](h *Handlers, ctx context.Context, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	value, _, err := cacheLoadJSONWithStale(h, ctx, key, ttl, 0, load)
	return value, err
}

func cacheLoadJSONWithStale[T any](h *Handlers, ctx context.Context, key string, ttl, maxStale time.Duration, load func() (T, error)) (T, bool, error) {
	var value T
	if cacheGetJSON(ctx, h.db, key, &value) {
		return value, false, nil
	}
	lock := h.cacheLock(key)
	lock.Lock()
	defer lock.Unlock()
	if cacheGetJSON(ctx, h.db, key, &value) {
		return value, false, nil
	}
	value, err := load()
	if err != nil {
		if cacheGetStaleJSON(ctx, h.db, key, maxStale, &value) {
			h.debug("serving stale cache after load failure", "key", key, "err", security.RedactErr(err))
			return value, true, nil
		}
		return value, false, err
	}
	h.cacheSetJSON(ctx, key, value, ttl)
	return value, false, nil
}
