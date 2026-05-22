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

type EmailRepository interface {
	Create(ctx context.Context, e *models.EmailLog) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.EmailStatus, smtpMessageID, smtpResponse string) error
	UpdateBounceStatus(ctx context.Context, id uuid.UUID, status models.EmailStatus, bounceType models.BounceType, reason string) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.EmailLog, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.EmailLog, error)
	// Tracking
	IncrementOpenCount(ctx context.Context, id uuid.UUID) error
	IncrementClickCount(ctx context.Context, id uuid.UUID) error
	// Analytics
	CountByUserAndStatus(ctx context.Context, userID uuid.UUID, startDate, endDate string) (map[string]int, error)
	DailyBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]DailyStat, error)
	DomainBreakdown(ctx context.Context, userID uuid.UUID) ([]DomainStat, error)
	ListFiltered(ctx context.Context, userID uuid.UUID, filters EmailLogFilters) ([]*models.EmailLog, int64, error)
	// Admin analytics
	CountAllByStatus(ctx context.Context, startDate, endDate string) (map[string]int, error)
	// Cleanup
	CleanupOldLogs(ctx context.Context) (int64, int64, error)
}

// DailyStat holds per-day email statistics.
type DailyStat struct {
	Date      string `json:"date"`
	Sent      int    `json:"sent"`
	Delivered int    `json:"delivered"`
	Bounced   int    `json:"bounced"`
	Opened    int    `json:"opened"`
	Clicked   int    `json:"clicked"`
	Failed    int    `json:"failed"`
}

// DomainStat holds per-domain email statistics.
type DomainStat struct {
	DomainName string `json:"domain_name"`
	Total      int    `json:"total"`
	Sent       int    `json:"sent"`
	Delivered  int    `json:"delivered"`
	Bounced    int    `json:"bounced"`
	Opened     int    `json:"opened"`
}

// EmailLogFilters holds filter parameters for listing email logs.
type EmailLogFilters struct {
	Status    string `json:"status"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	DomainID  string `json:"domain_id"`
	Search    string `json:"search"` // search in to_address or subject
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
}

type postgresEmailRepository struct {
	db *pgxpool.Pool
}

func NewEmailRepository(db *pgxpool.Pool) EmailRepository {
	return &postgresEmailRepository{db: db}
}

func (r *postgresEmailRepository) Create(ctx context.Context, e *models.EmailLog) error {
	query := `
		INSERT INTO email_logs (
			id, user_id, api_key_id, domain_id, from_address, to_address, cc_addresses, bcc_addresses,
			subject, content_type, status, bounce_type, bounce_reason, smtp_message_id, smtp_response,
			opened_at, opened_count, clicked_at, clicked_count, tags, metadata, headers,
			queued_at, sent_at, delivered_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22,
			$23, $24, $25, $26
		)
	`
	
	// Convert tags to slice if nil, or keep nil
	cc := e.CCAddresses
	if cc == nil {
		cc = []string{}
	}
	bcc := e.BCCAddresses
	if bcc == nil {
		bcc = []string{}
	}
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}

	_, err := r.db.Exec(ctx, query,
		e.ID, e.UserID, e.APIKeyID, e.DomainID, e.FromAddress, e.ToAddress, cc, bcc,
		e.Subject, e.ContentType, e.Status, e.BounceType, e.BounceReason, e.SMTPMessageID, e.SMTPResponse,
		e.OpenedAt, e.OpenedCount, e.ClickedAt, e.ClickedCount, tags, e.Metadata, e.Headers,
		e.QueuedAt, e.SentAt, e.DeliveredAt, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create email log db: %w", err)
	}
	return nil
}

func (r *postgresEmailRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.EmailStatus, smtpMessageID, smtpResponse string) error {
	now := time.Now()
	var query string
	
	if status == models.StatusSent {
		query = `
			UPDATE email_logs
			SET status = $1, smtp_message_id = $2, smtp_response = $3, sent_at = $4
			WHERE id = $5
		`
		_, err := r.db.Exec(ctx, query, status, smtpMessageID, smtpResponse, now, id)
		return err
	} else if status == models.StatusDelivered {
		query = `
			UPDATE email_logs
			SET status = $1, smtp_message_id = CASE WHEN $2 <> '' THEN $2 ELSE smtp_message_id END,
			    smtp_response = CASE WHEN $3 <> '' THEN $3 ELSE smtp_response END,
			    delivered_at = $4
			WHERE id = $5
		`
		_, err := r.db.Exec(ctx, query, status, smtpMessageID, smtpResponse, now, id)
		return err
	} else {
		query = `
			UPDATE email_logs
			SET status = $1, smtp_message_id = CASE WHEN $2 <> '' THEN $2 ELSE smtp_message_id END,
			    smtp_response = CASE WHEN $3 <> '' THEN $3 ELSE smtp_response END
			WHERE id = $4
		`
		_, err := r.db.Exec(ctx, query, status, smtpMessageID, smtpResponse, id)
		return err
	}
}

func (r *postgresEmailRepository) UpdateBounceStatus(ctx context.Context, id uuid.UUID, status models.EmailStatus, bounceType models.BounceType, reason string) error {
	query := `
		UPDATE email_logs
		SET status = $1, bounce_type = $2, bounce_reason = $3
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, status, bounceType, reason, id)
	return err
}

func (r *postgresEmailRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.EmailLog, error) {
	query := `
		SELECT 
			id, user_id, api_key_id, domain_id, from_address, to_address, cc_addresses, bcc_addresses,
			subject, content_type, status, bounce_type, bounce_reason, smtp_message_id, smtp_response,
			opened_at, opened_count, clicked_at, clicked_count, tags, metadata, headers,
			queued_at, sent_at, delivered_at, created_at
		FROM email_logs
		WHERE id = $1
	`
	var e models.EmailLog
	err := r.db.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.UserID, &e.APIKeyID, &e.DomainID, &e.FromAddress, &e.ToAddress, &e.CCAddresses, &e.BCCAddresses,
		&e.Subject, &e.ContentType, &e.Status, &e.BounceType, &e.BounceReason, &e.SMTPMessageID, &e.SMTPResponse,
		&e.OpenedAt, &e.OpenedCount, &e.ClickedAt, &e.ClickedCount, &e.Tags, &e.Metadata, &e.Headers,
		&e.QueuedAt, &e.SentAt, &e.DeliveredAt, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan email log by id: %w", err)
	}

	// Format helper presentation strings
	if e.BounceReason.Valid {
		e.BounceReasonStr = e.BounceReason.String
	}
	if e.SMTPMessageID.Valid {
		e.SMTPMessageIDStr = e.SMTPMessageID.String
	}
	if e.SMTPResponse.Valid {
		e.SMTPResponseStr = e.SMTPResponse.String
	}
	if e.OpenedAt.Valid {
		e.OpenedAtStr = e.OpenedAt.Time.Format(time.RFC3339)
	}
	if e.ClickedAt.Valid {
		e.ClickedAtStr = e.ClickedAt.Time.Format(time.RFC3339)
	}
	if e.SentAt.Valid {
		e.SentAtStr = e.SentAt.Time.Format(time.RFC3339)
	}
	if e.DeliveredAt.Valid {
		e.DeliveredAtStr = e.DeliveredAt.Time.Format(time.RFC3339)
	}

	return &e, nil
}

func (r *postgresEmailRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.EmailLog, error) {
	query := `
		SELECT 
			id, user_id, api_key_id, domain_id, from_address, to_address, cc_addresses, bcc_addresses,
			subject, content_type, status, bounce_type, bounce_reason, smtp_message_id, smtp_response,
			opened_at, opened_count, clicked_at, clicked_count, tags, metadata, headers,
			queued_at, sent_at, delivered_at, created_at
		FROM email_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query email logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.EmailLog
	for rows.Next() {
		var e models.EmailLog
		err := rows.Scan(
			&e.ID, &e.UserID, &e.APIKeyID, &e.DomainID, &e.FromAddress, &e.ToAddress, &e.CCAddresses, &e.BCCAddresses,
			&e.Subject, &e.ContentType, &e.Status, &e.BounceType, &e.BounceReason, &e.SMTPMessageID, &e.SMTPResponse,
			&e.OpenedAt, &e.OpenedCount, &e.ClickedAt, &e.ClickedCount, &e.Tags, &e.Metadata, &e.Headers,
			&e.QueuedAt, &e.SentAt, &e.DeliveredAt, &e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan email log item: %w", err)
		}

		if e.BounceReason.Valid {
			e.BounceReasonStr = e.BounceReason.String
		}
		if e.SMTPMessageID.Valid {
			e.SMTPMessageIDStr = e.SMTPMessageID.String
		}
		if e.SMTPResponse.Valid {
			e.SMTPResponseStr = e.SMTPResponse.String
		}
		if e.OpenedAt.Valid {
			e.OpenedAtStr = e.OpenedAt.Time.Format(time.RFC3339)
		}
		if e.ClickedAt.Valid {
			e.ClickedAtStr = e.ClickedAt.Time.Format(time.RFC3339)
		}
		if e.SentAt.Valid {
			e.SentAtStr = e.SentAt.Time.Format(time.RFC3339)
		}
		if e.DeliveredAt.Valid {
			e.DeliveredAtStr = e.DeliveredAt.Time.Format(time.RFC3339)
		}

		logs = append(logs, &e)
	}

	return logs, nil
}

func (r *postgresEmailRepository) IncrementOpenCount(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE email_logs
		SET opened_count = opened_count + 1,
		    opened_at = CASE WHEN opened_at IS NULL THEN NOW() ELSE opened_at END
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *postgresEmailRepository) IncrementClickCount(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE email_logs
		SET clicked_count = clicked_count + 1,
		    clicked_at = CASE WHEN clicked_at IS NULL THEN NOW() ELSE clicked_at END
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *postgresEmailRepository) CountByUserAndStatus(ctx context.Context, userID uuid.UUID, startDate, endDate string) (map[string]int, error) {
	query := `
		SELECT status::text, COUNT(*)::int
		FROM email_logs
		WHERE user_id = $1
		  AND created_at >= $2::timestamptz
		  AND created_at <= $3::timestamptz
		GROUP BY status
	`
	rows, err := r.db.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		result[status] = count
	}
	return result, nil
}

func (r *postgresEmailRepository) DailyBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]DailyStat, error) {
	query := `
		SELECT
			TO_CHAR(created_at, 'YYYY-MM-DD') as date,
			COUNT(*) FILTER (WHERE status IN ('sent', 'delivered', 'opened', 'clicked'))::int as sent,
			COUNT(*) FILTER (WHERE status = 'delivered')::int as delivered,
			COUNT(*) FILTER (WHERE status = 'bounced')::int as bounced,
			COUNT(*) FILTER (WHERE opened_count > 0)::int as opened,
			COUNT(*) FILTER (WHERE clicked_count > 0)::int as clicked,
			COUNT(*) FILTER (WHERE status = 'failed')::int as failed
		FROM email_logs
		WHERE user_id = $1
		  AND created_at >= $2::timestamptz
		  AND created_at <= $3::timestamptz
		GROUP BY TO_CHAR(created_at, 'YYYY-MM-DD')
		ORDER BY date ASC
	`
	rows, err := r.db.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("daily breakdown: %w", err)
	}
	defer rows.Close()

	var stats []DailyStat
	for rows.Next() {
		var s DailyStat
		if err := rows.Scan(&s.Date, &s.Sent, &s.Delivered, &s.Bounced, &s.Opened, &s.Clicked, &s.Failed); err != nil {
			return nil, fmt.Errorf("scan daily stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (r *postgresEmailRepository) DomainBreakdown(ctx context.Context, userID uuid.UUID) ([]DomainStat, error) {
	query := `
		SELECT
			COALESCE(d.domain_name, 'unknown') as domain_name,
			COUNT(*)::int as total,
			COUNT(*) FILTER (WHERE e.status IN ('sent', 'delivered', 'opened', 'clicked'))::int as sent,
			COUNT(*) FILTER (WHERE e.status = 'delivered')::int as delivered,
			COUNT(*) FILTER (WHERE e.status = 'bounced')::int as bounced,
			COUNT(*) FILTER (WHERE e.opened_count > 0)::int as opened
		FROM email_logs e
		LEFT JOIN domains d ON e.domain_id = d.id
		WHERE e.user_id = $1
		GROUP BY d.domain_name
		ORDER BY total DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("domain breakdown: %w", err)
	}
	defer rows.Close()

	var stats []DomainStat
	for rows.Next() {
		var s DomainStat
		if err := rows.Scan(&s.DomainName, &s.Total, &s.Sent, &s.Delivered, &s.Bounced, &s.Opened); err != nil {
			return nil, fmt.Errorf("scan domain stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (r *postgresEmailRepository) ListFiltered(ctx context.Context, userID uuid.UUID, filters EmailLogFilters) ([]*models.EmailLog, int64, error) {
	// Build dynamic WHERE clauses
	baseWhere := "WHERE user_id = $1"
	args := []interface{}{userID}
	argIdx := 2

	if filters.Status != "" {
		baseWhere += fmt.Sprintf(" AND status = $%d::email_status", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.StartDate != "" {
		baseWhere += fmt.Sprintf(" AND created_at >= $%d::timestamptz", argIdx)
		args = append(args, filters.StartDate)
		argIdx++
	}
	if filters.EndDate != "" {
		baseWhere += fmt.Sprintf(" AND created_at <= $%d::timestamptz", argIdx)
		args = append(args, filters.EndDate)
		argIdx++
	}
	if filters.DomainID != "" {
		baseWhere += fmt.Sprintf(" AND domain_id = $%d::uuid", argIdx)
		args = append(args, filters.DomainID)
		argIdx++
	}
	if filters.Search != "" {
		baseWhere += fmt.Sprintf(" AND (to_address ILIKE $%d OR subject ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+filters.Search+"%")
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM email_logs " + baseWhere
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count filtered: %w", err)
	}

	// Paginate
	perPage := filters.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	page := filters.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	selectQuery := fmt.Sprintf(`
		SELECT
			id, user_id, api_key_id, domain_id, from_address, to_address, cc_addresses, bcc_addresses,
			subject, content_type, status, bounce_type, bounce_reason, smtp_message_id, smtp_response,
			opened_at, opened_count, clicked_at, clicked_count, tags, metadata, headers,
			queued_at, sent_at, delivered_at, created_at
		FROM email_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, argIdx, argIdx+1)
	args = append(args, perPage, offset)

	rows, err := r.db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query filtered: %w", err)
	}
	defer rows.Close()

	var logs []*models.EmailLog
	for rows.Next() {
		var e models.EmailLog
		err := rows.Scan(
			&e.ID, &e.UserID, &e.APIKeyID, &e.DomainID, &e.FromAddress, &e.ToAddress, &e.CCAddresses, &e.BCCAddresses,
			&e.Subject, &e.ContentType, &e.Status, &e.BounceType, &e.BounceReason, &e.SMTPMessageID, &e.SMTPResponse,
			&e.OpenedAt, &e.OpenedCount, &e.ClickedAt, &e.ClickedCount, &e.Tags, &e.Metadata, &e.Headers,
			&e.QueuedAt, &e.SentAt, &e.DeliveredAt, &e.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan filtered email: %w", err)
		}

		if e.SentAt.Valid {
			e.SentAtStr = e.SentAt.Time.Format(time.RFC3339)
		}
		if e.OpenedAt.Valid {
			e.OpenedAtStr = e.OpenedAt.Time.Format(time.RFC3339)
		}
		if e.ClickedAt.Valid {
			e.ClickedAtStr = e.ClickedAt.Time.Format(time.RFC3339)
		}

		logs = append(logs, &e)
	}

	return logs, total, nil
}

func (r *postgresEmailRepository) CountAllByStatus(ctx context.Context, startDate, endDate string) (map[string]int, error) {
	query := `
		SELECT status::text, COUNT(*)::int
		FROM email_logs
		WHERE created_at >= $1::timestamptz
		  AND created_at <= $2::timestamptz
		GROUP BY status
	`
	rows, err := r.db.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("count all by status: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan all status count: %w", err)
		}
		result[status] = count
	}
	return result, nil
}

func (r *postgresEmailRepository) CleanupOldLogs(ctx context.Context) (int64, int64, error) {
	// 1. Delete old webhook logs
	webhookQuery := `
		DELETE FROM webhook_logs
		WHERE id IN (
			SELECT wl.id
			FROM webhook_logs wl
			JOIN webhooks w ON wl.webhook_id = w.id
			LEFT JOIN subscriptions s ON w.user_id = s.user_id AND s.status = 'active'
			LEFT JOIN plans p ON s.plan_id = p.id
			WHERE wl.created_at < NOW() - (COALESCE(p.log_retention_days, 7) || ' days')::INTERVAL
		)
	`
	resWebhook, err := r.db.Exec(ctx, webhookQuery)
	if err != nil {
		return 0, 0, fmt.Errorf("cleanup webhook logs: %w", err)
	}
	webhooksDeleted := resWebhook.RowsAffected()

	// 2. Delete old email logs
	emailQuery := `
		DELETE FROM email_logs
		WHERE id IN (
			SELECT e.id
			FROM email_logs e
			LEFT JOIN subscriptions s ON e.user_id = s.user_id AND s.status = 'active'
			LEFT JOIN plans p ON s.plan_id = p.id
			WHERE e.created_at < NOW() - (COALESCE(p.log_retention_days, 7) || ' days')::INTERVAL
		)
	`
	resEmail, err := r.db.Exec(ctx, emailQuery)
	if err != nil {
		return 0, 0, fmt.Errorf("cleanup email logs: %w", err)
	}
	emailsDeleted := resEmail.RowsAffected()

	return emailsDeleted, webhooksDeleted, nil
}

