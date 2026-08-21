package mediaserver

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/mayvqt/veyra/internal/config"
)

func TestProviderLatestItemsRequests(t *testing.T) {
	tests := []struct {
		name       string
		serverType config.MediaServerType
		wantPath   string
		queryKeys  []string
	}{
		{
			name:       "jellyfin",
			serverType: config.MediaServerJellyfin,
			wantPath:   "/Items/Latest",
			queryKeys:  []string{"userId", "limit", "fields", "includeItemTypes", "enableImages", "imageTypeLimit", "enableImageTypes", "groupItems"},
		},
		{
			name:       "emby",
			serverType: config.MediaServerEmby,
			wantPath:   "/Users/user%2Fone/Items/Latest",
			queryKeys:  []string{"Limit", "Fields", "IncludeItemTypes", "EnableImages", "ImageTypeLimit", "EnableImageTypes", "GroupItems"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := newProvider(test.serverType)
			if err != nil {
				t.Fatal(err)
			}
			requestURL, err := url.Parse(provider.LatestItemsPath("user/one", "Movie", 12))
			if err != nil {
				t.Fatal(err)
			}
			if requestURL.EscapedPath() != test.wantPath {
				t.Fatalf("expected path %q, got %q", test.wantPath, requestURL.EscapedPath())
			}
			for _, key := range test.queryKeys {
				if _, ok := requestURL.Query()[key]; !ok {
					t.Fatalf("expected query key %q in %q", key, requestURL.RawQuery)
				}
			}
		})
	}
}

func TestProviderItemLinksAreProviderSpecific(t *testing.T) {
	jellyfin, _ := newProvider(config.MediaServerJellyfin)
	emby, _ := newProvider(config.MediaServerEmby)

	if got := jellyfin.ItemURL("https://watch.example", "item/one"); got != "https://watch.example/web/#/details?id=item%2Fone" {
		t.Fatalf("unexpected Jellyfin item URL %q", got)
	}
	if got := emby.ItemURL("https://watch.example", "item/one"); got != "https://watch.example/web/index.html#!/item?id=item%2Fone" {
		t.Fatalf("unexpected Emby item URL %q", got)
	}
}

func TestProviderVirtualFolderRoutesAreProviderSpecific(t *testing.T) {
	jellyfin, _ := newProvider(config.MediaServerJellyfin)
	emby, _ := newProvider(config.MediaServerEmby)

	if got := jellyfin.VirtualFoldersPath(); got != "/Library/VirtualFolders" {
		t.Fatalf("unexpected Jellyfin virtual folders path %q", got)
	}
	if got := emby.VirtualFoldersPath(); got != "/Library/VirtualFolders/Query" {
		t.Fatalf("unexpected Emby virtual folders path %q", got)
	}
}

func TestProviderUserRoutesAreProviderSpecific(t *testing.T) {
	jellyfin, _ := newProvider(config.MediaServerJellyfin)
	emby, _ := newProvider(config.MediaServerEmby)

	if got := jellyfin.UsersPath(); got != "/Users" {
		t.Fatalf("unexpected Jellyfin users path %q", got)
	}
	if got := emby.UsersPath(); got != "/Users/Query" {
		t.Fatalf("unexpected Emby users path %q", got)
	}
}

func TestProviderTokenAuthorizationIsShared(t *testing.T) {
	for _, serverType := range []config.MediaServerType{config.MediaServerJellyfin, config.MediaServerEmby} {
		provider, err := newProvider(serverType)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodGet, "https://watch.example/System/Ping", nil)
		if err != nil {
			t.Fatal(err)
		}
		provider.Authorize(req, "token")
		if req.Header.Get("X-Emby-Token") != "token" {
			t.Fatalf("%s did not apply the shared token headers", serverType)
		}
	}
}
