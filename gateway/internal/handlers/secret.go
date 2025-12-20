package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/vault"
)

type SecretHandler struct {
	vault  *vault.Client
	logger *logger.Logger
}

func NewSecretHandler(vault *vault.Client, log *logger.Logger) *SecretHandler {
	return &SecretHandler{
		vault:  vault,
		logger: log,
	}
}

type CreateSecretRequest struct {
	Path       string `json:"path"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

func (h *SecretHandler) HandleCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateSecretRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Error("Failed to decode create secret request", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Path == "" {
			h.logger.Warn("Create secret failed: Path is required")
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		if req.Username == "" {
			http.Error(w, "Username is required", http.StatusBadRequest)
			return
		}

		// Need at least password or private key
		if req.Password == "" && req.PrivateKey == "" {
			http.Error(w, "Password or Private Key is required", http.StatusBadRequest)
			return
		}

		// Write to Vault
		// We reuse the vault client's specific secret format
		// Gateway vault client expects "data" wrapping in Read/Write logic or handles it?
		// Let's check gateway/internal/client.go logic.
		// It reads nested "data". Writing usually needs explicit wrapping if using raw client,
		// BUT gateway vault client might not have a Write method exposed yet?
		// Use generic Logical().Write if needed.

		ctx := r.Context()

		// We need to access the underlying vault client to write?
		// Or add a Write method to gateway vault client like we did for activity service.
		// Let's modify vault client first if needed.

		// Assuming we'll add WriteCredentials to gateway vault client too.
		creds := &vault.Credentials{
			Username:   req.Username,
			Password:   req.Password,
			PrivateKey: req.PrivateKey,
		}

		if err := h.vault.WriteCredentials(ctx, req.Path, creds); err != nil {
			h.logger.Error("Failed to write secret to Vault", map[string]interface{}{
				"path":  req.Path,
				"error": err.Error(),
			})
			http.Error(w, "Failed to store secret", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}
