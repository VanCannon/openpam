package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

// HandleCreate creates a new target
func (h *TargetHandler) HandleCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		var req struct {
			ZoneID      string `json:"zone_id"`
			Name        string `json:"name"`
			Hostname    string `json:"hostname"`
			Protocol    string `json:"protocol"`
			Port        int    `json:"port"`
			Description string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate
		if req.Name == "" || req.Hostname == "" || req.Protocol == "" || req.ZoneID == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		if req.Protocol != models.ProtocolSSH && req.Protocol != models.ProtocolRDP {
			http.Error(w, "Invalid protocol", http.StatusBadRequest)
			return
		}

		if req.Port <= 0 || req.Port > 65535 {
			http.Error(w, "Invalid port", http.StatusBadRequest)
			return
		}

		zoneID, err := uuid.Parse(req.ZoneID)
		if err != nil {
			http.Error(w, "Invalid zone ID", http.StatusBadRequest)
			return
		}

		target := &models.Target{
			ZoneID:      zoneID,
			Name:        req.Name,
			Hostname:    req.Hostname,
			Protocol:    req.Protocol,
			Port:        req.Port,
			Description: req.Description,
			Enabled:     true,
		}

		if err := h.targetRepo.Create(ctx, target); err != nil {
			h.logger.Error("Failed to create target", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to create target", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(target)
	}
}

// HandleGet gets a target by ID
func (h *TargetHandler) HandleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		targetID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid target ID", http.StatusBadRequest)
			return
		}

		target, err := h.targetRepo.GetByID(ctx, targetID)
		if err != nil {
			http.Error(w, "Target not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(target)
	}
}

// HandleUpdate updates a target
func (h *TargetHandler) HandleUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		targetID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid target ID", http.StatusBadRequest)
			return
		}

		var req struct {
			ZoneID      string `json:"zone_id"`
			Name        string `json:"name"`
			Hostname    string `json:"hostname"`
			Protocol    string `json:"protocol"`
			Port        int    `json:"port"`
			Description string `json:"description"`
			Enabled     bool   `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		target, err := h.targetRepo.GetByID(ctx, targetID)
		if err != nil {
			http.Error(w, "Target not found", http.StatusNotFound)
			return
		}

		zoneID, err := uuid.Parse(req.ZoneID)
		if err != nil {
			http.Error(w, "Invalid zone ID", http.StatusBadRequest)
			return
		}

		target.ZoneID = zoneID
		target.Name = req.Name
		target.Hostname = req.Hostname
		target.Protocol = req.Protocol
		target.Port = req.Port
		target.Description = req.Description
		target.Enabled = req.Enabled

		if err := h.targetRepo.Update(ctx, target); err != nil {
			h.logger.Error("Failed to update target", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to update target", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(target)
	}
}

// HandleDelete deletes a target
func (h *TargetHandler) HandleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		targetID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid target ID", http.StatusBadRequest)
			return
		}

		if err := h.targetRepo.Delete(ctx, targetID); err != nil {
			h.logger.Error("Failed to delete target", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to delete target", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
