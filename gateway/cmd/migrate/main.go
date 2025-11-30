package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/database"
)

func main() {
	var (
		action   = flag.String("action", "up", "Migration action: up, down, status")
		host     = flag.String("host", getEnv("DB_HOST", "localhost"), "Database host")
		port     = flag.Int("port", getEnvInt("DB_PORT", 5432), "Database port")
		user     = flag.String("user", getEnv("DB_USER", "openpam"), "Database user")
		password = flag.String("password", getEnv("DB_PASSWORD", "openpam"), "Database password")
		dbname   = flag.String("dbname", getEnv("DB_NAME", "openpam"), "Database name")
		sslmode  = flag.String("sslmode", getEnv("DB_SSLMODE", "disable"), "SSL mode")
	)

	flag.Parse()

	cfg := database.Config{
		Host:            *host,
		Port:            *port,
		User:            *user,
		Password:        *password,
		Database:        *dbname,
		SSLMode:         *sslmode,
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	db, err := database.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	migrator, err := database.NewMigrator(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create migrator: %v\n", err)
		os.Exit(1)
	}

	switch *action {
	case "up":
		if err := migrator.Up(); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("All migrations applied successfully")

	case "down":
		if err := migrator.Down(); err != nil {
			fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Rollback completed successfully")

	case "status":
		current, total, err := migrator.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Current version: %d\n", current)
		fmt.Printf("Total migrations: %d\n", total)
		if current < total {
			fmt.Printf("Pending migrations: %d\n", total-current)
		} else {
			fmt.Println("Database is up to date")
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", *action)
		fmt.Fprintf(os.Stderr, "Valid actions: up, down, status\n")
		os.Exit(1)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
