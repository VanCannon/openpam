package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/VanCannon/openpam/scheduling/internal/config"
	"github.com/VanCannon/openpam/scheduling/internal/database"
	"github.com/VanCannon/openpam/scheduling/internal/events"
	"github.com/VanCannon/openpam/scheduling/internal/handlers"
	"github.com/VanCannon/openpam/scheduling/internal/schedule"
	"github.com/VanCannon/openpam/scheduling/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	log.Info("Starting Scheduling Agent", map[string]interface{}{
		"port": cfg.Server.Port,
	})

	// Connect to database
	db, err := database.New(&cfg.Database, log)
	if err != nil {
		log.Fatal("Failed to connect to database", map[string]interface{}{
			"error": err.Error(),
		})
	}
	defer db.Close()

	// Initialize service
	svc := schedule.NewService(db.DB(), log)

	// Initialize NATS publisher
	publisher, err := events.NewPublisher(cfg.NATS.URL, log)
	if err != nil {
		log.Fatal("Failed to create NATS publisher", map[string]interface{}{
			"error": err.Error(),
		})
	}
	defer publisher.Close()

	// Start scheduler
	scheduler := schedule.NewScheduler(
		svc,
		log,
		cfg.Scheduler.GetCheckInterval(),
		cfg.Scheduler.GetLookaheadWindow(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scheduler.Start(ctx)

	// Register with Consul
	consulClient, err := registerWithConsul(cfg, log)
	if err != nil {
		log.Warn("Failed to register with Consul", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Initialize HTTP handlers
	handler := handlers.New(svc, log)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/v1/schedules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.CreateSchedule(w, r)
		} else if r.Method == http.MethodGet {
			handler.ListSchedules(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/schedules/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/check") {
			handler.CheckAccess(w, r)
		} else if r.Method == http.MethodGet {
			handler.GetSchedule(w, r)
		} else if r.Method == http.MethodPut || r.Method == http.MethodPatch {
			handler.UpdateSchedule(w, r)
		} else if r.Method == http.MethodDelete {
			handler.DeleteSchedule(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/schedule/check", handler.CheckAccess)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("HTTP server listening", map[string]interface{}{
			"address": addr,
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...", nil)

	// Stop scheduler
	scheduler.Stop()
	cancel()

	// Deregister from Consul
	if consulClient != nil {
		if err := consulClient.Agent().ServiceDeregister(cfg.Consul.ServiceID); err != nil {
			log.Error("Failed to deregister from Consul", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", map[string]interface{}{
			"error": err.Error(),
		})
	}

	log.Info("Server stopped", nil)
}

func registerWithConsul(cfg *config.Config, log *logger.Logger) (*consulapi.Client, error) {
	config := consulapi.DefaultConfig()
	config.Address = cfg.Consul.Address

	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Consul client: %w", err)
	}

	registration := &consulapi.AgentServiceRegistration{
		ID:   cfg.Consul.ServiceID,
		Name: cfg.Consul.ServiceName,
		Port: cfg.Server.Port,
		Check: &consulapi.AgentServiceCheck{
			HTTP:                           fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.Port),
			Interval:                       cfg.Consul.CheckInterval,
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: cfg.Consul.DeregisterCriticalServiceAfter,
		},
		Tags: []string{"scheduling", "pam", "v1"},
	}

	if err := client.Agent().ServiceRegister(registration); err != nil {
		return nil, fmt.Errorf("failed to register service: %w", err)
	}

	log.Info("Registered with Consul", map[string]interface{}{
		"service_id": cfg.Consul.ServiceID,
		"address":    cfg.Consul.Address,
	})

	return client, nil
}
