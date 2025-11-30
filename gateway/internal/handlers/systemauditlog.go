package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// SystemAuditLogHandler handles system audit log-related requests
type SystemAuditLogHandler struct {
	auditRepo *repository.SystemAuditLogRepository
	logger    *logger.Logger
}

// NewSystemAuditLogHandler creates a new system audit log handler
func NewSystemAuditLogHandler(auditRepo *repository.SystemAuditLogRepository, log *logger.Logger) *SystemAuditLogHandler {
	return &SystemAuditLogHandler{
		auditRepo: auditRepo,
		logger:    log,
	}
}

// HandleList lists system audit logs with pagination
func (h *SystemAuditLogHandler) HandleList() http.HandlerFunc {
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

		// Check for filters
		eventType := r.URL.Query().Get("event_type")
		userIDStr := r.URL.Query().Get("user_id")

		var logs interface{}
		var err error

		if eventType != "" {
			logs, err = h.auditRepo.ListByEventType(ctx, eventType, limit, offset)
		} else if userIDStr != "" {
			userID, parseErr := uuid.Parse(userIDStr)
			if parseErr != nil {
				http.Error(w, "Invalid user ID", http.StatusBadRequest)
				return
			}
			logs, err = h.auditRepo.ListByUser(ctx, userID, limit, offset)
		} else {
			logs, err = h.auditRepo.List(ctx, limit, offset)
		}

		if err != nil {
			h.logger.Error("Failed to list system audit logs", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list system audit logs", http.StatusInternalServerError)
			return
		}

		// Get total count
		total, err := h.auditRepo.Count(ctx)
		if err != nil {
			h.logger.Error("Failed to count system audit logs", map[string]interface{}{
				"error": err.Error(),
			})
			total = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":   logs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// HandleGet retrieves a single system audit log by ID
func (h *SystemAuditLogHandler) HandleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		// Extract ID from URL path: /api/v1/system-audit-logs/{id}
		idStr := r.URL.Path[len("/api/v1/system-audit-logs/"):]

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "Invalid audit log ID", http.StatusBadRequest)
			return
		}

		log, err := h.auditRepo.GetByID(ctx, id)
		if err != nil {
			h.logger.Error("Failed to get system audit log", map[string]interface{}{
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
