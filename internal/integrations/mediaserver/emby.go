package mediaserver

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/mayvqt/veyra/internal/config"
)

type embyProvider struct{}

func (embyProvider) Name() string { return config.MediaServerEmby.Label() }

func (embyProvider) AuthorizeLogin(req *http.Request) {
	req.Header.Set("Authorization", clientAuthorizationHeader("Emby", "veyra-emby"))
}

func (embyProvider) Authorize(req *http.Request, token string) {
	authorizeWithToken(req, token)
}

func (embyProvider) LatestItemsPath(userID, itemTypes string, limit int) string {
	query := url.Values{}
	query.Set("Limit", fmt.Sprintf("%d", limit))
	query.Set("Fields", "DateCreated,ProductionYear")
	query.Set("IncludeItemTypes", itemTypes)
	query.Set("EnableImages", "true")
	query.Set("ImageTypeLimit", "1")
	query.Set("EnableImageTypes", "Primary")
	query.Set("GroupItems", "false")
	return fmt.Sprintf("/Users/%s/Items/Latest?%s", url.PathEscape(userID), query.Encode())
}

func (embyProvider) VirtualFoldersPath() string {
	return "/Library/VirtualFolders/Query"
}

func (embyProvider) UsersPath() string {
	return "/Users/Query"
}

func (embyProvider) ItemURL(publicURL, itemID string) string {
	return publicURL + "/web/index.html#!/item?id=" + url.QueryEscape(itemID)
}
