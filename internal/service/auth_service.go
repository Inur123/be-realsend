package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/utils"
)

type AuthService interface {
	Register(ctx context.Context, email, password, fullName string) (*models.User, error)
	Login(ctx context.Context, email, password string) (string, *models.User, error)
	GetProfile(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, fullName, companyName string) (*models.User, error)
	ChangePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error
}

type authService struct {
	cfg      *config.Config
	dbPool   *pgxpool.Pool
	userRepo repository.UserRepository
	subRepo  repository.SubscriptionRepository
	planRepo repository.PlanRepository
}

func NewAuthService(
	cfg *config.Config,
	dbPool *pgxpool.Pool,
	userRepo repository.UserRepository,
	subRepo repository.SubscriptionRepository,
	planRepo repository.PlanRepository,
) AuthService {
	return &authService{
		cfg:      cfg,
		dbPool:   dbPool,
		userRepo: userRepo,
		subRepo:  subRepo,
		planRepo: planRepo,
	}
}

func (s *authService) Register(ctx context.Context, email, password, fullName string) (*models.User, error) {
	// Check if user already exists
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		return nil, errors.New("email address is already registered")
	}

	// Hash password
	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	userID := uuid.New()
	freePlanID := uuid.MustParse("a0000000-0000-0000-0000-000000000001") // Free Plan UUID

	// Verify plan exists
	freePlan, err := s.planRepo.GetByID(ctx, freePlanID)
	if err != nil {
		return nil, fmt.Errorf("get free plan: %w", err)
	}
	if freePlan == nil {
		return nil, errors.New("default Free plan not found in database")
	}

	// Begin transaction
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create user
	user := &models.User{
		ID:            userID,
		Email:         email,
		PasswordHash:  passwordHash,
		FullName:      fullName,
		Role:          models.RoleUser,
		Status:        models.StatusActive, // Auto-active for dev ease, or pending_verification if email setup done. Let's start with active for easy testing!
		EmailVerified: true,               // Let's set it verified by default for local development setup without real SMTP/bounces.
		CurrentPlanID: uuid.NullUUID{UUID: freePlanID, Valid: true},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = s.userRepo.Create(ctx, tx, user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Create subscription
	sub := &models.Subscription{
		ID:                  uuid.New(),
		UserID:              userID,
		PlanID:              freePlanID,
		Status:              models.SubscriptionActive,
		StartedAt:           time.Now(),
		BillingCycle:        "monthly",
		AmountIDR:           0,
		PaymentMethod:       "free",
		EmailsSentThisMonth: 0,
		EmailsSentToday:     0,
		MonthResetAt:        time.Now().AddDate(0, 1, 0),
		DayResetAt:          time.Now().AddDate(0, 0, 1),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	err = s.subRepo.Create(ctx, tx, sub)
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Fetch fresh complete user structure (with join info)
	return s.userRepo.GetByID(ctx, userID)
}

func (s *authService) Login(ctx context.Context, email, password string) (string, *models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", nil, fmt.Errorf("get user by email: %w", err)
	}
	if user == nil {
		return "", nil, errors.New("invalid email or password")
	}

	// Compare password
	if !utils.ComparePassword(user.PasswordHash, password) {
		return "", nil, errors.New("invalid email or password")
	}

	// Check user status
	if user.Status == models.StatusSuspended {
		return "", nil, errors.New("your account has been suspended. Please contact support")
	}

	// Generate JWT token
	token, err := utils.GenerateToken(s.cfg.JWTSecret, s.cfg.JWTExpireHours, user.ID, string(user.Role))
	if err != nil {
		return "", nil, fmt.Errorf("generate jwt: %w", err)
	}

	// Update last login
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	return token, user, nil
}

func (s *authService) GetProfile(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *authService) UpdateProfile(ctx context.Context, id uuid.UUID, fullName, companyName string) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user for update: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	user.FullName = fullName
	if companyName != "" {
		user.CompanyName = sql.NullString{String: companyName, Valid: true}
	} else {
		user.CompanyName = sql.NullString{Valid: false}
	}

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("update user profile: %w", err)
	}

	return s.userRepo.GetByID(ctx, id)
}

func (s *authService) ChangePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get user for password change: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	// Verify old password
	if !utils.ComparePassword(user.PasswordHash, oldPassword) {
		return errors.New("incorrect old password")
	}

	// Hash new password
	newHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	err = s.userRepo.UpdatePassword(ctx, id, newHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}
