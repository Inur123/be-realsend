package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/realsend/be-realsend/internal/models"
)
type UserRepository interface {
	Create(ctx context.Context, tx pgx.Tx, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateVerifyStatus(ctx context.Context, id uuid.UUID, verified bool) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	ListAll(ctx context.Context, limit, offset int, search string) ([]*models.User, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.UserStatus) error
	UpdateRole(ctx context.Context, id uuid.UUID, role models.UserRole) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type postgresUserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) Create(ctx context.Context, tx pgx.Tx, user *models.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, full_name, company_name, role, status, email_verified, verify_token, current_plan_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
		user.ID,
		user.Email,
		user.PasswordHash,
		user.FullName,
		user.CompanyName,
		user.Role,
		user.Status,
		user.EmailVerified,
		user.VerifyToken,
		user.CurrentPlanID,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create user db: %w", err)
	}

	return nil
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT u.id, u.email, u.password_hash, u.full_name, u.company_name, u.role, u.status, u.email_verified, u.verify_token, u.current_plan_id, u.last_login_at, u.created_at, u.updated_at,
		       p.name as plan_name, p.slug as plan_slug
		FROM users u
		LEFT JOIN plans p ON u.current_plan_id = p.id
		WHERE u.id = $1
	`
	row := r.db.QueryRow(ctx, query, id)

	var user models.User
	var planName sql.NullString
	var planSlug sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.CompanyName,
		&user.Role,
		&user.Status,
		&user.EmailVerified,
		&user.VerifyToken,
		&user.CurrentPlanID,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&planName,
		&planSlug,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user by id: %w", err)
	}

	if user.CompanyName.Valid {
		user.CompanyNameStr = user.CompanyName.String
	}
	if planName.Valid {
		user.PlanName = planName.String
	}
	if planSlug.Valid {
		user.PlanSlug = planSlug.String
	}

	return &user, nil
}

func (r *postgresUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT u.id, u.email, u.password_hash, u.full_name, u.company_name, u.role, u.status, u.email_verified, u.verify_token, u.current_plan_id, u.last_login_at, u.created_at, u.updated_at,
		       p.name as plan_name, p.slug as plan_slug
		FROM users u
		LEFT JOIN plans p ON u.current_plan_id = p.id
		WHERE u.email = $1
	`
	row := r.db.QueryRow(ctx, query, email)

	var user models.User
	var planName sql.NullString
	var planSlug sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.CompanyName,
		&user.Role,
		&user.Status,
		&user.EmailVerified,
		&user.VerifyToken,
		&user.CurrentPlanID,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&planName,
		&planSlug,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user by email: %w", err)
	}

	if user.CompanyName.Valid {
		user.CompanyNameStr = user.CompanyName.String
	}
	if planName.Valid {
		user.PlanName = planName.String
	}
	if planSlug.Valid {
		user.PlanSlug = planSlug.String
	}

	return &user, nil
}

func (r *postgresUserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET full_name = $1, company_name = $2, current_plan_id = $3, status = $4, role = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := r.db.Exec(ctx, query,
		user.FullName,
		user.CompanyName,
		user.CurrentPlanID,
		user.Status,
		user.Role,
		time.Now(),
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user db: %w", err)
	}
	return nil
}

func (r *postgresUserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, passwordHash, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update password db: %w", err)
	}
	return nil
}

func (r *postgresUserRepository) UpdateVerifyStatus(ctx context.Context, id uuid.UUID, verified bool) error {
	query := `
		UPDATE users
		SET email_verified = $1, status = $2, verify_token = NULL, updated_at = $3
		WHERE id = $4
	`
	status := models.StatusPendingVerification
	if verified {
		status = models.StatusActive
	}
	_, err := r.db.Exec(ctx, query, verified, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update verification status db: %w", err)
	}
	return nil
}

func (r *postgresUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET last_login_at = $1
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update last login db: %w", err)
	}
	return nil
}

func (r *postgresUserRepository) ListAll(ctx context.Context, limit, offset int, search string) ([]*models.User, int64, error) {
	var total int64
	var countQuery string
	var countArgs []interface{}

	if search != "" {
		countQuery = "SELECT COUNT(*) FROM users WHERE email ILIKE $1 OR full_name ILIKE $1"
		countArgs = []interface{}{"%" + search + "%"}
	} else {
		countQuery = "SELECT COUNT(*) FROM users"
	}

	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var selectQuery string
	var selectArgs []interface{}

	if search != "" {
		selectQuery = `
			SELECT u.id, u.email, u.full_name, u.company_name, u.role, u.status, u.email_verified, u.current_plan_id, u.last_login_at, u.created_at, u.updated_at,
			       p.name as plan_name, p.slug as plan_slug
			FROM users u
			LEFT JOIN plans p ON u.current_plan_id = p.id
			WHERE u.email ILIKE $1 OR u.full_name ILIKE $1
			ORDER BY u.created_at DESC
			LIMIT $2 OFFSET $3
		`
		selectArgs = []interface{}{"%" + search + "%", limit, offset}
	} else {
		selectQuery = `
			SELECT u.id, u.email, u.full_name, u.company_name, u.role, u.status, u.email_verified, u.current_plan_id, u.last_login_at, u.created_at, u.updated_at,
			       p.name as plan_name, p.slug as plan_slug
			FROM users u
			LEFT JOIN plans p ON u.current_plan_id = p.id
			ORDER BY u.created_at DESC
			LIMIT $1 OFFSET $2
		`
		selectArgs = []interface{}{limit, offset}
	}

	rows, err := r.db.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var planName sql.NullString
		var planSlug sql.NullString

		err := rows.Scan(
			&user.ID, &user.Email, &user.FullName, &user.CompanyName, &user.Role, &user.Status, &user.EmailVerified, &user.CurrentPlanID, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
			&planName, &planSlug,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}

		if user.CompanyName.Valid {
			user.CompanyNameStr = user.CompanyName.String
		}
		if planName.Valid {
			user.PlanName = planName.String
		}
		if planSlug.Valid {
			user.PlanSlug = planSlug.String
		}

		users = append(users, &user)
	}

	return users, total, nil
}

func (r *postgresUserRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.UserStatus) error {
	query := "UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2"
	_, err := r.db.Exec(ctx, query, status, id)
	return err
}

func (r *postgresUserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role models.UserRole) error {
	query := "UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2"
	_, err := r.db.Exec(ctx, query, role, id)
	return err
}

func (r *postgresUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM users WHERE id = $1"
	_, err := r.db.Exec(ctx, query, id)
	return err
}

