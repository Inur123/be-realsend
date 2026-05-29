package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentPaid     PaymentStatus = "paid"
	PaymentFailed   PaymentStatus = "failed"
	PaymentRefunded PaymentStatus = "refunded"
	PaymentExpired  PaymentStatus = "expired"
)

type Payment struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	UserID         uuid.UUID     `json:"user_id" db:"user_id"`
	SubscriptionID *uuid.UUID    `json:"subscription_id,omitempty" db:"subscription_id"`
	PlanID         *uuid.UUID    `json:"plan_id,omitempty" db:"plan_id"`
	BillingCycle   string        `json:"billing_cycle" db:"billing_cycle"`
	AmountIDR      int           `json:"amount_idr" db:"amount_idr"`
	PaymentMethod  string        `json:"payment_method" db:"payment_method"`
	ExternalID     string        `json:"external_id" db:"external_id"`
	Status         PaymentStatus `json:"status" db:"status"`
	InvoiceNumber  string        `json:"invoice_number" db:"invoice_number"`
	InvoiceURL     string        `json:"invoice_url,omitempty" db:"invoice_url"`
	PaidAt         *time.Time    `json:"paid_at,omitempty" db:"paid_at"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
}

type AdminPayment struct {
	Payment
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
	PlanName  string `json:"plan_name"`
}

type TransactionStats struct {
	TotalVolumeIDR int64 `json:"total_volume_idr"`
	TotalCount     int64 `json:"total_count"`
	PaidCount      int64 `json:"paid_count"`
	PendingCount   int64 `json:"pending_count"`
	FailedCount    int64 `json:"failed_count"`
}

