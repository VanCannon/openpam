package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/database"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

// AuditLogRepository handles audit log data operations
type AuditLogRepository struct {
	db *database.DB
}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *database.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Create creates a new audit log entry
func (r *AuditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			id, user_id, target_id, credential_id, start_time, session_status,
			client_ip, bytes_sent, bytes_received, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	log.ID = uuid.New()
	log.StartTime = time.Now()
	log.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		log.ID,
		log.UserID,
		log.TargetID,
		log.CredentialID,
		log.StartTime,
		log.SessionStatus,
		log.ClientIP,
		log.BytesSent,
		log.BytesReceived,
		log.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// UpdateStatus updates the status and end time of an audit log
func (r *AuditLogRepository) UpdateStatus(ctx context.Context, log *models.AuditLog) error {
	query := `
		UPDATE audit_logs
		SET end_time = $1, bytes_sent = $2, bytes_received = $3,
		    session_status = $4, error_message = $5, recording_path = $6
		WHERE id = $7
	`

	endTime := time.Now()
	log.EndTime.Time = endTime
	log.EndTime.Valid = true

	_, err := r.db.ExecContext(ctx, query,
		log.EndTime,
		log.BytesSent,
		log.BytesReceived,
		log.SessionStatus,
		log.ErrorMessage,
		log.RecordingPath,
		log.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update audit log: %w", err)
	}

	return nil
}

// GetByID retrieves an audit log by ID
func (r *AuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT a.id, a.user_id, a.target_id, a.credential_id, a.start_time, a.end_time,
		       a.bytes_sent, a.bytes_received, a.session_status, a.client_ip,
		       a.error_message, a.recording_path, a.created_at, t.protocol
		FROM audit_logs a
		JOIN targets t ON a.target_id = t.id
		WHERE a.id = $1
	`

	var log models.AuditLog
	err := r.db.GetContext(ctx, &log, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	return &log, nil
}

// ListByUser retrieves audit logs for a specific user
func (r *AuditLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT a.id, a.user_id, a.target_id, a.credential_id, a.start_time, a.end_time,
		       a.bytes_sent, a.bytes_received, a.session_status, a.client_ip,
		       a.error_message, a.recording_path, a.created_at, t.protocol
		FROM audit_logs a
		JOIN targets t ON a.target_id = t.id
		WHERE a.user_id = $1
		ORDER BY a.start_time DESC
		LIMIT $2 OFFSET $3
	`

	var logs []*models.AuditLog
	err := r.db.SelectContext(ctx, &logs, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs by user: %w", err)
	}

	return logs, nil
}

// ListByTarget retrieves audit logs for a specific target
func (r *AuditLogRepository) ListByTarget(ctx context.Context, targetID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT a.id, a.user_id, a.target_id, a.credential_id, a.start_time, a.end_time,
		       a.bytes_sent, a.bytes_received, a.session_status, a.client_ip,
		       a.error_message, a.recording_path, a.created_at, t.protocol
		FROM audit_logs a
		JOIN targets t ON a.target_id = t.id
		WHERE a.target_id = $1
		ORDER BY a.start_time DESC
		LIMIT $2 OFFSET $3
	`

	var logs []*models.AuditLog
	err := r.db.SelectContext(ctx, &logs, query, targetID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs by target: %w", err)
	}

	return logs, nil
}

// List retrieves all audit logs with pagination
func (r *AuditLogRepository) List(ctx context.Context, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT a.id, a.user_id, a.target_id, a.credential_id, a.start_time, a.end_time,
		       a.bytes_sent, a.bytes_received, a.session_status, a.client_ip,
		       a.error_message, a.recording_path, a.created_at, t.protocol
		FROM audit_logs a
		JOIN targets t ON a.target_id = t.id
		ORDER BY a.start_time DESC
		LIMIT $1 OFFSET $2
	`

	var logs []*models.AuditLog
	err := r.db.SelectContext(ctx, &logs, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}

	return logs, nil
}

// ListActive retrieves all active sessions
func (r *AuditLogRepository) ListActive(ctx context.Context) ([]*models.AuditLog, error) {
	query := `
		SELECT a.id, a.user_id, a.target_id, a.credential_id, a.start_time, a.end_time,
		       a.bytes_sent, a.bytes_received, a.session_status, a.client_ip,
		       a.error_message, a.recording_path, a.created_at, t.protocol
		FROM audit_logs a
		JOIN targets t ON a.target_id = t.id
		WHERE a.session_status = $1
		ORDER BY a.start_time DESC
	`

	var logs []*models.AuditLog
	err := r.db.SelectContext(ctx, &logs, query, models.SessionStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}

	return logs, nil
}
