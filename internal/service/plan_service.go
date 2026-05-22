package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

type PlanService interface {
	GetPublicPlans(ctx context.Context) ([]*models.Plan, error)
	GetPlanByID(ctx context.Context, id uuid.UUID) (*models.Plan, error)
}

type planService struct {
	planRepo repository.PlanRepository
}

func NewPlanService(planRepo repository.PlanRepository) PlanService {
	return &planService{planRepo: planRepo}
}

func (s *planService) GetPublicPlans(ctx context.Context) ([]*models.Plan, error) {
	plans, err := s.planRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get public plans: %w", err)
	}
	return plans, nil
}

func (s *planService) GetPlanByID(ctx context.Context, id uuid.UUID) (*models.Plan, error) {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get plan by id: %w", err)
	}
	return plan, nil
}
