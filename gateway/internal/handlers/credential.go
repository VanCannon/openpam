package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// CredentialHandler handles credential-related requests
type CredentialHandler struct {
	credRepo *repository.CredentialRepository
	logger   *logger.Logger
}

// NewCredentialHandler creates a new credential handler
func NewCredentialHandler(credRepo *repository.CredentialRepository, log *logger.Logger) *CredentialHandler {
	return &CredentialHandler{
		credRepo: credRepo,
		logger:   log,
	}
}

// HandleListByTarget lists credentials for a target
func (h *CredentialHandler) HandleListByTarget() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		targetIDStr := r.URL.Query().Get("target_id")

		targetID, err := uuid.Parse(targetIDStr)
		if err != nil {
			http.Error(w, "Invalid target ID", http.StatusBadRequest)
			return
		}

		creds, err := h.credRepo.GetByTargetID(ctx, targetID)
		if err != nil {
			h.logger.Error("Failed to list credentials", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list credentials", http.StatusInternalServerError)
			return
		}

		// Don't expose vault_secret_path to API consumers
		type credResponse struct {
			ID          string `json:"id"`
			TargetID    string `json:"target_id"`
			Username    string `json:"username"`
			Description string `json:"description,omitempty"`
		}

		response := make([]credResponse, len(creds))
		for i, cred := range creds {
			response[i] = credResponse{
				ID:          cred.ID.String(),
				TargetID:    cred.TargetID.String(),
				Username:    cred.Username,
				Description: cred.Description,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"credentials": response,
			"count":       len(response),
		})
	}
}

// HandleCreate creates a new credential
func (h *CredentialHandler) HandleCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		var req struct {
			TargetID        string `json:"target_id"`
			Username        string `json:"username"`
			VaultSecretPath string `json:"vault_secret_path"`
			Description     string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.TargetID == "" || req.Username == "" || req.VaultSecretPath == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		targetID, err := uuid.Parse(req.TargetID)
		if err != nil {
			http.Error(w, "Invalid target ID", http.StatusBadRequest)
			return
		}

		cred := &models.Credential{
			TargetID:        targetID,
			Username:        req.Username,
			VaultSecretPath: req.VaultSecretPath,
			Description:     req.Description,
		}

		if err := h.credRepo.Create(ctx, cred); err != nil {
			h.logger.Error("Failed to create credential", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to create credential", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(cred)
	}
}

// HandleUpdate updates an existing credential
func (h *CredentialHandler) HandleUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		credID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid credential ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Username        string `json:"username"`
			VaultSecretPath string `json:"vault_secret_path"`
			Description     string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Username == "" || req.VaultSecretPath == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		// Get existing credential to preserve other fields
		existingCred, err := h.credRepo.GetByID(ctx, credID)
		if err != nil {
			h.logger.Error("Failed to get credential for update", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Credential not found", http.StatusNotFound)
			return
		}

		existingCred.Username = req.Username
		existingCred.VaultSecretPath = req.VaultSecretPath
		existingCred.Description = req.Description

		if err := h.credRepo.Update(ctx, existingCred); err != nil {
			h.logger.Error("Failed to update credential", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to update credential", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(existingCred)
	}
}

// HandleDelete deletes a credential
func (h *CredentialHandler) HandleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		id := r.URL.Query().Get("id")

		credID, err := uuid.Parse(id)
		if err != nil {
			http.Error(w, "Invalid credential ID", http.StatusBadRequest)
			return
		}

		if err := h.credRepo.Delete(ctx, credID); err != nil {
			h.logger.Error("Failed to delete credential", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to delete credential", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
