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
