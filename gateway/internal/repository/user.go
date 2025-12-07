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

// UserRepository handles user data operations
type UserRepository struct {
	db *database.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, entra_id, email, display_name, enabled, role, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	if user.Role == "" {
		user.Role = models.RoleUser
	}
	if user.Source == "" {
		user.Source = "local"
	}

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.EntraID,
		user.Email,
		user.DisplayName,
		user.Enabled,
		user.Role,
		user.Source,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, entra_id, email, display_name, enabled, role, source, created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetByEntraID retrieves a user by EntraID
func (r *UserRepository) GetByEntraID(ctx context.Context, entraID string) (*models.User, error) {
	query := `
		SELECT id, entra_id, email, display_name, enabled, role, source, created_at, updated_at, last_login_at
		FROM users
		WHERE entra_id = $1
	`

	var user models.User
	err := r.db.GetContext(ctx, &user, query, entraID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, entra_id, email, display_name, enabled, role, source, created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET email = $1, display_name = $2, enabled = $3, role = $4, source = $5, updated_at = $6
		WHERE id = $7
	`

	user.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.DisplayName,
		user.Enabled,
		user.Role,
		user.Source,
		user.UpdatedAt,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET last_login_at = $1
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// Delete deletes a user permanently
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Start a transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete associated audit logs first (ON DELETE RESTRICT)
	_, err = tx.ExecContext(ctx, "DELETE FROM audit_logs WHERE user_id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user audit logs: %w", err)
	}

	// Delete the user
	query := `DELETE FROM users WHERE id = $1`
	result, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// List retrieves all users with pagination
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	query := `
		SELECT id, entra_id, email, display_name, enabled, role, source, created_at, updated_at, last_login_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	var users []*models.User
	err := r.db.SelectContext(ctx, &users, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// GetOrCreate retrieves a user by EntraID or creates a new one
func (r *UserRepository) GetOrCreate(ctx context.Context, entraID, email, displayName string) (*models.User, error) {
	// Try to get existing user
	user, err := r.GetByEntraID(ctx, entraID)
	if err == nil {
		// User exists, update last login
		if err := r.UpdateLastLogin(ctx, user.ID); err != nil {
			return nil, fmt.Errorf("failed to update last login: %w", err)
		}
		return user, nil
	}

	// User doesn't exist, create new one
	user = &models.User{
		EntraID:     entraID,
		Email:       email,
		DisplayName: displayName,
		Enabled:     true,
	}

	if err := r.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Update last login
	user.LastLoginAt = sql.NullTime{Time: time.Now(), Valid: true}
	if err := r.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to update last login: %w", err)
	}

	return user, nil
}
