package schedule

import (
	"time"
)

type Schedule struct {
	ID             string                 `json:"id"`
	UserID         string                 `json:"user_id"`
	TargetID       string                 `json:"target_id"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	RecurrenceRule *string                `json:"recurrence_rule,omitempty"`
	Timezone       string                 `json:"timezone"`
	Status         string                 `json:"status"`
	CreatedBy      *string                `json:"created_by,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type CreateScheduleRequest struct {
	UserID         string                 `json:"user_id"`
	TargetID       string                 `json:"target_id"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	RecurrenceRule *string                `json:"recurrence_rule,omitempty"`
	Timezone       string                 `json:"timezone"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type UpdateScheduleRequest struct {
	StartTime      *time.Time             `json:"start_time,omitempty"`
	EndTime        *time.Time             `json:"end_time,omitempty"`
	RecurrenceRule *string                `json:"recurrence_rule,omitempty"`
	Status         *string                `json:"status,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ActiveSchedulesRequest struct {
	UserID   *string    `json:"user_id,omitempty"`
	TargetID *string    `json:"target_id,omitempty"`
	Time     *time.Time `json:"time,omitempty"`
}

type ListSchedulesRequest struct {
	UserID   *string `json:"user_id,omitempty"`
	TargetID *string `json:"target_id,omitempty"`
	Status   *string `json:"status,omitempty"`
	Limit    int     `json:"limit"`
	Offset   int     `json:"offset"`
}

type ScheduleCheckRequest struct {
	UserID   string `json:"user_id"`
	TargetID string `json:"target_id"`
}

type ScheduleCheckResponse struct {
	Allowed   bool       `json:"allowed"`
	Schedule  *Schedule  `json:"schedule,omitempty"`
	Message   string     `json:"message,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
