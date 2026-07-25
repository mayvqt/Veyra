package seerr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func stringPath(row map[string]any, keys ...string) (string, bool) {
	var current any = row
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = m[key]
		if !ok {
			return "", false
		}
	}
	s, ok := current.(string)
	return strings.TrimSpace(s), ok
}

func intPath(row map[string]any, keys ...string) (int, bool) {
	var current any = row
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		current, ok = m[key]
		if !ok {
			return 0, false
		}
	}
	return anyInt(current)
}

func directInt(row map[string]any, key string) (int, bool) {
	return anyInt(row[key])
}

func anyInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

func anySlicePath(row map[string]any, keys ...string) ([]any, bool) {
	var current any = row
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	out, ok := current.([]any)
	return out, ok
}

func seerrHTTPError(resp *http.Response) string {
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		for _, key := range []string{"message", "error"} {
			if msg, ok := payload[key].(string); ok && strings.TrimSpace(msg) != "" {
				return fmt.Sprintf("%d %s", resp.StatusCode, strings.TrimSpace(msg))
			}
		}
	}
	return fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func decodeSeerrJSON(resp *http.Response, out any) error {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("seerr returned non-JSON response: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("seerr returned invalid JSON: HTTP %d", resp.StatusCode)
	}
	return nil
}
