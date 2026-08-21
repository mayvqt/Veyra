package mediaserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mayvqt/veyra/internal/integrations"
)

type AuthenticatedUser struct {
	ID       string
	Username string
	IsAdmin  bool
}

type authenticationResponse struct {
	AccessToken string `json:"AccessToken"`
	User        struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Policy struct {
			IsAdministrator bool `json:"IsAdministrator"`
		} `json:"Policy"`
	} `json:"User"`
}

func (c *Client) Authenticate(ctx context.Context, username, password string) (AuthenticatedUser, string, error) {
	body, err := json.Marshal(struct {
		Username string `json:"Username"`
		Password string `json:"Pw"`
	}{
		Username: username,
		Password: password,
	})
	if err != nil {
		return AuthenticatedUser{}, "", fmt.Errorf("encode %s authentication request: %w", c.Name(), err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/Users/AuthenticateByName", "", bytes.NewReader(body))
	if err != nil {
		return AuthenticatedUser{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.provider.AuthorizeLogin(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return AuthenticatedUser{}, "", fmt.Errorf("authenticate with %s: %w", c.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		statusErr := integrations.NewHTTPStatusError(c.Name(), "authentication", resp.StatusCode)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return AuthenticatedUser{}, "", fmt.Errorf("%s rejected credentials: %w", c.Name(), statusErr)
		}
		return AuthenticatedUser{}, "", statusErr
	}

	var payload authenticationResponse
	if err := decodeMediaServerJSON(resp.Body, &payload); err != nil {
		return AuthenticatedUser{}, "", fmt.Errorf("decode %s authentication response: %w", c.Name(), err)
	}
	payload.AccessToken = strings.TrimSpace(payload.AccessToken)
	payload.User.ID = strings.TrimSpace(payload.User.ID)
	payload.User.Name = strings.TrimSpace(payload.User.Name)
	if payload.AccessToken == "" || payload.User.ID == "" || payload.User.Name == "" {
		return AuthenticatedUser{}, "", fmt.Errorf("%s returned an invalid authentication response", c.Name())
	}
	return AuthenticatedUser{
		ID:       payload.User.ID,
		Username: payload.User.Name,
		IsAdmin:  payload.User.Policy.IsAdministrator,
	}, payload.AccessToken, nil
}

func (c *Client) FetchUserAdminStatus(ctx context.Context, userID, token string) (bool, error) {
	path := "/Users/" + url.PathEscape(userID)
	var payload struct {
		Policy struct {
			IsAdministrator bool `json:"IsAdministrator"`
		} `json:"Policy"`
	}
	if err := c.getJSON(ctx, path, token, &payload); err != nil {
		return false, fmt.Errorf("fetch %s user: %w", c.Name(), err)
	}
	return payload.Policy.IsAdministrator, nil
}

func (c *Client) Logout(ctx context.Context, token string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/Sessions/Logout", token, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("log out from %s: %w", c.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return integrations.NewHTTPStatusError(c.Name(), "logout", resp.StatusCode)
	}
	return nil
}
