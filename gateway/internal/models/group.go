package models

import (
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	DN          string    `json:"dn" db:"dn"`
	Description string    `json:"description" db:"description"`
	Role        string    `json:"role" db:"role"`
	Source      string    `json:"source" db:"source"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
