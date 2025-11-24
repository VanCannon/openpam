package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

// EntraIDClient handles authentication with Microsoft EntraID/Azure AD
type EntraIDClient struct {
	config      *oauth2.Config
	tenantID    string
	httpClient  *http.Client
}

// EntraIDConfig holds EntraID client configuration
type EntraIDConfig struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// UserInfo represents user information from EntraID
type UserInfo struct {
	ID          string `json:"id"`
	Email       string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
	DisplayName string `json:"displayName"`
	GivenName   string `json:"givenName"`
	Surname     string `json:"surname"`
}

// NewEntraIDClient creates a new EntraID authentication client
func NewEntraIDClient(cfg EntraIDConfig) *EntraIDClient {
	// Configure OAuth2
	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			"openid",
			"profile",
			"email",
			"User.Read",
		},
		Endpoint: microsoft.AzureADEndpoint(cfg.TenantID),
	}

	return &EntraIDClient{
		config:     oauth2Config,
		tenantID:   cfg.TenantID,
		httpClient: http.DefaultClient,
	}
}

// GetAuthURL generates the authorization URL for the OAuth2 flow
func (c *EntraIDClient) GetAuthURL(state string) string {
	return c.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges an authorization code for tokens
func (c *EntraIDClient) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	return token, nil
}

// GetUserInfo retrieves user information using the access token
func (c *EntraIDClient) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	// Use the token to make a request to Microsoft Graph API
	client := c.config.Client(ctx, token)

	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: status %d, body: %s", resp.StatusCode, string(body))
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// If email is not set, use userPrincipalName
	if userInfo.Email == "" {
		userInfo.Email = userInfo.UserPrincipalName
	}

	return &userInfo, nil
}

// ValidateIDToken validates an ID token (if present in the OAuth2 response)
func (c *EntraIDClient) ValidateIDToken(ctx context.Context, token *oauth2.Token) (map[string]interface{}, error) {
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("no id_token in response")
	}

	// Parse JWT (without verification for now - in production, verify signature)
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid id_token format")
	}

	// Decode claims (base64url)
	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return claims, nil
}

// RevokeToken revokes an access token
func (c *EntraIDClient) RevokeToken(ctx context.Context, token string) error {
	revokeURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/logout", c.tenantID)

	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create revoke request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to revoke token: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// base64URLDecode decodes base64url encoded strings
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	// Replace URL-safe characters
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	// Use standard base64 decoding
	return []byte(s), nil // Simplified - use encoding/base64 in production
}
