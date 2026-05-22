package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID         uuid.UUID     `json:"id" db:"id"`
	UserID     uuid.UUID     `json:"user_id" db:"user_id"`
	Name       string        `json:"name" db:"name"`
	KeyPrefix  string        `json:"key_prefix" db:"key_prefix"`
	KeyHash    string        `json:"-" db:"key_hash"` // Hide from JSON!
	Last4      string        `json:"last_4" db:"last_4"`
	Scopes     []string      `json:"scopes" db:"scopes"`
	DomainID   uuid.NullUUID `json:"domain_id" db:"domain_id"`
	IsActive   bool          `json:"is_active" db:"is_active"`
	LastUsedAt sql.NullTime  `json:"last_used_at" db:"last_used_at"`
	ExpiresAt  sql.NullTime  `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time     `json:"created_at" db:"created_at"`

	// Easy string conversion formatting
	LastUsedAtStr string `json:"last_used_at_str,omitempty"`
	ExpiresAtStr  string `json:"expires_at_str,omitempty"`
	DomainIDStr   string `json:"domain_id_str,omitempty"`
}
