package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type DomainStatus string

const (
	DomainPending  DomainStatus = "pending"
	DomainVerified DomainStatus = "verified"
	DomainFailed   DomainStatus = "failed"
	DomainExpired  DomainStatus = "expired"
)

type Domain struct {
	ID              uuid.UUID    `json:"id" db:"id"`
	UserID          uuid.UUID    `json:"user_id" db:"user_id"`
	DomainName      string       `json:"domain_name" db:"domain_name"`
	Status          DomainStatus `json:"status" db:"status"`
	SPFRecord       string       `json:"spf_record" db:"spf_record"`
	DKIMSelector    string       `json:"dkim_selector" db:"dkim_selector"`
	DKIMPublicKey   string       `json:"dkim_public_key" db:"dkim_public_key"`
	DKIMPrivateKey  string       `json:"-" db:"dkim_private_key"` // Sensitive! Hidden from JSON
	DMARCRecord     string       `json:"dmarc_record" db:"dmarc_record"`
	ReturnPathCNAME string       `json:"return_path_cname" db:"return_path_cname"`
	SPFVerified     bool         `json:"spf_verified" db:"spf_verified"`
	DKIMVerified    bool         `json:"dkim_verified" db:"dkim_verified"`
	DMARCVerified   bool         `json:"dmarc_verified" db:"dmarc_verified"`
	LastVerifiedAt  sql.NullTime `json:"last_verified_at" db:"last_verified_at"`
	VerifiedAt      sql.NullTime `json:"verified_at" db:"verified_at"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at" db:"updated_at"`

	// Strings for easily formatted json nullable outputs
	LastVerifiedAtStr string `json:"last_verified_at_str,omitempty"`
	VerifiedAtStr     string `json:"verified_at_str,omitempty"`
}
