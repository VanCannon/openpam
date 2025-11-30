package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
)

// ScheduleHandler handles schedule-related requests
type ScheduleHandler struct {
	// In the future, this will call the scheduling service via HTTP/gRPC
	// For now, we'll define the structure
	logger *logger.Logger
}

// NewScheduleHandler creates a new schedule handler
func NewScheduleHandler(log *logger.Logger) *ScheduleHandler {
	return &ScheduleHandler{
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

// HandleRequestSchedule handles schedule requests from users
func (h *ScheduleHandler) HandleRequestSchedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserID(ctx)
		userRole := middleware.GetUserRole(ctx)

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CreateScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Invalid request body", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Users can only request schedules for themselves
		if userRole != models.RoleAdmin && req.UserID != userID {
			http.Error(w, "You can only request schedules for yourself", http.StatusForbidden)
			return
		}

		// Validate time format
		startTime, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			http.Error(w, "Invalid start_time format (use RFC3339)", http.StatusBadRequest)
			return
		}

		endTime, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			http.Error(w, "Invalid end_time format (use RFC3339)", http.StatusBadRequest)
			return
		}

		// Validate time range
		if endTime.Before(startTime) || endTime.Equal(startTime) {
			http.Error(w, "end_time must be after start_time", http.StatusBadRequest)
			return
		}

		// TODO: Call scheduling service to create schedule
		// For now, return a placeholder response
		h.logger.Info("Schedule request created", map[string]interface{}{
			"user_id":    userID,
			"target_id":  req.TargetID,
			"start_time": startTime,
			"end_time":   endTime,
		})

		response := map[string]interface{}{
			"success": true,
			"message": "Schedule request created successfully",
			"schedule": map[string]interface{}{
				"id":              "placeholder-id",
				"user_id":         req.UserID,
				"target_id":       req.TargetID,
				"start_time":      startTime,
				"end_time":        endTime,
				"status":          "pending",
				"approval_status": "pending",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleListSchedules handles listing schedules
func (h *ScheduleHandler) HandleListSchedules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserID(ctx)
		userRole := middleware.GetUserRole(ctx)

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse query parameters
		targetID := r.URL.Query().Get("target_id")
		status := r.URL.Query().Get("status")
		approvalStatus := r.URL.Query().Get("approval_status")
		filterUserID := r.URL.Query().Get("user_id")

		// Non-admins can only see their own schedules
		if userRole != models.RoleAdmin {
			filterUserID = userID
		}

		// TODO: Call scheduling service to list schedules
		h.logger.Info("Listing schedules", map[string]interface{}{
			"user_id":         filterUserID,
			"target_id":       targetID,
			"status":          status,
			"approval_status": approvalStatus,
		})

		response := map[string]interface{}{
			"success":   true,
			"schedules": []interface{}{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleApproveSchedule handles schedule approval (Admin only)
func (h *ScheduleHandler) HandleApproveSchedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := middleware.GetUserID(ctx)

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ApproveScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// TODO: Call scheduling service to approve schedule
		h.logger.Info("Schedule approved", map[string]interface{}{
			"schedule_id": req.ScheduleID,
			"approved_by": userID,
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
		userID := middleware.GetUserID(ctx)

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RejectScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Reason == "" {
			http.Error(w, "Reason is required", http.StatusBadRequest)
			return
		}

		// TODO: Call scheduling service to reject schedule
		h.logger.Info("Schedule rejected", map[string]interface{}{
			"schedule_id": req.ScheduleID,
			"rejected_by": userID,
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
