package handlers

import (
	"net/http"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/middleware"
	"github.com/bvanc/openpam/gateway/internal/models"
	"github.com/bvanc/openpam/gateway/internal/repository"
	"github.com/bvanc/openpam/gateway/internal/ssh"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// MonitorHandler handles live session monitoring requests
type MonitorHandler struct {
	auditRepo *repository.AuditLogRepository
	userRepo  *repository.UserRepository
	monitor   *ssh.Monitor
	recorder  *ssh.Recorder
	logger    *logger.Logger
	devMode   bool
}

// NewMonitorHandler creates a new monitor handler
func NewMonitorHandler(
	auditRepo *repository.AuditLogRepository,
	userRepo *repository.UserRepository,
	monitor *ssh.Monitor,
	recorder *ssh.Recorder,
	log *logger.Logger,
	devMode bool,
) *MonitorHandler {
	return &MonitorHandler{
		auditRepo: auditRepo,
		userRepo:  userRepo,
		monitor:   monitor,
		recorder:  recorder,
		logger:    log,
		devMode:   devMode,
	}
}

// HandleMonitor handles WebSocket connections for live session monitoring
func (h *MonitorHandler) HandleMonitor() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from URL path
		// Expected format: /api/ws/monitor/{session_id}
		sessionIDStr := r.URL.Path[len("/api/ws/monitor/"):]

		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			h.logger.Warn("Invalid session ID for monitoring", map[string]interface{}{
				"session_id": sessionIDStr,
				"error":      err.Error(),
			})
			http.Error(w, "Invalid session ID", http.StatusBadRequest)
			return
		}

		// Verify the session exists
		ctx := r.Context()
		auditLog, err := h.auditRepo.GetByID(ctx, sessionID)
		if err != nil {
			h.logger.Error("Failed to get audit log for monitoring", map[string]interface{}{
				"session_id": sessionID.String(),
				"error":      err.Error(),
			})
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		// Check if session is active
		if auditLog.SessionStatus != models.SessionStatusActive {
			h.logger.Warn("Attempt to monitor non-active session", map[string]interface{}{
				"session_id": sessionID.String(),
				"status":     auditLog.SessionStatus,
			})
			http.Error(w, "Session is not active", http.StatusBadRequest)
			return
		}

		// Upgrade to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Error("Failed to upgrade to WebSocket for monitoring", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer conn.Close()

		h.logger.Info("Monitor connected to session", map[string]interface{}{
			"session_id": sessionID.String(),
		})

		// Get monitor user info from context
		monitorUser := middleware.GetUserEmail(r.Context())

		// In dev mode, use a default email if not available
		if monitorUser == "" {
			if h.devMode {
				monitorUser = "dev@localhost"
			} else {
				monitorUser = "unknown"
			}
		}

		// Subscribe to session updates FIRST (before broadcasting)
		dataChan := h.monitor.Subscribe(sessionID.String())
		defer h.monitor.Unsubscribe(sessionID.String(), dataChan)

		// Write audit message to recording and broadcast
		if h.recorder != nil {
			auditMsg := []byte("\r\n\r\n[--- Live monitoring by " + monitorUser + " started ---]\r\n\r\n")
			if writer := h.recorder.GetWriter(sessionID.String()); writer != nil {
				writer.Write(auditMsg)
			}
			// Broadcast to all monitors (including the one that just subscribed)
			h.monitor.Broadcast(sessionID.String(), auditMsg)
		}

		// Forward data from monitor to WebSocket
		for data := range dataChan {
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				h.logger.Debug("Monitor WebSocket write error", map[string]interface{}{
					"session_id": sessionID.String(),
					"error":      err.Error(),
				})
				return
			}
		}

		// Write audit message when monitoring ends
		if h.recorder != nil {
			auditMsg := []byte("\r\n\r\n[--- Live monitoring by " + monitorUser + " ended ---]\r\n\r\n")
			if writer := h.recorder.GetWriter(sessionID.String()); writer != nil {
				writer.Write(auditMsg)
			}
			// Also broadcast to other monitors
			h.monitor.Broadcast(sessionID.String(), auditMsg)
		}

		h.logger.Info("Monitor disconnected from session", map[string]interface{}{
			"session_id": sessionID.String(),
		})
	}
}
