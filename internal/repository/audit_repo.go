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

// AuditLogRepository handles CRUD for audit logs.
type AuditLogRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	List(ctx context.Context, limit, offset int) ([]*models.AuditLog, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
}

type postgresAuditLogRepository struct {
	db *pgxpool.Pool
}

// NewAuditLogRepository creates a new AuditLogRepository.
func NewAuditLogRepository(db *pgxpool.Pool) AuditLogRepository {
	return &postgresAuditLogRepository{db: db}
}

func (r *postgresAuditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, actor_id, action, target_type, target_id, details, ip_address, user_agent, location, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID, log.ActorID, log.Action, log.TargetType, log.TargetID, log.Details, log.IPAddress, log.UserAgent, log.Location, log.CreatedAt,
	)
	return err
}

func (r *postgresAuditLogRepository) List(ctx context.Context, limit, offset int) ([]*models.AuditLog, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	query := `
		SELECT a.id, a.actor_id, a.action, a.target_type, a.target_id, a.details, a.ip_address::text, COALESCE(a.user_agent, '') as user_agent, COALESCE(a.location, '') as location, a.created_at, COALESCE(u.email, 'deleted_user') as actor_email
		FROM audit_logs a
		LEFT JOIN users u ON a.actor_id = u.id
		ORDER BY a.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		err := rows.Scan(
			&l.ID, &l.ActorID, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.IPAddress, &l.UserAgent, &l.Location, &l.CreatedAt, &l.ActorEmail,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, &l)
	}

	return logs, total, nil
}

func (r *postgresAuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT a.id, a.actor_id, a.action, a.target_type, a.target_id, a.details, a.ip_address::text, COALESCE(a.user_agent, '') as user_agent, COALESCE(a.location, '') as location, a.created_at, COALESCE(u.email, 'deleted_user') as actor_email
		FROM audit_logs a
		LEFT JOIN users u ON a.actor_id = u.id
		WHERE a.id = $1
	`
	var l models.AuditLog
	err := r.db.QueryRow(ctx, query, id).Scan(
		&l.ID, &l.ActorID, &l.Action, &l.TargetType, &l.TargetID, &l.Details, &l.IPAddress, &l.UserAgent, &l.Location, &l.CreatedAt, &l.ActorEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan audit log by id: %w", err)
	}
	return &l, nil
}
