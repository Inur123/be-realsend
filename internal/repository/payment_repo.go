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

type PaymentRepository interface {
	Create(ctx context.Context, tx pgx.Tx, payment *models.Payment) error
	GetByExternalID(ctx context.Context, externalID string) (*models.Payment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Payment, int64, error)
	UpdateStatusByExternalID(ctx context.Context, tx pgx.Tx, externalID string, status models.PaymentStatus, paidAt *time.Time, invoiceURL string) error
}

type postgresPaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) PaymentRepository {
	return &postgresPaymentRepository{db: db}
}

func (r *postgresPaymentRepository) Create(ctx context.Context, tx pgx.Tx, payment *models.Payment) error {
	query := `
		INSERT INTO payments (id, user_id, subscription_id, plan_id, billing_cycle, amount_idr, payment_method, external_id, status, invoice_number, invoice_url, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	execFn := func(ctx context.Context, q string, args ...any) error {
		if tx != nil {
			_, err := tx.Exec(ctx, q, args...)
			return err
		}
		_, err := r.db.Exec(ctx, q, args...)
		return err
	}

	if err := execFn(ctx, query,
		payment.ID,
		payment.UserID,
		payment.SubscriptionID,
		payment.PlanID,
		payment.BillingCycle,
		payment.AmountIDR,
		payment.PaymentMethod,
		payment.ExternalID,
		payment.Status,
		payment.InvoiceNumber,
		payment.InvoiceURL,
		payment.PaidAt,
		payment.CreatedAt,
	); err != nil {
		return fmt.Errorf("create payment db: %w", err)
	}
	return nil
}

func (r *postgresPaymentRepository) GetByExternalID(ctx context.Context, externalID string) (*models.Payment, error) {
	query := `
		SELECT id, user_id, subscription_id, plan_id, billing_cycle, amount_idr, payment_method, external_id, status, invoice_number, COALESCE(invoice_url, ''), paid_at, created_at
		FROM payments
		WHERE external_id = $1
	`
	var p models.Payment
	err := r.db.QueryRow(ctx, query, externalID).Scan(
		&p.ID, &p.UserID, &p.SubscriptionID, &p.PlanID, &p.BillingCycle, &p.AmountIDR, &p.PaymentMethod, &p.ExternalID, &p.Status,
		&p.InvoiceNumber, &p.InvoiceURL, &p.PaidAt, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan payment by external id: %w", err)
	}
	return &p, nil
}

func (r *postgresPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	query := `
		SELECT id, user_id, subscription_id, plan_id, billing_cycle, amount_idr, payment_method, external_id, status, invoice_number, COALESCE(invoice_url, ''), paid_at, created_at
		FROM payments
		WHERE id = $1
	`
	var p models.Payment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.SubscriptionID, &p.PlanID, &p.BillingCycle, &p.AmountIDR, &p.PaymentMethod, &p.ExternalID, &p.Status,
		&p.InvoiceNumber, &p.InvoiceURL, &p.PaidAt, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan payment by id: %w", err)
	}
	return &p, nil
}

func (r *postgresPaymentRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Payment, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE user_id = $1", userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count payments: %w", err)
	}

	query := `
		SELECT id, user_id, subscription_id, plan_id, billing_cycle, amount_idr, payment_method, external_id, status, invoice_number, COALESCE(invoice_url, ''), paid_at, created_at
		FROM payments
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.SubscriptionID, &p.PlanID, &p.BillingCycle, &p.AmountIDR, &p.PaymentMethod, &p.ExternalID, &p.Status,
			&p.InvoiceNumber, &p.InvoiceURL, &p.PaidAt, &p.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan payment row: %w", err)
		}
		payments = append(payments, &p)
	}

	return payments, total, nil
}

func (r *postgresPaymentRepository) UpdateStatusByExternalID(ctx context.Context, tx pgx.Tx, externalID string, status models.PaymentStatus, paidAt *time.Time, invoiceURL string) error {
	query := `
		UPDATE payments
		SET status = $1, paid_at = $2, invoice_url = COALESCE(NULLIF($3, ''), invoice_url)
		WHERE external_id = $4
	`
	execFn := func(ctx context.Context, q string, args ...any) error {
		if tx != nil {
			_, err := tx.Exec(ctx, q, args...)
			return err
		}
		_, err := r.db.Exec(ctx, q, args...)
		return err
	}

	if err := execFn(ctx, query, status, paidAt, invoiceURL, externalID); err != nil {
		return fmt.Errorf("update payment status db: %w", err)
	}
	return nil
}
