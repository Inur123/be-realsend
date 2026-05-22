package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuppressionRepository handles the suppression_list table.
type SuppressionRepository interface {
	IsSuppressed(ctx context.Context, userID uuid.UUID, emailAddress string) (bool, error)
}

type postgresSuppressionRepository struct {
	db *pgxpool.Pool
}

// NewSuppressionRepository creates a new SuppressionRepository.
func NewSuppressionRepository(db *pgxpool.Pool) SuppressionRepository {
	return &postgresSuppressionRepository{db: db}
}

// IsSuppressed checks if an email address is on the user's or global suppression list.
func (r *postgresSuppressionRepository) IsSuppressed(ctx context.Context, userID uuid.UUID, emailAddress string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM suppression_list
			WHERE email_address = $1
			AND (user_id = $2 OR user_id IS NULL)
		)
	`
	var exists bool
	err := r.db.QueryRow(ctx, query, emailAddress, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check suppression list: %w", err)
	}
	return exists, nil
}
