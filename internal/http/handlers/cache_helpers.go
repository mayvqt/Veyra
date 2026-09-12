package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

// Numeric upstream identities and cached data belong to the configuration
// that produced them, including across restarts and environment changes.
func integrationCacheNamespace(cfg config.Config) string {
	values, _ := json.Marshal([]string{cfg.MediaServerType.String(), cfg.MediaServerURL, cfg.MediaServerAPIKey,
		cfg.MediaServerPublicURL, cfg.SeerrURL, cfg.SeerrAPIKey, cfg.SeerrPublicURL,
		cfg.SonarrURL, cfg.SonarrAPIKey, cfg.RadarrURL, cfg.RadarrAPIKey, cfg.ProwlarrURL, cfg.ProwlarrAPIKey})
	return fmt.Sprintf("integrations:v3:%x:", sha256.Sum256(values))
}

func (h *Handlers) cacheKey(key string) string { return h.cacheNamespace + key }

type cacheLoad struct {
	done     chan struct{}
	value    []byte
	stale    bool
	canceled bool
	err      error
	expires  time.Time
}

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
	if err := cacheSetJSON(ctx, h.db, h.cacheKey(key), v, ttl); err != nil {
		h.debug("cache write failed", "key", key, "err", security.RedactErr(err))
	}
}

func cacheLoadJSON[T any](h *Handlers, ctx context.Context, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	value, _, err := cacheLoadJSONWithStale(h, ctx, key, ttl, 0, load)
	return value, err
}

func cacheLoadJSONWithStale[T any](h *Handlers, ctx context.Context, key string, ttl, maxStale time.Duration, load func() (T, error)) (T, bool, error) {
	var value T
	if err := ctx.Err(); err != nil {
		return value, false, err
	}
	if cacheGetJSON(ctx, h.db, h.cacheKey(key), &value) {
		return value, false, nil
	}
	h.cacheMu.Lock()
	if h.cacheLoads == nil {
		h.cacheLoads = make(map[string]*cacheLoad)
	}
	if pending, ok := h.cacheLoads[key]; ok && (pending.expires.IsZero() || time.Now().Before(pending.expires)) {
		h.cacheMu.Unlock()
		select {
		case <-ctx.Done():
			return value, false, ctx.Err()
		case <-pending.done:
			// A disconnected leader must not cancel another user's live request.
			if pending.canceled && ctx.Err() == nil {
				return cacheLoadJSONWithStale(h, ctx, key, ttl, maxStale, load)
			}
			if err := json.Unmarshal(pending.value, &value); err != nil && pending.err == nil {
				return value, false, err
			}
			return value, pending.stale, pending.err
		}
	}
	// Bound retained failures; in-flight entries live only until completion.
	for k, entry := range h.cacheLoads {
		if !entry.expires.IsZero() && (time.Now().After(entry.expires) || len(h.cacheLoads) >= 256) {
			delete(h.cacheLoads, k)
		}
	}
	pending := &cacheLoad{done: make(chan struct{})}
	h.cacheLoads[key] = pending
	h.cacheMu.Unlock()
	failed := false
	defer func() {
		if recovered := recover(); recovered != nil {
			pending.err = fmt.Errorf("cache loader panicked")
			h.finishCacheLoad(key, pending, false)
			panic(recovered)
		}
		pending.canceled = ctx.Err() != nil
		h.finishCacheLoad(key, pending, failed && !pending.canceled)
	}()
	if !cacheGetJSON(ctx, h.db, h.cacheKey(key), &value) {
		var err error
		value, err = load()
		failed = err != nil
		pending.err = err
		if err == nil {
			h.cacheSetJSON(ctx, key, value, ttl)
		} else {
			var previous T
			if cacheGetStaleJSON(ctx, h.db, h.cacheKey(key), maxStale, &previous) {
				h.debug("serving stale cache after load failure", "key", key, "err", security.RedactErr(err))
				value, pending.stale, pending.err = previous, true, nil
			}
		}
	}
	var marshalErr error
	pending.value, marshalErr = json.Marshal(value)
	if marshalErr != nil && pending.err == nil {
		pending.err = marshalErr
	}
	return value, pending.stale, pending.err
}

func (h *Handlers) finishCacheLoad(key string, pending *cacheLoad, retain bool) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if retain {
		pending.expires = time.Now().Add(5 * time.Second)
	} else {
		delete(h.cacheLoads, key)
	}
	close(pending.done)
}
