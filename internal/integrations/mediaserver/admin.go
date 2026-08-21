package mediaserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/mayvqt/veyra/internal/integrations"
)

type AdminSummary struct {
	ServerName      string
	Version         string
	OperatingSystem string
	UserCount       int
	MovieCount      int
	SeriesCount     int
	EpisodeCount    int
	LibraryCount    int
	ActivePlaybacks []PlaybackSession
	Warnings        []string
}

type systemInfo struct {
	ServerName                 string `json:"ServerName"`
	Version                    string `json:"Version"`
	OperatingSystem            string `json:"OperatingSystem"`
	OperatingSystemDisplayName string `json:"OperatingSystemDisplayName"`
}

type itemCounts struct {
	MovieCount   int `json:"MovieCount"`
	SeriesCount  int `json:"SeriesCount"`
	EpisodeCount int `json:"EpisodeCount"`
}

type userSummary struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

type itemsPayload[T any] struct {
	Items []T
}

func (p *itemsPayload[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	if data[0] != '[' {
		var wrapped struct {
			Items []T `json:"Items"`
		}
		if err := json.Unmarshal(data, &wrapped); err != nil {
			return err
		}
		p.Items = wrapped.Items
		return nil
	}
	return json.Unmarshal(data, &p.Items)
}

type librarySummary struct {
	Name string `json:"Name"`
}

type PlaybackSession struct {
	User       string
	Client     string
	DeviceName string
	Title      string
	MediaType  string
}

type sessionPayload struct {
	UserName       string `json:"UserName"`
	Client         string `json:"Client"`
	DeviceName     string `json:"DeviceName"`
	NowPlayingItem *struct {
		Name string `json:"Name"`
		Type string `json:"Type"`
	} `json:"NowPlayingItem"`
}

func (c *Client) AdminSummary(ctx context.Context) (AdminSummary, error) {
	if c.baseURL == "" {
		return AdminSummary{}, fmt.Errorf("%s is not configured", c.Name())
	}
	infoPath := "/System/Info/Public"
	if c.apiKey != "" {
		infoPath = "/System/Info"
	}
	var info systemInfo
	if err := c.getJSON(ctx, infoPath, c.apiKey, &info); err != nil {
		return AdminSummary{}, err
	}
	operatingSystem := info.OperatingSystemDisplayName
	if operatingSystem == "" {
		operatingSystem = info.OperatingSystem
	}
	out := AdminSummary{ServerName: info.ServerName, Version: info.Version, OperatingSystem: operatingSystem}
	if c.apiKey == "" {
		return out, nil
	}
	var (
		users    itemsPayload[userSummary]
		counts   itemCounts
		libs     itemsPayload[librarySummary]
		sessions []sessionPayload
		warnings []string
		mu       sync.Mutex
		wg       sync.WaitGroup
	)
	recordWarning := func(label string, err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, fmt.Sprintf("%s unavailable: %v", label, err))
	}
	wg.Add(4)
	go func() {
		defer wg.Done()
		recordWarning("Users", c.getJSONWithFallback(ctx, c.provider.UsersPaths(), c.apiKey, &users))
	}()
	go func() {
		defer wg.Done()
		recordWarning("Item counts", c.getJSON(ctx, "/Items/Counts", c.apiKey, &counts))
	}()
	go func() {
		defer wg.Done()
		recordWarning("Libraries", c.getJSONWithFallback(ctx, c.provider.VirtualFoldersPaths(), c.apiKey, &libs))
	}()
	go func() {
		defer wg.Done()
		recordWarning("Sessions", c.getJSON(ctx, "/Sessions", c.apiKey, &sessions))
	}()
	wg.Wait()
	sort.Strings(warnings)
	out.UserCount = len(users.Items)
	out.MovieCount = counts.MovieCount
	out.SeriesCount = counts.SeriesCount
	out.EpisodeCount = counts.EpisodeCount
	out.LibraryCount = len(libs.Items)
	out.ActivePlaybacks = activePlaybackSessions(sessions)
	out.Warnings = warnings
	return out, nil
}

func activePlaybackSessions(rows []sessionPayload) []PlaybackSession {
	out := make([]PlaybackSession, 0, len(rows))
	for _, row := range rows {
		if row.NowPlayingItem == nil || row.NowPlayingItem.Name == "" {
			continue
		}
		out = append(out, PlaybackSession{
			User:       row.UserName,
			Client:     row.Client,
			DeviceName: row.DeviceName,
			Title:      row.NowPlayingItem.Name,
			MediaType:  row.NowPlayingItem.Type,
		})
	}
	return out
}

func (c *Client) getJSON(ctx context.Context, path, token string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return integrations.NewHTTPStatusError(c.Name(), path, resp.StatusCode)
	}
	return decodeMediaServerJSON(resp.Body, out)
}

func (c *Client) getJSONWithFallback(ctx context.Context, paths []string, token string, out any) error {
	if len(paths) == 0 {
		return fmt.Errorf("%s endpoint is not configured", c.Name())
	}
	for i, path := range paths {
		err := c.getJSON(ctx, path, token, out)
		if err == nil {
			return nil
		}
		if i == len(paths)-1 || !integrations.IsHTTPStatus(err, http.StatusNotFound) {
			return err
		}
	}
	return nil
}
