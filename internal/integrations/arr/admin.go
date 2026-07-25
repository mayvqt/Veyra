package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type AdminSummary struct {
	AppName       string
	Version       string
	Branch        string
	Runtime       string
	HealthIssues  []HealthIssue
	DiskSpace     []DiskSpace
	DiskWarnings  []string
	HealthWarning string
}

type HealthIssue struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl"`
}

type DiskSpace struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	FreeBytes  int64  `json:"freeSpace"`
	TotalBytes int64  `json:"totalSpace"`
}

func (c *Client) AdminSummary(ctx context.Context) (AdminSummary, error) {
	if c == nil {
		//lint:ignore ST1005 Arr is a product name.
		return AdminSummary{}, fmt.Errorf("Arr service is not configured")
	}
	if c.baseURL == "" || c.apiKey == "" {
		return AdminSummary{}, fmt.Errorf("%s is not configured", c.name)
	}
	var out AdminSummary
	if err := c.getJSON(ctx, c.apiPath("/system/status"), &out); err != nil {
		return AdminSummary{}, err
	}
	if out.AppName == "" {
		out.AppName = c.name
	}
	out.Runtime = firstNonEmpty(out.Runtime, out.Branch)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = c.getJSON(ctx, c.apiPath("/health"), &out.HealthIssues)
	}()
	go func() {
		defer wg.Done()
		_ = c.getJSON(ctx, c.apiPath("/diskspace"), &out.DiskSpace)
	}()
	wg.Wait()
	out.DiskWarnings = diskWarnings(out.DiskSpace)
	if len(out.HealthIssues) > 0 {
		out.HealthWarning = fmt.Sprintf("%d health warning(s)", len(out.HealthIssues))
	}
	return out, nil
}

func (c *Client) apiPath(path string) string {
	base := c.apiBase
	if base == "" {
		base = "/api/v3"
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s failed: %d", c.name, path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func diskWarnings(rows []DiskSpace) []string {
	out := make([]string, 0)
	for _, row := range rows {
		if row.TotalBytes <= 0 || row.FreeBytes < 0 {
			continue
		}
		percentFree := float64(row.FreeBytes) / float64(row.TotalBytes)
		if row.FreeBytes < 10*1024*1024*1024 || percentFree < 0.1 {
			label := firstNonEmpty(row.Label, row.Path, "disk")
			out = append(out, fmt.Sprintf("%s has %s free", label, formatBytes(float64(row.FreeBytes))))
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
