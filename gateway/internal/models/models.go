package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Zone represents a network zone (hub or satellite gateway)
type Zone struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Type        string    `json:"type" db:"type"` // "hub" or "satellite"
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Target represents a server/system that users can connect to
type Target struct {
	ID          uuid.UUID `json:"id" db:"id"`
	ZoneID      uuid.UUID `json:"zone_id" db:"zone_id"`
	Name        string    `json:"name" db:"name"`
	Hostname    string    `json:"hostname" db:"hostname"`
	Protocol    string    `json:"protocol" db:"protocol"` // "ssh" or "rdp"
	Port        int       `json:"port" db:"port"`
	Description string    `json:"description,omitempty" db:"description"`
	Enabled     bool      `json:"enabled" db:"enabled"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Credential maps a target to its credentials stored in Vault
type Credential struct {
	ID              uuid.UUID `json:"id" db:"id"`
	TargetID        uuid.UUID `json:"target_id" db:"target_id"`
	Username        string    `json:"username" db:"username"`
	VaultSecretPath string    `json:"vault_secret_path" db:"vault_secret_path"`
	Description     string    `json:"description,omitempty" db:"description"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// User stores user information from EntraID/AD
type User struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	EntraID     string       `json:"entra_id" db:"entra_id"`
	Email       string       `json:"email" db:"email"`
	DisplayName string       `json:"display_name,omitempty" db:"display_name"`
	Enabled     bool         `json:"enabled" db:"enabled"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
	LastLoginAt sql.NullTime `json:"last_login_at,omitempty" db:"last_login_at"`
}

// AuditLog records all connection sessions
type AuditLog struct {
	ID            uuid.UUID     `json:"id" db:"id"`
	UserID        uuid.UUID     `json:"user_id" db:"user_id"`
	TargetID      uuid.UUID     `json:"target_id" db:"target_id"`
	CredentialID  uuid.NullUUID `json:"credential_id,omitempty" db:"credential_id"`
	StartTime     time.Time     `json:"start_time" db:"start_time"`
	EndTime       sql.NullTime  `json:"end_time,omitempty" db:"end_time"`
	BytesSent     int64         `json:"bytes_sent" db:"bytes_sent"`
	BytesReceived int64         `json:"bytes_received" db:"bytes_received"`
	SessionStatus string        `json:"session_status" db:"session_status"` // "active", "completed", "failed", "terminated"
	ClientIP      *string       `json:"client_ip,omitempty" db:"client_ip"`
	ErrorMessage  *string       `json:"error_message,omitempty" db:"error_message"`
	RecordingPath *string       `json:"recording_path,omitempty" db:"recording_path"`
	Protocol      string        `json:"protocol" db:"protocol"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
}

// SessionStatus constants
const (
	SessionStatusActive     = "active"
	SessionStatusCompleted  = "completed"
	SessionStatusFailed     = "failed"
	SessionStatusTerminated = "terminated"
)

// ZoneType constants
const (
	ZoneTypeHub       = "hub"
	ZoneTypeSatellite = "satellite"
)

// Protocol constants
const (
	ProtocolSSH = "ssh"
	ProtocolRDP = "rdp"
)
