package integrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPStatusError struct {
	Service    string
	Operation  string
	StatusCode int
	Detail     string
}

func (e *HTTPStatusError) Error() string {
	status := http.StatusText(e.StatusCode)
	if status == "" {
		status = "unknown status"
	}
	prefix := e.Service
	if e.Operation != "" {
		prefix += " " + e.Operation
	}
	message := fmt.Sprintf("%s failed: HTTP %d %s", prefix, e.StatusCode, status)
	if e.Detail != "" {
		message += ": " + e.Detail
	}
	return message
}

func NewHTTPStatusErrorWithDetail(service, operation string, statusCode int, detail string) error {
	return &HTTPStatusError{Service: service, Operation: operation, StatusCode: statusCode, Detail: detail}
}

func NewHTTPStatusError(service, operation string, statusCode int) error {
	return &HTTPStatusError{Service: service, Operation: operation, StatusCode: statusCode}
}

func IsHTTPStatus(err error, statusCode int) bool {
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == statusCode
}

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
