package worker

import (
	"context"
	"time"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/VanCannon/openpam/gateway/internal/sse"
)

const activityServiceURL = "http://activity:8083"

// ScheduleWorker handles background tasks for schedules
type ScheduleWorker struct {
	repo        *repository.ScheduleRepository
	logger      *logger.Logger
	broadcaster *sse.Broadcaster
}

// NewScheduleWorker creates a new schedule worker
func NewScheduleWorker(repo *repository.ScheduleRepository, log *logger.Logger, broadcaster *sse.Broadcaster) *ScheduleWorker {
	return &ScheduleWorker{
		repo:        repo,
		logger:      log,
		broadcaster: broadcaster,
	}
}

// Start starts the worker
func (w *ScheduleWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	w.logger.Info("Schedule worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Schedule worker stopping")
			return
		case <-ticker.C:
			w.processSchedules(ctx)
		}
	}
}

// processSchedules checks for schedules that need status updates
func (w *ScheduleWorker) processSchedules(ctx context.Context) {
	// 1. Activate pending schedules where start_time <= now
	activated, err := w.repo.UpdatePendingToActive(ctx)
	if err != nil {
		w.logger.Error("Failed to activate pending schedules", map[string]interface{}{
			"error": err.Error(),
		})
	} else if len(activated) > 0 {
		w.logger.Info("Activated schedules", map[string]interface{}{
			"count": len(activated),
		})

		// Trigger Activity Service
		for _, schedule := range activated {
			go w.handleSessionStart(ctx, schedule)
		}
	}

	// 2. Expire active schedules where end_time <= now
	expired, err := w.repo.UpdateActiveToExpired(ctx)
	if err != nil {
		w.logger.Error("Failed to expire active schedules", map[string]interface{}{
			"error": err.Error(),
		})
	} else if len(expired) > 0 {
		w.logger.Info("Expired schedules", map[string]interface{}{
			"count": len(expired),
		})

		// Trigger Activity Service
		for _, schedule := range expired {
			go w.handleSessionEnd(ctx, schedule)
		}
	}
}

func (w *ScheduleWorker) handleSessionStart(ctx context.Context, s *models.Schedule) {
	w.logger.Info("Handling session start", map[string]interface{}{"schedule_id": s.ID, "type": s.AccountType})

	switch s.AccountType {
	case "managed":
		// Enable account
		// Extract username/DN from AccountDetails
		username, ok := s.AccountDetails["sam_account_name"].(string)
		if !ok {
			w.logger.Error("Missing username in account details for managed account", nil)
			return
		}

		if err := w.callActivityService("/api/v1/activity/accounts/enable", map[string]interface{}{
			"sam_account_name": username,
		}); err != nil {
			w.logger.Error("Failed to enable managed account", map[string]interface{}{"error": err.Error()})
		}

	case "ephemeral":
		// Create ephemeral account
		// Prefix optionally in details
		prefix, _ := s.AccountDetails["prefix"].(string)
		if prefix == "" {
			prefix = "temp"
		}

		resp, err := w.callActivityServiceWithResponse("/api/v1/activity/ephemeral/create", map[string]interface{}{
			"prefix": prefix,
		})
		if err != nil {
			w.logger.Error("Failed to create ephemeral account", map[string]interface{}{"error": err.Error()})
			return
		}

		// Update schedule with new details (vault path, username, dn)
		s.AccountDetails["username"] = resp["username"]
		s.AccountDetails["dn"] = resp["dn"]
		s.AccountDetails["vault_secret_path"] = resp["vault_path"]

		if err := w.repo.UpdateAccountDetails(ctx, s.ID, s.AccountDetails); err != nil {
			w.logger.Error("Failed to update schedule details with ephemeral info", map[string]interface{}{"error": err.Error()})
		}

	case "promotion":
		// Promote user
		username, ok := s.AccountDetails["sam_account_name"].(string)
		group, ok2 := s.AccountDetails["group_dn"].(string)
		if !ok || !ok2 {
			w.logger.Error("Missing username or group_dn for promotion", nil)
			return
		}
		if err := w.callActivityService("/api/v1/activity/promotion/promote", map[string]interface{}{
			"sam_account_name": username,
			"group_dn":         group,
		}); err != nil {
			w.logger.Error("Failed to promote user", map[string]interface{}{"error": err.Error()})
		}
	}

	// Broadcast update
	w.broadcaster.BroadcastScheduleUpdate(s.ID.String(), s.UserID.String(), "schedule.activated", s)
}

func (w *ScheduleWorker) handleSessionEnd(ctx context.Context, s *models.Schedule) {
	w.logger.Info("Handling session end", map[string]interface{}{"schedule_id": s.ID, "type": s.AccountType})

	switch s.AccountType {
	case "managed":
		// Rotate password & Disable
		username, ok := s.AccountDetails["sam_account_name"].(string)
		vaultPath, ok2 := s.AccountDetails["vault_secret_path"].(string)
		if !ok {
			return
		}

		// If explicit vault path provided, use it (required for Rotation)
		if ok2 && vaultPath != "" {
			if err := w.callActivityService("/api/v1/activity/accounts/rotate", map[string]interface{}{
				"sam_account_name":  username,
				"vault_secret_path": vaultPath,
			}); err != nil {
				w.logger.Error("Failed to rotate password", map[string]interface{}{"error": err.Error()})
			}
		}

		if err := w.callActivityService("/api/v1/activity/accounts/disable", map[string]interface{}{
			"sam_account_name": username,
		}); err != nil {
			w.logger.Error("Failed to disable account", map[string]interface{}{"error": err.Error()})
		}

	case "ephemeral":
		// Delete account
		username, ok := s.AccountDetails["username"].(string) // Use the generated username
		if !ok {
			return
		}
		if err := w.callActivityService("/api/v1/activity/ephemeral/delete", map[string]interface{}{
			"sam_account_name": username,
		}); err != nil {
			w.logger.Error("Failed to delete ephemeral account", map[string]interface{}{"error": err.Error()})
		}

	case "promotion":
		// Demote
		username, ok := s.AccountDetails["sam_account_name"].(string)
		group, ok2 := s.AccountDetails["group_dn"].(string)
		if ok && ok2 {
			if err := w.callActivityService("/api/v1/activity/promotion/demote", map[string]interface{}{
				"sam_account_name": username,
				"group_dn":         group,
			}); err != nil {
				w.logger.Error("Failed to demote user", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	w.broadcaster.BroadcastScheduleUpdate(s.ID.String(), s.UserID.String(), "schedule.expired", s)
}

func (w *ScheduleWorker) callActivityService(path string, body interface{}) error {
	_, err := w.callActivityServiceWithResponse(path, body)
	return err
}

func (w *ScheduleWorker) callActivityServiceWithResponse(path string, body interface{}) (map[string]interface{}, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(activityServiceURL+path, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("activity service returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Ignore decode error if body is empty or not JSON, unless we need it
		return nil, nil
	}
	return result, nil
}
