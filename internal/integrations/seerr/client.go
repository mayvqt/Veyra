package seerr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/integrations"
)

type Client struct {
	baseURL   string
	publicURL string
	apiKey    string
	http      *http.Client
}

var errNotConfigured = errors.New("seerr is not configured")

func NewClient(baseURL, publicURL, apiKey string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		publicURL: strings.TrimRight(publicURL, "/"),
		apiKey:    strings.TrimSpace(apiKey),
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) ID() string   { return "seerr" }
func (c *Client) Name() string { return "Seerr" }
func (c *Client) Kind() string { return "requests" }

func (c *Client) Health(ctx context.Context) integrations.HealthStatus {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/status", nil)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: healthErrorMessage(err)}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return integrations.HealthStatus{OK: true, Message: "Online"}
	}
	return integrations.HealthStatus{OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func (c *Client) setAPIKey(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if err := c.requireConfigured(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	c.setAPIKey(req)
	return req, nil
}

func (c *Client) configured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

func (c *Client) requireConfigured() error {
	if c.configured() {
		return nil
	}
	return errNotConfigured
}

func healthErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, errNotConfigured) {
		return "not configured"
	}
	return err.Error()
}
