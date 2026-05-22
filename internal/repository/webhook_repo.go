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

// WebhookRepository handles webhook and webhook_logs tables.
type WebhookRepository interface {
	Create(ctx context.Context, w *models.Webhook) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Webhook, error)
	ListByUserAndEvent(ctx context.Context, userID uuid.UUID, eventType string) ([]*models.Webhook, error)
	Update(ctx context.Context, w *models.Webhook) error
	Delete(ctx context.Context, id uuid.UUID) error
	IncrementFailureCount(ctx context.Context, id uuid.UUID) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	UpdateLastTriggered(ctx context.Context, id uuid.UUID) error
	// Logs
	CreateLog(ctx context.Context, log *models.WebhookLog) error
	ListLogsByWebhookID(ctx context.Context, webhookID uuid.UUID, limit int) ([]*models.WebhookLog, error)
}

type postgresWebhookRepository struct {
	db *pgxpool.Pool
}

func NewWebhookRepository(db *pgxpool.Pool) WebhookRepository {
	return &postgresWebhookRepository{db: db}
}

func (r *postgresWebhookRepository) Create(ctx context.Context, w *models.Webhook) error {
	query := `
		INSERT INTO webhooks (id, user_id, url, secret, events, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		w.ID, w.UserID, w.URL, w.Secret, w.Events, w.IsActive, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create webhook: %w", err)
	}
	return nil
}

func (r *postgresWebhookRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	query := `
		SELECT id, user_id, url, secret, events, is_active, last_triggered, failure_count, created_at, updated_at
		FROM webhooks WHERE id = $1
	`
	var w models.Webhook
	err := r.db.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.UserID, &w.URL, &w.Secret, &w.Events, &w.IsActive,
		&w.LastTriggered, &w.FailureCount, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get webhook: %w", err)
	}
	if w.LastTriggered.Valid {
		w.LastTriggeredStr = w.LastTriggered.Time.Format(time.RFC3339)
	}
	return &w, nil
}

func (r *postgresWebhookRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Webhook, error) {
	query := `
		SELECT id, user_id, url, secret, events, is_active, last_triggered, failure_count, created_at, updated_at
		FROM webhooks WHERE user_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*models.Webhook
	for rows.Next() {
		var w models.Webhook
		if err := rows.Scan(&w.ID, &w.UserID, &w.URL, &w.Secret, &w.Events, &w.IsActive,
			&w.LastTriggered, &w.FailureCount, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		if w.LastTriggered.Valid {
			w.LastTriggeredStr = w.LastTriggered.Time.Format(time.RFC3339)
		}
		// Mask secret for listing
		w.Secret = ""
		webhooks = append(webhooks, &w)
	}
	return webhooks, nil
}

func (r *postgresWebhookRepository) ListByUserAndEvent(ctx context.Context, userID uuid.UUID, eventType string) ([]*models.Webhook, error) {
	query := `
		SELECT id, user_id, url, secret, events, is_active, last_triggered, failure_count, created_at, updated_at
		FROM webhooks
		WHERE user_id = $1 AND is_active = true AND $2 = ANY(events)
	`
	rows, err := r.db.Query(ctx, query, userID, eventType)
	if err != nil {
		return nil, fmt.Errorf("list webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []*models.Webhook
	for rows.Next() {
		var w models.Webhook
		if err := rows.Scan(&w.ID, &w.UserID, &w.URL, &w.Secret, &w.Events, &w.IsActive,
			&w.LastTriggered, &w.FailureCount, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}
		webhooks = append(webhooks, &w)
	}
	return webhooks, nil
}

func (r *postgresWebhookRepository) Update(ctx context.Context, w *models.Webhook) error {
	query := `
		UPDATE webhooks SET url = $1, events = $2, is_active = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, w.URL, w.Events, w.IsActive, time.Now(), w.ID)
	return err
}

func (r *postgresWebhookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM webhooks WHERE id = $1", id)
	return err
}

func (r *postgresWebhookRepository) IncrementFailureCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE webhooks SET failure_count = failure_count + 1 WHERE id = $1", id)
	return err
}

func (r *postgresWebhookRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE webhooks SET is_active = false, updated_at = NOW() WHERE id = $1", id)
	return err
}

func (r *postgresWebhookRepository) UpdateLastTriggered(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "UPDATE webhooks SET last_triggered = NOW() WHERE id = $1", id)
	return err
}

func (r *postgresWebhookRepository) CreateLog(ctx context.Context, log *models.WebhookLog) error {
	query := `
		INSERT INTO webhook_logs (id, webhook_id, email_log_id, event_type, payload, response_status, response_body, attempts, success, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID, log.WebhookID, log.EmailLogID, log.EventType, log.Payload,
		log.ResponseStatus, log.ResponseBody, log.Attempts, log.Success, log.CreatedAt,
	)
	return err
}

func (r *postgresWebhookRepository) ListLogsByWebhookID(ctx context.Context, webhookID uuid.UUID, limit int) ([]*models.WebhookLog, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT id, webhook_id, email_log_id, event_type, payload, response_status, response_body, attempts, success, created_at
		FROM webhook_logs WHERE webhook_id = $1
		ORDER BY created_at DESC LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, webhookID, limit)
	if err != nil {
		return nil, fmt.Errorf("list webhook logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.WebhookLog
	for rows.Next() {
		var l models.WebhookLog
		if err := rows.Scan(&l.ID, &l.WebhookID, &l.EmailLogID, &l.EventType, &l.Payload,
			&l.ResponseStatus, &l.ResponseBody, &l.Attempts, &l.Success, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook log: %w", err)
		}
		logs = append(logs, &l)
	}
	return logs, nil
}
