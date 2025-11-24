package vault

import (
	"context"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// Client wraps the Vault API client with OpenPAM-specific methods
type Client struct {
	client *vault.Client
	token  string
}

// Config holds Vault client configuration
type Config struct {
	Address  string
	Token    string
	RoleID   string
	SecretID string
}

// Credentials represents retrieved credentials from Vault
type Credentials struct {
	Username   string
	Password   string
	PrivateKey string
}

// New creates a new Vault client
func New(cfg Config) (*Client, error) {
	config := vault.DefaultConfig()
	config.Address = cfg.Address

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	c := &Client{
		client: client,
	}

	// Authenticate using token or AppRole
	if cfg.Token != "" {
		c.client.SetToken(cfg.Token)
		c.token = cfg.Token
	} else if cfg.RoleID != "" && cfg.SecretID != "" {
		if err := c.loginWithAppRole(cfg.RoleID, cfg.SecretID); err != nil {
			return nil, fmt.Errorf("failed to authenticate with AppRole: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no authentication method provided")
	}

	return c, nil
}

// loginWithAppRole authenticates to Vault using AppRole
func (c *Client) loginWithAppRole(roleID, secretID string) error {
	data := map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	secret, err := c.client.Logical().Write("auth/approle/login", data)
	if err != nil {
		return fmt.Errorf("failed to login with AppRole: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("no auth info returned from AppRole login")
	}

	c.token = secret.Auth.ClientToken
	c.client.SetToken(c.token)

	return nil
}

// GetCredentials retrieves credentials from Vault at the specified path
func (c *Client) GetCredentials(ctx context.Context, path string) (*Credentials, error) {
	// Read from KV v2 secrets engine
	secret, err := c.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found at path: %s", path)
	}

	// For KV v2, data is nested under "data" key
	var data map[string]interface{}
	if v2Data, ok := secret.Data["data"].(map[string]interface{}); ok {
		data = v2Data
	} else {
		data = secret.Data
	}

	creds := &Credentials{}

	if username, ok := data["username"].(string); ok {
		creds.Username = username
	}

	if password, ok := data["password"].(string); ok {
		creds.Password = password
	}

	if privateKey, ok := data["private_key"].(string); ok {
		creds.PrivateKey = privateKey
	}

	// Validate that we got at least username and either password or private key
	if creds.Username == "" {
		return nil, fmt.Errorf("username not found in secret")
	}

	if creds.Password == "" && creds.PrivateKey == "" {
		return nil, fmt.Errorf("neither password nor private_key found in secret")
	}

	return creds, nil
}

// HealthCheck verifies the Vault connection is healthy
func (c *Client) HealthCheck(ctx context.Context) error {
	health, err := c.client.Sys().HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to check vault health: %w", err)
	}

	if health.Sealed {
		return fmt.Errorf("vault is sealed")
	}

	return nil
}

// RenewToken attempts to renew the current token
func (c *Client) RenewToken(ctx context.Context) error {
	secret, err := c.client.Auth().Token().RenewSelfWithContext(ctx, 0)
	if err != nil {
		return fmt.Errorf("failed to renew token: %w", err)
	}

	if secret == nil {
		return fmt.Errorf("no secret returned from token renewal")
	}

	return nil
}

// StartTokenRenewal starts a background goroutine to renew the token
func (c *Client) StartTokenRenewal(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.RenewToken(ctx); err != nil {
					// Log error but continue trying
					fmt.Printf("Failed to renew Vault token: %v\n", err)
				}
			}
		}
	}()
}

// Close cleans up the Vault client
func (c *Client) Close() error {
	// Vault client doesn't require explicit cleanup
	return nil
}
