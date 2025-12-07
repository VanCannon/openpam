package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// UserHandler handles user management requests
type UserHandler struct {
	repo   *repository.UserRepository
	logger *logger.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(repo *repository.UserRepository, log *logger.Logger) *UserHandler {
	return &UserHandler{
		repo:   repo,
		logger: log,
	}
}

// HandleList lists all users
func (h *UserHandler) HandleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Parse pagination
		limit := 50
		offset := 0

		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 {
				limit = v
			}
		}

		if o := r.URL.Query().Get("offset"); o != "" {
			if v, err := strconv.Atoi(o); err == nil && v >= 0 {
				offset = v
			}
		}

		users, err := h.repo.List(ctx, limit, offset)
		if err != nil {
			h.logger.Error("Failed to list users", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list users", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"users": users,
			"total": len(users), // TODO: Add count query for real total
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleUpdateRole updates a user's role
func (h *UserHandler) HandleUpdateRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		idStr := r.PathValue("id")

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Role string `json:"role"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate role
		if req.Role != models.RoleAdmin && req.Role != models.RoleUser && req.Role != models.RoleAuditor {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}

		user, err := h.repo.GetByID(ctx, id)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		user.Role = req.Role
		if err := h.repo.Update(ctx, user); err != nil {
			h.logger.Error("Failed to update user role", map[string]interface{}{
				"error":   err.Error(),
				"user_id": id,
			})
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

// HandleUpdateEnabled updates a user's enabled status
func (h *UserHandler) HandleUpdateEnabled() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		idStr := r.PathValue("id")

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Enabled bool `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		user, err := h.repo.GetByID(ctx, id)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		user.Enabled = req.Enabled
		if err := h.repo.Update(ctx, user); err != nil {
			h.logger.Error("Failed to update user status", map[string]interface{}{
				"error":   err.Error(),
				"user_id": id,
			})
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

// HandleDelete deletes a user
func (h *UserHandler) HandleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		idStr := r.PathValue("id")

		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		// Check if user exists
		_, err = h.repo.GetByID(ctx, id)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		if err := h.repo.Delete(ctx, id); err != nil {
			h.logger.Error("Failed to delete user", map[string]interface{}{
				"error":   err.Error(),
				"user_id": id,
			})
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
