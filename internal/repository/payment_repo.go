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
	UpdateStatusByExternalID(ctx context.Context, tx pgx.Tx, externalID string, status models.PaymentStatus, paidAt *time.Time, invoiceURL string, paymentMethod string) error
	ListAll(ctx context.Context, limit, offset int, search string, status string) ([]*models.AdminPayment, int64, error)
	GetStats(ctx context.Context) (*models.TransactionStats, error)
	GetAdminByID(ctx context.Context, id uuid.UUID) (*models.AdminPayment, error)
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

func (r *postgresPaymentRepository) UpdateStatusByExternalID(ctx context.Context, tx pgx.Tx, externalID string, status models.PaymentStatus, paidAt *time.Time, invoiceURL string, paymentMethod string) error {
	query := `
		UPDATE payments
		SET status = $1, 
		    paid_at = $2, 
		    invoice_url = COALESCE(NULLIF($3, ''), invoice_url),
		    payment_method = COALESCE(NULLIF($4, ''), payment_method)
		WHERE external_id = $5
	`
	execFn := func(ctx context.Context, q string, args ...any) error {
		if tx != nil {
			_, err := tx.Exec(ctx, q, args...)
			return err
		}
		_, err := r.db.Exec(ctx, q, args...)
		return err
	}

	if err := execFn(ctx, query, status, paidAt, invoiceURL, paymentMethod, externalID); err != nil {
		return fmt.Errorf("update payment status db: %w", err)
	}
	return nil
}

func (r *postgresPaymentRepository) ListAll(ctx context.Context, limit, offset int, search string, status string) ([]*models.AdminPayment, int64, error) {
	var countQuery = `
		SELECT COUNT(*)
		FROM payments p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN plans pl ON p.plan_id = pl.id
		WHERE 1=1
	`
	var selectQuery = `
		SELECT p.id, p.user_id, p.subscription_id, p.plan_id, p.billing_cycle, p.amount_idr, p.payment_method, p.external_id, p.status, p.invoice_number, COALESCE(p.invoice_url, ''), p.paid_at, p.created_at,
		       u.email, u.full_name, COALESCE(pl.name, '')
		FROM payments p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN plans pl ON p.plan_id = pl.id
		WHERE 1=1
	`

	var args []any
	var countArgs []any
	var placeholderIndex = 1

	if status != "" {
		countQuery += fmt.Sprintf(" AND p.status = $%d", placeholderIndex)
		selectQuery += fmt.Sprintf(" AND p.status = $%d", placeholderIndex)
		args = append(args, status)
		countArgs = append(countArgs, status)
		placeholderIndex++
	}

	if search != "" {
		countQuery += fmt.Sprintf(" AND (p.invoice_number ILIKE $%d OR u.email ILIKE $%d OR u.full_name ILIKE $%d)", placeholderIndex, placeholderIndex, placeholderIndex)
		selectQuery += fmt.Sprintf(" AND (p.invoice_number ILIKE $%d OR u.email ILIKE $%d OR u.full_name ILIKE $%d)", placeholderIndex, placeholderIndex, placeholderIndex)
		searchValue := "%" + search + "%"
		args = append(args, searchValue)
		countArgs = append(countArgs, searchValue)
		placeholderIndex++
	}

	var total int64
	err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count all payments: %w", err)
	}

	selectQuery += fmt.Sprintf(" ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d", placeholderIndex, placeholderIndex+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list all payments select: %w", err)
	}
	defer rows.Close()

	var payments []*models.AdminPayment
	for rows.Next() {
		var p models.AdminPayment
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.SubscriptionID, &p.PlanID, &p.BillingCycle, &p.AmountIDR, &p.PaymentMethod, &p.ExternalID, &p.Status,
			&p.InvoiceNumber, &p.InvoiceURL, &p.PaidAt, &p.CreatedAt,
			&p.UserEmail, &p.UserName, &p.PlanName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan admin payment row: %w", err)
		}
		payments = append(payments, &p)
	}

	return payments, total, nil
}

func (r *postgresPaymentRepository) GetStats(ctx context.Context) (*models.TransactionStats, error) {
	var stats models.TransactionStats

	err := r.db.QueryRow(ctx, "SELECT COALESCE(SUM(amount_idr), 0) FROM payments WHERE status = 'paid'").Scan(&stats.TotalVolumeIDR)
	if err != nil {
		return nil, fmt.Errorf("get stats volume: %w", err)
	}

	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM payments").Scan(&stats.TotalCount)
	if err != nil {
		return nil, fmt.Errorf("get stats count: %w", err)
	}

	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE status = 'paid'").Scan(&stats.PaidCount)
	if err != nil {
		return nil, fmt.Errorf("get stats paid count: %w", err)
	}

	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE status = 'pending'").Scan(&stats.PendingCount)
	if err != nil {
		return nil, fmt.Errorf("get stats pending count: %w", err)
	}

	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE status IN ('failed', 'expired', 'refunded')").Scan(&stats.FailedCount)
	if err != nil {
		return nil, fmt.Errorf("get stats failed count: %w", err)
	}

	return &stats, nil
}

func (r *postgresPaymentRepository) GetAdminByID(ctx context.Context, id uuid.UUID) (*models.AdminPayment, error) {
	query := `
		SELECT p.id, p.user_id, p.subscription_id, p.plan_id, p.billing_cycle, p.amount_idr, p.payment_method, p.external_id, p.status, p.invoice_number, COALESCE(p.invoice_url, ''), p.paid_at, p.created_at,
		       u.email, u.full_name, COALESCE(pl.name, '')
		FROM payments p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN plans pl ON p.plan_id = pl.id
		WHERE p.id = $1
	`
	var p models.AdminPayment
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.SubscriptionID, &p.PlanID, &p.BillingCycle, &p.AmountIDR, &p.PaymentMethod, &p.ExternalID, &p.Status,
		&p.InvoiceNumber, &p.InvoiceURL, &p.PaidAt, &p.CreatedAt,
		&p.UserEmail, &p.UserName, &p.PlanName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan admin payment by id: %w", err)
	}
	return &p, nil
}

