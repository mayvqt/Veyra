package mediaserver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mayvqt/veyra/internal/buildinfo"
	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/testutil"
)

func TestFetchUserAdminStatusEscapesUserIDPath(t *testing.T) {
	client := newTestClient(t, config.MediaServerJellyfin, "http://mediaserver.local", "", "")
	client.http = &http.Client{Transport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.EscapedPath() != "/Users/user%2Fwith%20slash" {
			t.Fatalf("expected escaped user path, got path=%q escaped=%q", req.URL.Path, req.URL.EscapedPath())
		}
		if got := req.Header.Get("X-Emby-Token"); got != "token" {
			t.Fatalf("expected user token, got %q", got)
		}
		return response(http.StatusOK, `{"Policy":{"IsAdministrator":true}}`), nil
	})}

	isAdmin, err := client.FetchUserAdminStatus(context.Background(), "user/with slash", "token")
	if err != nil {
		t.Fatal(err)
	}
	if !isAdmin {
		t.Fatal("expected administrator status")
	}
}

func TestProviderLoginAuthorizationIsExplicit(t *testing.T) {
	tests := []struct {
		name       string
		serverType config.MediaServerType
		scheme     string
		deviceID   string
	}{
		{name: "jellyfin", serverType: config.MediaServerJellyfin, scheme: "MediaBrowser", deviceID: "veyra-jellyfin"},
		{name: "emby", serverType: config.MediaServerEmby, scheme: "Emby", deviceID: "veyra-emby"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, test.serverType, "http://mediaserver.local", "", "")
			client.http = &http.Client{Transport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				header := req.Header.Get("Authorization")
				for _, expected := range []string{
					test.scheme + " ",
					`DeviceId="` + test.deviceID + `"`,
					`Version="` + buildinfo.Version + `"`,
				} {
					if !strings.Contains(header, expected) {
						t.Fatalf("expected %q in authorization header %q", expected, header)
					}
				}
				return response(http.StatusOK, `{"AccessToken":"token","User":{"Id":"user-1","Name":"Angel","Policy":{"IsAdministrator":true}}}`), nil
			})}

			user, token, err := client.Authenticate(context.Background(), "Angel", "secret")
			if err != nil {
				t.Fatal(err)
			}
			if token != "token" || user.ID != "user-1" || user.Username != "Angel" || !user.IsAdmin {
				t.Fatalf("unexpected authentication result: user=%+v token=%q", user, token)
			}
		})
	}
}

func TestAuthenticateRejectsMalformedSuccessResponse(t *testing.T) {
	client := newTestClient(t, config.MediaServerEmby, "http://mediaserver.local", "", "")
	client.http = &http.Client{Transport: testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"AccessToken":"","User":{"Id":"","Name":""}}`), nil
	})}
	if _, _, err := client.Authenticate(context.Background(), "user", "password"); err == nil {
		t.Fatal("expected malformed authentication response to fail")
	}
}

func TestLogoutRevokesProviderToken(t *testing.T) {
	client := newTestClient(t, config.MediaServerEmby, "http://mediaserver.local", "", "")
	client.http = &http.Client{Transport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/Sessions/Logout" {
			t.Fatalf("unexpected logout request: %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("X-Emby-Token"); got != "token" {
			t.Fatalf("expected logout token, got %q", got)
		}
		return response(http.StatusNoContent, ""), nil
	})}
	if err := client.Logout(context.Background(), "token"); err != nil {
		t.Fatal(err)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
