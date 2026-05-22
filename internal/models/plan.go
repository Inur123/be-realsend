package models

import (
	"time"

	"github.com/google/uuid"
)

type Plan struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	Name               string    `json:"name" db:"name"`
	Slug               string    `json:"slug" db:"slug"`
	Description        string    `json:"description" db:"description"`
	MonthlyEmailLimit  int       `json:"monthly_email_limit" db:"monthly_email_limit"`
	DailyEmailLimit    int       `json:"daily_email_limit" db:"daily_email_limit"`
	RatePerMinute      int       `json:"rate_per_minute" db:"rate_per_minute"`
	MaxDomains         int       `json:"max_domains" db:"max_domains"`
	MaxAPIKeys         int       `json:"max_api_keys" db:"max_api_keys"`
	MaxWebhooks        int       `json:"max_webhooks" db:"max_webhooks"`
	LogRetentionDays   int       `json:"log_retention_days" db:"log_retention_days"`
	PriceMonthlyIDR    int       `json:"price_monthly_idr" db:"price_monthly_idr"`
	PriceYearlyIDR     int       `json:"price_yearly_idr" db:"price_yearly_idr"`
	OveragePer1kIDR    int       `json:"overage_per_1k_idr" db:"overage_per_1k_idr"`
	IsPublic           bool      `json:"is_public" db:"is_public"`
	IsActive           bool      `json:"is_active" db:"is_active"`
	SortOrder          int       `json:"sort_order" db:"sort_order"`
	BadgeText          string    `json:"badge_text" db:"badge_text"`
	BadgeColor         string    `json:"badge_color" db:"badge_color"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`

	// Loaded relation features
	Features []string `json:"features,omitempty"`
}

type PlanFeature struct {
	ID         uuid.UUID `json:"id" db:"id"`
	PlanID     uuid.UUID `json:"plan_id" db:"plan_id"`
	FeatureKey string    `json:"feature_key" db:"feature_key"`
	Enabled    bool      `json:"enabled" db:"enabled"`
}
