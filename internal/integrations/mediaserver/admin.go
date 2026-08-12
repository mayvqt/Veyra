package mediaserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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
	ServerName      string `json:"ServerName"`
	Version         string `json:"Version"`
	OperatingSystem string `json:"OperatingSystem"`
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

type libraryPayload struct {
	Items []struct {
		Name string `json:"Name"`
	} `json:"Items"`
}

func (p *libraryPayload) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	if data[0] != '[' {
		var wrapped struct {
			Items []struct {
				Name string `json:"Name"`
			} `json:"Items"`
		}
		if err := json.Unmarshal(data, &wrapped); err != nil {
			return err
		}
		p.Items = wrapped.Items
		return nil
	}
	var items []struct {
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	p.Items = items
	return nil
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
	var info systemInfo
	if err := c.getJSON(ctx, "/System/Info/Public", "", &info); err != nil {
		return AdminSummary{}, err
	}
	out := AdminSummary{ServerName: info.ServerName, Version: info.Version, OperatingSystem: info.OperatingSystem}
	if c.apiKey == "" {
		return out, nil
	}
	var (
		users    []userSummary
		counts   itemCounts
		libs     libraryPayload
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
		recordWarning("Users", c.getJSON(ctx, "/Users", c.apiKey, &users))
	}()
	go func() {
		defer wg.Done()
		recordWarning("Item counts", c.getJSON(ctx, "/Items/Counts", c.apiKey, &counts))
	}()
	go func() {
		defer wg.Done()
		recordWarning("Libraries", c.getJSON(ctx, "/Library/VirtualFolders", c.apiKey, &libs))
	}()
	go func() {
		defer wg.Done()
		recordWarning("Sessions", c.getJSON(ctx, "/Sessions", c.apiKey, &sessions))
	}()
	wg.Wait()
	out.UserCount = len(users)
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
		return fmt.Errorf("%s %s failed: %d", c.Name(), path, resp.StatusCode)
	}
	return decodeMediaServerJSON(resp.Body, out)
}
