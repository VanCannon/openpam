package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/database"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

// ZoneRepository handles zone data operations
type ZoneRepository struct {
	db *database.DB
}

// NewZoneRepository creates a new zone repository
func NewZoneRepository(db *database.DB) *ZoneRepository {
	return &ZoneRepository{db: db}
}

// Create creates a new zone
func (r *ZoneRepository) Create(ctx context.Context, zone *models.Zone) error {
	query := `
		INSERT INTO zones (id, name, type, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	zone.ID = uuid.New()
	zone.CreatedAt = time.Now()
	zone.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		zone.ID,
		zone.Name,
		zone.Type,
		zone.Description,
		zone.CreatedAt,
		zone.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create zone: %w", err)
	}

	return nil
}

// GetByID retrieves a zone by ID
func (r *ZoneRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Zone, error) {
	query := `
		SELECT id, name, type, description, created_at, updated_at
		FROM zones
		WHERE id = $1
	`

	var zone models.Zone
	err := r.db.GetContext(ctx, &zone, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("zone not found")
		}
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}

	return &zone, nil
}

// GetByName retrieves a zone by name
func (r *ZoneRepository) GetByName(ctx context.Context, name string) (*models.Zone, error) {
	query := `
		SELECT id, name, type, description, created_at, updated_at
		FROM zones
		WHERE name = $1
	`

	var zone models.Zone
	err := r.db.GetContext(ctx, &zone, query, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("zone not found")
		}
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}

	return &zone, nil
}

// List retrieves all zones
func (r *ZoneRepository) List(ctx context.Context) ([]*models.Zone, error) {
	query := `
		SELECT id, name, type, description, created_at, updated_at
		FROM zones
		ORDER BY name ASC
	`

	var zones []*models.Zone
	err := r.db.SelectContext(ctx, &zones, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	return zones, nil
}

// Update updates a zone
func (r *ZoneRepository) Update(ctx context.Context, zone *models.Zone) error {
	query := `
		UPDATE zones
		SET name = $1, type = $2, description = $3, updated_at = $4
		WHERE id = $5
	`

	zone.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		zone.Name,
		zone.Type,
		zone.Description,
		zone.UpdatedAt,
		zone.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update zone: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("zone not found")
	}

	return nil
}

// Delete deletes a zone
func (r *ZoneRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM zones WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete zone: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("zone not found")
	}

	return nil
}
