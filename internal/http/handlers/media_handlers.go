package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mayvqt/veyra/internal/dashboard"
	"github.com/mayvqt/veyra/internal/security"
)

func limitMediaItems(items []dashboard.MediaItem, limit int) []dashboard.MediaItem {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func (h *Handlers) fetchMediaRecentlyAddedWithAPIKey(ctx context.Context, mediaserverUserID string) ([]dashboard.MediaItem, error) {
	if !h.mediaserver.HasAPIKey() {
		h.log.Warn("recently added fetch skipped: mediaserver api key missing", "user_id", mediaserverUserID)
		return nil, fmt.Errorf("media server API key is missing")
	}
	items, err := h.mediaserver.RecentlyAdded(ctx, mediaserverUserID, "", dashboardRecentMediaLimit)
	if err != nil {
		h.log.Warn("recently added fetch failed", "user_id", mediaserverUserID, "mode", "api_key", "err", security.RedactErr(err))
		return nil, err
	}
	return limitMediaItems(items, dashboardRecentMediaLimit), nil
}

func (h *Handlers) MediaServerPoster(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	if itemID == "" {
		http.NotFound(w, r)
		return
	}
	if !h.mediaserver.HasAPIKey() {
		http.Error(w, "poster unavailable", http.StatusBadGateway)
		return
	}
	img, err := h.mediaserver.PrimaryImage(r.Context(), itemID, r.URL.Query().Get("tag"), "", 360)
	if err != nil {
		http.Error(w, "poster unavailable", http.StatusBadGateway)
		return
	}
	defer img.Body.Close()
	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = io.Copy(w, img.Body)
}
