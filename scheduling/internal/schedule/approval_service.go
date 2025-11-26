package schedule

import (
	"fmt"
	"time"
)

// ApproveSchedule approves a pending schedule request
func (s *Service) ApproveSchedule(scheduleID, approvedBy string, modifyStartTime, modifyEndTime *time.Time) (*Schedule, error) {
	// Get the current schedule
	schedule, err := s.GetSchedule(scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	// Verify schedule is pending approval
	if schedule.ApprovalStatus != "pending" {
		return nil, fmt.Errorf("schedule %s is not pending approval (current status: %s)", scheduleID, schedule.ApprovalStatus)
	}

	// Use provided times or keep original
	startTime := schedule.StartTime
	endTime := schedule.EndTime
	if modifyStartTime != nil {
		startTime = *modifyStartTime
	}
	if modifyEndTime != nil {
		endTime = *modifyEndTime
	}

	// Update schedule
	now := time.Now()
	query := `
		UPDATE schedules
		SET approval_status = 'approved',
		    approved_by = $1,
		    approved_at = $2,
		    start_time = $3,
		    end_time = $4,
		    updated_at = $5
		WHERE id = $6
	`

	_, err = s.db.Exec(query, approvedBy, now, startTime, endTime, now, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to approve schedule: %w", err)
	}

	s.logger.Info("Schedule approved", map[string]interface{}{
		"schedule_id": scheduleID,
		"approved_by": approvedBy,
		"start_time":  startTime,
		"end_time":    endTime,
	})

	// Return updated schedule
	return s.GetSchedule(scheduleID)
}

// RejectSchedule rejects a pending schedule request
func (s *Service) RejectSchedule(scheduleID, rejectedBy, reason string) error {
	// Get the current schedule
	schedule, err := s.GetSchedule(scheduleID)
	if err != nil {
		return fmt.Errorf("failed to get schedule: %w", err)
	}

	// Verify schedule is pending approval
	if schedule.ApprovalStatus != "pending" {
		return fmt.Errorf("schedule %s is not pending approval (current status: %s)", scheduleID, schedule.ApprovalStatus)
	}

	// Update schedule
	now := time.Now()
	query := `
		UPDATE schedules
		SET approval_status = 'rejected',
		    rejection_reason = $1,
		    approved_by = $2,
		    approved_at = $3,
		    updated_at = $4
		WHERE id = $5
	`

	_, err = s.db.Exec(query, reason, rejectedBy, now, now, scheduleID)
	if err != nil {
		return fmt.Errorf("failed to reject schedule: %w", err)
	}

	s.logger.Info("Schedule rejected", map[string]interface{}{
		"schedule_id": scheduleID,
		"rejected_by": rejectedBy,
		"reason":      reason,
	})

	return nil
}
