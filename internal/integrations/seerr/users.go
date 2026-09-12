package seerr

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/mayvqt/veyra/internal/integrations"
)

var ErrUserNotLinked = errors.New("media server account is not linked in Seerr")

func (c *Client) ResolveUserByMediaServerID(ctx context.Context, mediaServerUserID string) (*UserIdentity, error) {
	mediaServerUserID = strings.TrimSpace(mediaServerUserID)
	if mediaServerUserID == "" {
		return nil, ErrUserNotLinked
	}
	path := mediaServerUserLookupPath(mediaServerUserID)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrUserNotLinked
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, integrations.NewHTTPStatusError("Seerr", "media server user lookup", resp.StatusCode)
	}
	var row userDTO
	if err := decodeLimitedSeerrJSON(resp.Body, &row); err != nil {
		return nil, err
	}
	user, ok := userIdentity(row)
	if !ok {
		return nil, ErrUserNotLinked
	}
	return user, nil
}

func mediaServerUserLookupPath(mediaServerUserID string) string {
	// Seerr stores Jellyfin and Emby links in the same jellyfinUserId field and
	// exposes that lookup through this route for both server types.
	return "/api/v1/user/jellyfin/" + url.PathEscape(mediaServerUserID)
}

func userIdentity(row userDTO) (*UserIdentity, bool) {
	if row.ID <= 0 {
		return nil, false
	}
	return &UserIdentity{
		ID:          row.ID,
		Username:    firstNonEmptyString(row.Username, row.PlexUsername, row.JellyfinUsername, row.Email),
		DisplayName: firstNonEmptyString(row.DisplayName, row.Username, row.PlexUsername, row.JellyfinUsername, row.Email),
	}, true
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstString(row map[string]any, fields ...string) string {
	for _, field := range fields {
		if s, ok := stringPath(row, field); ok && s != "" {
			return s
		}
	}
	return ""
}
