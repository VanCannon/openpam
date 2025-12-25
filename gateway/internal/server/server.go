package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/auth"
	"github.com/VanCannon/openpam/gateway/internal/config"
	"github.com/VanCannon/openpam/gateway/internal/database"
	"github.com/VanCannon/openpam/gateway/internal/handlers"
	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/rdp"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/VanCannon/openpam/gateway/internal/sse"
	"github.com/VanCannon/openpam/gateway/internal/ssh"
	"github.com/VanCannon/openpam/gateway/internal/vault"
)

// Server represents the OpenPAM gateway server
type Server struct {
	config            *config.Config
	db                *database.DB
	vault             *vault.Client
	logger            *logger.Logger
	httpServer        *http.Server
	router            *http.ServeMux
	authHandler       *handlers.AuthHandler
	userHandler       *handlers.UserHandler
	groupHandler      *handlers.GroupHandler
	targetHandler     *handlers.TargetHandler
	connectionHandler *handlers.ConnectionHandler
	scheduleHandler   *handlers.ScheduleHandler
	identityHandler   *handlers.IdentityHandler
	secretHandler     *handlers.SecretHandler
	sseHandler        *handlers.SSEHandler
	tokenManager      *auth.TokenManager
	sessionStore      auth.SessionStore
	sseBroadcaster    *sse.Broadcaster
}

// New creates a new server instance
func New(cfg *config.Config, db *database.DB, vaultClient *vault.Client, log *logger.Logger) *Server {
	// Initialize authentication components
	tokenManager := auth.NewTokenManager(cfg.Session.Secret, cfg.Session.Timeout)
	sessionStore := auth.NewMemorySessionStore()
	stateStore := auth.NewMemoryStateStore()

	// Start session cleanup
	ctx := context.Background()
	sessionStore.StartCleanup(ctx, 15*time.Minute)
	stateStore.StartCleanup(ctx, 15*time.Minute)

	// Initialize EntraID client
	entraIDClient := auth.NewEntraIDClient(auth.EntraIDConfig{
		TenantID:     cfg.EntraID.TenantID,
		ClientID:     cfg.EntraID.ClientID,
		ClientSecret: cfg.EntraID.ClientSecret,
		RedirectURL:  cfg.EntraID.RedirectURL,
	})

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	zoneRepo := repository.NewZoneRepository(db)
	targetRepo := repository.NewTargetRepository(db)
	credRepo := repository.NewCredentialRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	systemAuditRepo := repository.NewSystemAuditLogRepository(db)

	// Initialize protocol handlers
	sshRecorder, err := ssh.NewRecorder("./recordings")
	if err != nil {
		log.Error("Failed to create SSH recorder", map[string]interface{}{
			"error": err.Error(),
		})
		sshRecorder = nil // Continue without recording
	}

	// Use async recorder for enterprise-scale RDP recording
	rdpAsyncRecorder, err := rdp.NewAsyncRecorder("./recordings")
	if err != nil {
		log.Error("Failed to create async RDP recorder", map[string]interface{}{
			"error": err.Error(),
		})
		rdpAsyncRecorder = nil // Continue without recording
	}

	// Create async monitor for enterprise-scale live monitoring
	rdpAsyncMonitor := rdp.NewAsyncMonitor()

	// Create legacy session monitor for SSH (still works fine for text-based sessions)
	// Create legacy session monitor for SSH (still works fine for text-based sessions)
	sshMonitor := ssh.NewMonitor()

	sshProxy := ssh.NewProxy(log, sshRecorder, sshMonitor, cfg.GeminiAPIKey)
	// Use async RDP proxy for enterprise scale
	rdpProxy := rdp.NewProxyAsync("localhost:4822", log, rdpAsyncRecorder, rdpAsyncMonitor)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(
		entraIDClient,
		tokenManager,
		sessionStore,
		stateStore,
		userRepo,
		groupRepo,
		systemAuditRepo,
		log,
		cfg.DevMode,
		cfg.Server.FrontendURL,
		cfg.Identity.URL,
	)

	userHandler := handlers.NewUserHandler(userRepo, systemAuditRepo, log)
	groupHandler := handlers.NewGroupHandler(groupRepo, systemAuditRepo, log)

	targetHandler := handlers.NewTargetHandler(targetRepo, systemAuditRepo, log)
	zoneHandler := handlers.NewZoneHandler(zoneRepo, systemAuditRepo, log)
	auditHandler := handlers.NewAuditLogHandler(auditRepo, sshRecorder, log)
	systemAuditHandler := handlers.NewSystemAuditLogHandler(systemAuditRepo, log)
	monitorHandler := handlers.NewMonitorHandler(auditRepo, userRepo, sshMonitor, rdpAsyncMonitor, sshRecorder, log, cfg.DevMode)

	scheduleRepo := repository.NewScheduleRepository(db)

	// Initialize SSE broadcaster for real-time updates (before schedule handler)
	sseBroadcaster := sse.NewBroadcaster()
	sseHandler := handlers.NewSSEHandler(sseBroadcaster, log)
	scheduleHandler := handlers.NewScheduleHandler(scheduleRepo, systemAuditRepo, log, sseBroadcaster)

	identityHandler := handlers.NewIdentityHandler(cfg.Identity.URL, cfg.Orchestrator.URL, systemAuditRepo, log)
	secretHandler := handlers.NewSecretHandler(vaultClient, log)

	connectionHandler := handlers.NewConnectionHandler(
		vaultClient,
		targetRepo,
		credRepo,
		auditRepo,
		systemAuditRepo,
		sshProxy,
		rdpProxy,
		log,
		scheduleRepo,
		cfg.Activity.URL,
		cfg.Identity.URL,
	)

	s := &Server{
		config:            cfg,
		db:                db,
		vault:             vaultClient,
		logger:            log,
		router:            http.NewServeMux(),
		authHandler:       authHandler,
		userHandler:       userHandler,
		groupHandler:      groupHandler,
		targetHandler:     targetHandler,
		connectionHandler: connectionHandler,
		scheduleHandler:   scheduleHandler,
		identityHandler:   identityHandler,
		secretHandler:     secretHandler,
		sseHandler:        sseHandler,
		tokenManager:      tokenManager,
		sessionStore:      sessionStore,
		sseBroadcaster:    sseBroadcaster,
	}

	// Zone routes - support both GET and POST on /api/v1/zones
	s.router.Handle("/api/v1/zones", s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			zoneHandler.HandleList().ServeHTTP(w, r)
		case http.MethodPost:
			zoneHandler.HandleCreate().ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	s.router.Handle("/api/v1/zones/create", s.requireAuth(zoneHandler.HandleCreate()))
	s.router.Handle("/api/v1/zones/get", s.requireAuth(zoneHandler.HandleGet()))
	s.router.Handle("/api/v1/zones/update", s.requireAuth(zoneHandler.HandleUpdate()))
	s.router.Handle("/api/v1/zones/delete", s.requireAuth(zoneHandler.HandleDelete()))

	s.router.Handle("/api/v1/targets/create", s.requireAuth(targetHandler.HandleCreate()))
	s.router.Handle("/api/v1/targets/get", s.requireAuth(targetHandler.HandleGet()))
	s.router.Handle("/api/v1/targets/update", s.requireAuth(targetHandler.HandleUpdate()))
	s.router.Handle("/api/v1/targets/delete", s.requireAuth(targetHandler.HandleDelete()))

	s.router.Handle("/api/v1/audit-logs", s.requireAuth(auditHandler.HandleList()))
	s.router.Handle("/api/v1/audit-logs/", s.requireAuth(auditHandler.HandleGet()))
	s.router.Handle("/api/v1/audit-logs/user", s.requireAuth(auditHandler.HandleListByUser()))
	s.router.Handle("/api/v1/audit-logs/active", s.requireAuth(auditHandler.HandleListActive()))
	s.router.Handle("/api/v1/audit-logs/recording", s.requireAuth(auditHandler.HandleGetRecording()))

	// System audit logs (admin and auditor only)
	s.router.Handle("/api/v1/system-audit-logs", s.requireAuth(systemAuditHandler.HandleList()))
	s.router.Handle("/api/v1/system-audit-logs/", s.requireAuth(systemAuditHandler.HandleGet()))

	// Live session monitoring WebSocket endpoint
	s.router.Handle("/api/ws/monitor/", s.requireAuth(monitorHandler.HandleMonitor()))

	s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      middleware.CORS([]string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:3001", "http://127.0.0.1:3001"})(s.router),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check endpoints (no auth required)
	s.router.HandleFunc("/health", s.handleHealth())
	s.router.HandleFunc("/ready", s.handleReady())

	// Authentication routes (no auth required)
	s.router.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.authHandler.HandleDirectLogin().ServeHTTP(w, r)
		} else {
			s.authHandler.HandleLogin().ServeHTTP(w, r)
		}
	})
	s.router.HandleFunc("/api/v1/auth/callback", s.authHandler.HandleCallback())
	s.router.HandleFunc("/api/v1/auth/logout", s.authHandler.HandleLogout())

	// Protected routes (auth required)
	s.router.Handle("/api/v1/auth/me", s.requireAuth(s.authHandler.HandleMe()))

	// User management routes
	// List users - accessible by admin and auditor (auditor needs it for session audit display)
	s.router.Handle("/api/v1/users", s.requireAnyRole([]string{models.RoleAdmin, models.RoleAuditor}, s.userHandler.HandleList()))
	// User modification routes (admin only)
	s.router.Handle("/api/v1/users/{id}/role", s.requireRole(models.RoleAdmin, s.userHandler.HandleUpdateRole()))
	s.router.Handle("/api/v1/users/{id}/enabled", s.requireRole(models.RoleAdmin, s.userHandler.HandleUpdateEnabled()))
	s.router.Handle("/api/v1/users/{id}", s.requireRole(models.RoleAdmin, s.userHandler.HandleDelete()))

	// Group management routes (admin only)
	s.router.Handle("/api/v1/groups", s.requireRole(models.RoleAdmin, s.groupHandler.HandleList()))
	s.router.Handle("/api/v1/groups/{id}", s.requireRole(models.RoleAdmin, s.groupHandler.HandleDelete()))

	s.router.Handle("/api/v1/targets", s.requireAuth(s.targetHandler.HandleTargets()))

	// Schedule routes
	// Users can request schedules
	s.router.Handle("/api/v1/schedules/request", s.requireAuth(s.scheduleHandler.HandleRequestSchedule()))
	// Anyone authenticated can list schedules (filtered by role in handler)
	s.router.Handle("/api/v1/schedules", s.requireAuth(s.scheduleHandler.HandleListSchedules()))
	// Admin-only routes for approval/rejection
	s.router.Handle("/api/v1/schedules/approve", s.requireRole(models.RoleAdmin, s.scheduleHandler.HandleApproveSchedule()))
	s.router.Handle("/api/v1/schedules/reject", s.requireRole(models.RoleAdmin, s.scheduleHandler.HandleRejectSchedule()))
	s.router.Handle("/api/v1/schedules/", s.requireRole(models.RoleAdmin, s.scheduleHandler.HandleDeleteSchedule()))

	// SSE endpoint for real-time schedule updates
	s.router.Handle("/api/v1/schedules/stream", s.requireAuth(s.sseHandler.HandleScheduleUpdates()))

	// Identity and AD Sync routes (Admin only)
	s.router.Handle("/api/v1/identity/config", s.requireRole(models.RoleAdmin, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.identityHandler.HandleGetConfig().ServeHTTP(w, r)
		} else if r.Method == http.MethodPost {
			s.identityHandler.HandleSaveConfig().ServeHTTP(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	s.router.Handle("/api/v1/ad-users", s.requireRole(models.RoleAdmin, s.identityHandler.HandleListADUsers()))
	s.router.Handle("/api/v1/ad-computers", s.requireRole(models.RoleAdmin, s.identityHandler.HandleListADComputers()))
	s.router.Handle("/api/v1/ad-groups", s.requireRole(models.RoleAdmin, s.identityHandler.HandleListADGroups()))
	s.router.Handle("/api/v1/users/import", s.requireRole(models.RoleAdmin, s.identityHandler.HandleImportUser()))
	s.router.Handle("/api/v1/groups/import", s.requireRole(models.RoleAdmin, s.identityHandler.HandleImportGroup()))
	s.router.Handle("/api/v1/computers/import", s.requireRole(models.RoleAdmin, s.identityHandler.HandleImportComputer()))
	s.router.Handle("/api/v1/orchestrator/sync/ad", s.requireRole(models.RoleAdmin, s.identityHandler.HandleSyncAD()))

	// Managed Accounts (Auth required)
	s.router.Handle("/api/v1/managed-accounts", s.requireAuth(s.identityHandler.HandleListManagedAccounts()))
	s.router.Handle("/api/v1/managed-accounts/{id}", s.requireAuth(s.identityHandler.HandleDeleteManagedAccount()))

	// Secrets management (Admin only) - for initial Managed Account passwords
	s.router.Handle("/api/v1/secrets", s.requireRole(models.RoleAdmin, s.secretHandler.HandleCreate()))

	// WebSocket endpoint for connections (auth required)
	s.router.Handle("/api/ws/connect/", s.requireAuth(s.connectionHandler.HandleConnect()))
}

// requireAuth wraps a handler with authentication middleware
func (s *Server) requireAuth(handler http.HandlerFunc) http.Handler {
	return middleware.RequireAuth(s.tokenManager, s.logger)(handler)
}

// requireRole wraps a handler with authentication and role-based access control
func (s *Server) requireRole(role string, handler http.HandlerFunc) http.Handler {
	return middleware.RequireAuth(s.tokenManager, s.logger)(
		middleware.RequireRole(role, s.logger)(handler),
	)
}

// requireAnyRole wraps a handler with authentication and allows any of the specified roles
func (s *Server) requireAnyRole(roles []string, handler http.HandlerFunc) http.Handler {
	return middleware.RequireAuth(s.tokenManager, s.logger)(
		middleware.RequireAnyRole(roles, s.logger)(handler),
	)
}

// GetBroadcaster returns the SSE broadcaster
func (s *Server) GetBroadcaster() *sse.Broadcaster {
	return s.sseBroadcaster
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting OpenPAM Gateway", map[string]interface{}{
		"addr":      s.httpServer.Addr,
		"zone_type": s.config.Zone.Type,
		"zone_name": s.config.Zone.Name,
	})

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("Error shutting down HTTP server", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Close database connection
	if err := s.db.Close(); err != nil {
		s.logger.Error("Error closing database", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Close Vault client
	if err := s.vault.Close(); err != nil {
		s.logger.Error("Error closing Vault client", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	s.logger.Info("Server shutdown complete")
	return nil
}

// handleHealth returns a basic health check
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

// handleReady returns a readiness check (checks dependencies)
func (s *Server) handleReady() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check database
		if err := s.db.HealthCheck(ctx); err != nil {
			s.logger.Error("Database health check failed", map[string]interface{}{
				"error": err.Error(),
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(fmt.Sprintf(`{"status":"error","message":"database unhealthy: %s"}`, err.Error())))
			return
		}

		// Check Vault
		if err := s.vault.HealthCheck(ctx); err != nil {
			s.logger.Error("Vault health check failed", map[string]interface{}{
				"error": err.Error(),
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(fmt.Sprintf(`{"status":"error","message":"vault unhealthy: %s"}`, err.Error())))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	}
}
