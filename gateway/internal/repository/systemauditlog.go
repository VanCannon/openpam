package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bvanc/openpam/gateway/internal/database"
	"github.com/bvanc/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

// SystemAuditLogRepository handles system audit log data operations
type SystemAuditLogRepository struct {
	db *database.DB
}

// NewSystemAuditLogRepository creates a new system audit log repository
func NewSystemAuditLogRepository(db *database.DB) *SystemAuditLogRepository {
	return &SystemAuditLogRepository{db: db}
}

// Create creates a new system audit log entry
func (r *SystemAuditLogRepository) Create(ctx context.Context, log *models.SystemAuditLog) error {
	query := `
		INSERT INTO system_audit_logs (
			id, timestamp, event_type, user_id, target_user_id, resource_type,
			resource_id, resource_name, action, status, ip_address, user_agent, details, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	log.ID = uuid.New()
	log.Timestamp = time.Now()
	log.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		log.ID,
		log.Timestamp,
		log.EventType,
		log.UserID,
		log.TargetUserID,
		log.ResourceType,
		log.ResourceID,
		log.ResourceName,
		log.Action,
		log.Status,
		log.IPAddress,
		log.UserAgent,
		log.Details,
		log.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create system audit log: %w", err)
	}

	return nil
}

// CreateSimple is a helper method to create a simple audit log entry
func (r *SystemAuditLogRepository) CreateSimple(
	ctx context.Context,
	eventType string,
	userID *uuid.UUID,
	action string,
	status string,
	ipAddress *string,
	details map[string]interface{},
) error {
	log := &models.SystemAuditLog{
		EventType: eventType,
		Action:    action,
		Status:    status,
		IPAddress: ipAddress,
	}

	if userID != nil {
		log.UserID = uuid.NullUUID{UUID: *userID, Valid: true}
	}

	if details != nil {
		detailsJSON, err := json.Marshal(details)
		if err == nil {
			detailsStr := string(detailsJSON)
			log.Details = &detailsStr
		}
	}

	return r.Create(ctx, log)
}

// List retrieves all system audit logs with pagination
func (r *SystemAuditLogRepository) List(ctx context.Context, limit, offset int) ([]*models.SystemAuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, user_id, target_user_id, resource_type,
		       resource_id, resource_name, action, status, ip_address, user_agent, details, created_at
		FROM system_audit_logs
		ORDER BY timestamp DESC
		LIMIT $1 OFFSET $2
	`

	var logs []*models.SystemAuditLog
	err := r.db.SelectContext(ctx, &logs, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list system audit logs: %w", err)
	}

	return logs, nil
}

// ListByUser retrieves system audit logs for a specific user
func (r *SystemAuditLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.SystemAuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, user_id, target_user_id, resource_type,
		       resource_id, resource_name, action, status, ip_address, user_agent, details, created_at
		FROM system_audit_logs
		WHERE user_id = $1 OR target_user_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	var logs []*models.SystemAuditLog
	err := r.db.SelectContext(ctx, &logs, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list system audit logs by user: %w", err)
	}

	return logs, nil
}

// ListByEventType retrieves system audit logs by event type
func (r *SystemAuditLogRepository) ListByEventType(ctx context.Context, eventType string, limit, offset int) ([]*models.SystemAuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, user_id, target_user_id, resource_type,
		       resource_id, resource_name, action, status, ip_address, user_agent, details, created_at
		FROM system_audit_logs
		WHERE event_type = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	var logs []*models.SystemAuditLog
	err := r.db.SelectContext(ctx, &logs, query, eventType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list system audit logs by event type: %w", err)
	}

	return logs, nil
}

// ListByResource retrieves system audit logs for a specific resource
func (r *SystemAuditLogRepository) ListByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit, offset int) ([]*models.SystemAuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, user_id, target_user_id, resource_type,
		       resource_id, resource_name, action, status, ip_address, user_agent, details, created_at
		FROM system_audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY timestamp DESC
		LIMIT $3 OFFSET $4
	`

	var logs []*models.SystemAuditLog
	err := r.db.SelectContext(ctx, &logs, query, resourceType, resourceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list system audit logs by resource: %w", err)
	}

	return logs, nil
}

// GetByID retrieves a system audit log by ID
func (r *SystemAuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SystemAuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, user_id, target_user_id, resource_type,
		       resource_id, resource_name, action, status, ip_address, user_agent, details, created_at
		FROM system_audit_logs
		WHERE id = $1
	`

	var log models.SystemAuditLog
	err := r.db.GetContext(ctx, &log, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get system audit log: %w", err)
	}

	return &log, nil
}

// Count returns the total number of system audit logs
func (r *SystemAuditLogRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM system_audit_logs`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count system audit logs: %w", err)
	}

	return count, nil
}
