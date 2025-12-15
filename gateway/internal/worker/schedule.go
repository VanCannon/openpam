package worker

import (
	"context"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/repository"
)

// ScheduleWorker handles background tasks for schedules
type ScheduleWorker struct {
	repo   *repository.ScheduleRepository
	logger *logger.Logger
}

// NewScheduleWorker creates a new schedule worker
func NewScheduleWorker(repo *repository.ScheduleRepository, log *logger.Logger) *ScheduleWorker {
	return &ScheduleWorker{
		repo:   repo,
		logger: log,
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
	if err := w.repo.UpdatePendingToActive(ctx); err != nil {
		w.logger.Error("Failed to activate pending schedules", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 2. Expire active schedules where end_time <= now
	if err := w.repo.UpdateActiveToExpired(ctx); err != nil {
		w.logger.Error("Failed to expire active schedules", map[string]interface{}{
			"error": err.Error(),
		})
	}
}
