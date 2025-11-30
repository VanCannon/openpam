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
	Role        string       `json:"role" db:"role"`
	Source      string       `json:"source" db:"source"`
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

// SystemAuditLog records system events (logins, user changes, etc.)
type SystemAuditLog struct {
	ID           uuid.UUID     `json:"id" db:"id"`
	Timestamp    time.Time     `json:"timestamp" db:"timestamp"`
	EventType    string        `json:"event_type" db:"event_type"`
	UserID       uuid.NullUUID `json:"user_id,omitempty" db:"user_id"`
	TargetUserID uuid.NullUUID `json:"target_user_id,omitempty" db:"target_user_id"`
	ResourceType *string       `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID   uuid.NullUUID `json:"resource_id,omitempty" db:"resource_id"`
	ResourceName *string       `json:"resource_name,omitempty" db:"resource_name"`
	Action       string        `json:"action" db:"action"`
	Status       string        `json:"status" db:"status"`
	IPAddress    *string       `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    *string       `json:"user_agent,omitempty" db:"user_agent"`
	Details      *string       `json:"details,omitempty" db:"details"` // JSONB stored as string
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
}

// Role constants
const (
	RoleAdmin   = "admin"
	RoleUser    = "user"
	RoleAuditor = "auditor"
)

// ApprovalStatus constants
const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
)

// System Audit Event Types
const (
	EventTypeLoginSuccess      = "login_success"
	EventTypeLoginFailed       = "login_failed"
	EventTypeLogout            = "logout"
	EventTypeUserCreated       = "user_created"
	EventTypeUserUpdated       = "user_updated"
	EventTypeUserDeleted       = "user_deleted"
	EventTypeTargetCreated     = "target_created"
	EventTypeTargetUpdated     = "target_updated"
	EventTypeTargetDeleted     = "target_deleted"
	EventTypeCredentialCreated = "credential_created"
	EventTypeCredentialUpdated = "credential_updated"
	EventTypeCredentialDeleted = "credential_deleted"
	EventTypeSessionStarted    = "session_started"
	EventTypeSessionEnded      = "session_ended"
	EventTypePermissionChanged = "permission_changed"
	EventTypeZoneCreated       = "zone_created"
	EventTypeZoneUpdated       = "zone_updated"
	EventTypeZoneDeleted       = "zone_deleted"
)

// Audit Status constants
const (
	AuditStatusSuccess = "success"
	AuditStatusFailure = "failure"
	AuditStatusPending = "pending"
)
