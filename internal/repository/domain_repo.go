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

type DomainRepository interface {
	Create(ctx context.Context, domain *models.Domain) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Domain, error)
	GetByDomainName(ctx context.Context, userID uuid.UUID, domainName string) (*models.Domain, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Domain, error)
	UpdateVerificationStatus(ctx context.Context, id uuid.UUID, status models.DomainStatus, spfOk, dkimOk, dmarcOk bool) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type postgresDomainRepository struct {
	db *pgxpool.Pool
}

func NewDomainRepository(db *pgxpool.Pool) DomainRepository {
	return &postgresDomainRepository{db: db}
}

func (r *postgresDomainRepository) Create(ctx context.Context, d *models.Domain) error {
	query := `
		INSERT INTO domains (id, user_id, domain_name, status, spf_record, dkim_selector, dkim_public_key, dkim_private_key, dmarc_record, return_path_cname, spf_verified, dkim_verified, dmarc_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := r.db.Exec(ctx, query,
		d.ID,
		d.UserID,
		d.DomainName,
		d.Status,
		d.SPFRecord,
		d.DKIMSelector,
		d.DKIMPublicKey,
		d.DKIMPrivateKey,
		d.DMARCRecord,
		d.ReturnPathCNAME,
		d.SPFVerified,
		d.DKIMVerified,
		d.DMARCVerified,
		d.CreatedAt,
		d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create domain db: %w", err)
	}
	return nil
}

func (r *postgresDomainRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Domain, error) {
	query := `
		SELECT id, user_id, domain_name, status, COALESCE(spf_record, ''), COALESCE(dkim_selector, ''), COALESCE(dkim_public_key, ''), COALESCE(dkim_private_key, ''), COALESCE(dmarc_record, ''), COALESCE(return_path_cname, ''),
		       spf_verified, dkim_verified, dmarc_verified, last_verified_at, verified_at, created_at, updated_at
		FROM domains
		WHERE id = $1
	`
	var d models.Domain
	err := r.db.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.UserID, &d.DomainName, &d.Status, &d.SPFRecord, &d.DKIMSelector, &d.DKIMPublicKey, &d.DKIMPrivateKey, &d.DMARCRecord, &d.ReturnPathCNAME,
		&d.SPFVerified, &d.DKIMVerified, &d.DMARCVerified, &d.LastVerifiedAt, &d.VerifiedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan domain by id: %w", err)
	}

	if d.LastVerifiedAt.Valid {
		d.LastVerifiedAtStr = d.LastVerifiedAt.Time.Format(time.RFC3339)
	}
	if d.VerifiedAt.Valid {
		d.VerifiedAtStr = d.VerifiedAt.Time.Format(time.RFC3339)
	}

	return &d, nil
}

func (r *postgresDomainRepository) GetByDomainName(ctx context.Context, userID uuid.UUID, domainName string) (*models.Domain, error) {
	query := `
		SELECT id, user_id, domain_name, status, COALESCE(spf_record, ''), COALESCE(dkim_selector, ''), COALESCE(dkim_public_key, ''), COALESCE(dkim_private_key, ''), COALESCE(dmarc_record, ''), COALESCE(return_path_cname, ''),
		       spf_verified, dkim_verified, dmarc_verified, last_verified_at, verified_at, created_at, updated_at
		FROM domains
		WHERE user_id = $1 AND domain_name = $2
	`
	var d models.Domain
	err := r.db.QueryRow(ctx, query, userID, domainName).Scan(
		&d.ID, &d.UserID, &d.DomainName, &d.Status, &d.SPFRecord, &d.DKIMSelector, &d.DKIMPublicKey, &d.DKIMPrivateKey, &d.DMARCRecord, &d.ReturnPathCNAME,
		&d.SPFVerified, &d.DKIMVerified, &d.DMARCVerified, &d.LastVerifiedAt, &d.VerifiedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan domain by name: %w", err)
	}

	if d.LastVerifiedAt.Valid {
		d.LastVerifiedAtStr = d.LastVerifiedAt.Time.Format(time.RFC3339)
	}
	if d.VerifiedAt.Valid {
		d.VerifiedAtStr = d.VerifiedAt.Time.Format(time.RFC3339)
	}

	return &d, nil
}

func (r *postgresDomainRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Domain, error) {
	query := `
		SELECT id, user_id, domain_name, status, COALESCE(spf_record, ''), COALESCE(dkim_selector, ''), COALESCE(dkim_public_key, ''), COALESCE(dkim_private_key, ''), COALESCE(dmarc_record, ''), COALESCE(return_path_cname, ''),
		       spf_verified, dkim_verified, dmarc_verified, last_verified_at, verified_at, created_at, updated_at
		FROM domains
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query domains: %w", err)
	}
	defer rows.Close()

	var domains []*models.Domain
	for rows.Next() {
		var d models.Domain
		err := rows.Scan(
			&d.ID, &d.UserID, &d.DomainName, &d.Status, &d.SPFRecord, &d.DKIMSelector, &d.DKIMPublicKey, &d.DKIMPrivateKey, &d.DMARCRecord, &d.ReturnPathCNAME,
			&d.SPFVerified, &d.DKIMVerified, &d.DMARCVerified, &d.LastVerifiedAt, &d.VerifiedAt, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan domain item: %w", err)
		}

		if d.LastVerifiedAt.Valid {
			d.LastVerifiedAtStr = d.LastVerifiedAt.Time.Format(time.RFC3339)
		}
		if d.VerifiedAt.Valid {
			d.VerifiedAtStr = d.VerifiedAt.Time.Format(time.RFC3339)
		}

		domains = append(domains, &d)
	}

	return domains, nil
}

func (r *postgresDomainRepository) UpdateVerificationStatus(ctx context.Context, id uuid.UUID, status models.DomainStatus, spfOk, dkimOk, dmarcOk bool) error {
	now := time.Now()
	// Use $7 (statusStr as plain string) in the CASE WHEN to avoid PostgreSQL
	// being unable to deduce a single type for $1 (enum vs text).
	query := `
		UPDATE domains
		SET status = $1, spf_verified = $2, dkim_verified = $3, dmarc_verified = $4,
		    last_verified_at = $5,
		    verified_at = CASE WHEN $7 = 'verified' THEN $5 ELSE verified_at END,
		    updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query, status, spfOk, dkimOk, dmarcOk, now, id, string(status))
	if err != nil {
		return fmt.Errorf("update domain status: %w", err)
	}
	return nil
}

func (r *postgresDomainRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM domains WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete domain db: %w", err)
	}
	return nil
}
