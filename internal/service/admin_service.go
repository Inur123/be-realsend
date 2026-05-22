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
	CreatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan) error
	UpdatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan) error
	DeletePlan(ctx context.Context, actorID uuid.UUID, id uuid.UUID) error
	// Users
	ListUsers(ctx context.Context, page, perPage int, search string) ([]*models.User, int64, error)
	SuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, note string) error
	UnsuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID) error
	ChangeUserRole(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, role models.UserRole) error
	OverrideUserFeature(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey, value, note string, durationDays int) error
	DeleteUserOverride(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey string) error
	// Audit Logs
	ListAuditLogs(ctx context.Context, page, perPage int) ([]*models.AuditLog, int64, error)
}

type adminService struct {
	userRepo   repository.UserRepository
	planRepo   repository.PlanRepository
	subRepo    repository.SubscriptionRepository
	auditRepo  repository.AuditLogRepository
}

// NewAdminService creates a new AdminService.
func NewAdminService(
	userRepo repository.UserRepository,
	planRepo repository.PlanRepository,
	subRepo repository.SubscriptionRepository,
	auditRepo repository.AuditLogRepository,
) AdminService {
	return &adminService{
		userRepo:   userRepo,
		planRepo:   planRepo,
		subRepo:    subRepo,
		auditRepo:  auditRepo,
	}
}

func (s *adminService) logAction(ctx context.Context, actorID uuid.UUID, action string, targetType string, targetID uuid.UUID, details interface{}) {
	detailsJSON, _ := json.Marshal(details)
	log := &models.AuditLog{
		ID:         uuid.New(),
		ActorID:    actorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   &targetID,
		Details:    detailsJSON,
		IPAddress:  "0.0.0.0", // Filled by handler/middleware if needed
		CreatedAt:  time.Now(),
	}
	_ = s.auditRepo.Create(ctx, log)
}

func (s *adminService) CreatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan) error {
	plan.ID = uuid.New()
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return fmt.Errorf("create plan service: %w", err)
	}

	s.logAction(ctx, actorID, "plan.created", "plan", plan.ID, plan)
	return nil
}

func (s *adminService) UpdatePlan(ctx context.Context, actorID uuid.UUID, plan *models.Plan) error {
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

	s.logAction(ctx, actorID, "plan.updated", "plan", plan.ID, plan)
	return nil
}

func (s *adminService) DeletePlan(ctx context.Context, actorID uuid.UUID, id uuid.UUID) error {
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

	s.logAction(ctx, actorID, "plan.deleted", "plan", id, existing)
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

	return s.userRepo.ListAll(ctx, perPage, offset, search)
}

func (s *adminService) SuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, note string) error {
	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if err := s.userRepo.UpdateStatus(ctx, targetUserID, models.StatusSuspended); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.suspended", "user", targetUserID, map[string]string{"reason": note})
	return nil
}

func (s *adminService) UnsuspendUser(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID) error {
	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	if err := s.userRepo.UpdateStatus(ctx, targetUserID, models.StatusActive); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.unsuspended", "user", targetUserID, nil)
	return nil
}

func (s *adminService) ChangeUserRole(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, role models.UserRole) error {
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

	s.logAction(ctx, actorID, "user.role_changed", "user", targetUserID, map[string]string{"old_role": string(target.Role), "new_role": string(role)})
	return nil
}

func (s *adminService) OverrideUserFeature(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey, value, note string, durationDays int) error {
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

	s.logAction(ctx, actorID, "user.override_set", "user", targetUserID, override)
	return nil
}

func (s *adminService) DeleteUserOverride(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, featureKey string) error {
	if err := s.subRepo.DeleteOverride(ctx, targetUserID, featureKey); err != nil {
		return err
	}

	s.logAction(ctx, actorID, "user.override_deleted", "user", targetUserID, map[string]string{"feature_key": featureKey})
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
