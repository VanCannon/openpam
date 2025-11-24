package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// AuditLogHandler handles audit log-related requests
type AuditLogHandler struct {
	auditRepo *repository.AuditLogRepository
	logger    *logger.Logger
}

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler(auditRepo *repository.AuditLogRepository, log *logger.Logger) *AuditLogHandler {
	return &AuditLogHandler{
		auditRepo: auditRepo,
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
