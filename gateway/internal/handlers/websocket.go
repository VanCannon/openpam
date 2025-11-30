package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/rdp"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/VanCannon/openpam/gateway/internal/ssh"
	"github.com/VanCannon/openpam/gateway/internal/vault"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	Subprotocols:    []string{"guacamole"}, // Support Guacamole WebSocket protocol
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking in production
		return true
	},
}

// ConnectionHandler handles WebSocket connection requests
type ConnectionHandler struct {
	vault      *vault.Client
	targetRepo *repository.TargetRepository
	credRepo   *repository.CredentialRepository
	auditRepo  *repository.AuditLogRepository
	sshProxy   *ssh.Proxy
	rdpProxy   *rdp.Proxy
	logger     *logger.Logger
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler(
	vaultClient *vault.Client,
	targetRepo *repository.TargetRepository,
	credRepo *repository.CredentialRepository,
	auditRepo *repository.AuditLogRepository,
	sshProxy *ssh.Proxy,
	rdpProxy *rdp.Proxy,
	log *logger.Logger,
) *ConnectionHandler {
	return &ConnectionHandler{
		vault:      vaultClient,
		targetRepo: targetRepo,
		credRepo:   credRepo,
		auditRepo:  auditRepo,
		sshProxy:   sshProxy,
		rdpProxy:   rdpProxy,
		logger:     log,
	}
}

// HandleConnect handles WebSocket connection requests
// Route: /api/ws/connect/{protocol}/{target_id}
func (h *ConnectionHandler) HandleConnect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user info from context (set by auth middleware)
		userID := middleware.GetUserID(ctx)
		userEmail := middleware.GetUserEmail(ctx)

		if userID == "" {
			h.logger.Error("User ID not found in context")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse path: /api/ws/connect/{protocol}/{target_id}
		path := strings.TrimPrefix(r.URL.Path, "/api/ws/connect/")
		parts := strings.Split(path, "/")

		if len(parts) != 2 {
			h.logger.Warn("Invalid connection path", map[string]interface{}{
				"path": r.URL.Path,
			})
			http.Error(w, "Invalid path format", http.StatusBadRequest)
			return
		}

		protocol := parts[0]
		targetIDStr := parts[1]

		// Validate protocol
		if protocol != models.ProtocolSSH && protocol != models.ProtocolRDP {
			h.logger.Warn("Invalid protocol", map[string]interface{}{
				"protocol": protocol,
			})
			http.Error(w, "Invalid protocol", http.StatusBadRequest)
			return
		}

		// Parse target ID
		targetID, err := uuid.Parse(targetIDStr)
		if err != nil {
			h.logger.Warn("Invalid target ID", map[string]interface{}{
				"target_id": targetIDStr,
				"error":     err.Error(),
			})
			http.Error(w, "Invalid target ID", http.StatusBadRequest)
			return
		}

		h.logger.Info("Connection request", map[string]interface{}{
			"user":      userEmail,
			"protocol":  protocol,
			"target_id": targetID.String(),
		})

		// Get target from database
		target, err := h.targetRepo.GetByID(ctx, targetID)
		if err != nil {
			h.logger.Error("Failed to get target", map[string]interface{}{
				"target_id": targetID.String(),
				"error":     err.Error(),
			})
			http.Error(w, "Target not found", http.StatusNotFound)
			return
		}

		// Check if target is enabled
		if !target.Enabled {
			h.logger.Warn("Attempt to connect to disabled target", map[string]interface{}{
				"target_id": targetID.String(),
				"user":      userEmail,
			})
			http.Error(w, "Target is disabled", http.StatusForbidden)
			return
		}

		// Verify protocol matches
		if target.Protocol != protocol {
			h.logger.Warn("Protocol mismatch", map[string]interface{}{
				"requested": protocol,
				"actual":    target.Protocol,
			})
			http.Error(w, "Protocol mismatch", http.StatusBadRequest)
			return
		}

		// Get credentials for target
		credentials, err := h.credRepo.GetByTargetID(ctx, targetID)
		if err != nil || len(credentials) == 0 {
			h.logger.Error("No credentials found for target", map[string]interface{}{
				"target_id": targetID.String(),
				"error":     err,
			})
			http.Error(w, "No credentials configured", http.StatusInternalServerError)
			return
		}

		// Use first credential (TODO: implement credential selection)
		cred := credentials[0]

		// If a specific credential ID was requested, use that one
		credentialId := r.URL.Query().Get("credential_id")

		// Defensive fix: client library seems to append ?undefined
		if strings.Contains(credentialId, "?undefined") {
			credentialId = strings.ReplaceAll(credentialId, "?undefined", "")
		}

		if credentialId != "" {
			credUUID, err := uuid.Parse(credentialId)
			if err == nil {
				for _, c := range credentials {
					if c.ID == credUUID {
						cred = c
						break
					}
				}
			}
		}

		// Check if using raw password (for testing/dev)
		var vaultCreds *vault.Credentials
		if strings.HasPrefix(cred.VaultSecretPath, "raw:") {
			password := strings.TrimPrefix(cred.VaultSecretPath, "raw:")
			vaultCreds = &vault.Credentials{
				Username: cred.Username,
				Password: password,
			}
			h.logger.Info("Using raw password credentials", map[string]interface{}{
				"target_id": targetID.String(),
				"username":  cred.Username,
			})
		} else {
			// Retrieve secret from Vault
			var err error
			vaultCreds, err = h.vault.GetCredentials(ctx, cred.VaultSecretPath)
			if err != nil {
				h.logger.Error("Failed to retrieve credentials from Vault", map[string]interface{}{
					"vault_path": cred.VaultSecretPath,
					"error":      err.Error(),
				})
				http.Error(w, "Failed to retrieve credentials", http.StatusInternalServerError)
				return
			}

			h.logger.Info("Credentials retrieved from Vault", map[string]interface{}{
				"target_id": targetID.String(),
				"username":  vaultCreds.Username,
			})
		}

		// Upgrade to WebSocket
		h.logger.Info("Incoming WebSocket connection", map[string]interface{}{
			"url":           r.URL.String(),
			"remote_addr":   r.RemoteAddr,
			"x_forwarded":   r.Header.Get("X-Forwarded-For"),
			"protocol":      protocol,
			"target_id":     targetID.String(),
			"credential_id": cred.ID.String(),
		})
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Error("Failed to upgrade to WebSocket", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer conn.Close()

		// Create audit log entry
		userUUID, _ := uuid.Parse(userID)
		auditLog := &models.AuditLog{
			UserID:        userUUID,
			TargetID:      targetID,
			CredentialID:  uuid.NullUUID{UUID: cred.ID, Valid: true},
			SessionStatus: models.SessionStatusActive,
			ClientIP:      &r.RemoteAddr,
		}

		if err := h.auditRepo.Create(ctx, auditLog); err != nil {
			h.logger.Error("Failed to create audit log", map[string]interface{}{
				"error": err.Error(),
			})
			conn.WriteMessage(websocket.TextMessage, []byte("Failed to create audit log"))
			return
		}

		h.logger.Info("Session started", map[string]interface{}{
			"audit_log_id": auditLog.ID.String(),
			"user":         userEmail,
			"target":       target.Name,
		})

		// Handle connection based on protocol
		switch protocol {
		case models.ProtocolSSH:
			err = h.handleSSHConnection(ctx, conn, target, vaultCreds, auditLog)
		case models.ProtocolRDP:
			// Parse resolution from query params
			width := 1024
			height := 768

			if wStr := r.URL.Query().Get("width"); wStr != "" {
				if w, err := strconv.Atoi(wStr); err == nil && w > 0 {
					width = w
				}
			}
			if hStr := r.URL.Query().Get("height"); hStr != "" {
				if h, err := strconv.Atoi(hStr); err == nil && h > 0 {
					height = h
				}
			}

			err = h.handleRDPConnection(ctx, conn, target, vaultCreds, auditLog, width, height)
		}

		// Update audit log with final status
		if err != nil {
			auditLog.SessionStatus = models.SessionStatusFailed
			errMsg := err.Error()
			auditLog.ErrorMessage = &errMsg
			h.logger.Error("Session failed", map[string]interface{}{
				"audit_log_id": auditLog.ID.String(),
				"error":        err.Error(),
			})
		} else {
			auditLog.SessionStatus = models.SessionStatusCompleted
		}

		// Use a new context for the update since the request context might be cancelled
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.auditRepo.UpdateStatus(updateCtx, auditLog); err != nil {
			h.logger.Error("Failed to update audit log", map[string]interface{}{
				"error": err.Error(),
			})
		}

		h.logger.Info("Session ended", map[string]interface{}{
			"audit_log_id": auditLog.ID.String(),
			"status":       auditLog.SessionStatus,
		})
	}
}

// handleSSHConnection handles an SSH connection
func (h *ConnectionHandler) handleSSHConnection(
	ctx context.Context,
	conn *websocket.Conn,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
) error {
	h.logger.Info("Starting SSH proxy", map[string]interface{}{
		"target":   target.Hostname,
		"port":     target.Port,
		"username": creds.Username,
	})

	err := h.sshProxy.Handle(ctx, conn, target, creds, auditLog)
	if err != nil {
		return fmt.Errorf("SSH proxy error: %w", err)
	}

	return nil
}

// handleRDPConnection handles an RDP connection
func (h *ConnectionHandler) handleRDPConnection(
	ctx context.Context,
	conn *websocket.Conn,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
	width int,
	height int,
) error {
	h.logger.Info("Starting RDP proxy", map[string]interface{}{
		"target":   target.Hostname,
		"port":     target.Port,
		"username": creds.Username,
		"width":    width,
		"height":   height,
	})

	err := h.rdpProxy.Handle(ctx, conn, target, creds, auditLog, width, height)
	if err != nil {
		return fmt.Errorf("RDP proxy error: %w", err)
	}

	return nil
}
