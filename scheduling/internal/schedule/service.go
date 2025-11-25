package schedule

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/openpam/scheduling/pkg/logger"
)

type Service struct {
	db     *sql.DB
	logger *logger.Logger
}

func NewService(db *sql.DB, log *logger.Logger) *Service {
	return &Service{
		db:     db,
		logger: log,
	}
}

func (s *Service) CreateSchedule(req *CreateScheduleRequest, createdBy string) (*Schedule, error) {
	schedule := &Schedule{
		ID:             uuid.New().String(),
		UserID:         req.UserID,
		TargetID:       req.TargetID,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		RecurrenceRule: req.RecurrenceRule,
		Timezone:       req.Timezone,
		Status:         "pending",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Metadata:       req.Metadata,
	}

	if createdBy != "" {
		schedule.CreatedBy = &createdBy
	}

	metadataJSON, _ := json.Marshal(schedule.Metadata)

	query := `
		INSERT INTO schedules (
			id, user_id, target_id, start_time, end_time, recurrence_rule,
			timezone, status, created_by, created_at, updated_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := s.db.Exec(query,
		schedule.ID, schedule.UserID, schedule.TargetID, schedule.StartTime,
		schedule.EndTime, schedule.RecurrenceRule, schedule.Timezone, schedule.Status,
		schedule.CreatedBy, schedule.CreatedAt, schedule.UpdatedAt, metadataJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	s.logger.Info("Schedule created", map[string]interface{}{
		"schedule_id": schedule.ID,
		"user_id":     schedule.UserID,
		"target_id":   schedule.TargetID,
	})

	return schedule, nil
}

func (s *Service) GetSchedule(id string) (*Schedule, error) {
	var schedule Schedule
	var metadataJSON []byte
	var recurrenceRule, createdBy sql.NullString

	query := `
		SELECT id, user_id, target_id, start_time, end_time, recurrence_rule,
		       timezone, status, created_by, created_at, updated_at, metadata
		FROM schedules
		WHERE id = $1
	`

	err := s.db.QueryRow(query, id).Scan(
		&schedule.ID, &schedule.UserID, &schedule.TargetID, &schedule.StartTime,
		&schedule.EndTime, &recurrenceRule, &schedule.Timezone, &schedule.Status,
		&createdBy, &schedule.CreatedAt, &schedule.UpdatedAt, &metadataJSON,
	)

	if err != nil {
		return nil, err
	}

	if recurrenceRule.Valid {
		schedule.RecurrenceRule = &recurrenceRule.String
	}
	if createdBy.Valid {
		schedule.CreatedBy = &createdBy.String
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &schedule.Metadata)
	}

	return &schedule, nil
}

func (s *Service) UpdateSchedule(id string, req *UpdateScheduleRequest) (*Schedule, error) {
	schedule, err := s.GetSchedule(id)
	if err != nil {
		return nil, err
	}

	if req.StartTime != nil {
		schedule.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		schedule.EndTime = *req.EndTime
	}
	if req.RecurrenceRule != nil {
		schedule.RecurrenceRule = req.RecurrenceRule
	}
	if req.Status != nil {
		schedule.Status = *req.Status
	}
	if req.Metadata != nil {
		schedule.Metadata = req.Metadata
	}

	schedule.UpdatedAt = time.Now()

	metadataJSON, _ := json.Marshal(schedule.Metadata)

	query := `
		UPDATE schedules
		SET start_time = $1, end_time = $2, recurrence_rule = $3, status = $4,
		    updated_at = $5, metadata = $6
		WHERE id = $7
	`

	_, err = s.db.Exec(query,
		schedule.StartTime, schedule.EndTime, schedule.RecurrenceRule,
		schedule.Status, schedule.UpdatedAt, metadataJSON, id,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	s.logger.Info("Schedule updated", map[string]interface{}{
		"schedule_id": schedule.ID,
	})

	return schedule, nil
}

func (s *Service) DeleteSchedule(id string) error {
	query := `DELETE FROM schedules WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	s.logger.Info("Schedule deleted", map[string]interface{}{
		"schedule_id": id,
	})

	return nil
}

func (s *Service) ListSchedules(req *ListSchedulesRequest) ([]*Schedule, error) {
	query := `
		SELECT id, user_id, target_id, start_time, end_time, recurrence_rule,
		       timezone, status, created_by, created_at, updated_at, metadata
		FROM schedules
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if req.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, *req.UserID)
		argCount++
	}

	if req.TargetID != nil {
		query += fmt.Sprintf(" AND target_id = $%d", argCount)
		args = append(args, *req.TargetID)
		argCount++
	}

	if req.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, *req.Status)
		argCount++
	}

	query += " ORDER BY start_time DESC"

	if req.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, req.Limit)
		argCount++
	}

	if req.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, req.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	defer rows.Close()

	schedules := []*Schedule{}
	for rows.Next() {
		var schedule Schedule
		var metadataJSON []byte
		var recurrenceRule, createdBy sql.NullString

		err := rows.Scan(
			&schedule.ID, &schedule.UserID, &schedule.TargetID, &schedule.StartTime,
			&schedule.EndTime, &recurrenceRule, &schedule.Timezone, &schedule.Status,
			&createdBy, &schedule.CreatedAt, &schedule.UpdatedAt, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if recurrenceRule.Valid {
			schedule.RecurrenceRule = &recurrenceRule.String
		}
		if createdBy.Valid {
			schedule.CreatedBy = &createdBy.String
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &schedule.Metadata)
		}

		schedules = append(schedules, &schedule)
	}

	return schedules, nil
}

func (s *Service) CheckAccess(userID, targetID string) (*ScheduleCheckResponse, error) {
	now := time.Now()

	query := `
		SELECT id, user_id, target_id, start_time, end_time, recurrence_rule,
		       timezone, status, created_by, created_at, updated_at, metadata
		FROM schedules
		WHERE user_id = $1 AND target_id = $2 AND status = 'active'
		  AND start_time <= $3 AND end_time >= $3
		LIMIT 1
	`

	var schedule Schedule
	var metadataJSON []byte
	var recurrenceRule, createdBy sql.NullString

	err := s.db.QueryRow(query, userID, targetID, now).Scan(
		&schedule.ID, &schedule.UserID, &schedule.TargetID, &schedule.StartTime,
		&schedule.EndTime, &recurrenceRule, &schedule.Timezone, &schedule.Status,
		&createdBy, &schedule.CreatedAt, &schedule.UpdatedAt, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return &ScheduleCheckResponse{
			Allowed: false,
			Message: "No active schedule found for this user and target",
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to check access: %w", err)
	}

	if recurrenceRule.Valid {
		schedule.RecurrenceRule = &recurrenceRule.String
	}
	if createdBy.Valid {
		schedule.CreatedBy = &createdBy.String
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &schedule.Metadata)
	}

	return &ScheduleCheckResponse{
		Allowed:   true,
		Schedule:  &schedule,
		Message:   "Access granted",
		ExpiresAt: &schedule.EndTime,
	}, nil
}

func (s *Service) GetUpcomingSchedules(window time.Duration) ([]*Schedule, error) {
	now := time.Now()
	future := now.Add(window)

	query := `
		SELECT id, user_id, target_id, start_time, end_time, recurrence_rule,
		       timezone, status, created_by, created_at, updated_at, metadata
		FROM schedules
		WHERE status IN ('pending', 'active')
		  AND start_time <= $1 AND end_time >= $2
		ORDER BY start_time ASC
	`

	rows, err := s.db.Query(query, future, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get upcoming schedules: %w", err)
	}
	defer rows.Close()

	schedules := []*Schedule{}
	for rows.Next() {
		var schedule Schedule
		var metadataJSON []byte
		var recurrenceRule, createdBy sql.NullString

		err := rows.Scan(
			&schedule.ID, &schedule.UserID, &schedule.TargetID, &schedule.StartTime,
			&schedule.EndTime, &recurrenceRule, &schedule.Timezone, &schedule.Status,
			&createdBy, &schedule.CreatedAt, &schedule.UpdatedAt, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if recurrenceRule.Valid {
			schedule.RecurrenceRule = &recurrenceRule.String
		}
		if createdBy.Valid {
			schedule.CreatedBy = &createdBy.String
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &schedule.Metadata)
		}

		schedules = append(schedules, &schedule)
	}

	return schedules, nil
}

func (s *Service) UpdateScheduleStatuses() error {
	now := time.Now()

	// Activate pending schedules that have started
	activateQuery := `
		UPDATE schedules
		SET status = 'active', updated_at = $1
		WHERE status = 'pending' AND start_time <= $1
	`
	_, err := s.db.Exec(activateQuery, now)
	if err != nil {
		return fmt.Errorf("failed to activate schedules: %w", err)
	}

	// Expire active schedules that have ended
	expireQuery := `
		UPDATE schedules
		SET status = 'expired', updated_at = $1
		WHERE status = 'active' AND end_time < $1
	`
	_, err = s.db.Exec(expireQuery, now)
	if err != nil {
		return fmt.Errorf("failed to expire schedules: %w", err)
	}

	return nil
}
