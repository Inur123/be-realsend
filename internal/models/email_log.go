package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type EmailStatus string

const (
	StatusQueued     EmailStatus = "queued"
	StatusProcessing EmailStatus = "processing"
	StatusSent       EmailStatus = "sent"
	StatusDelivered  EmailStatus = "delivered"
	StatusBounced    EmailStatus = "bounced"
	StatusRejected   EmailStatus = "rejected"
	StatusFailed     EmailStatus = "failed"
	StatusOpened     EmailStatus = "opened"
	StatusClicked    EmailStatus = "clicked"
)

type BounceType string

const (
	BounceHard BounceType = "hard"
	BounceSoft BounceType = "soft"
	BounceNone BounceType = "none"
)

type EmailLog struct {
	ID            uuid.UUID     `json:"id" db:"id"`
	UserID        uuid.UUID     `json:"user_id" db:"user_id"`
	APIKeyID      uuid.NullUUID `json:"api_key_id" db:"api_key_id"`
	DomainID      uuid.NullUUID `json:"domain_id" db:"domain_id"`
	FromAddress   string        `json:"from_address" db:"from_address"`
	ToAddress     string        `json:"to_address" db:"to_address"`
	CCAddresses   []string      `json:"cc_addresses" db:"cc_addresses"`
	BCCAddresses  []string      `json:"bcc_addresses" db:"bcc_addresses"`
	Subject       string        `json:"subject" db:"subject"`
	ContentType   string        `json:"content_type" db:"content_type"`
	Status        EmailStatus   `json:"status" db:"status"`
	BounceType    BounceType    `json:"bounce_type" db:"bounce_type"`
	BounceReason  sql.NullString `json:"bounce_reason" db:"bounce_reason"`
	SMTPMessageID sql.NullString `json:"smtp_message_id" db:"smtp_message_id"`
	SMTPResponse  sql.NullString `json:"smtp_response" db:"smtp_response"`
	OpenedAt      sql.NullTime   `json:"opened_at" db:"opened_at"`
	OpenedCount   int           `json:"opened_count" db:"opened_count"`
	ClickedAt     sql.NullTime   `json:"clicked_at" db:"clicked_at"`
	ClickedCount  int           `json:"clicked_count" db:"clicked_count"`
	Tags          []string      `json:"tags" db:"tags"`
	Metadata      []byte        `json:"metadata" db:"metadata"` // Raw JSONB
	Headers       []byte        `json:"headers" db:"headers"`   // Raw JSONB
	QueuedAt      time.Time     `json:"queued_at" db:"queued_at"`
	SentAt        sql.NullTime   `json:"sent_at" db:"sent_at"`
	DeliveredAt   sql.NullTime   `json:"delivered_at" db:"delivered_at"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`

	// Helper presentation strings
	BounceReasonStr  string `json:"bounce_reason_str,omitempty"`
	SMTPMessageIDStr string `json:"smtp_message_id_str,omitempty"`
	SMTPResponseStr  string `json:"smtp_response_str,omitempty"`
	OpenedAtStr      string `json:"opened_at_str,omitempty"`
	ClickedAtStr     string `json:"clicked_at_str,omitempty"`
	SentAtStr        string `json:"sent_at_str,omitempty"`
	DeliveredAtStr   string `json:"delivered_at_str,omitempty"`
}
