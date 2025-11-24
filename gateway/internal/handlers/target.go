package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/repository"
)

// TargetHandler handles target-related requests
type TargetHandler struct {
	targetRepo *repository.TargetRepository
	logger     *logger.Logger
}

// NewTargetHandler creates a new target handler
func NewTargetHandler(targetRepo *repository.TargetRepository, log *logger.Logger) *TargetHandler {
	return &TargetHandler{
		targetRepo: targetRepo,
		logger:     log,
	}
}

// HandleList returns a list of available targets
func (h *TargetHandler) HandleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse pagination parameters
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 50 // default
		offset := 0

		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		// Get targets from database
		targets, err := h.targetRepo.List(ctx, limit, offset)
		if err != nil {
			h.logger.Error("Failed to list targets", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list targets", http.StatusInternalServerError)
			return
		}

		// Build response
		type targetResponse struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Hostname    string `json:"hostname"`
			Protocol    string `json:"protocol"`
			Port        int    `json:"port"`
			Description string `json:"description,omitempty"`
		}

		response := make([]targetResponse, len(targets))
		for i, target := range targets {
			response[i] = targetResponse{
				ID:          target.ID.String(),
				Name:        target.Name,
				Hostname:    target.Hostname,
				Protocol:    target.Protocol,
				Port:        target.Port,
				Description: target.Description,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"targets": response,
			"count":   len(response),
			"limit":   limit,
			"offset":  offset,
		})
	}
}
