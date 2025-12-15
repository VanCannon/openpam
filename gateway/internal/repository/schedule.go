package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/database"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

// ScheduleRepository handles database operations for schedules
type ScheduleRepository struct {
	db *database.DB
}

// NewScheduleRepository creates a new schedule repository
func NewScheduleRepository(db *database.DB) *ScheduleRepository {
	return &ScheduleRepository{
		db: db,
	}
}

// Create creates a new schedule
func (r *ScheduleRepository) Create(ctx context.Context, schedule *models.Schedule) error {
	query := `
		INSERT INTO schedules (
			id, user_id, target_id, start_time, end_time, recurrence_rule, timezone,
			status, created_by, created_at, updated_at, metadata,
			approval_status, rejection_reason, approved_by, approved_at,
			type, account_type, account_details
		) VALUES (
			:id, :user_id, :target_id, :start_time, :end_time, :recurrence_rule, :timezone,
			:status, :created_by, :created_at, :updated_at, :metadata,
			:approval_status, :rejection_reason, :approved_by, :approved_at,
			:type, :account_type, :account_details
		)
	`
	_, err := r.db.NamedExecContext(ctx, query, schedule)
	return err
}

// GetByID retrieves a schedule by ID
func (r *ScheduleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Schedule, error) {
	var schedule models.Schedule
	query := `SELECT * FROM schedules WHERE id = $1`
	if err := r.db.GetContext(ctx, &schedule, query, id); err != nil {
		return nil, err
	}
	return &schedule, nil
}

// List retrieves a list of schedules based on filters
func (r *ScheduleRepository) List(ctx context.Context, userID *uuid.UUID, targetID *uuid.UUID, status *models.ScheduleStatus, approvalStatus *string, scheduleType *string) ([]models.Schedule, error) {
	query := `SELECT * FROM schedules WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if userID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *userID)
		argIdx++
	}

	if targetID != nil {
		query += fmt.Sprintf(" AND target_id = $%d", argIdx)
		args = append(args, *targetID)
		argIdx++
	}

	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	if approvalStatus != nil {
		query += fmt.Sprintf(" AND approval_status = $%d", argIdx)
		args = append(args, *approvalStatus)
		argIdx++
	}

	if scheduleType != nil {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, *scheduleType)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	var schedules []models.Schedule
	if err := r.db.SelectContext(ctx, &schedules, query, args...); err != nil {
		return nil, err
	}
	return schedules, nil
}

// UpdateStatus updates the status of a schedule
func (r *ScheduleRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ScheduleStatus) error {
	query := `UPDATE schedules SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

// UpdateApprovalStatus updates the approval status of a schedule
func (r *ScheduleRepository) UpdateApprovalStatus(ctx context.Context, id uuid.UUID, status string, reason *string, approvedBy *uuid.UUID) error {
	query := `
		UPDATE schedules 
		SET approval_status = $1, rejection_reason = $2, approved_by = $3, approved_at = $4, updated_at = $5 
		WHERE id = $6
	`
	var approvedAt *time.Time
	if status == models.ApprovalStatusApproved {
		now := time.Now()
		approvedAt = &now
	}

	_, err := r.db.ExecContext(ctx, query, status, reason, approvedBy, approvedAt, time.Now(), id)
	return err
}

// Delete deletes a schedule by ID
func (r *ScheduleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM schedules WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdateScheduleDetails updates the details of a schedule (start/end time, type)
func (r *ScheduleRepository) UpdateScheduleDetails(ctx context.Context, id uuid.UUID, startTime *time.Time, endTime *time.Time, scheduleType *string) error {
	query := `UPDATE schedules SET updated_at = $1`
	args := []interface{}{time.Now()}
	argIdx := 2

	if startTime != nil {
		query += fmt.Sprintf(", start_time = $%d", argIdx)
		args = append(args, *startTime)
		argIdx++
	}

	if endTime != nil {
		query += fmt.Sprintf(", end_time = $%d", argIdx)
		args = append(args, *endTime)
		argIdx++
	}

	if scheduleType != nil {
		query += fmt.Sprintf(", type = $%d", argIdx)
		args = append(args, *scheduleType)
		argIdx++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// UpdatePendingToActive updates pending/approved schedules to active if start time has passed
func (r *ScheduleRepository) UpdatePendingToActive(ctx context.Context) error {
	query := `
		UPDATE schedules 
		SET status = $1, updated_at = $2 
		WHERE (status = $3 OR (status = $4 AND approval_status = $5))
		AND start_time <= $6
		AND type = 'scheduled'
	`
	// Status: Active
	// Where: (Pending OR (Pending AND Approved)) -- actually logic is:
	// We want to activate schedules that are:
	// 1. Status is 'pending' (which is default)
	// 2. ApprovalStatus is 'approved'
	// 3. StartTime <= Now

	// Correct query:
	query = `
		UPDATE schedules 
		SET status = $1, updated_at = $2 
		WHERE status = $3 
		AND approval_status = $4
		AND start_time <= $5
		AND type = 'scheduled'
	`

	_, err := r.db.ExecContext(ctx, query,
		models.ScheduleStatusActive,
		time.Now(),
		models.ScheduleStatusPending,
		models.ApprovalStatusApproved,
		time.Now(),
	)
	return err
}

// UpdateActiveToExpired updates active schedules to expired/completed if end time has passed
func (r *ScheduleRepository) UpdateActiveToExpired(ctx context.Context) error {
	query := `
		UPDATE schedules 
		SET status = $1, updated_at = $2 
		WHERE status = $3 
		AND end_time <= $4
		AND type = 'scheduled'
	`

	_, err := r.db.ExecContext(ctx, query,
		models.ScheduleStatusExpired,
		time.Now(),
		models.ScheduleStatusActive,
		time.Now(),
	)
	return err
}
