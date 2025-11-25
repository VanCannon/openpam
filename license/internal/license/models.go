package license

import (
	"time"
)

type License struct {
	ID               string                 `json:"id"`
	LicenseKey       string                 `json:"license_key"`
	LicenseType      string                 `json:"license_type"`
	IssuedTo         string                 `json:"issued_to"`
	IssuedAt         time.Time              `json:"issued_at"`
	ExpiresAt        *time.Time             `json:"expires_at,omitempty"`
	MaxUsers         *int                   `json:"max_users,omitempty"`
	MaxTargets       *int                   `json:"max_targets,omitempty"`
	MaxSessions      *int                   `json:"max_sessions,omitempty"`
	Features         map[string]interface{} `json:"features"`
	IsActive         bool                   `json:"is_active"`
	ActivatedAt      *time.Time             `json:"activated_at,omitempty"`
	LastCheckedAt    *time.Time             `json:"last_checked_at,omitempty"`
	ValidationErrors []string               `json:"validation_errors,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type ValidationRequest struct {
	LicenseKey string `json:"license_key"`
}

type ValidationResponse struct {
	Valid            bool                   `json:"valid"`
	License          *License               `json:"license,omitempty"`
	Features         map[string]interface{} `json:"features"`
	Errors           []string               `json:"errors,omitempty"`
	RemainingUsers   *int                   `json:"remaining_users,omitempty"`
	RemainingTargets *int                   `json:"remaining_targets,omitempty"`
	RemainingDays    *int                   `json:"remaining_days,omitempty"`
}

type UsageStats struct {
	CurrentUsers    int       `json:"current_users"`
	CurrentTargets  int       `json:"current_targets"`
	CurrentSessions int       `json:"current_sessions"`
	Timestamp       time.Time `json:"timestamp"`
}

type FeatureCheckRequest struct {
	Feature string `json:"feature"`
}

type FeatureCheckResponse struct {
	Enabled bool   `json:"enabled"`
	Feature string `json:"feature"`
	Message string `json:"message,omitempty"`
}
