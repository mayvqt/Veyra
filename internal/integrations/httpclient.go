package integrations

import (
	"encoding/json"
	"fmt"
	"io"
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

// DecodeJSON reads and decodes exactly one JSON value while enforcing a hard
// response-size limit. Reading one extra byte lets callers distinguish a body
// at the limit from a truncated body.
func DecodeJSON(body io.Reader, out any, maxBytes int64) error {
	if maxBytes <= 0 {
		return fmt.Errorf("JSON response limit must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return fmt.Errorf("read JSON response: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("JSON response exceeds %d bytes", maxBytes)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}
	return nil
}
