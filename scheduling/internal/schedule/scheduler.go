package schedule

import (
	"context"
	"time"

	"github.com/openpam/scheduling/pkg/logger"
)

type Scheduler struct {
	service  *Service
	logger   *logger.Logger
	interval time.Duration
	window   time.Duration
	stopChan chan struct{}
}

func NewScheduler(service *Service, logger *logger.Logger, interval, window time.Duration) *Scheduler {
	return &Scheduler{
		service:  service,
		logger:   logger,
		interval: interval,
		window:   window,
		stopChan: make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("Scheduler started", map[string]interface{}{
		"interval": s.interval.String(),
		"window":   s.window.String(),
	})

	// Run immediately on start
	s.processSchedules()

	for {
		select {
		case <-ticker.C:
			s.processSchedules()
		case <-s.stopChan:
			s.logger.Info("Scheduler stopped", nil)
			return
		case <-ctx.Done():
			s.logger.Info("Scheduler context cancelled", nil)
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) processSchedules() {
	// Update schedule statuses (activate pending, expire ended)
	if err := s.service.UpdateScheduleStatuses(); err != nil {
		s.logger.Error("Failed to update schedule statuses", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Get upcoming schedules
	schedules, err := s.service.GetUpcomingSchedules(s.window)
	if err != nil {
		s.logger.Error("Failed to get upcoming schedules", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	s.logger.Debug("Processed schedules", map[string]interface{}{
		"count": len(schedules),
	})
}
