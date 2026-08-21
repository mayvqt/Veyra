package mediaserver

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/mayvqt/veyra/internal/config"
)

type jellyfinProvider struct{}

func (jellyfinProvider) Name() string { return config.MediaServerJellyfin.Label() }

func (jellyfinProvider) AuthorizeLogin(req *http.Request) {
	req.Header.Set("Authorization", clientAuthorizationHeader("MediaBrowser", "veyra-jellyfin"))
}

func (jellyfinProvider) Authorize(req *http.Request, token string) {
	authorizeWithToken(req, token)
}

func (jellyfinProvider) LatestItemsPath(userID, itemTypes string, limit int) string {
	query := url.Values{}
	query.Set("userId", userID)
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Set("fields", "DateCreated,ProductionYear")
	query.Set("includeItemTypes", itemTypes)
	query.Set("enableImages", "true")
	query.Set("imageTypeLimit", "1")
	query.Set("enableImageTypes", "Primary")
	query.Set("groupItems", "false")
	return "/Items/Latest?" + query.Encode()
}

func (jellyfinProvider) VirtualFoldersPath() string {
	return "/Library/VirtualFolders"
}

func (jellyfinProvider) UsersPath() string {
	return "/Users"
}

func (jellyfinProvider) ItemURL(publicURL, itemID string) string {
	return publicURL + "/web/#/details?id=" + url.QueryEscape(itemID)
}
