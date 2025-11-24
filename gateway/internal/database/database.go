package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Config holds database configuration
type Config struct {
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

// DB wraps sqlx.DB with additional functionality
type DB struct {
	*sqlx.DB
}

// New creates a new database connection with the provided configuration
func New(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	return &DB{DB: db}, nil
}

// Ping verifies the database connection is alive
func (db *DB) Ping(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// HealthCheck performs a comprehensive health check
func (db *DB) HealthCheck(ctx context.Context) error {
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Test a simple query
	var result int
	if err := db.GetContext(ctx, &result, "SELECT 1"); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// DefaultConfig returns a default database configuration
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		User:            "openpam",
		Password:        "openpam",
		Database:        "openpam",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}
