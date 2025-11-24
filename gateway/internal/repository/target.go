package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bvanc/openpam/gateway/internal/database"
	"github.com/bvanc/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

// TargetRepository handles target data operations
type TargetRepository struct {
	db *database.DB
}

// NewTargetRepository creates a new target repository
func NewTargetRepository(db *database.DB) *TargetRepository {
	return &TargetRepository{db: db}
}

// Create creates a new target
func (r *TargetRepository) Create(ctx context.Context, target *models.Target) error {
	query := `
		INSERT INTO targets (id, zone_id, name, hostname, protocol, port, description, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	target.ID = uuid.New()
	target.CreatedAt = time.Now()
	target.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		target.ID,
		target.ZoneID,
		target.Name,
		target.Hostname,
		target.Protocol,
		target.Port,
		target.Description,
		target.Enabled,
		target.CreatedAt,
		target.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create target: %w", err)
	}

	return nil
}

// GetByID retrieves a target by ID
func (r *TargetRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Target, error) {
	query := `
		SELECT id, zone_id, name, hostname, protocol, port, description, enabled, created_at, updated_at
		FROM targets
		WHERE id = $1
	`

	var target models.Target
	err := r.db.GetContext(ctx, &target, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("target not found")
		}
		return nil, fmt.Errorf("failed to get target: %w", err)
	}

	return &target, nil
}

// List retrieves all enabled targets with pagination
func (r *TargetRepository) List(ctx context.Context, limit, offset int) ([]*models.Target, error) {
	query := `
		SELECT id, zone_id, name, hostname, protocol, port, description, enabled, created_at, updated_at
		FROM targets
		WHERE enabled = true
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`

	var targets []*models.Target
	err := r.db.SelectContext(ctx, &targets, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}

	return targets, nil
}

// ListByZone retrieves targets for a specific zone
func (r *TargetRepository) ListByZone(ctx context.Context, zoneID uuid.UUID) ([]*models.Target, error) {
	query := `
		SELECT id, zone_id, name, hostname, protocol, port, description, enabled, created_at, updated_at
		FROM targets
		WHERE zone_id = $1 AND enabled = true
		ORDER BY name ASC
	`

	var targets []*models.Target
	err := r.db.SelectContext(ctx, &targets, query, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to list targets by zone: %w", err)
	}

	return targets, nil
}

// Update updates a target
func (r *TargetRepository) Update(ctx context.Context, target *models.Target) error {
	query := `
		UPDATE targets
		SET zone_id = $1, name = $2, hostname = $3, protocol = $4, port = $5,
		    description = $6, enabled = $7, updated_at = $8
		WHERE id = $9
	`

	target.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		target.ZoneID,
		target.Name,
		target.Hostname,
		target.Protocol,
		target.Port,
		target.Description,
		target.Enabled,
		target.UpdatedAt,
		target.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update target: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("target not found")
	}

	return nil
}

// Delete deletes a target
func (r *TargetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM targets WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete target: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("target not found")
	}

	return nil
}
