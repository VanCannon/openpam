package repository

import (
	"context"
	"fmt"

	"github.com/VanCannon/openpam/gateway/internal/database"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/google/uuid"
)

type GroupRepository struct {
	db *database.DB
}

func NewGroupRepository(db *database.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

func (r *GroupRepository) List(ctx context.Context) ([]models.Group, error) {
	query := `SELECT id, name, COALESCE(dn, '') as dn, COALESCE(description, '') as description, role, source, created_at FROM groups`

	var groups []models.Group
	err := r.db.SelectContext(ctx, &groups, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	return groups, nil
}

func (r *GroupRepository) GetByDN(ctx context.Context, dn string) (*models.Group, error) {
	query := `SELECT id, name, COALESCE(dn, '') as dn, COALESCE(description, '') as description, role, source, created_at FROM groups WHERE dn = $1`

	var group models.Group
	err := r.db.GetContext(ctx, &group, query, dn)
	if err != nil {
		return nil, fmt.Errorf("failed to get group by DN: %w", err)
	}

	return &group, nil
}

func (r *GroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM groups WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	return nil
}
