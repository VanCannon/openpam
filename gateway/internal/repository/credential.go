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

// CredentialRepository handles credential data operations
type CredentialRepository struct {
	db *database.DB
}

// NewCredentialRepository creates a new credential repository
func NewCredentialRepository(db *database.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

// Create creates a new credential
func (r *CredentialRepository) Create(ctx context.Context, cred *models.Credential) error {
	query := `
		INSERT INTO credentials (id, target_id, username, vault_secret_path, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	cred.ID = uuid.New()
	cred.CreatedAt = time.Now()
	cred.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		cred.ID,
		cred.TargetID,
		cred.Username,
		cred.VaultSecretPath,
		cred.Description,
		cred.CreatedAt,
		cred.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}

	return nil
}

// GetByID retrieves a credential by ID
func (r *CredentialRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Credential, error) {
	query := `
		SELECT id, target_id, username, vault_secret_path, description, created_at, updated_at
		FROM credentials
		WHERE id = $1
	`

	var cred models.Credential
	err := r.db.GetContext(ctx, &cred, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credential not found")
		}
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	return &cred, nil
}

// GetByTargetID retrieves all credentials for a target
func (r *CredentialRepository) GetByTargetID(ctx context.Context, targetID uuid.UUID) ([]*models.Credential, error) {
	query := `
		SELECT id, target_id, username, vault_secret_path, description, created_at, updated_at
		FROM credentials
		WHERE target_id = $1
		ORDER BY username ASC
	`

	var creds []*models.Credential
	err := r.db.SelectContext(ctx, &creds, query, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials by target: %w", err)
	}

	return creds, nil
}

// Update updates a credential
func (r *CredentialRepository) Update(ctx context.Context, cred *models.Credential) error {
	query := `
		UPDATE credentials
		SET username = $1, vault_secret_path = $2, description = $3, updated_at = $4
		WHERE id = $5
	`

	cred.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		cred.Username,
		cred.VaultSecretPath,
		cred.Description,
		cred.UpdatedAt,
		cred.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("credential not found")
	}

	return nil
}

// Delete deletes a credential
func (r *CredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM credentials WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("credential not found")
	}

	return nil
}
