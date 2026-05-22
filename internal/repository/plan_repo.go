package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/realsend/be-realsend/internal/models"
)

type PlanRepository interface {
	GetAll(ctx context.Context) ([]*models.Plan, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Plan, error)
	GetBySlug(ctx context.Context, slug string) (*models.Plan, error)
	GetFeatures(ctx context.Context, planID uuid.UUID) ([]string, error)
	Create(ctx context.Context, p *models.Plan) error
	Update(ctx context.Context, p *models.Plan) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateFeatures(ctx context.Context, planID uuid.UUID, features []string) error
}

type postgresPlanRepository struct {
	db *pgxpool.Pool
}

func NewPlanRepository(db *pgxpool.Pool) PlanRepository {
	return &postgresPlanRepository{db: db}
}

func (r *postgresPlanRepository) GetAll(ctx context.Context) ([]*models.Plan, error) {
	query := `
		SELECT id, name, slug, COALESCE(description, ''), monthly_email_limit, daily_email_limit, rate_per_minute,
		       max_domains, max_api_keys, max_webhooks, log_retention_days, price_monthly_idr,
		       price_yearly_idr, overage_per_1k_idr, is_public, is_active, sort_order, COALESCE(badge_text, ''), COALESCE(badge_color, ''), created_at, updated_at
		FROM plans
		WHERE is_active = true AND is_public = true
		ORDER BY sort_order ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all plans: %w", err)
	}
	defer rows.Close()

	var plans []*models.Plan
	for rows.Next() {
		var p models.Plan
		err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description, &p.MonthlyEmailLimit, &p.DailyEmailLimit, &p.RatePerMinute,
			&p.MaxDomains, &p.MaxAPIKeys, &p.MaxWebhooks, &p.LogRetentionDays, &p.PriceMonthlyIDR,
			&p.PriceYearlyIDR, &p.OveragePer1kIDR, &p.IsPublic, &p.IsActive, &p.SortOrder, &p.BadgeText, &p.BadgeColor,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		plans = append(plans, &p)
	}

	// Load features for each plan
	for _, p := range plans {
		features, err := r.GetFeatures(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("get plan features for %s: %w", p.Slug, err)
		}
		p.Features = features
	}

	return plans, nil
}

func (r *postgresPlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Plan, error) {
	query := `
		SELECT id, name, slug, COALESCE(description, ''), monthly_email_limit, daily_email_limit, rate_per_minute,
		       max_domains, max_api_keys, max_webhooks, log_retention_days, price_monthly_idr,
		       price_yearly_idr, overage_per_1k_idr, is_public, is_active, sort_order, COALESCE(badge_text, ''), COALESCE(badge_color, ''), created_at, updated_at
		FROM plans
		WHERE id = $1
	`
	var p models.Plan
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.MonthlyEmailLimit, &p.DailyEmailLimit, &p.RatePerMinute,
		&p.MaxDomains, &p.MaxAPIKeys, &p.MaxWebhooks, &p.LogRetentionDays, &p.PriceMonthlyIDR,
		&p.PriceYearlyIDR, &p.OveragePer1kIDR, &p.IsPublic, &p.IsActive, &p.SortOrder, &p.BadgeText, &p.BadgeColor,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan plan by id: %w", err)
	}

	features, err := r.GetFeatures(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("get features: %w", err)
	}
	p.Features = features

	return &p, nil
}

func (r *postgresPlanRepository) GetBySlug(ctx context.Context, slug string) (*models.Plan, error) {
	query := `
		SELECT id, name, slug, COALESCE(description, ''), monthly_email_limit, daily_email_limit, rate_per_minute,
		       max_domains, max_api_keys, max_webhooks, log_retention_days, price_monthly_idr,
		       price_yearly_idr, overage_per_1k_idr, is_public, is_active, sort_order, COALESCE(badge_text, ''), COALESCE(badge_color, ''), created_at, updated_at
		FROM plans
		WHERE slug = $1
	`
	var p models.Plan
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.MonthlyEmailLimit, &p.DailyEmailLimit, &p.RatePerMinute,
		&p.MaxDomains, &p.MaxAPIKeys, &p.MaxWebhooks, &p.LogRetentionDays, &p.PriceMonthlyIDR,
		&p.PriceYearlyIDR, &p.OveragePer1kIDR, &p.IsPublic, &p.IsActive, &p.SortOrder, &p.BadgeText, &p.BadgeColor,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan plan by slug: %w", err)
	}

	features, err := r.GetFeatures(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("get features: %w", err)
	}
	p.Features = features

	return &p, nil
}

func (r *postgresPlanRepository) GetFeatures(ctx context.Context, planID uuid.UUID) ([]string, error) {
	query := `
		SELECT feature_key
		FROM plan_features
		WHERE plan_id = $1 AND enabled = true
	`
	rows, err := r.db.Query(ctx, query, planID)
	if err != nil {
		return nil, fmt.Errorf("query plan features: %w", err)
	}
	defer rows.Close()

	var features []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, fmt.Errorf("scan feature: %w", err)
		}
		features = append(features, f)
	}

	return features, nil
}

func (r *postgresPlanRepository) Create(ctx context.Context, p *models.Plan) error {
	query := `
		INSERT INTO plans (
			id, name, slug, description, monthly_email_limit, daily_email_limit, rate_per_minute,
			max_domains, max_api_keys, max_webhooks, log_retention_days, price_monthly_idr,
			price_yearly_idr, overage_per_1k_idr, is_public, is_active, sort_order, badge_text, badge_color, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)
	`
	_, err := r.db.Exec(ctx, query,
		p.ID, p.Name, p.Slug, p.Description, p.MonthlyEmailLimit, p.DailyEmailLimit, p.RatePerMinute,
		p.MaxDomains, p.MaxAPIKeys, p.MaxWebhooks, p.LogRetentionDays, p.PriceMonthlyIDR,
		p.PriceYearlyIDR, p.OveragePer1kIDR, p.IsPublic, p.IsActive, p.SortOrder, p.BadgeText, p.BadgeColor,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return r.UpdateFeatures(ctx, p.ID, p.Features)
}

func (r *postgresPlanRepository) Update(ctx context.Context, p *models.Plan) error {
	query := `
		UPDATE plans SET
			name = $1, slug = $2, description = $3, monthly_email_limit = $4, daily_email_limit = $5, rate_per_minute = $6,
			max_domains = $7, max_api_keys = $8, max_webhooks = $9, log_retention_days = $10, price_monthly_idr = $11,
			price_yearly_idr = $12, overage_per_1k_idr = $13, is_public = $14, is_active = $15, sort_order = $16,
			badge_text = $17, badge_color = $18, updated_at = $19
		WHERE id = $20
	`
	_, err := r.db.Exec(ctx, query,
		p.Name, p.Slug, p.Description, p.MonthlyEmailLimit, p.DailyEmailLimit, p.RatePerMinute,
		p.MaxDomains, p.MaxAPIKeys, p.MaxWebhooks, p.LogRetentionDays, p.PriceMonthlyIDR,
		p.PriceYearlyIDR, p.OveragePer1kIDR, p.IsPublic, p.IsActive, p.SortOrder, p.BadgeText, p.BadgeColor,
		p.UpdatedAt, p.ID,
	)
	if err != nil {
		return err
	}

	return r.UpdateFeatures(ctx, p.ID, p.Features)
}

func (r *postgresPlanRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM plans WHERE id = $1", id)
	return err
}

func (r *postgresPlanRepository) UpdateFeatures(ctx context.Context, planID uuid.UUID, features []string) error {
	// First, disable all existing features for this plan
	_, err := r.db.Exec(ctx, "UPDATE plan_features SET enabled = false WHERE plan_id = $1", planID)
	if err != nil {
		return fmt.Errorf("disable current features: %w", err)
	}

	// Insert or update features to be enabled
	for _, f := range features {
		query := `
			INSERT INTO plan_features (id, plan_id, feature_key, enabled)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (plan_id, feature_key) DO UPDATE SET enabled = true
		`
		_, err := r.db.Exec(ctx, query, uuid.New(), planID, f)
		if err != nil {
			return fmt.Errorf("upsert feature %s: %w", f, err)
		}
	}

	return nil
}

