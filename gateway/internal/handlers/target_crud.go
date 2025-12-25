package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/VanCannon/openpam/gateway/internal/middleware"
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

		// Audit log
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if actorIDStr != "" {
			if uid, err := uuid.Parse(actorIDStr); err == nil {
				actorID = &uid
			}
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeTargetCreated, actorID, "create_target", models.AuditStatusSuccess, nil, map[string]interface{}{
			"target_id":   target.ID,
			"target_name": target.Name,
		})

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

		// Audit log
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if actorIDStr != "" {
			if uid, err := uuid.Parse(actorIDStr); err == nil {
				actorID = &uid
			}
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeTargetUpdated, actorID, "update_target", models.AuditStatusSuccess, nil, map[string]interface{}{
			"target_id":   target.ID,
			"target_name": target.Name,
		})

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
				"id":    targetID,
				"error": err.Error(),
			})

			// Check for foreign key constraint violation (Postgres specific check via error string)
			// Ideally we would unwrap the error to *pq.Error but string matching is safer across drivers if generic
			errMsg := err.Error()
			if strings.Contains(errMsg, "violates foreign key constraint") {
				if strings.Contains(errMsg, "audit_logs") {
					http.Error(w, "Cannot delete target because it has associated session history. Please disable it instead.", http.StatusConflict)
					return
				}
				if strings.Contains(errMsg, "schedules") {
					http.Error(w, "Cannot delete target because it has active schedules.", http.StatusConflict)
					return
				}
				http.Error(w, "Cannot delete target because it is in use.", http.StatusConflict)
				return
			}

			http.Error(w, "Failed to delete target", http.StatusInternalServerError)
			return
		}

		// Audit log
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if actorIDStr != "" {
			if uid, err := uuid.Parse(actorIDStr); err == nil {
				actorID = &uid
			}
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeTargetDeleted, actorID, "delete_target", models.AuditStatusSuccess, nil, map[string]interface{}{
			"target_id": targetID,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
