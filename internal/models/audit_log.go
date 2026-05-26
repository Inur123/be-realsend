package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog represents a log of admin actions.
type AuditLog struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	ActorID    uuid.UUID  `json:"actor_id" db:"actor_id"`
	Action     string     `json:"action" db:"action"`
	TargetType string     `json:"target_type" db:"target_type"`
	TargetID   *uuid.UUID `json:"target_id" db:"target_id"`
	Details    []byte     `json:"details" db:"details"` // JSONB details
	IPAddress  string     `json:"ip_address" db:"ip_address"`
	UserAgent  string     `json:"user_agent,omitempty" db:"user_agent"`
	Location   string     `json:"location,omitempty" db:"location"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`

	// Presentational helper fields
	ActorEmail string `json:"actor_email,omitempty"`
}
