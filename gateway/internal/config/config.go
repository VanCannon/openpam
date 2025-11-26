package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Vault    VaultConfig
	EntraID  EntraIDConfig
	Session  SessionConfig
	Zone     ZoneConfig
	DevMode  bool // Enable development mode (bypasses EntraID auth)
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	FrontendURL  string
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// VaultConfig holds HashiCorp Vault configuration
type VaultConfig struct {
	Address  string
	Token    string
	RoleID   string
	SecretID string
}

// EntraIDConfig holds Azure AD/EntraID configuration
type EntraIDConfig struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// SessionConfig holds session management configuration
type SessionConfig struct {
	Secret  string
	Timeout time.Duration
}

// ZoneConfig holds zone-specific configuration
type ZoneConfig struct {
	Type       string // "hub" or "satellite"
	Name       string
	ID         string // Zone UUID
	HubAddress string // For satellite mode: WebSocket URL of hub
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:3000"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "openpam"),
			Password:        getEnv("DB_PASSWORD", "openpam"),
			Database:        getEnv("DB_NAME", "openpam"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
		},
		Vault: VaultConfig{
			Address:  getEnv("VAULT_ADDR", "http://localhost:8200"),
			Token:    getEnv("VAULT_TOKEN", ""),
			RoleID:   getEnv("VAULT_ROLE_ID", ""),
			SecretID: getEnv("VAULT_SECRET_ID", ""),
		},
		EntraID: EntraIDConfig{
			TenantID:     getEnv("ENTRA_TENANT_ID", ""),
			ClientID:     getEnv("ENTRA_CLIENT_ID", ""),
			ClientSecret: getEnv("ENTRA_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("ENTRA_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback"),
		},
		Session: SessionConfig{
			Secret:  getEnv("SESSION_SECRET", "change-me-in-production"),
			Timeout: getEnvDuration("SESSION_TIMEOUT", 3600*time.Second),
		},
		Zone: ZoneConfig{
			Type:       getEnv("ZONE_TYPE", "hub"),
			Name:       getEnv("ZONE_NAME", "default"),
			ID:         getEnv("ZONE_ID", ""),
			HubAddress: getEnv("HUB_ADDRESS", ""),
		},
		DevMode: getEnv("DEV_MODE", "false") == "true",
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Zone.Type != "hub" && c.Zone.Type != "satellite" {
		return fmt.Errorf("invalid zone type: %s (must be 'hub' or 'satellite')", c.Zone.Type)
	}

	if c.Zone.Name == "" {
		return fmt.Errorf("zone name cannot be empty")
	}

	// Satellite-specific validation
	if c.Zone.Type == "satellite" {
		if c.Zone.HubAddress == "" {
			return fmt.Errorf("satellite mode requires HUB_ADDRESS to be set")
		}
		if c.Zone.ID == "" {
			return fmt.Errorf("satellite mode requires ZONE_ID to be set")
		}
	}

	if c.Session.Secret == "change-me-in-production" {
		fmt.Fprintf(os.Stderr, "WARNING: Using default session secret. Set SESSION_SECRET in production!\n")
	}

	// Skip validation of external services in dev mode
	if !c.DevMode {
		if c.Vault.Token == "" && (c.Vault.RoleID == "" || c.Vault.SecretID == "") {
			return fmt.Errorf("vault authentication requires either VAULT_TOKEN or both VAULT_ROLE_ID and VAULT_SECRET_ID")
		}

		if c.EntraID.ClientID == "" || c.EntraID.ClientSecret == "" || c.EntraID.TenantID == "" {
			return fmt.Errorf("EntraID authentication requires ENTRA_CLIENT_ID, ENTRA_CLIENT_SECRET, and ENTRA_TENANT_ID")
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: Development mode enabled. Authentication and Vault validation disabled!\n")
	}

	return nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvDuration retrieves a duration environment variable or returns a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
