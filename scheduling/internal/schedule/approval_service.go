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

	// Prepare approvedBy value (handle empty string)
	var approvedByVal interface{} = approvedBy
	if approvedBy == "" {
		approvedByVal = nil
	}

	// Update schedule
	now := time.Now()
	status := schedule.Status
	if !startTime.After(now) {
		status = "active"
	}

	query := `
		UPDATE schedules
		SET approval_status = 'approved',
		    approved_by = $1,
		    approved_at = $2,
		    start_time = $3,
		    end_time = $4,
		    updated_at = $5,
		    status = $6
		WHERE id = $7
	`

	_, err = s.db.Exec(query, approvedByVal, now, startTime, endTime, now, status, scheduleID)
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

	// Prepare rejectedBy value (handle empty string)
	var rejectedByVal interface{} = rejectedBy
	if rejectedBy == "" {
		rejectedByVal = nil
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

	_, err = s.db.Exec(query, reason, rejectedByVal, now, now, scheduleID)
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
