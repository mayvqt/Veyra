package mediaserver

import (
	"fmt"
	"net/http"

	"github.com/mayvqt/veyra/internal/config"
)

type provider interface {
	Name() string
	AuthorizeLogin(*http.Request)
	Authorize(*http.Request, string)
	LatestItemsPath(userID, itemTypes string, limit int) string
	UsersPaths() []string
	VirtualFoldersPaths() []string
	ItemURL(publicURL, itemID string) string
}

func newProvider(serverType config.MediaServerType) (provider, error) {
	switch serverType {
	case config.MediaServerJellyfin:
		return jellyfinProvider{}, nil
	case config.MediaServerEmby:
		return embyProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported media server type %q", serverType)
	}
}
