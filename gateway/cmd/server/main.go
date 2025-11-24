package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bvanc/openpam/gateway/internal/config"
	"github.com/bvanc/openpam/gateway/internal/database"
	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/server"
	"github.com/bvanc/openpam/gateway/internal/vault"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	log := logger.New(logger.LevelInfo, os.Stdout)
	log.Info("Starting OpenPAM Gateway", map[string]interface{}{
		"version":   "0.1.0",
		"zone_type": cfg.Zone.Type,
		"zone_name": cfg.Zone.Name,
	})

	// Connect to database
	dbConfig := database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	log.Info("Connected to database", map[string]interface{}{
		"host": cfg.Database.Host,
		"port": cfg.Database.Port,
		"name": cfg.Database.Database,
	})

	// Verify database health
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.HealthCheck(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Initialize Vault client
	vaultConfig := vault.Config{
		Address:  cfg.Vault.Address,
		Token:    cfg.Vault.Token,
		RoleID:   cfg.Vault.RoleID,
		SecretID: cfg.Vault.SecretID,
	}

	vaultClient, err := vault.New(vaultConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize vault client: %w", err)
	}
	defer vaultClient.Close()

	log.Info("Connected to Vault", map[string]interface{}{
		"address": cfg.Vault.Address,
	})

	// Start token renewal if using AppRole
	if cfg.Vault.RoleID != "" && cfg.Vault.SecretID != "" {
		vaultClient.StartTokenRenewal(context.Background(), 15*time.Minute)
		log.Info("Started Vault token renewal")
	}

	// Create and start server
	srv := server.New(cfg, db, vaultClient, log)

	// Channel to listen for errors from the server
	serverErrors := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		log.Info("HTTP server starting", map[string]interface{}{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		})
		serverErrors <- srv.Start()
	}()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info("Shutdown signal received", map[string]interface{}{
			"signal": sig.String(),
		})

		// Give outstanding requests 30 seconds to complete
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Error during shutdown", map[string]interface{}{
				"error": err.Error(),
			})
			return fmt.Errorf("shutdown error: %w", err)
		}

		log.Info("Shutdown complete")
	}

	return nil
}
