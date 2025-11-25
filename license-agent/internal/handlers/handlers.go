package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/openpam/license-agent/internal/license"
	"github.com/openpam/license-agent/pkg/logger"
)

type Handler struct {
	service *license.Service
	logger  *logger.Logger
}

func New(service *license.Service, log *logger.Logger) *Handler {
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
		"service": "license-agent",
	})
}

func (h *Handler) ValidateLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req license.ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.LicenseKey == "" {
		h.errorResponse(w, "License key is required", http.StatusBadRequest)
		return
	}

	response, err := h.service.ValidateLicense(req.LicenseKey)
	if err != nil {
		h.logger.Error("Failed to validate license", map[string]interface{}{
			"error": err.Error(),
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, response, http.StatusOK)
}

func (h *Handler) GetUsageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.service.GetUsageStats()
	if err != nil {
		h.logger.Error("Failed to get usage stats", map[string]interface{}{
			"error": err.Error(),
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, stats, http.StatusOK)
}

func (h *Handler) CheckFeature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req license.FeatureCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Feature == "" {
		h.errorResponse(w, "Feature name is required", http.StatusBadRequest)
		return
	}

	response, err := h.service.CheckFeature(req.Feature)
	if err != nil {
		h.logger.Error("Failed to check feature", map[string]interface{}{
			"error":   err.Error(),
			"feature": req.Feature,
		})
		h.errorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, response, http.StatusOK)
}

func (h *Handler) GetLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	license, err := h.service.GetActiveLicense()
	if err != nil {
		h.logger.Error("Failed to get license", map[string]interface{}{
			"error": err.Error(),
		})
		h.errorResponse(w, "License not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, license, http.StatusOK)
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
