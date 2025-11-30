package license

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanCannon/openpam/license/pkg/logger"
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

func (s *Service) ValidateLicense(licenseKey string) (*ValidationResponse, error) {
	license, err := s.getLicense(licenseKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return &ValidationResponse{
				Valid:  false,
				Errors: []string{"Invalid license key"},
			}, nil
		}
		return nil, fmt.Errorf("failed to get license: %w", err)
	}

	errors := s.checkLicenseValidity(license)

	response := &ValidationResponse{
		Valid:    len(errors) == 0,
		License:  license,
		Features: license.Features,
		Errors:   errors,
	}

	if len(errors) == 0 {
		// Calculate remaining resources
		if err := s.calculateRemaining(license, response); err != nil {
			s.logger.Error("Failed to calculate remaining resources", map[string]interface{}{
				"error": err.Error(),
			})
		}

		// Update last checked timestamp
		if err := s.updateLastChecked(license.ID); err != nil {
			s.logger.Error("Failed to update last checked", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return response, nil
}

func (s *Service) getLicense(licenseKey string) (*License, error) {
	var license License
	var featuresJSON []byte
	var expiresAt, activatedAt, lastCheckedAt sql.NullTime
	var maxUsers, maxTargets, maxSessions sql.NullInt64

	query := `
		SELECT id, license_key, license_type, issued_to, issued_at, expires_at,
		       max_users, max_targets, max_sessions, features, is_active,
		       activated_at, last_checked_at, created_at, updated_at
		FROM license_info
		WHERE license_key = $1
	`

	err := s.db.QueryRow(query, licenseKey).Scan(
		&license.ID, &license.LicenseKey, &license.LicenseType, &license.IssuedTo,
		&license.IssuedAt, &expiresAt, &maxUsers, &maxTargets, &maxSessions,
		&featuresJSON, &license.IsActive, &activatedAt, &lastCheckedAt,
		&license.CreatedAt, &license.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		license.ExpiresAt = &expiresAt.Time
	}
	if activatedAt.Valid {
		license.ActivatedAt = &activatedAt.Time
	}
	if lastCheckedAt.Valid {
		license.LastCheckedAt = &lastCheckedAt.Time
	}
	if maxUsers.Valid {
		val := int(maxUsers.Int64)
		license.MaxUsers = &val
	}
	if maxTargets.Valid {
		val := int(maxTargets.Int64)
		license.MaxTargets = &val
	}
	if maxSessions.Valid {
		val := int(maxSessions.Int64)
		license.MaxSessions = &val
	}

	if err := json.Unmarshal(featuresJSON, &license.Features); err != nil {
		license.Features = make(map[string]interface{})
	}

	return &license, nil
}

func (s *Service) checkLicenseValidity(license *License) []string {
	var errors []string

	if !license.IsActive {
		errors = append(errors, "License is not active")
	}

	if license.ExpiresAt != nil && time.Now().After(*license.ExpiresAt) {
		errors = append(errors, "License has expired")
	}

	return errors
}

func (s *Service) calculateRemaining(license *License, response *ValidationResponse) error {
	// Get current usage
	usage, err := s.GetUsageStats()
	if err != nil {
		return err
	}

	// Calculate remaining users
	if license.MaxUsers != nil {
		remaining := *license.MaxUsers - usage.CurrentUsers
		response.RemainingUsers = &remaining
	}

	// Calculate remaining targets
	if license.MaxTargets != nil {
		remaining := *license.MaxTargets - usage.CurrentTargets
		response.RemainingTargets = &remaining
	}

	// Calculate remaining days
	if license.ExpiresAt != nil {
		days := int(time.Until(*license.ExpiresAt).Hours() / 24)
		response.RemainingDays = &days
	}

	return nil
}

func (s *Service) updateLastChecked(licenseID string) error {
	query := `UPDATE license_info SET last_checked_at = $1 WHERE id = $2`
	_, err := s.db.Exec(query, time.Now(), licenseID)
	return err
}

func (s *Service) GetUsageStats() (*UsageStats, error) {
	stats := &UsageStats{
		Timestamp: time.Now(),
	}

	// Count users
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE status = 'active'").Scan(&stats.CurrentUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Count targets
	err = s.db.QueryRow("SELECT COUNT(*) FROM targets WHERE status = 'active'").Scan(&stats.CurrentTargets)
	if err != nil {
		return nil, fmt.Errorf("failed to count targets: %w", err)
	}

	// Count active sessions
	err = s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE status = 'active'").Scan(&stats.CurrentSessions)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	return stats, nil
}

func (s *Service) CheckFeature(feature string) (*FeatureCheckResponse, error) {
	// Get the active license
	var featuresJSON []byte
	query := `SELECT features FROM license_info WHERE is_active = true ORDER BY created_at DESC LIMIT 1`

	err := s.db.QueryRow(query).Scan(&featuresJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return &FeatureCheckResponse{
				Enabled: false,
				Feature: feature,
				Message: "No active license found",
			}, nil
		}
		return nil, fmt.Errorf("failed to get license features: %w", err)
	}

	var features map[string]interface{}
	if err := json.Unmarshal(featuresJSON, &features); err != nil {
		return nil, fmt.Errorf("failed to parse features: %w", err)
	}

	enabled := false
	if val, ok := features[feature]; ok {
		if boolVal, ok := val.(bool); ok {
			enabled = boolVal
		}
	}

	response := &FeatureCheckResponse{
		Enabled: enabled,
		Feature: feature,
	}

	if !enabled {
		response.Message = "Feature not enabled in license"
	}

	return response, nil
}

func (s *Service) GetActiveLicense() (*License, error) {
	var license License
	var featuresJSON []byte
	var expiresAt, activatedAt, lastCheckedAt sql.NullTime
	var maxUsers, maxTargets, maxSessions sql.NullInt64

	query := `
		SELECT id, license_key, license_type, issued_to, issued_at, expires_at,
		       max_users, max_targets, max_sessions, features, is_active,
		       activated_at, last_checked_at, created_at, updated_at
		FROM license_info
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := s.db.QueryRow(query).Scan(
		&license.ID, &license.LicenseKey, &license.LicenseType, &license.IssuedTo,
		&license.IssuedAt, &expiresAt, &maxUsers, &maxTargets, &maxSessions,
		&featuresJSON, &license.IsActive, &activatedAt, &lastCheckedAt,
		&license.CreatedAt, &license.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		license.ExpiresAt = &expiresAt.Time
	}
	if activatedAt.Valid {
		license.ActivatedAt = &activatedAt.Time
	}
	if lastCheckedAt.Valid {
		license.LastCheckedAt = &lastCheckedAt.Time
	}
	if maxUsers.Valid {
		val := int(maxUsers.Int64)
		license.MaxUsers = &val
	}
	if maxTargets.Valid {
		val := int(maxTargets.Int64)
		license.MaxTargets = &val
	}
	if maxSessions.Valid {
		val := int(maxSessions.Int64)
		license.MaxSessions = &val
	}

	if err := json.Unmarshal(featuresJSON, &license.Features); err != nil {
		license.Features = make(map[string]interface{})
	}

	return &license, nil
}
