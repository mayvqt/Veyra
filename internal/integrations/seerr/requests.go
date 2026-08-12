package seerr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/dashboard"
)

const (
	seerrRequestStatusPending   = 1
	seerrRequestStatusApproved  = 2
	seerrRequestStatusDeclined  = 3
	seerrRequestStatusFailed    = 4
	seerrRequestStatusAvailable = 5

	seerrMediaStatusPending            = 2
	seerrMediaStatusProcessing         = 3
	seerrMediaStatusPartiallyAvailable = 4
	seerrMediaStatusAvailable          = 5

	requestLifecycleStateDone    = "done"
	requestLifecycleStateCurrent = "current"
	requestLifecycleStateTodo    = "todo"
)

func (c *Client) RecentRequests(ctx context.Context, limit int) ([]dashboard.RequestItem, error) {
	return c.recentRequestsFrom(ctx, requestListPath(limit, 0), limit)
}

func (c *Client) RecentRequestsForUser(ctx context.Context, user UserIdentity, limit int) ([]dashboard.RequestItem, error) {
	if limit <= 0 {
		limit = 10
	}
	if user.ID > 0 {
		return c.recentRequestsFrom(ctx, requestListPath(limit, user.ID), limit)
	}
	reqs, err := c.recentRequestsFrom(ctx, requestListPath(limit*4, 0), limit*4)
	if err != nil {
		return nil, err
	}
	out := make([]dashboard.RequestItem, 0, limit)
	for _, req := range reqs {
		if sameLooseName(req.User, user.Username) || sameLooseName(req.User, user.DisplayName) {
			out = append(out, req)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func requestListPath(limit, requestedBy int) string {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("take", strconv.Itoa(limit))
	q.Set("skip", "0")
	q.Set("sort", "added")
	if requestedBy > 0 {
		q.Set("requestedBy", strconv.Itoa(requestedBy))
	}
	return "/api/v1/request?" + q.Encode()
}

func (c *Client) recentRequestsFrom(ctx context.Context, path string, limit int) ([]dashboard.RequestItem, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("seerr request list failed: %d", resp.StatusCode)
	}
	var payload requestResp
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]dashboard.RequestItem, 0, len(payload.Results))
	for _, r := range payload.Results {
		title := requestTitle(r)
		mediaType := requestMediaType(r)
		statusCode, _ := directInt(r, "status")
		mediaStatusCode := requestMediaStatus(r)
		createdAt, _ := directTime(r, "createdAt")
		user := requestUser(r)
		if title == "" {
			title = c.resolveMediaTitle(ctx, mediaType, r)
		}
		if title == "" {
			title = fallbackRequestTitle(mediaType, r)
		}
		status := lifecycleStatus(requestStatusCode(statusCode), mediaStatusCode)
		out = append(out, dashboard.RequestItem{
			Title:     title,
			Status:    status.Label,
			StatusKey: status.Key,
			Media:     mediaType,
			Created:   createdAt,
			User:      user,
			Lifecycle: requestLifecycle(requestStatusCode(statusCode), mediaStatusCode),
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (c *Client) CreateRequest(ctx context.Context, in CreateRequestInput) (CreatedRequest, error) {
	mediaType := normalizeSeerrMediaType(in.MediaType)
	if in.MediaID <= 0 || mediaType == "" {
		return CreatedRequest{}, fmt.Errorf("invalid request target")
	}
	payload := map[string]any{
		"mediaId":   in.MediaID,
		"mediaType": mediaType,
		"is4k":      in.Is4K,
	}
	if mediaType == "tv" {
		if len(in.Seasons) > 0 {
			payload["seasons"] = in.Seasons
		} else {
			return CreatedRequest{}, fmt.Errorf("choose at least one season")
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return CreatedRequest{}, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/request", bytes.NewReader(body))
	if err != nil {
		return CreatedRequest{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if in.UserID > 0 {
		req.Header.Set("X-Api-User", strconv.Itoa(in.UserID))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return CreatedRequest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return CreatedRequest{}, fmt.Errorf("seerr request failed: %s", seerrHTTPError(resp))
	}
	var row map[string]any
	if err := decodeSeerrJSON(resp, &row); err != nil {
		return CreatedRequest{}, err
	}
	id, _ := directInt(row, "id")
	statusCode, _ := directInt(row, "status")
	return CreatedRequest{ID: id, Status: statusLabel(statusCode)}, nil
}

func requestTitle(row map[string]any) string {
	for _, path := range [][]string{
		{"media", "title"},
		{"media", "name"},
		{"media", "originalTitle"},
		{"media", "originalName"},
		{"media", "movie", "title"},
		{"media", "tv", "name"},
		{"media", "show", "name"},
		{"movie", "title"},
		{"tv", "name"},
		{"title"},
		{"name"},
	} {
		if s, ok := stringPath(row, path...); ok && s != "" {
			return s
		}
	}
	return ""
}

func requestMediaType(row map[string]any) string {
	for _, path := range [][]string{
		{"media", "mediaType"},
		{"media", "type"},
		{"type"},
	} {
		if s, ok := stringPath(row, path...); ok && s != "" {
			return normalizeMediaType(s)
		}
	}
	return "Request"
}

func requestUser(row map[string]any) string {
	for _, path := range [][]string{
		{"requestedBy", "displayName"},
		{"requestedBy", "username"},
		{"requestedBy", "plexUsername"},
		{"requestedBy", "jellyfinUsername"},
		{"requestedBy", "embyUsername"},
		{"modifiedBy", "displayName"},
		{"modifiedBy", "username"},
	} {
		if s, ok := stringPath(row, path...); ok && s != "" {
			return s
		}
	}
	return ""
}

func requestMediaStatus(row map[string]any) int {
	for _, path := range [][]string{
		{"media", "status"},
		{"mediaInfo", "status"},
	} {
		if n, ok := intPath(row, path...); ok {
			return n
		}
	}
	return 0
}

func (c *Client) resolveMediaTitle(ctx context.Context, mediaType string, row map[string]any) string {
	tmdbID := mediaID(row, "tmdbId")
	if tmdbID <= 0 {
		return ""
	}
	path := ""
	switch strings.ToLower(mediaType) {
	case "movie":
		path = fmt.Sprintf("/api/v1/movie/%d", tmdbID)
	case "series", "tv":
		path = fmt.Sprintf("/api/v1/tv/%d", tmdbID)
	default:
		return ""
	}
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return ""
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	for _, field := range []string{"title", "name", "originalTitle", "originalName"} {
		if s, ok := stringPath(payload, field); ok && s != "" {
			return s
		}
	}
	return ""
}

func mediaID(row map[string]any, key string) int {
	for _, path := range [][]string{
		{"media", key},
		{key},
	} {
		if n, ok := intPath(row, path...); ok {
			return n
		}
	}
	return 0
}

func fallbackRequestTitle(mediaType string, row map[string]any) string {
	for _, path := range [][]string{
		{"media", "tmdbId"},
		{"media", "tvdbId"},
		{"media", "id"},
	} {
		if n, ok := intPath(row, path...); ok && n > 0 {
			return fmt.Sprintf("%s #%d", mediaType, n)
		}
	}
	return "Untitled request"
}

func normalizeMediaType(s string) string {
	trimmed := strings.TrimSpace(s)
	switch strings.ToLower(trimmed) {
	case "movie":
		return "Movie"
	case "tv", "series":
		return "Series"
	default:
		if trimmed == "" {
			return "Request"
		}
		return strings.ToUpper(trimmed[:1]) + trimmed[1:]
	}
}

func statusLabel(code int) string {
	switch code {
	case seerrRequestStatusPending:
		return "Pending"
	case seerrRequestStatusApproved:
		return "Approved"
	case seerrRequestStatusDeclined:
		return "Declined"
	case seerrRequestStatusFailed:
		return "Failed"
	case seerrRequestStatusAvailable:
		return "Available"
	default:
		return "Unknown"
	}
}

type requestStatusCode int

type requestLifecycleStatus struct {
	Label string
	Key   string
}

func lifecycleStatus(requestStatus requestStatusCode, mediaStatus int) requestLifecycleStatus {
	switch requestStatus {
	case seerrRequestStatusDeclined:
		return requestLifecycleStatus{Label: "Declined", Key: "declined"}
	case seerrRequestStatusFailed:
		return requestLifecycleStatus{Label: "Failed", Key: "failed"}
	case seerrRequestStatusAvailable:
		return requestLifecycleStatus{Label: "Available", Key: "available"}
	}
	switch mediaStatus {
	case seerrMediaStatusPending:
		return requestLifecycleStatus{Label: "Approved", Key: "approved"}
	case seerrMediaStatusProcessing:
		return requestLifecycleStatus{Label: "Downloading", Key: "downloading"}
	case seerrMediaStatusPartiallyAvailable:
		return requestLifecycleStatus{Label: "Partially available", Key: "partial"}
	case seerrMediaStatusAvailable:
		return requestLifecycleStatus{Label: "Available", Key: "available"}
	}
	if requestStatus == seerrRequestStatusApproved {
		return requestLifecycleStatus{Label: "Approved", Key: "approved"}
	}
	if requestStatus == seerrRequestStatusPending {
		return requestLifecycleStatus{Label: "Submitted", Key: "submitted"}
	}
	return requestLifecycleStatus{Label: statusLabel(int(requestStatus)), Key: "unknown"}
}

func requestLifecycle(requestStatus requestStatusCode, mediaStatus int) []dashboard.RequestLifecycleStep {
	steps := []dashboard.RequestLifecycleStep{
		{Label: "Submitted", Key: "submitted", State: requestLifecycleStateTodo},
		{Label: "Approved", Key: "approved", State: requestLifecycleStateTodo},
		{Label: "Downloading", Key: "downloading", State: requestLifecycleStateTodo},
		{Label: "Partial", Key: "partial", State: requestLifecycleStateTodo},
		{Label: "Available", Key: "available", State: requestLifecycleStateTodo},
	}

	current := lifecycleCurrentIndex(requestStatus, mediaStatus)
	if requestStatus == seerrRequestStatusDeclined || requestStatus == seerrRequestStatusFailed {
		current = 0
	}
	for i := range steps {
		switch {
		case i < current:
			steps[i].State = requestLifecycleStateDone
		case i == current:
			steps[i].State = requestLifecycleStateCurrent
		}
	}
	return steps
}

func lifecycleCurrentIndex(requestStatus requestStatusCode, mediaStatus int) int {
	if requestStatus == seerrRequestStatusAvailable || mediaStatus == seerrMediaStatusAvailable {
		return 4
	}
	switch mediaStatus {
	case seerrMediaStatusPartiallyAvailable:
		return 3
	case seerrMediaStatusProcessing:
		return 2
	}
	if requestStatus == seerrRequestStatusApproved {
		return 1
	}
	return 0
}

func directTime(row map[string]any, key string) (time.Time, bool) {
	switch v := row[key].(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		return t, err == nil
	case time.Time:
		return v, true
	default:
		return time.Time{}, false
	}
}
