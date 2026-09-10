package mediaserver

import (
	"fmt"
	"net/http"

	"github.com/mayvqt/veyra/internal/buildinfo"
)

func authorizeWithToken(req *http.Request, token string) {
	if token == "" {
		return
	}
	req.Header.Set("X-Emby-Token", token)
}

func clientAuthorizationHeader(scheme, deviceID, token string) string {
	header := fmt.Sprintf(
		`%s Client="Veyra", Device="Web", DeviceId=%q, Version=%q`,
		scheme,
		deviceID,
		buildinfo.Version,
	)
	if token != "" {
		header += fmt.Sprintf(`, Token=%q`, token)
	}
	return header
}
