package models

import (
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Source      string    `json:"source" db:"source"`
	ADGUID      *string   `json:"ad_guid,omitempty" db:"ad_guid"`
	DN          *string   `json:"dn,omitempty" db:"dn"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type UserGroup struct {
	UserID  uuid.UUID `json:"user_id" db:"user_id"`
	GroupID uuid.UUID `json:"group_id" db:"group_id"`
}
