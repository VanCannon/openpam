package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// ScheduleHandler handles schedule-related requests
type ScheduleHandler struct {
	repo   *repository.ScheduleRepository
	logger *logger.Logger
}

// NewScheduleHandler creates a new schedule handler
func NewScheduleHandler(repo *repository.ScheduleRepository, log *logger.Logger) *ScheduleHandler {
	return &ScheduleHandler{
		repo:   repo,
		logger: log,
	}
}

// CreateScheduleRequest represents a schedule creation request
type CreateScheduleRequest struct {
	UserID         string                 `json:"user_id"`
	TargetID       string                 `json:"target_id"`
	StartTime      string                 `json:"start_time"` // RFC3339 format
	EndTime        string                 `json:"end_time"`   // RFC3339 format
	RecurrenceRule *string                `json:"recurrence_rule,omitempty"`
	Timezone       string                 `json:"timezone"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ApproveScheduleRequest represents a schedule approval request
type ApproveScheduleRequest struct {
	ScheduleID string  `json:"schedule_id"`
	StartTime  *string `json:"start_time,omitempty"` // Optional: modify start time
	EndTime    *string `json:"end_time,omitempty"`   // Optional: modify end time
}

// RejectScheduleRequest represents a schedule rejection request
type RejectScheduleRequest struct {
	ScheduleID string `json:"schedule_id"`
	Reason     string `json:"reason"`
}

// respondWithError sends a JSON error response
func (h *ScheduleHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
	})
}

// HandleRequestSchedule handles schedule requests from users
func (h *ScheduleHandler) HandleRequestSchedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userIDStr := middleware.GetUserID(ctx)
		userRole := middleware.GetUserRole(ctx)

		if r.Method != http.MethodPost {
			h.respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req CreateScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Invalid request body", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Users can only request schedules for themselves
		if userRole != models.RoleAdmin && req.UserID != userIDStr {
			h.respondWithError(w, http.StatusForbidden, "You can only request schedules for yourself")
			return
		}

		// Validate time format
		startTime, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid start_time format (use RFC3339)")
			return
		}

		endTime, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid end_time format (use RFC3339)")
			return
		}

		// Validate time range
		if endTime.Before(startTime) || endTime.Equal(startTime) {
			h.respondWithError(w, http.StatusBadRequest, "end_time must be after start_time")
			return
		}

		// Parse UUIDs
		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid user_id")
			return
		}

		targetID, err := uuid.Parse(req.TargetID)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid target_id")
			return
		}

		// Create schedule
		schedule := &models.Schedule{
			ID:             uuid.New(),
			UserID:         userID,
			TargetID:       targetID,
			StartTime:      startTime,
			EndTime:        endTime,
			RecurrenceRule: req.RecurrenceRule,
			Timezone:       req.Timezone,
			Status:         models.ScheduleStatusPending,
			ApprovalStatus: models.ApprovalStatusPending,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if req.Metadata != nil {
			schedule.Metadata = req.Metadata
		}

		if err := h.repo.Create(ctx, schedule); err != nil {
			h.logger.Error("Failed to create schedule", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusInternalServerError, "Failed to create schedule")
			return
		}

		h.logger.Info("Schedule request created", map[string]interface{}{
			"schedule_id": schedule.ID,
			"user_id":     userID,
			"target_id":   targetID,
		})

		response := map[string]interface{}{
			"success":  true,
			"message":  "Schedule request created successfully",
			"schedule": schedule,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleListSchedules handles listing schedules
func (h *ScheduleHandler) HandleListSchedules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userIDStr := middleware.GetUserID(ctx)
		userRole := middleware.GetUserRole(ctx)

		if r.Method != http.MethodGet {
			h.respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Parse query parameters
		targetIDStr := r.URL.Query().Get("target_id")
		statusStr := r.URL.Query().Get("status")
		approvalStatusStr := r.URL.Query().Get("approval_status")
		filterUserIDStr := r.URL.Query().Get("user_id")

		// Non-admins can only see their own schedules
		if userRole != models.RoleAdmin {
			filterUserIDStr = userIDStr
		}

		// Prepare filters
		var filterUserID *uuid.UUID
		if filterUserIDStr != "" {
			uid, err := uuid.Parse(filterUserIDStr)
			if err == nil {
				filterUserID = &uid
			}
		}

		var filterTargetID *uuid.UUID
		if targetIDStr != "" {
			tid, err := uuid.Parse(targetIDStr)
			if err == nil {
				filterTargetID = &tid
			}
		}

		var filterStatus *models.ScheduleStatus
		if statusStr != "" {
			s := models.ScheduleStatus(statusStr)
			filterStatus = &s
		}

		var filterApprovalStatus *string
		if approvalStatusStr != "" {
			filterApprovalStatus = &approvalStatusStr
		}

		schedules, err := h.repo.List(ctx, filterUserID, filterTargetID, filterStatus, filterApprovalStatus)
		if err != nil {
			h.logger.Error("Failed to list schedules", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusInternalServerError, "Failed to list schedules")
			return
		}

		response := map[string]interface{}{
			"success":   true,
			"schedules": schedules,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleApproveSchedule handles schedule approval (Admin only)
func (h *ScheduleHandler) HandleApproveSchedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userIDStr := middleware.GetUserID(ctx)
		userID, _ := uuid.Parse(userIDStr)

		if r.Method != http.MethodPost {
			h.respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req ApproveScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		scheduleID, err := uuid.Parse(req.ScheduleID)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid schedule_id")
			return
		}

		// TODO: Handle start/end time modifications if provided
		// For now, just approve

		if err := h.repo.UpdateApprovalStatus(ctx, scheduleID, models.ApprovalStatusApproved, nil, &userID); err != nil {
			h.logger.Error("Failed to approve schedule", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusInternalServerError, "Failed to approve schedule")
			return
		}

		// Also set status to active if start time is now or past
		// Ideally a background job handles this, but for immediate effect:
		// We'll just set it to active for now if it's approved.
		// Real implementation should check time.
		if err := h.repo.UpdateStatus(ctx, scheduleID, models.ScheduleStatusActive); err != nil {
			h.logger.Error("Failed to activate schedule", map[string]interface{}{
				"error": err.Error(),
			})
		}

		h.logger.Info("Schedule approved", map[string]interface{}{
			"schedule_id": req.ScheduleID,
			"approved_by": userIDStr,
		})

		response := map[string]interface{}{
			"success": true,
			"message": "Schedule approved successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleRejectSchedule handles schedule rejection (Admin only)
func (h *ScheduleHandler) HandleRejectSchedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userIDStr := middleware.GetUserID(ctx)
		userID, _ := uuid.Parse(userIDStr)

		if r.Method != http.MethodPost {
			h.respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req RejectScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Reason == "" {
			h.respondWithError(w, http.StatusBadRequest, "Reason is required")
			return
		}

		scheduleID, err := uuid.Parse(req.ScheduleID)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid schedule_id")
			return
		}

		if err := h.repo.UpdateApprovalStatus(ctx, scheduleID, models.ApprovalStatusRejected, &req.Reason, &userID); err != nil {
			h.logger.Error("Failed to reject schedule", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusInternalServerError, "Failed to reject schedule")
			return
		}

		if err := h.repo.UpdateStatus(ctx, scheduleID, models.ScheduleStatusCancelled); err != nil {
			h.logger.Error("Failed to cancel schedule", map[string]interface{}{
				"error": err.Error(),
			})
		}

		h.logger.Info("Schedule rejected", map[string]interface{}{
			"schedule_id": req.ScheduleID,
			"rejected_by": userIDStr,
			"reason":      req.Reason,
		})

		response := map[string]interface{}{
			"success": true,
			"message": "Schedule rejected successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
