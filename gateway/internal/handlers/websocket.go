package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
	ReadBufferSize:    16384,                 // 16KB
	WriteBufferSize:   16384,                 // 16KB
	EnableCompression: false,                 // Disable compression - can interfere with Guacamole protocol
	Subprotocols:      []string{"guacamole"}, // Support Guacamole WebSocket protocol
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking in production
		return true
	},
}

// ConnectionHandler handles WebSocket connection requests
type ConnectionHandler struct {
	vault           *vault.Client
	targetRepo      *repository.TargetRepository
	credRepo        *repository.CredentialRepository
	auditRepo       *repository.AuditLogRepository
	systemAuditRepo *repository.SystemAuditLogRepository
	sshProxy        *ssh.Proxy
	rdpProxy        *rdp.Proxy
	logger          *logger.Logger
	scheduleRepo    *repository.ScheduleRepository
	activityURL     string
	identityURL     string
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler(
	vaultClient *vault.Client,
	targetRepo *repository.TargetRepository,
	credRepo *repository.CredentialRepository,
	auditRepo *repository.AuditLogRepository,
	systemAuditRepo *repository.SystemAuditLogRepository,
	sshProxy *ssh.Proxy,
	rdpProxy *rdp.Proxy,
	log *logger.Logger,
	scheduleRepo *repository.ScheduleRepository,
	activityURL string,
	identityURL string,
) *ConnectionHandler {
	return &ConnectionHandler{
		vault:           vaultClient,
		targetRepo:      targetRepo,
		credRepo:        credRepo,
		auditRepo:       auditRepo,
		systemAuditRepo: systemAuditRepo,
		sshProxy:        sshProxy,
		rdpProxy:        rdpProxy,
		logger:          log,
		scheduleRepo:    scheduleRepo,
		activityURL:     activityURL,
		identityURL:     identityURL,
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

		// Check for active schedule
		// We need to find a schedule that is:
		// 1. For this user
		// 2. For this target
		// 3. Status is Active
		// 4. ApprovalStatus is Approved
		// 5. Current time is between StartTime and EndTime
		// Since List doesn't support complex time filtering, we'll fetch active schedules and filter in memory
		// or rely on the fact that "Active" status implies it's currently valid (if the background job is running)
		// But for safety, we should check time here too.

		userUUID, err := uuid.Parse(userID)
		if err != nil {
			h.logger.Error("Invalid user ID", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Invalid user ID", http.StatusInternalServerError)
			return
		}

		// Filter by user, target, and Active status
		activeStatus := models.ScheduleStatusActive
		approvedStatus := models.ApprovalStatusApproved
		schedules, err := h.scheduleRepo.List(ctx, &userUUID, &targetID, &activeStatus, &approvedStatus, nil)
		if err != nil {
			h.logger.Error("Failed to list schedules", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Check if any schedule is currently valid based on time
		var validSchedule *models.Schedule
		now := time.Now()
		for _, s := range schedules {
			if (s.StartTime.Before(now) || s.StartTime.Equal(now)) && s.EndTime.After(now) {
				validSchedule = &s
				break
			}
		}

		// Also check for "Standing Access" (Type=standing) which might be Active
		if validSchedule == nil {
			standingType := "standing"
			standingSchedules, err := h.scheduleRepo.List(ctx, &userUUID, &targetID, &activeStatus, &approvedStatus, &standingType)
			if err == nil && len(standingSchedules) > 0 {
				validSchedule = &standingSchedules[0]
			}
		}

		// If user is admin, they bypass schedule check (optional, but good for emergency access)
		// But the requirement says "Only Admins should be automatically approved", implying they still need a schedule, just auto-approved.
		// So we should enforce schedule existence for everyone, but Admins get theirs auto-created/approved.
		// However, if an Admin tries to connect WITHOUT creating a schedule first, should we allow it?
		// For now, let's enforce schedule for everyone to be safe.
		// If no valid schedule found:
		if validSchedule == nil {
			h.logger.Warn("No active schedule found for connection", map[string]interface{}{
				"user":      userEmail,
				"target_id": targetID.String(),
			})
			http.Error(w, "No active approved schedule found for this target", http.StatusForbidden)
			return
		}

		h.logger.Info("Schedule validated", map[string]interface{}{
			"schedule_id": validSchedule.ID,
			"user":        userEmail,
		})

		// Resolve credentials based on Account Type
		var vaultCreds *vault.Credentials
		var legacyCredID *uuid.UUID

		if validSchedule.AccountType == "managed" {
			vaultPath, _ := validSchedule.AccountDetails["vault_secret_path"].(string)
			managedAccountID, _ := validSchedule.AccountDetails["managed_account_id"].(string)

			var samAccountName, dn string

			// Attempt to get AD details (this also returns vault_secret_path if it was missing)
			var dbVaultPath string
			samAccountName, dn, dbVaultPath, err = h.scheduleRepo.GetManagedAccountADDetails(ctx, managedAccountID)
			if err != nil || samAccountName == "" {
				// Fallback 1: Try searching by vault path (if provided in details)
				if vaultPath != "" {
					h.logger.Warn("Managed account ID lookup failed, trying vault path search", map[string]interface{}{
						"managed_account_id": managedAccountID,
						"vault_path":         vaultPath,
					})
					var pathErr error
					samAccountName, dn, _, pathErr = h.scheduleRepo.GetManagedAccountADDetailsByPath(ctx, vaultPath)
					if pathErr != nil {
						h.logger.Error("Failed to resolve AD details by vault path", map[string]interface{}{"vault_path": vaultPath, "error": pathErr.Error()})
					}
				}

				// Fallback 2: Try searching by name (in case ID is actually a name)
				if samAccountName == "" && managedAccountID != "" {
					h.logger.Warn("Managed account search by ID/Path failed, trying name search", map[string]interface{}{
						"name": managedAccountID,
					})
					var nameErr error
					samAccountName, dn, _, nameErr = h.scheduleRepo.GetManagedAccountADDetailsByName(ctx, managedAccountID)
					if nameErr != nil {
						h.logger.Error("Failed to resolve AD details by name", map[string]interface{}{"name": managedAccountID, "error": nameErr.Error()})
					}
				}
			}

			// If we found a path in the database, use it if we didn't have one
			if vaultPath == "" && dbVaultPath != "" {
				vaultPath = dbVaultPath
			}

			if samAccountName != "" && dn != "" {
				// Enable account before session
				if err := h.enableAccount(ctx, samAccountName, dn); err != nil {
					h.logger.Error("Failed to enable managed account", map[string]interface{}{
						"sam_account_name": samAccountName,
						"error":            err.Error(),
					})
					http.Error(w, "Failed to activate account in AD", http.StatusInternalServerError)
					return
				}

				// Wait for AD to propagate the enable operation before attempting RDP connection
				h.logger.Info("Account enabled, waiting for AD propagation", map[string]interface{}{
					"sam_account_name": samAccountName,
					"delay_seconds":    2,
				})
				time.Sleep(2 * time.Second)

				// Schedule rotation and disablement after session ends
				defer func(sName, d string, vPath string) {
					cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					h.logger.Info("Starting session end tasks for managed account", map[string]interface{}{
						"sam_account_name": sName,
					})

					if err := h.rotatePassword(cleanupCtx, sName, d, vPath); err != nil {
						h.logger.Error("Cleanup: failed to rotate password", map[string]interface{}{"error": err.Error()})
					}

					if err := h.disableAccount(cleanupCtx, sName, d); err != nil {
						h.logger.Error("Cleanup: failed to disable account", map[string]interface{}{"error": err.Error()})
					}
				}(samAccountName, dn, vaultPath)
			} else {
				h.logger.Warn("Could not resolve AD details for managed account, skipping enable/disable hooks")
			}

			// FINAL VALIDATION of vaultPath before fetching credentials
			if vaultPath == "" {
				h.logger.Error("Missing vault path for managed account", map[string]interface{}{
					"schedule_id": validSchedule.ID,
				})
				http.Error(w, "Invalid account configuration: missing vault path", http.StatusInternalServerError)
				return
			}

			vaultCreds, err = h.vault.GetCredentials(ctx, vaultPath)
			if err != nil {
				h.logger.Error("Failed to get managed account credentials from Vault", map[string]interface{}{
					"vault_path": vaultPath,
					"error":      err.Error(),
				})
				http.Error(w, "Failed to retrieve credentials", http.StatusInternalServerError)
				return
			}
		} else if validSchedule.AccountType == "ephemeral" {
			// Ephemeral accounts: Create on-the-fly, use, then delete
			ephemeralPrefix, _ := validSchedule.AccountDetails["ephemeral_prefix"].(string)
			if ephemeralPrefix == "" {
				h.logger.Error("Missing ephemeral prefix", map[string]interface{}{
					"schedule_id": validSchedule.ID,
				})
				http.Error(w, "Invalid ephemeral account configuration: missing prefix", http.StatusBadRequest)
				return
			}

			// Create ephemeral account via Activity service
			samAccountName, dn, upn, password, err := h.createEphemeralAccount(ctx, ephemeralPrefix)
			if err != nil {
				h.logger.Error("Failed to create ephemeral account", map[string]interface{}{
					"prefix": ephemeralPrefix,
					"error":  err.Error(),
				})
				http.Error(w, "Failed to create ephemeral account in AD", http.StatusInternalServerError)
				return
			}

			// Wait for AD to propagate the new account
			h.logger.Info("Ephemeral account created, waiting for AD propagation", map[string]interface{}{
				"sam_account_name": samAccountName,
				"upn":              upn,
				"delay_seconds":    5,
			})
			time.Sleep(5 * time.Second)

			// Schedule deletion after session ends (no rotation needed, just delete)
			defer func(sName, d string) {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				h.logger.Info("Starting session end tasks for ephemeral account", map[string]interface{}{
					"sam_account_name": sName,
				})

				if err := h.deleteEphemeralAccount(cleanupCtx, sName, d); err != nil {
					h.logger.Error("Cleanup: failed to delete ephemeral account", map[string]interface{}{"error": err.Error()})
				}
			}(samAccountName, dn)

			// Get credentials directly from ephemeral account creation (no Vault lookup)
			// Use UPN as username for RDP to ensure correct domain context (e.g. user@domain.com)
			rUser := samAccountName
			if upn != "" {
				rUser = upn
			}

			vaultCreds = &vault.Credentials{
				Username: rUser,
				Password: password,
			}

			h.logger.Info("Ephemeral account ready for RDP", map[string]interface{}{
				"sam_account_name": samAccountName,
				"rdp_username":     rUser,
			})
		} else if validSchedule.AccountType == "promotion" {
			// For promotion, user uses their own AD credentials.
			// Check if they need to be temporarily added to Domain Admins for this session.
			username, ok := validSchedule.AccountDetails["sam_account_name"].(string)
			if !ok || username == "" {
				// Try to get username from email (take part before @)
				if userEmail != "" {
					username = userEmail
					if atIdx := strings.Index(username, "@"); atIdx > 0 {
						username = username[:atIdx]
					}
				}
			}

			// If still empty, try to extract from display name (first word)
			if username == "" {
				// Get display name from JWT claims (set by auth middleware)
				displayName := middleware.GetDisplayName(ctx)
				if displayName != "" {
					// Use display name as the username (for AD, often the sAMAccountName)
					// Take first word if it's "John User" format, otherwise use as-is
					if spaceIdx := strings.Index(displayName, " "); spaceIdx > 0 {
						username = displayName[:spaceIdx]
					} else {
						username = displayName
					}
					h.logger.Info("Using display name for promotion username", map[string]interface{}{
						"display_name": displayName,
						"username":     username,
					})
				}
			}

			if username == "" {
				h.logger.Error("Cannot determine username for promotion", map[string]interface{}{
					"user_id":    userID,
					"user_email": userEmail,
				})
				http.Error(w, "Cannot determine username for promotion", http.StatusBadRequest)
				return
			}

			// Get user DN from AD
			userDN, err := h.getUserDN(ctx, username)
			if err != nil {
				h.logger.Error("Failed to get user DN for promotion", map[string]interface{}{
					"username": username,
					"error":    err.Error(),
				})
				http.Error(w, "Failed to resolve user in AD", http.StatusInternalServerError)
				return
			}

			// Check if user is already in Domain Admins
			wasAlreadyMember, err := h.checkGroupMembership(ctx, userDN)
			if err != nil {
				h.logger.Warn("Failed to check group membership, assuming not a member", map[string]interface{}{
					"user_dn": userDN,
					"error":   err.Error(),
				})
				wasAlreadyMember = false
			}

			if !wasAlreadyMember {
				// Promote user to Domain Admins
				if err := h.promoteUser(ctx, userDN); err != nil {
					h.logger.Error("Failed to promote user to Domain Admins", map[string]interface{}{
						"user_dn": userDN,
						"error":   err.Error(),
					})
					http.Error(w, "Failed to promote user for session", http.StatusInternalServerError)
					return
				}
				h.logger.Info("User promoted to Domain Admins for session", map[string]interface{}{
					"username": username,
					"user_dn":  userDN,
				})

				// Wait for AD propagation
				time.Sleep(2 * time.Second)

				// Schedule demotion after session ends
				defer func(dn string) {
					cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					h.logger.Info("Demoting user from Domain Admins after session", map[string]interface{}{
						"user_dn": dn,
					})

					if err := h.demoteUser(cleanupCtx, dn); err != nil {
						h.logger.Error("Cleanup: failed to demote user", map[string]interface{}{"error": err.Error()})
					}
				}(userDN)
			} else {
				h.logger.Info("User already in Domain Admins, skipping promotion/demotion", map[string]interface{}{
					"username": username,
				})
			}

			// For promotion, use password from URL query parameter
			promotionPassword := r.URL.Query().Get("password")
			vaultCreds = &vault.Credentials{
				Username: username,
				Password: promotionPassword,
			}
		} else {
			// Fallback to legacy static credentials (target-based)
			credentials, err := h.credRepo.GetByTargetID(ctx, targetID)
			if err != nil || len(credentials) == 0 {
				h.logger.Error("No legacy credentials found for target", map[string]interface{}{
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

			legacyCredID = &cred.ID

			if strings.HasPrefix(cred.VaultSecretPath, "raw:") {
				password := strings.TrimPrefix(cred.VaultSecretPath, "raw:")
				vaultCreds = &vault.Credentials{
					Username: cred.Username,
					Password: password,
				}
			} else {
				var err error
				vaultCreds, err = h.vault.GetCredentials(ctx, cred.VaultSecretPath)
				if err != nil {
					h.logger.Error("Failed to retrieve legacy credentials from Vault", map[string]interface{}{
						"vault_path": cred.VaultSecretPath,
						"error":      err.Error(),
					})
					http.Error(w, "Failed to retrieve credentials", http.StatusInternalServerError)
					return
				}
			}
		}

		// Upgrade to WebSocket
		credIDStr := ""
		if legacyCredID != nil {
			credIDStr = legacyCredID.String()
		}

		h.logger.Info("Incoming WebSocket connection", map[string]interface{}{
			"url":           r.URL.String(),
			"remote_addr":   r.RemoteAddr,
			"x_forwarded":   r.Header.Get("X-Forwarded-For"),
			"protocol":      protocol,
			"target_id":     targetID.String(),
			"credential_id": credIDStr,
		})

		// Log session connection attempt
		var actorID *uuid.UUID
		if uid, err := uuid.Parse(userID); err == nil {
			actorID = &uid
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeSessionConnected, actorID, "connect_session", models.AuditStatusSuccess, nil, map[string]interface{}{
			"target_id": targetID,
			"protocol":  protocol,
		})

		// Upgrade to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Error("Failed to upgrade connection", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer conn.Close()

		// Log session disconnection when handler returns
		defer func() {
			h.systemAuditRepo.CreateSimple(context.Background(), models.EventTypeSessionDisconnected, actorID, "disconnect_session", models.AuditStatusSuccess, nil, map[string]interface{}{
				"target_id": targetID,
				"protocol":  protocol,
			})
		}()

		// Set deadlines to prevent hanging connections
		conn.SetReadDeadline(time.Time{})  // No read deadline
		conn.SetWriteDeadline(time.Time{}) // No write deadline

		// Create audit log entry
		// userUUID already parsed above
		auditCredID := uuid.NullUUID{Valid: false}
		if legacyCredID != nil {
			auditCredID = uuid.NullUUID{UUID: *legacyCredID, Valid: true}
		}

		auditLog := &models.AuditLog{
			UserID:        userUUID,
			TargetID:      targetID,
			CredentialID:  auditCredID,
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

			// Attempt to send error details to the client so they know why it failed
			errorMsg := map[string]string{
				"type":    "error",
				"message": err.Error(),
			}
			if jsonBytes, marshalErr := json.Marshal(errorMsg); marshalErr == nil {
				// Set a short write deadline to ensure we don't hang if client is gone
				conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
				conn.WriteMessage(websocket.TextMessage, jsonBytes)
			}
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

func (h *ConnectionHandler) callActivityService(ctx context.Context, endpoint string, samAccountName, dn, vaultPath string) error {
	if h.activityURL == "" {
		return fmt.Errorf("activity service URL not configured")
	}

	payload := map[string]string{
		"sam_account_name": samAccountName,
		"dn":               dn,
		"vault_path":       vaultPath,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", h.activityURL+endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("activity service returned status: %s", resp.Status)
	}

	return nil
}

func (h *ConnectionHandler) enableAccount(ctx context.Context, samAccountName, dn string) error {
	h.logger.Info("Enabling AD account", map[string]interface{}{"sam_account_name": samAccountName})
	return h.callActivityService(ctx, "/api/v1/activity/accounts/enable", samAccountName, dn, "")
}

func (h *ConnectionHandler) disableAccount(ctx context.Context, samAccountName, dn string) error {
	h.logger.Info("Disabling AD account", map[string]interface{}{"sam_account_name": samAccountName})
	return h.callActivityService(ctx, "/api/v1/activity/accounts/disable", samAccountName, dn, "")
}

func (h *ConnectionHandler) rotatePassword(ctx context.Context, samAccountName, dn, vaultPath string) error {
	h.logger.Info("Rotating AD password", map[string]interface{}{"sam_account_name": samAccountName, "vault_path": vaultPath})
	return h.callActivityService(ctx, "/api/v1/activity/accounts/rotate", samAccountName, dn, vaultPath)
}

// createEphemeralAccount creates a temporary AD account via the Activity service
// Returns the created username, dn, upn, and password
func (h *ConnectionHandler) createEphemeralAccount(ctx context.Context, prefix string) (string, string, string, string, error) {
	if h.activityURL == "" {
		return "", "", "", "", fmt.Errorf("activity service URL not configured")
	}

	h.logger.Info("Creating ephemeral AD account", map[string]interface{}{"prefix": prefix})

	// Fetch BaseDN from Identity Service
	baseDN := h.fetchADBaseDN(ctx)

	payload := map[string]string{
		"prefix":      prefix,
		"description": "OpenPAM Ephemeral Account",
		"base_dn":     baseDN,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", h.activityURL+"/api/v1/activity/ephemeral/create", bytes.NewBuffer(body))
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", "", "", fmt.Errorf("activity service returned status %d", resp.StatusCode)
	}

	var result struct {
		Username string `json:"username"`
		DN       string `json:"dn"`
		UPN      string `json:"upn"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	h.logger.Info("Ephemeral account created", map[string]interface{}{
		"username": result.Username,
		"dn":       result.DN,
		"upn":      result.UPN,
	})

	return result.Username, result.DN, result.UPN, result.Password, nil
}

// deleteEphemeralAccount deletes a temporary AD account via the Activity service
func (h *ConnectionHandler) deleteEphemeralAccount(ctx context.Context, samAccountName, dn string) error {
	h.logger.Info("Deleting ephemeral AD account", map[string]interface{}{"sam_account_name": samAccountName})
	return h.callActivityService(ctx, "/api/v1/activity/ephemeral/delete", samAccountName, dn, "")
}

// getUserDN looks up a user's DN via the Activity service
func (h *ConnectionHandler) getUserDN(ctx context.Context, username string) (string, error) {
	if h.activityURL == "" {
		return "", fmt.Errorf("activity service URL not configured")
	}

	payload := map[string]string{
		"sam_account_name": username,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", h.activityURL+"/api/v1/activity/promotion/lookup-user-dn", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("activity service returned status: %s", resp.Status)
	}

	var result struct {
		DN string `json:"dn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.DN, nil
}

// checkGroupMembership checks if user is in Domain Admins via Activity service
func (h *ConnectionHandler) checkGroupMembership(ctx context.Context, userDN string) (bool, error) {
	if h.activityURL == "" {
		return false, fmt.Errorf("activity service URL not configured")
	}

	baseDN := "DC=vancannon,DC=com" // TODO: get from config
	domainAdminsDN := fmt.Sprintf("CN=Domain Admins,CN=Users,%s", baseDN)

	payload := map[string]string{
		"user_dn":  userDN,
		"group_dn": domainAdminsDN,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", h.activityURL+"/api/v1/activity/promotion/check-membership", bytes.NewBuffer(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("activity service returned status: %s", resp.Status)
	}

	var result struct {
		IsMember bool `json:"is_member"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.IsMember, nil
}

// promoteUser adds user to Domain Admins via Activity service
func (h *ConnectionHandler) promoteUser(ctx context.Context, userDN string) error {
	if h.activityURL == "" {
		return fmt.Errorf("activity service URL not configured")
	}

	baseDN := "DC=vancannon,DC=com" // TODO: get from config
	domainAdminsDN := fmt.Sprintf("CN=Domain Admins,CN=Users,%s", baseDN)

	payload := map[string]string{
		"user_dn":  userDN,
		"group_dn": domainAdminsDN,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", h.activityURL+"/api/v1/activity/promotion/promote", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("activity service returned status: %s", resp.Status)
	}

	return nil
}

// demoteUser removes user from Domain Admins via Activity service
func (h *ConnectionHandler) demoteUser(ctx context.Context, userDN string) error {
	if h.activityURL == "" {
		return fmt.Errorf("activity service URL not configured")
	}

	baseDN := "DC=vancannon,DC=com" // TODO: get from config
	domainAdminsDN := fmt.Sprintf("CN=Domain Admins,CN=Users,%s", baseDN)

	payload := map[string]string{
		"user_dn":  userDN,
		"group_dn": domainAdminsDN,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", h.activityURL+"/api/v1/activity/promotion/demote", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("activity service returned status: %s", resp.Status)
	}

	return nil
}

func (h *ConnectionHandler) fetchADBaseDN(ctx context.Context) string {
	if h.identityURL == "" {
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, "GET", h.identityURL+"/api/v1/identity/config", nil)
	if err != nil {
		h.logger.Error("Failed to create request for AD config", map[string]interface{}{"error": err.Error()})
		return ""
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("Failed to fetch AD config", map[string]interface{}{"error": err.Error()})
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("Identity service returned error", map[string]interface{}{"status": resp.StatusCode})
		return ""
	}

	var config struct {
		BaseDN string `json:"base_dn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		h.logger.Error("Failed to decode AD config", map[string]interface{}{"error": err.Error()})
		return ""
	}

	return config.BaseDN
}
