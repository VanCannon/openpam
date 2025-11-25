package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	NATS     NATSConfig     `yaml:"nats"`
	Consul   ConsulConfig   `yaml:"consul"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Name         string `yaml:"name"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	SSLMode      string `yaml:"sslmode"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type NATSConfig struct {
	URL       string `yaml:"url"`
	ClusterID string `yaml:"cluster_id"`
}

type ConsulConfig struct {
	Address                        string `yaml:"address"`
	ServiceName                    string `yaml:"service_name"`
	ServiceID                      string `yaml:"service_id"`
	CheckInterval                  string `yaml:"check_interval"`
	DeregisterCriticalServiceAfter string `yaml:"deregister_critical_service_after"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.Database.Host = dbHost
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		cfg.Database.Password = dbPass
	}
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		cfg.NATS.URL = natsURL
	}
	if consulAddr := os.Getenv("CONSUL_ADDRESS"); consulAddr != "" {
		cfg.Consul.Address = consulAddr
	}

	return &cfg, nil
}

func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}
