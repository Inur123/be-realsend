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

type APIKeyRepository interface {
	Create(ctx context.Context, key *models.APIKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error)
	GetByHash(ctx context.Context, hash string) (*models.APIKey, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.APIKey, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type postgresAPIKeyRepository struct {
	db *pgxpool.Pool
}

func NewAPIKeyRepository(db *pgxpool.Pool) APIKeyRepository {
	return &postgresAPIKeyRepository{db: db}
}

func (r *postgresAPIKeyRepository) Create(ctx context.Context, k *models.APIKey) error {
	query := `
		INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, last_4, scopes, domain_id, is_active, last_used_at, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.Exec(ctx, query,
		k.ID,
		k.UserID,
		k.Name,
		k.KeyPrefix,
		k.KeyHash,
		k.Last4,
		k.Scopes,
		k.DomainID,
		k.IsActive,
		k.LastUsedAt,
		k.ExpiresAt,
		k.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create api key db: %w", err)
	}
	return nil
}

func (r *postgresAPIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	query := `
		SELECT id, user_id, name, key_prefix, key_hash, last_4, scopes, domain_id, is_active, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE id = $1
	`
	var k models.APIKey
	err := r.db.QueryRow(ctx, query, id).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Last4, &k.Scopes, &k.DomainID, &k.IsActive, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan api key by id: %w", err)
	}

	if k.LastUsedAt.Valid {
		k.LastUsedAtStr = k.LastUsedAt.Time.Format(time.RFC3339)
	}
	if k.ExpiresAt.Valid {
		k.ExpiresAtStr = k.ExpiresAt.Time.Format(time.RFC3339)
	}
	if k.DomainID.Valid {
		k.DomainIDStr = k.DomainID.UUID.String()
	}

	return &k, nil
}

func (r *postgresAPIKeyRepository) GetByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	query := `
		SELECT id, user_id, name, key_prefix, key_hash, last_4, scopes, domain_id, is_active, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE key_hash = $1 AND is_active = true
	`
	var k models.APIKey
	err := r.db.QueryRow(ctx, query, hash).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Last4, &k.Scopes, &k.DomainID, &k.IsActive, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan api key by hash: %w", err)
	}

	if k.LastUsedAt.Valid {
		k.LastUsedAtStr = k.LastUsedAt.Time.Format(time.RFC3339)
	}
	if k.ExpiresAt.Valid {
		k.ExpiresAtStr = k.ExpiresAt.Time.Format(time.RFC3339)
	}
	if k.DomainID.Valid {
		k.DomainIDStr = k.DomainID.UUID.String()
	}

	return &k, nil
}

func (r *postgresAPIKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.APIKey, error) {
	query := `
		SELECT id, user_id, name, key_prefix, key_hash, last_4, scopes, domain_id, is_active, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Last4, &k.Scopes, &k.DomainID, &k.IsActive, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan api key item: %w", err)
		}

		if k.LastUsedAt.Valid {
			k.LastUsedAtStr = k.LastUsedAt.Time.Format(time.RFC3339)
		}
		if k.ExpiresAt.Valid {
			k.ExpiresAtStr = k.ExpiresAt.Time.Format(time.RFC3339)
		}
		if k.DomainID.Valid {
			k.DomainIDStr = k.DomainID.UUID.String()
		}

		keys = append(keys, &k)
	}

	return keys, nil
}

func (r *postgresAPIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE api_keys
		SET last_used_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}

func (r *postgresAPIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete api key db: %w", err)
	}
	return nil
}
