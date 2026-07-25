package seerr

import (
	"context"
	"encoding/json"
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
	var payload requestResp
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
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
	var row map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&row); err != nil {
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

func userIdentity(row map[string]any) (*UserIdentity, bool) {
	id, _ := directInt(row, "id")
	if id <= 0 {
		return nil, false
	}
	nameFields := seerrUsernameFields()
	return &UserIdentity{
		ID:          id,
		Username:    firstString(row, nameFields...),
		DisplayName: firstString(row, append([]string{"displayName"}, nameFields...)...),
	}, true
}

func userMatches(row map[string]any, names ...string) bool {
	for _, field := range append([]string{"displayName"}, seerrUsernameFields()...) {
		value, ok := stringPath(row, field)
		if !ok || value == "" {
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

func seerrUsernameFields() []string {
	return []string{"username", "plexUsername", "jellyfinUsername", "email"}
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
