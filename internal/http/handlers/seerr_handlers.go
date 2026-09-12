package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mayvqt/veyra/internal/http/middleware"
	"github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/security"
	"github.com/mayvqt/veyra/internal/store"
)

const maxSeerrSearchResults = 6

type seerrSearchResponse struct {
	Results []seerr.SearchResult `json:"results"`
	Error   string               `json:"error,omitempty"`
}

type seerrCreateRequestResponse struct {
	RequestID int    `json:"requestId,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (h *Handlers) SeerrSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	w.Header().Set("Content-Type", "application/json")
	if len(query) < 2 {
		_ = json.NewEncoder(w).Encode(seerrSearchResponse{Results: []seerr.SearchResult{}})
		return
	}
	results, err := h.seerr.Search(r.Context(), query, maxSeerrSearchResults)
	if err != nil {
		h.log.Warn("seerr search failed", "err", security.RedactErr(err))
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(seerrSearchResponse{Error: "Seerr search is unavailable right now."})
		return
	}
	_ = json.NewEncoder(w).Encode(seerrSearchResponse{Results: results})
}

func (h *Handlers) SeerrRequestPost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !middleware.ValidateCSRF(r) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Your session expired. Refresh and try again."})
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	mediaID, err := strconv.Atoi(strings.TrimSpace(r.FormValue("media_id")))
	if err != nil || mediaID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Choose a valid title first."})
		return
	}
	mediaType := strings.TrimSpace(r.FormValue("media_type"))
	if mediaType != "movie" && mediaType != "tv" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Only movies and shows can be requested."})
		return
	}
	seasons, err := parseSeasonSelection(r.FormValue("seasons"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Choose a valid season."})
		return
	}
	if mediaType == "tv" && len(seasons) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Choose at least one season."})
		return
	}
	if mediaType != "tv" {
		seasons = nil
	}

	seerrUser, err := h.resolveSeerrUser(r, user)
	if err != nil && !errors.Is(err, seerr.ErrUserNotLinked) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Seerr is unavailable right now. Try again shortly."})
		return
	}
	if seerrUser.ID <= 0 {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "Link this " + h.cfg.MediaServerType.Label() + " user in Seerr before requesting."})
		return
	}
	created, err := h.seerr.CreateRequest(r.Context(), seerr.CreateRequestInput{
		MediaID:   mediaID,
		MediaType: mediaType,
		UserID:    seerrUser.ID,
		Is4K:      false,
		Seasons:   seasons,
	})
	if err != nil {
		if errors.Is(err, seerr.ErrRequestConflict) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: "That title or season is already requested or available. Search again to see the current choices."})
			return
		}
		h.log.Warn("seerr request failed", "user", user.Username, "media_id", mediaID, "media_type", mediaType, "err", security.RedactErr(err))
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{Error: cleanSeerrClientError()})
		return
	}
	metadata := map[string]string{"media_type": mediaType, "seerr_request_id": strconv.Itoa(created.ID)}
	if len(seasons) > 0 {
		metadata["seasons"] = seasonNumbersString(seasons)
	}
	h.warnPersistence("audit media request", store.InsertAuditLog(r.Context(), h.db, &user.ID, "request.created", strconv.Itoa(mediaID), auditMetadata(metadata), middleware.ClientIP(r)))
	_ = json.NewEncoder(w).Encode(seerrCreateRequestResponse{RequestID: created.ID, Status: created.Status, Message: "Request sent to Seerr."})
}

func parseSeasonSelection(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if raw == "all" {
		return nil, errors.New("all seasons is not allowed")
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			return nil, errors.New("invalid season")
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

func seasonNumbersString(seasons []int) string {
	parts := make([]string, 0, len(seasons))
	for _, season := range seasons {
		parts = append(parts, strconv.Itoa(season))
	}
	return strings.Join(parts, ",")
}

func cleanSeerrClientError() string {
	return "Seerr could not create that request."
}
