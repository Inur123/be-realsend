package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

// AdminService manages admin-only backend actions and audits.
type AdminService interface {
	// Plans
	ListPlans(ctx context.Context) ([]*models.Plan, error)
	CreatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan, meta AuditMeta) error
	UpdatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan, meta AuditMeta) error
	DeletePlan(ctx context.Context, actorID uuid.UUID, id uuid.UUID, meta AuditMeta) error
	// Users
	ListUsers(ctx context.Context, page, perPage int, search string) ([]*models.User, int64, error)
	SuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, note string, meta AuditMeta) error
	UnsuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, meta AuditMeta) error
	ChangeUserRole(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, role models.UserRole, meta AuditMeta) error
	DeleteUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, meta AuditMeta) error
	OverrideUserFeature(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey, value, note string, durationDays int, meta AuditMeta) error
	DeleteUserOverride(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey string, meta AuditMeta) error
	// Audit Logs
	ListAuditLogs(ctx context.Context, page, perPage int) ([]*models.AuditLog, int64, error)
	GetAuditLog(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	// Transactions
	ListTransactions(ctx context.Context, page, perPage int, search string, status string) ([]*models.AdminPayment, int64, *models.TransactionStats, error)
	GetTransactionByID(ctx context.Context, id uuid.UUID) (*models.AdminPayment, error)
}

type AuditMeta struct {
	IPAddress string
	UserAgent string
	Location  string
}

type adminService struct {
	userRepo    repository.UserRepository
	planRepo    repository.PlanRepository
	subRepo     repository.SubscriptionRepository
	auditRepo   repository.AuditLogRepository
	paymentRepo repository.PaymentRepository
}

// NewAdminService creates a new AdminService.
func NewAdminService(
	userRepo repository.UserRepository,
	planRepo repository.PlanRepository,
	subRepo repository.SubscriptionRepository,
	auditRepo repository.AuditLogRepository,
	paymentRepo repository.PaymentRepository,
) AdminService {
	return &adminService{
		userRepo:    userRepo,
		planRepo:    planRepo,
		subRepo:     subRepo,
		auditRepo:   auditRepo,
		paymentRepo: paymentRepo,
	}
}

func (s *adminService) logAction(ctx context.Context, actorID uuid.UUID, action string, targetType string, targetID uuid.UUID, details interface{}, meta AuditMeta) {
	detailsJSON, _ := json.Marshal(details)
	log := &models.AuditLog{
		ID:         uuid.New(),
		ActorID:    actorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   &targetID,
		Details:    detailsJSON,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
		Location:   meta.Location,
		CreatedAt:  time.Now(),
	}
	_ = s.auditRepo.Create(ctx, log)
}

func (s *adminService) ListPlans(ctx context.Context) ([]*models.Plan, error) {
	plans, err := s.planRepo.GetAllAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plans admin service: %w", err)
	}
	return plans, nil
}

func (s *adminService) CreatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan, meta AuditMeta) error {
	plan.ID = uuid.New()
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return fmt.Errorf("create plan service: %w", err)
	}

	s.logAction(ctx, actorID, "plan.created", "plan", plan.ID, plan, meta)
	return nil
}

func (s *adminService) UpdatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan, meta AuditMeta) error {
	existing, err := s.planRepo.GetByID(ctx, plan.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("plan not found")
	}

	plan.UpdatedAt = time.Now()
	plan.CreatedAt = existing.CreatedAt

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return fmt.Errorf("update plan service: %w", err)
	}

	s.logAction(ctx, actorID, "plan.updated", "plan", plan.ID, plan, meta)
	return nil
}

func (s *adminService) DeletePlan(ctx context.Context, actorID uuid.UUID, id uuid.UUID, meta AuditMeta) error {
	existing, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("plan not found")
	}

	if err := s.planRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete plan service: %w", err)
	}

	s.logAction(ctx, actorID, "plan.deleted", "plan", id, existing, meta)
	return nil
}

func (s *adminService) ListUsers(ctx context.Context, page, perPage int, search string) ([]*models.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	users, total, err := s.userRepo.ListAll(ctx, perPage, offset, search)
	if err != nil {
		return nil, 0, err
	}

	for _, u := range users {
		sub, err := s.subRepo.GetByUserID(ctx, u.ID)
		if err == nil && sub != nil {
			u.Subscription = sub
		}
		overrides, err := s.subRepo.GetOverrides(ctx, u.ID)
		if err == nil && overrides != nil {
			u.Overrides = overrides
		} else {
			u.Overrides = []*models.UserPlanOverride{}
		}
	}

	return users, total, nil
}

func (s *adminService) SuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, note string, meta AuditMeta) error {
	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if target.Role == models.RoleSuperAdmin && actorID != targetUserID {
		actor, err := s.userRepo.GetByID(ctx, actorID)
		if err != nil {
			return err
		}
		if actor == nil || actor.Role != models.RoleSuperAdmin {
			return fmt.Errorf("only a super admin can modify a super admin user")
		}
	}

	if err := s.userRepo.UpdateStatus(ctx, targetUserID, models.StatusSuspended); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.suspended", "user", targetUserID, map[string]string{"reason": note}, meta)
	return nil
}

func (s *adminService) UnsuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, meta AuditMeta) error {
	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if target.Role == models.RoleSuperAdmin && actorID != targetUserID {
		actor, err := s.userRepo.GetByID(ctx, actorID)
		if err != nil {
			return err
		}
		if actor == nil || actor.Role != models.RoleSuperAdmin {
			return fmt.Errorf("only a super admin can modify a super admin user")
		}
	}

	if err := s.userRepo.UpdateStatus(ctx, targetUserID, models.StatusActive); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.unsuspended", "user", targetUserID, nil, meta)
	return nil
}

func (s *adminService) ChangeUserRole(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, role models.UserRole, meta AuditMeta) error {
	if actorID == targetUserID {
		return fmt.Errorf("you cannot change your own role")
	}

	actor, err := s.userRepo.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor == nil || actor.Role != models.RoleSuperAdmin {
		return fmt.Errorf("only a super admin can change user roles")
	}

	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if err := s.userRepo.UpdateRole(ctx, targetUserID, role); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.role_changed", "user", targetUserID, map[string]string{"old_role": string(target.Role), "new_role": string(role)}, meta)
	return nil
}

func (s *adminService) OverrideUserFeature(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey, value, note string, durationDays int, meta AuditMeta) error {
	actor, err := s.userRepo.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor == nil {
		return fmt.Errorf("actor not found")
	}

	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if actor.Role != models.RoleSuperAdmin {
		if actorID == targetUserID {
			return fmt.Errorf("only a super admin can modify their own limits & features")
		}
		if target.Role != models.RoleUser {
			return fmt.Errorf("admins can only manage limits & features of standard users")
		}
	}

	var expiresAt *time.Time
	if durationDays > 0 {
		exp := time.Now().AddDate(0, 0, durationDays)
		expiresAt = &exp
	}

	override := &models.UserPlanOverride{
		ID:            uuid.New(),
		UserID:        targetUserID,
		FeatureKey:    featureKey,
		OverrideValue: value,
		Note:          note,
		CreatedBy:     &actorID,
		ExpiresAt:     expiresAt,
		CreatedAt:     time.Now(),
	}

	if err := s.subRepo.SetOverride(ctx, override); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.override_set", "user", targetUserID, override, meta)
	return nil
}

func (s *adminService) DeleteUserOverride(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey string, meta AuditMeta) error {
	actor, err := s.userRepo.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor == nil {
		return fmt.Errorf("actor not found")
	}

	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if actor.Role != models.RoleSuperAdmin {
		if actorID == targetUserID {
			return fmt.Errorf("only a super admin can modify their own limits & features")
		}
		if target.Role != models.RoleUser {
			return fmt.Errorf("admins can only manage limits & features of standard users")
		}
	}

	if err := s.subRepo.DeleteOverride(ctx, targetUserID, featureKey); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.override_deleted", "user", targetUserID, map[string]string{"feature_key": featureKey}, meta)
	return nil
}

func (s *adminService) DeleteUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, meta AuditMeta) error {
	if actorID == targetUserID {
		return fmt.Errorf("you cannot delete your own account")
	}

	actor, err := s.userRepo.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor == nil || actor.Role != models.RoleSuperAdmin {
		return fmt.Errorf("only a super admin can delete users")
	}

	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if err := s.userRepo.Delete(ctx, targetUserID); err != nil {
		return fmt.Errorf("delete user service: %w", err)
	}

	s.logAction(ctx, actorID, "user.deleted", "user", targetUserID, map[string]string{"email": target.Email}, meta)
	return nil
}

func (s *adminService) ListAuditLogs(ctx context.Context, page, perPage int) ([]*models.AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	return s.auditRepo.List(ctx, perPage, offset)
}

func (s *adminService) GetAuditLog(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	log, err := s.auditRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if log == nil {
		return nil, nil
	}
	return log, nil
}

func (s *adminService) ListTransactions(ctx context.Context, page, perPage int, search string, status string) ([]*models.AdminPayment, int64, *models.TransactionStats, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	payments, total, err := s.paymentRepo.ListAll(ctx, perPage, offset, search, status)
	if err != nil {
		return nil, 0, nil, err
	}

	stats, err := s.paymentRepo.GetStats(ctx)
	if err != nil {
		return nil, 0, nil, err
	}

	return payments, total, stats, nil
}

func (s *adminService) GetTransactionByID(ctx context.Context, id uuid.UUID) (*models.AdminPayment, error) {
	payment, err := s.paymentRepo.GetAdminByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return payment, nil
}

