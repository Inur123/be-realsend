package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/realsend/be-realsend/internal/models"
)

type SubscriptionRepository interface {
	Create(ctx context.Context, tx pgx.Tx, sub *models.Subscription) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Subscription, error)
	UpdateUsage(ctx context.Context, userID uuid.UUID, dailyInc, monthlyInc int) error
	ResetDailyUsage(ctx context.Context) error
	ResetDueUsage(ctx context.Context) error
	ExpireEndedPaidSubscriptions(ctx context.Context, freePlanID uuid.UUID) (int64, error)
	GetOverrides(ctx context.Context, userID uuid.UUID) ([]*models.UserPlanOverride, error)
	SetOverride(ctx context.Context, override *models.UserPlanOverride) error
	DeleteOverride(ctx context.Context, userID uuid.UUID, featureKey string) error
	UpdatePlanByUserID(ctx context.Context, tx pgx.Tx, userID uuid.UUID, planID uuid.UUID, billingCycle string, amountIDR int, paymentMethod string, expiresAt *time.Time) error
}

type postgresSubscriptionRepository struct {
	db *pgxpool.Pool
}

func NewSubscriptionRepository(db *pgxpool.Pool) SubscriptionRepository {
	return &postgresSubscriptionRepository{db: db}
}

func (r *postgresSubscriptionRepository) Create(ctx context.Context, tx pgx.Tx, sub *models.Subscription) error {
	query := `
		INSERT INTO subscriptions (id, user_id, plan_id, status, started_at, expires_at, cancelled_at, billing_cycle, amount_idr, payment_method, emails_sent_this_month, emails_sent_today, month_reset_at, day_reset_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	execFn := func(ctx context.Context, q string, args ...any) error {
		if tx != nil {
			_, err := tx.Exec(ctx, q, args...)
			return err
		}
		_, err := r.db.Exec(ctx, q, args...)
		return err
	}

	err := execFn(ctx, query,
		sub.ID,
		sub.UserID,
		sub.PlanID,
		sub.Status,
		sub.StartedAt,
		sub.ExpiresAt,
		sub.CancelledAt,
		sub.BillingCycle,
		sub.AmountIDR,
		sub.PaymentMethod,
		sub.EmailsSentThisMonth,
		sub.EmailsSentToday,
		sub.MonthResetAt,
		sub.DayResetAt,
		sub.CreatedAt,
		sub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create subscription db: %w", err)
	}

	return nil
}

func (r *postgresSubscriptionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Subscription, error) {
	query := `
		SELECT id, user_id, plan_id, status, started_at, expires_at, cancelled_at, billing_cycle, amount_idr, payment_method,
		       emails_sent_this_month, emails_sent_today, month_reset_at, day_reset_at, created_at, updated_at
		FROM subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var s models.Subscription
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&s.ID, &s.UserID, &s.PlanID, &s.Status, &s.StartedAt, &s.ExpiresAt, &s.CancelledAt, &s.BillingCycle, &s.AmountIDR, &s.PaymentMethod,
		&s.EmailsSentThisMonth, &s.EmailsSentToday, &s.MonthResetAt, &s.DayResetAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan subscription by user_id: %w", err)
	}

	return &s, nil
}

func (r *postgresSubscriptionRepository) UpdateUsage(ctx context.Context, userID uuid.UUID, dailyInc, monthlyInc int) error {
	query := `
		UPDATE subscriptions
		SET emails_sent_today = emails_sent_today + $1,
		    emails_sent_this_month = emails_sent_this_month + $2,
		    updated_at = NOW()
		WHERE user_id = $3 AND status = 'active'
	`
	_, err := r.db.Exec(ctx, query, dailyInc, monthlyInc, userID)
	if err != nil {
		return fmt.Errorf("update subscription usage db: %w", err)
	}
	return nil
}

func (r *postgresSubscriptionRepository) ResetDailyUsage(ctx context.Context) error {
	query := `
		UPDATE subscriptions
		SET emails_sent_today = 0,
		    day_reset_at = NOW() + INTERVAL '1 day',
		    updated_at = NOW()
		WHERE status = 'active'
	`
	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("reset daily subscription usage db: %w", err)
	}
	return nil
}

func (r *postgresSubscriptionRepository) ResetDueUsage(ctx context.Context) error {
	dailyQuery := `
		UPDATE subscriptions
		SET emails_sent_today = 0,
		    day_reset_at = NOW() + INTERVAL '1 day',
		    updated_at = NOW()
		WHERE status = 'active' AND day_reset_at <= NOW()
	`
	if _, err := r.db.Exec(ctx, dailyQuery); err != nil {
		return fmt.Errorf("reset due daily subscription usage db: %w", err)
	}

	monthlyQuery := `
		UPDATE subscriptions
		SET emails_sent_this_month = 0,
		    emails_sent_today = 0,
		    month_reset_at = NOW() + INTERVAL '1 month',
		    day_reset_at = NOW() + INTERVAL '1 day',
		    updated_at = NOW()
		WHERE status = 'active' AND month_reset_at <= NOW()
	`
	if _, err := r.db.Exec(ctx, monthlyQuery); err != nil {
		return fmt.Errorf("reset due monthly subscription usage db: %w", err)
	}

	return nil
}

func (r *postgresSubscriptionRepository) ExpireEndedPaidSubscriptions(ctx context.Context, freePlanID uuid.UUID) (int64, error) {
	query := `
		WITH expired AS (
			UPDATE subscriptions
			SET status = 'expired',
			    updated_at = NOW()
			WHERE status = 'active'
			  AND expires_at IS NOT NULL
			  AND expires_at <= NOW()
			RETURNING user_id
		),
		inserted AS (
			INSERT INTO subscriptions (
				id, user_id, plan_id, status, started_at, expires_at, cancelled_at,
				billing_cycle, amount_idr, payment_method, emails_sent_this_month,
				emails_sent_today, month_reset_at, day_reset_at, created_at, updated_at
			)
			SELECT
				uuid_generate_v4(), user_id, $1, 'active', NOW(), NULL, NULL,
				'monthly', 0, 'free', 0, 0, NOW() + INTERVAL '1 month',
				NOW() + INTERVAL '1 day', NOW(), NOW()
			FROM expired
			RETURNING user_id
		)
		UPDATE users
		SET current_plan_id = $1,
		    updated_at = NOW()
		WHERE id IN (SELECT user_id FROM inserted)
	`
	res, err := r.db.Exec(ctx, query, freePlanID)
	if err != nil {
		return 0, fmt.Errorf("expire ended paid subscriptions db: %w", err)
	}
	return res.RowsAffected(), nil
}

func (r *postgresSubscriptionRepository) GetOverrides(ctx context.Context, userID uuid.UUID) ([]*models.UserPlanOverride, error) {
	query := `
		SELECT id, user_id, feature_key, override_value, COALESCE(note, ''), created_by, expires_at, created_at
		FROM user_plan_overrides
		WHERE user_id = $1 AND (expires_at IS NULL OR expires_at > NOW())
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query overrides: %w", err)
	}
	defer rows.Close()

	var overrides []*models.UserPlanOverride
	for rows.Next() {
		var o models.UserPlanOverride
		err := rows.Scan(&o.ID, &o.UserID, &o.FeatureKey, &o.OverrideValue, &o.Note, &o.CreatedBy, &o.ExpiresAt, &o.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan override: %w", err)
		}
		overrides = append(overrides, &o)
	}
	return overrides, nil
}

func (r *postgresSubscriptionRepository) SetOverride(ctx context.Context, o *models.UserPlanOverride) error {
	query := `
		INSERT INTO user_plan_overrides (id, user_id, feature_key, override_value, note, created_by, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, feature_key) DO UPDATE SET
			override_value = EXCLUDED.override_value,
			note = EXCLUDED.note,
			created_by = EXCLUDED.created_by,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at
	`
	_, err := r.db.Exec(ctx, query, o.ID, o.UserID, o.FeatureKey, o.OverrideValue, o.Note, o.CreatedBy, o.ExpiresAt, o.CreatedAt)
	return err
}

func (r *postgresSubscriptionRepository) DeleteOverride(ctx context.Context, userID uuid.UUID, featureKey string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM user_plan_overrides WHERE user_id = $1 AND feature_key = $2", userID, featureKey)
	return err
}

func (r *postgresSubscriptionRepository) UpdatePlanByUserID(ctx context.Context, tx pgx.Tx, userID uuid.UUID, planID uuid.UUID, billingCycle string, amountIDR int, paymentMethod string, expiresAt *time.Time) error {
	query := `
		UPDATE subscriptions
		SET plan_id = $1,
		    billing_cycle = $2,
		    amount_idr = $3,
		    payment_method = $4,
		    expires_at = $5,
		    status = 'active',
		    started_at = NOW(),
		    emails_sent_this_month = 0,
		    emails_sent_today = 0,
		    month_reset_at = NOW() + INTERVAL '1 month',
		    day_reset_at = NOW() + INTERVAL '1 day',
		    updated_at = NOW()
		WHERE id = (
		    SELECT id
		    FROM subscriptions
		    WHERE user_id = $6
		    ORDER BY created_at DESC
		    LIMIT 1
		)
	`
	execFn := func(ctx context.Context, q string, args ...any) error {
		if tx != nil {
			_, err := tx.Exec(ctx, q, args...)
			return err
		}
		_, err := r.db.Exec(ctx, q, args...)
		return err
	}

	if err := execFn(ctx, query, planID, billingCycle, amountIDR, paymentMethod, expiresAt, userID); err != nil {
		return fmt.Errorf("update subscription plan db: %w", err)
	}
	return nil
}
