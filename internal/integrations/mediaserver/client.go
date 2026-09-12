package mediaserver

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/integrations"
)

type Client struct {
	provider  provider
	baseURL   string
	publicURL string
	apiKey    string
	http      *http.Client
}

type ImageResponse struct {
	Body        io.ReadCloser
	ContentType string
}

const mediaServerJSONLimit = 2 << 20

func NewClient(serverType config.MediaServerType, baseURL, publicURL, apiKey string) (*Client, error) {
	provider, err := newProvider(serverType)
	if err != nil {
		return nil, err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if err := validateServerURL("internal URL", baseURL); err != nil {
		return nil, err
	}
	if err := validateServerURL("public URL", publicURL); err != nil {
		return nil, err
	}
	return &Client{
		provider:  provider,
		baseURL:   baseURL,
		publicURL: publicURL,
		apiKey:    strings.TrimSpace(apiKey),
		http:      integrations.NewHTTPClient(10 * time.Second),
	}, nil
}

func validateServerURL(label, value string) error {
	if err := config.ValidateHTTPURL(value, true); err != nil {
		return fmt.Errorf("invalid media server %s: %w", label, err)
	}
	return nil
}

func (c *Client) ID() string   { return "media_server" }
func (c *Client) Name() string { return c.provider.Name() }
func (c *Client) Kind() string { return "media" }
func (c *Client) HasAPIKey() bool {
	return c.apiKey != ""
}

func (c *Client) setAuthHeader(req *http.Request, token string) {
	key := strings.TrimSpace(token)
	if key == "" {
		key = c.apiKey
	}
	if key == "" {
		return
	}
	c.provider.Authorize(req, key)
}

func (c *Client) Health(ctx context.Context) integrations.HealthStatus {
	req, err := c.newRequest(ctx, http.MethodGet, "/System/Ping", "", nil)
	if err != nil {
		return integrations.HealthStatus{OK: false, Message: err.Error()}
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

func (c *Client) PrimaryImage(ctx context.Context, itemID, tag, token string, maxWidth int) (*ImageResponse, error) {
	q := url.Values{}
	if maxWidth > 0 {
		q.Set("maxWidth", fmt.Sprintf("%d", maxWidth))
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	path := fmt.Sprintf("/Items/%s/Images/Primary?%s", url.PathEscape(itemID), q.Encode())
	req, err := c.newRequest(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, integrations.NewHTTPStatusError(c.Name(), "primary image", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !allowedImageMediaType(mediaType) {
		resp.Body.Close()
		return nil, fmt.Errorf("%s image returned unsupported content type", c.Name())
	}
	return &ImageResponse{Body: resp.Body, ContentType: contentType}, nil
}

func allowedImageMediaType(mediaType string) bool {
	switch strings.ToLower(mediaType) {
	case "image/avif", "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func (c *Client) newRequest(ctx context.Context, method, path, token string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, fmt.Errorf("%s is not configured", c.Name())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req, token)
	return req, nil
}

func decodeMediaServerJSON(body io.Reader, out any) error {
	return integrations.DecodeJSON(body, out, mediaServerJSONLimit)
}
