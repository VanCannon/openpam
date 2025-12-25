package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// ZoneHandler handles zone-related requests
type ZoneHandler struct {
	zoneRepo        *repository.ZoneRepository
	systemAuditRepo *repository.SystemAuditLogRepository
	logger          *logger.Logger
}

// NewZoneHandler creates a new zone handler
func NewZoneHandler(zoneRepo *repository.ZoneRepository, systemAuditRepo *repository.SystemAuditLogRepository, log *logger.Logger) *ZoneHandler {
	return &ZoneHandler{
		zoneRepo:        zoneRepo,
		systemAuditRepo: systemAuditRepo,
		logger:          log,
	}
}

// HandleList lists all zones
func (h *ZoneHandler) HandleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		zones, err := h.zoneRepo.List(ctx)
		if err != nil {
			h.logger.Error("Failed to list zones", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list zones", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"zones": zones,
			"count": len(zones),
		})
	}
}

// HandleCreate creates a new zone
func (h *ZoneHandler) HandleCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		var req struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate
		if req.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		if req.Type != models.ZoneTypeHub && req.Type != models.ZoneTypeSatellite {
			http.Error(w, "Invalid zone type", http.StatusBadRequest)
			return
		}

		zone := &models.Zone{
			Name:        req.Name,
			Type:        req.Type,
			Description: req.Description,
		}

		if err := h.zoneRepo.Create(ctx, zone); err != nil {
			h.logger.Error("Failed to create zone", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to create zone", http.StatusInternalServerError)
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

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeZoneCreated, actorID, "create_zone", models.AuditStatusSuccess, nil, map[string]interface{}{
			"zone_id":   zone.ID,
			"zone_name": zone.Name,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(zone)
	}
}

// HandleGet gets a zone by ID
func (h *ZoneHandler) HandleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		zoneID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid zone ID", http.StatusBadRequest)
			return
		}

		zone, err := h.zoneRepo.GetByID(ctx, zoneID)
		if err != nil {
			h.logger.Error("Failed to get zone", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Zone not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(zone)
	}
}

// HandleUpdate updates a zone
func (h *ZoneHandler) HandleUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		zoneID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid zone ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		zone, err := h.zoneRepo.GetByID(ctx, zoneID)
		if err != nil {
			http.Error(w, "Zone not found", http.StatusNotFound)
			return
		}

		zone.Name = req.Name
		zone.Type = req.Type
		zone.Description = req.Description

		if err := h.zoneRepo.Update(ctx, zone); err != nil {
			h.logger.Error("Failed to update zone", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to update zone", http.StatusInternalServerError)
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

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeZoneUpdated, actorID, "update_zone", models.AuditStatusSuccess, nil, map[string]interface{}{
			"zone_id":   zone.ID,
			"zone_name": zone.Name,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(zone)
	}
}

// HandleDelete deletes a zone
func (h *ZoneHandler) HandleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		zoneID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid zone ID", http.StatusBadRequest)
			return
		}

		if err := h.zoneRepo.Delete(ctx, zoneID); err != nil {
			h.logger.Error("Failed to delete zone", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to delete zone", http.StatusInternalServerError)
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

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeZoneDeleted, actorID, "delete_zone", models.AuditStatusSuccess, nil, map[string]interface{}{
			"zone_id": zoneID,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
