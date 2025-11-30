package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/VanCannon/openpam/scheduling/internal/config"
	"github.com/VanCannon/openpam/scheduling/pkg/logger"
)

type Database struct {
	db     *sql.DB
	logger *logger.Logger
}

func New(cfg *config.DatabaseConfig, log *logger.Logger) (*Database, error) {
	db, err := sql.Open("postgres", cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection established", nil)

	return &Database{
		db:     db,
		logger: log,
	}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) Health() error {
	return d.db.Ping()
}

func (d *Database) DB() *sql.DB {
	return d.db
}
