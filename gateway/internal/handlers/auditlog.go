package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/VanCannon/openpam/gateway/internal/ssh"
	"github.com/google/uuid"
)

// AuditLogHandler handles audit log-related requests
type AuditLogHandler struct {
	auditRepo *repository.AuditLogRepository
	recorder  *ssh.Recorder
	logger    *logger.Logger
}

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler(auditRepo *repository.AuditLogRepository, recorder *ssh.Recorder, log *logger.Logger) *AuditLogHandler {
	return &AuditLogHandler{
		auditRepo: auditRepo,
		recorder:  recorder,
		logger:    log,
	}
}

// HandleList lists audit logs with pagination
func (h *AuditLogHandler) HandleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		// Parse pagination
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		if limit <= 0 || limit > 100 {
			limit = 50
		}
		if offset < 0 {
			offset = 0
		}

		logs, err := h.auditRepo.List(ctx, limit, offset)
		if err != nil {
			h.logger.Error("Failed to list audit logs", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list audit logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":   logs,
			"count":  len(logs),
			"limit":  limit,
			"offset": offset,
		})
	}
}

// HandleGet retrieves a single audit log by ID
func (h *AuditLogHandler) HandleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		// Extract ID from URL path: /api/v1/audit-logs/{id}
		idStr := r.URL.Path[len("/api/v1/audit-logs/"):]

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "Invalid audit log ID", http.StatusBadRequest)
			return
		}

		log, err := h.auditRepo.GetByID(ctx, id)
		if err != nil {
			h.logger.Error("Failed to get audit log", map[string]interface{}{
				"id":    id.String(),
				"error": err.Error(),
			})
			http.Error(w, "Audit log not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(log)
	}
}

// HandleListByUser lists audit logs for a specific user
func (h *AuditLogHandler) HandleListByUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		userIDStr := r.URL.Query().Get("user_id")

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		if limit <= 0 || limit > 100 {
			limit = 50
		}

		logs, err := h.auditRepo.ListByUser(ctx, userID, limit, offset)
		if err != nil {
			h.logger.Error("Failed to list audit logs by user", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list audit logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":  logs,
			"count": len(logs),
		})
	}
}

// HandleListActive lists all active sessions
func (h *AuditLogHandler) HandleListActive() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		logs, err := h.auditRepo.ListActive(ctx)
		if err != nil {
			h.logger.Error("Failed to list active sessions", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list active sessions", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": logs,
			"count":    len(logs),
		})
	}
}

// HandleGetRecording retrieves the recording file for a session
func (h *AuditLogHandler) HandleGetRecording() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		if h.recorder == nil {
			http.Error(w, "Recording not enabled", http.StatusNotImplemented)
			return
		}

		// Since the recorder stores active sessions in memory but we want to retrieve
		// completed sessions from disk, we need a way to find the file.
		// The recorder.GetRecordingPath only works for active sessions in the current implementation.
		// However, we know the file naming convention: [sessionID]-[timestamp].log
		// We'll search the recordings directory for a file matching the session ID.

		// TODO: Refactor Recorder to support looking up completed sessions or store the path in DB.
		// For now, we'll list files in the recordings directory.

		// This is a bit of a hack because Recorder struct doesn't expose the path publicly,
		// but we passed it in NewRecorder. We should probably expose it or add a method.
		// Let's assume the recordings are in "./recordings" as per server.go for now,
		// or better, we'll just search in the configured directory.

		// Actually, let's just list the directory and find the file.
		// We need to know the base path. It's not exposed in the Recorder struct.
		// Let's assume "./recordings" for this iteration as it's hardcoded in server.go.

		files, err := os.ReadDir("./recordings")
		if err != nil {
			h.logger.Error("Failed to read recordings directory", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to retrieve recording", http.StatusInternalServerError)
			return
		}

		var filePath string
		for _, file := range files {
			if !file.IsDir() && len(file.Name()) > len(sessionID) && file.Name()[:len(sessionID)] == sessionID {
				filePath = "./recordings/" + file.Name()
				break
			}
		}

		if filePath == "" {
			http.Error(w, "Recording not found", http.StatusNotFound)
			return
		}

		file, err := os.Open(filePath)
		if err != nil {
			h.logger.Error("Failed to open recording file", map[string]interface{}{
				"error": err.Error(),
				"path":  filePath,
			})
			http.Error(w, "Failed to open recording", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "text/plain")
		io.Copy(w, file)
	}
}
