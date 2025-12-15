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
	Type           string                 `json:"type"`
	AccountType    string                 `json:"account_type"`
	AccountDetails map[string]interface{} `json:"account_details,omitempty"`
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

		h.logger.Info("HandleRequestSchedule called", map[string]interface{}{
			"user_id": userIDStr,
			"role":    userRole,
		})

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
			Type:           req.Type,
			AccountType:    req.AccountType,
		}

		// Auto-approve if user is admin
		if userRole == models.RoleAdmin {
			schedule.ApprovalStatus = models.ApprovalStatusApproved

			// ApprovedBy should be the requester (admin), not the target user
			requesterID, _ := uuid.Parse(userIDStr)
			schedule.ApprovedBy = &requesterID

			now := time.Now()
			schedule.ApprovedAt = &now

			// If start time is now or in the past, OR if it's standing access, set status to active
			if schedule.Type == "standing" || schedule.StartTime.Before(now) || schedule.StartTime.Equal(now) {
				schedule.Status = models.ScheduleStatusActive
			}
		}

		if req.AccountDetails != nil {
			schedule.AccountDetails = req.AccountDetails
		}

		// Set defaults if missing
		if schedule.Type == "" {
			schedule.Type = "scheduled"
		}
		if schedule.AccountType == "" {
			schedule.AccountType = "static"
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
		typeStr := r.URL.Query().Get("type")
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

		var filterType *string
		if typeStr != "" {
			filterType = &typeStr
		}

		schedules, err := h.repo.List(ctx, filterUserID, filterTargetID, filterStatus, filterApprovalStatus, filterType)
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

		h.logger.Info("Approving schedule", map[string]interface{}{
			"schedule_id": req.ScheduleID,
			"req_start":   req.StartTime,
			"req_end":     req.EndTime,
		})

		// Handle start/end time modifications if provided
		var newStartTime, newEndTime *time.Time
		var newType *string

		if req.StartTime != nil {
			t, err := time.Parse(time.RFC3339, *req.StartTime)
			if err != nil {
				h.respondWithError(w, http.StatusBadRequest, "Invalid start_time format")
				return
			}
			newStartTime = &t
		}

		if req.EndTime != nil {
			t, err := time.Parse(time.RFC3339, *req.EndTime)
			if err != nil {
				h.respondWithError(w, http.StatusBadRequest, "Invalid end_time format")
				return
			}
			newEndTime = &t
		}

		if newStartTime != nil && newEndTime != nil {
			if newEndTime.Before(*newStartTime) {
				h.respondWithError(w, http.StatusBadRequest, "end_time must be after start_time")
				return
			}
			// If times are provided, force type to 'scheduled'
			t := "scheduled"
			newType = &t

			h.logger.Info("Updating schedule details", map[string]interface{}{
				"schedule_id": scheduleID,
				"new_type":    *newType,
				"new_start":   *newStartTime,
				"new_end":     *newEndTime,
			})

			if err := h.repo.UpdateScheduleDetails(ctx, scheduleID, newStartTime, newEndTime, newType); err != nil {
				h.logger.Error("Failed to update schedule details", map[string]interface{}{
					"error": err.Error(),
				})
				h.respondWithError(w, http.StatusInternalServerError, "Failed to update schedule details")
				return
			}
		} else {
			h.logger.Info("Skipping schedule details update", map[string]interface{}{
				"has_start": newStartTime != nil,
				"has_end":   newEndTime != nil,
			})
		}

		if err := h.repo.UpdateApprovalStatus(ctx, scheduleID, models.ApprovalStatusApproved, nil, &userID); err != nil {
			h.logger.Error("Failed to approve schedule", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusInternalServerError, "Failed to approve schedule")
			return
		}

		// Fetch updated schedule to check start time and type
		updatedSchedule, err := h.repo.GetByID(ctx, scheduleID)
		if err != nil {
			h.logger.Error("Failed to fetch updated schedule", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue but log error
		} else {
			now := time.Now()
			newStatus := models.ScheduleStatusPending

			// If start time is now or in the past, OR if it's standing access, set status to active
			if updatedSchedule.Type == "standing" || updatedSchedule.StartTime.Before(now) || updatedSchedule.StartTime.Equal(now) {
				newStatus = models.ScheduleStatusActive
			}

			if err := h.repo.UpdateStatus(ctx, scheduleID, newStatus); err != nil {
				h.logger.Error("Failed to update schedule status", map[string]interface{}{
					"error":  err.Error(),
					"status": newStatus,
				})
			}
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

// HandleDeleteSchedule handles schedule deletion (Admin only)
func (h *ScheduleHandler) HandleDeleteSchedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userIDStr := middleware.GetUserID(ctx)

		if r.Method != http.MethodDelete {
			h.respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Extract ID from URL path: /api/v1/schedules/{id}
		idStr := r.URL.Path[len("/api/v1/schedules/"):]

		scheduleID, err := uuid.Parse(idStr)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Invalid schedule_id")
			return
		}

		if err := h.repo.Delete(ctx, scheduleID); err != nil {
			h.logger.Error("Failed to delete schedule", map[string]interface{}{
				"error": err.Error(),
			})
			h.respondWithError(w, http.StatusInternalServerError, "Failed to delete schedule")
			return
		}

		h.logger.Info("Schedule deleted", map[string]interface{}{
			"schedule_id": scheduleID,
			"deleted_by":  userIDStr,
		})

		response := map[string]interface{}{
			"success": true,
			"message": "Schedule deleted successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
