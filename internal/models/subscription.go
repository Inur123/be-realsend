package models

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionStatus string

const (
	SubscriptionActive   SubscriptionStatus = "active"
	SubscriptionExpired  SubscriptionStatus = "expired"
	SubscriptionCancelled SubscriptionStatus = "cancelled"
	SubscriptionPastDue   SubscriptionStatus = "past_due"
)

type Subscription struct {
	ID                  uuid.UUID          `json:"id" db:"id"`
	UserID              uuid.UUID          `json:"user_id" db:"user_id"`
	PlanID              uuid.UUID          `json:"plan_id" db:"plan_id"`
	Status              SubscriptionStatus `json:"status" db:"status"`
	StartedAt           time.Time          `json:"started_at" db:"started_at"`
	ExpiresAt           *time.Time         `json:"expires_at" db:"expires_at"`
	CancelledAt         *time.Time         `json:"cancelled_at" db:"cancelled_at"`
	BillingCycle        string             `json:"billing_cycle" db:"billing_cycle"`
	AmountIDR           int                `json:"amount_idr" db:"amount_idr"`
	PaymentMethod       string             `json:"payment_method" db:"payment_method"`
	EmailsSentThisMonth int                `json:"emails_sent_this_month" db:"emails_sent_this_month"`
	EmailsSentToday     int                `json:"emails_sent_today" db:"emails_sent_today"`
	MonthResetAt        time.Time          `json:"month_reset_at" db:"month_reset_at"`
	DayResetAt          time.Time          `json:"day_reset_at" db:"day_reset_at"`
	CreatedAt           time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at" db:"updated_at"`
}

type UserPlanOverride struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	FeatureKey    string     `json:"feature_key" db:"feature_key"`
	OverrideValue string     `json:"override_value" db:"override_value"`
	Note          string     `json:"note" db:"note"`
	CreatedBy     *uuid.UUID `json:"created_by" db:"created_by"`
	ExpiresAt     *time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}
