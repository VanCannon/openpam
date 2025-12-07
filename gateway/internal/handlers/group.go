package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

type GroupHandler struct {
	repo   *repository.GroupRepository
	logger *logger.Logger
}

func NewGroupHandler(repo *repository.GroupRepository, log *logger.Logger) *GroupHandler {
	return &GroupHandler{
		repo:   repo,
		logger: log,
	}
}

func (h *GroupHandler) HandleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		groups, err := h.repo.List(ctx)
		if err != nil {
			h.logger.Error("Failed to list groups", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to list groups", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"groups": groups,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func (h *GroupHandler) HandleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		idStr := r.PathValue("id")
		h.logger.Info("HandleDelete called", map[string]interface{}{
			"id_str": idStr,
			"method": r.Method,
		})

		id, err := uuid.Parse(idStr)
		if err != nil {
			h.logger.Error("Invalid group ID format", map[string]interface{}{
				"id_str": idStr,
				"error":  err.Error(),
			})
			http.Error(w, "Invalid group ID", http.StatusBadRequest)
			return
		}

		if err := h.repo.Delete(ctx, id); err != nil {
			h.logger.Error("Failed to delete group", map[string]interface{}{
				"error":    err.Error(),
				"group_id": id,
			})
			http.Error(w, "Failed to delete group", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
