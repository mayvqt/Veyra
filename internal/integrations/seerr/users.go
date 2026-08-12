package seerr

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) ResolveUser(ctx context.Context, username, displayName string) (*UserIdentity, error) {
	q := url.Values{}
	q.Set("take", "200")
	q.Set("skip", "0")
	q.Set("sort", "displayname")
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/user?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("seerr user list failed: %d", resp.StatusCode)
	}
	var payload userListDTO
	if err := decodeLimitedSeerrJSON(resp.Body, &payload); err != nil {
		return nil, err
	}
	for _, row := range payload.Results {
		if userMatches(row, username, displayName) {
			user, ok := userIdentity(row)
			if !ok {
				return nil, fmt.Errorf("seerr user not found")
			}
			return user, nil
		}
	}
	return nil, fmt.Errorf("seerr user not found")
}

func (c *Client) ResolveUserByMediaServerID(ctx context.Context, mediaServerUserID string) (*UserIdentity, error) {
	mediaServerUserID = strings.TrimSpace(mediaServerUserID)
	if mediaServerUserID == "" {
		return nil, fmt.Errorf("media server user id missing")
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
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("seerr media server user lookup failed: %d", resp.StatusCode)
	}
	var row userDTO
	if err := decodeLimitedSeerrJSON(resp.Body, &row); err != nil {
		return nil, err
	}
	user, ok := userIdentity(row)
	if !ok {
		return nil, fmt.Errorf("seerr user not found")
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

func userMatches(row userDTO, names ...string) bool {
	for _, value := range []string{row.DisplayName, row.Username, row.PlexUsername, row.JellyfinUsername, row.Email} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		for _, name := range names {
			if sameLooseName(value, name) {
				return true
			}
		}
	}
	return false
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

func sameLooseName(a, b string) bool {
	return normalizeName(a) != "" && normalizeName(a) == normalizeName(b)
}

func normalizeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.Index(s, "@"); i > 0 {
		s = s[:i]
	}
	return strings.Join(strings.Fields(s), " ")
}
