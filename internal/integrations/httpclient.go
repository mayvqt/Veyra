package integrations

import (
	"net/http"
	"time"
)

// NewHTTPClient returns a bounded client that refuses redirects so credentials
// cannot be replayed to an unexpected or downgraded destination.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
