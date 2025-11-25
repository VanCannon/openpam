package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/openpam/scheduling-agent/internal/schedule"
	"github.com/openpam/scheduling-agent/pkg/logger"
)

type Handler struct {
	service *schedule.Service
	logger  *logger.Logger
}

func New(service *schedule.Service, log *logger.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  log,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "scheduling-agent",
	})
}

func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req schedule.CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Extract createdBy from auth context
	createdBy := r.Header.Get("X-User-ID")

	result, err := h.service.CreateSchedule(&req, createdBy)
	if err != nil {
		h.logger.Error("Failed to create schedule", map[string]interface{}{
			"error": err.Error(),
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, result, http.StatusCreated)
}

func (h *Handler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/schedules/")

	result, err := h.service.GetSchedule(id)
	if err != nil {
		h.logger.Error("Failed to get schedule", map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
		h.errorResponse(w, "Schedule not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, result, http.StatusOK)
}

func (h *Handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/schedules/")

	var req schedule.UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.service.UpdateSchedule(id, &req)
	if err != nil {
		h.logger.Error("Failed to update schedule", map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, result, http.StatusOK)
}

func (h *Handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/schedules/")

	if err := h.service.DeleteSchedule(id); err != nil {
		h.logger.Error("Failed to delete schedule", map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req schedule.ListSchedulesRequest

	if userID := r.URL.Query().Get("user_id"); userID != "" {
		req.UserID = &userID
	}
	if targetID := r.URL.Query().Get("target_id"); targetID != "" {
		req.TargetID = &targetID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		req.Status = &status
	}

	req.Limit = 50
	req.Offset = 0

	result, err := h.service.ListSchedules(&req)
	if err != nil {
		h.logger.Error("Failed to list schedules", map[string]interface{}{
			"error": err.Error(),
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, result, http.StatusOK)
}

func (h *Handler) CheckAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req schedule.ScheduleCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.TargetID == "" {
		h.errorResponse(w, "user_id and target_id are required", http.StatusBadRequest)
		return
	}

	result, err := h.service.CheckAccess(req.UserID, req.TargetID)
	if err != nil {
		h.logger.Error("Failed to check access", map[string]interface{}{
			"error": err.Error(),
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, result, http.StatusOK)
}

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) errorResponse(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
