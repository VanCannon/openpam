package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ScheduleStatus represents the status of a schedule
type ScheduleStatus string

const (
	ScheduleStatusPending   ScheduleStatus = "pending"
	ScheduleStatusActive    ScheduleStatus = "active"
	ScheduleStatusExpired   ScheduleStatus = "expired"
	ScheduleStatusCancelled ScheduleStatus = "cancelled"
)

// Schedule represents a scheduled access request
type Schedule struct {
	ID              uuid.UUID      `json:"id" db:"id"`
	UserID          uuid.UUID      `json:"user_id" db:"user_id"`
	TargetID        uuid.UUID      `json:"target_id" db:"target_id"`
	StartTime       time.Time      `json:"start_time" db:"start_time"`
	EndTime         time.Time      `json:"end_time" db:"end_time"`
	RecurrenceRule  *string        `json:"recurrence_rule,omitempty" db:"recurrence_rule"`
	Timezone        string         `json:"timezone" db:"timezone"`
	Status          ScheduleStatus `json:"status" db:"status"`
	CreatedBy       *uuid.UUID     `json:"created_by,omitempty" db:"created_by"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
	Metadata        JSONB          `json:"metadata,omitempty" db:"metadata"`
	ApprovalStatus  string         `json:"approval_status" db:"approval_status"`
	RejectionReason *string        `json:"rejection_reason,omitempty" db:"rejection_reason"`
	ApprovedBy      *uuid.UUID     `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt      *time.Time     `json:"approved_at,omitempty" db:"approved_at"`
}

// JSONB is a wrapper for JSONB fields
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &j)
}
