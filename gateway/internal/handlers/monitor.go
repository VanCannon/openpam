package handlers

import (
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/rdp"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/VanCannon/openpam/gateway/internal/ssh"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// MonitorHandler handles live session monitoring requests
type MonitorHandler struct {
	auditRepo      *repository.AuditLogRepository
	userRepo       *repository.UserRepository
	sshMonitor     *ssh.Monitor
	rdpMonitor     *rdp.AsyncMonitor
	sshRecorder    *ssh.Recorder
	logger         *logger.Logger
	devMode        bool
}

// NewMonitorHandler creates a new monitor handler
func NewMonitorHandler(
	auditRepo *repository.AuditLogRepository,
	userRepo *repository.UserRepository,
	sshMonitor *ssh.Monitor,
	rdpMonitor *rdp.AsyncMonitor,
	sshRecorder *ssh.Recorder,
	log *logger.Logger,
	devMode bool,
) *MonitorHandler {
	return &MonitorHandler{
		auditRepo:   auditRepo,
		userRepo:    userRepo,
		sshMonitor:  sshMonitor,
		rdpMonitor:  rdpMonitor,
		sshRecorder: sshRecorder,
		logger:      log,
		devMode:     devMode,
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

		// Handle monitoring based on protocol type
		if auditLog.Protocol == models.ProtocolRDP {
			// RDP session - use async monitor
			if h.rdpMonitor == nil {
				h.logger.Error("RDP monitor not available", map[string]interface{}{
					"session_id": sessionID.String(),
				})
				http.Error(w, "RDP monitoring not available", http.StatusServiceUnavailable)
				return
			}

			dataChan := h.rdpMonitor.Subscribe(sessionID.String())
			if dataChan == nil {
				h.logger.Error("Failed to subscribe to RDP session (max subscribers reached)", map[string]interface{}{
					"session_id": sessionID.String(),
				})
				http.Error(w, "Maximum subscribers reached for this session", http.StatusTooManyRequests)
				return
			}
			defer h.rdpMonitor.Unsubscribe(sessionID.String(), dataChan)

			h.logger.Info("RDP monitor subscribed", map[string]interface{}{
				"session_id":   sessionID.String(),
				"monitor_user": monitorUser,
			})

			// Forward data from RDP monitor to WebSocket (text messages for Guacamole protocol)
			for data := range dataChan {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					h.logger.Debug("RDP Monitor WebSocket write error", map[string]interface{}{
						"session_id": sessionID.String(),
						"error":      err.Error(),
					})
					return
				}
			}
		} else {
			// SSH session - use legacy monitor
			dataChan := h.sshMonitor.Subscribe(sessionID.String())
			defer h.sshMonitor.Unsubscribe(sessionID.String(), dataChan)

			// Write audit message to SSH recording
			if h.sshRecorder != nil {
				auditMsg := []byte("\r\n\r\n[--- Live monitoring by " + monitorUser + " started ---]\r\n\r\n")
				if writer := h.sshRecorder.GetWriter(sessionID.String()); writer != nil {
					writer.Write(auditMsg)
				}
			}

			// Forward data from SSH monitor to WebSocket
			for data := range dataChan {
				if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					h.logger.Debug("SSH Monitor WebSocket write error", map[string]interface{}{
						"session_id": sessionID.String(),
						"error":      err.Error(),
					})
					return
				}
			}

			// Write audit message when monitoring ends
			if h.sshRecorder != nil {
				auditMsg := []byte("\r\n\r\n[--- Live monitoring by " + monitorUser + " ended ---]\r\n\r\n")
				if writer := h.sshRecorder.GetWriter(sessionID.String()); writer != nil {
					writer.Write(auditMsg)
				}
			}
		}

		h.logger.Info("Monitor disconnected from session", map[string]interface{}{
			"session_id": sessionID.String(),
			"protocol":   auditLog.Protocol,
		})
	}
}
