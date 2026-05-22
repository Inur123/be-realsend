package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Webhook represents a user's webhook configuration.
type Webhook struct {
	ID            uuid.UUID      `json:"id" db:"id"`
	UserID        uuid.UUID      `json:"user_id" db:"user_id"`
	URL           string         `json:"url" db:"url"`
	Secret        string         `json:"secret,omitempty" db:"secret"`
	Events        []string       `json:"events" db:"events"`
	IsActive      bool           `json:"is_active" db:"is_active"`
	LastTriggered sql.NullTime   `json:"last_triggered" db:"last_triggered"`
	FailureCount  int            `json:"failure_count" db:"failure_count"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" db:"updated_at"`

	// Helper presentation strings
	LastTriggeredStr string `json:"last_triggered_str,omitempty"`
}

// WebhookLog represents a single webhook delivery attempt log.
type WebhookLog struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	WebhookID      uuid.UUID     `json:"webhook_id" db:"webhook_id"`
	EmailLogID     uuid.NullUUID `json:"email_log_id" db:"email_log_id"`
	EventType      string        `json:"event_type" db:"event_type"`
	Payload        []byte        `json:"payload" db:"payload"`
	ResponseStatus sql.NullInt32 `json:"response_status" db:"response_status"`
	ResponseBody   sql.NullString `json:"response_body" db:"response_body"`
	Attempts       int           `json:"attempts" db:"attempts"`
	Success        bool          `json:"success" db:"success"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
}

// Webhook event type constants.
const (
	EventEmailSent      = "email.sent"
	EventEmailDelivered = "email.delivered"
	EventEmailBounced   = "email.bounced"
	EventEmailOpened    = "email.opened"
	EventEmailClicked   = "email.clicked"
	EventEmailFailed    = "email.failed"
)
