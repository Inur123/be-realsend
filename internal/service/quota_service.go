package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/realsend/be-realsend/internal/repository"
)

type QuotaService interface {
	CheckAndIncrement(ctx context.Context, userID uuid.UUID) (bool, error)
	ResetDailyCounters(ctx context.Context) error
}

type quotaService struct {
	redisClient *redis.Client
	subRepo     repository.SubscriptionRepository
	planRepo    repository.PlanRepository
}

func NewQuotaService(redisClient *redis.Client, subRepo repository.SubscriptionRepository, planRepo repository.PlanRepository) QuotaService {
	return &quotaService{
		redisClient: redisClient,
		subRepo:     subRepo,
		planRepo:    planRepo,
	}
}

func (s *quotaService) CheckAndIncrement(ctx context.Context, userID uuid.UUID) (bool, error) {
	// 1. Get active subscription
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get user subscription: %w", err)
	}
	if sub == nil || sub.Status != "active" {
		return false, fmt.Errorf("user does not have an active subscription")
	}

	// 2. Get plan limits
	plan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return false, fmt.Errorf("get plan details: %w", err)
	}
	if plan == nil {
		return false, fmt.Errorf("plan not found")
	}

	now := time.Now()
	dayKey := fmt.Sprintf("quota:%s:day:%s", userID.String(), now.Format("20060102"))
	monthKey := fmt.Sprintf("quota:%s:month:%s", userID.String(), now.Format("200601"))

	// 3. Check and increment Redis day counter
	dayCount, err := s.redisClient.Get(ctx, dayKey).Int()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("get redis day quota: %w", err)
	}

	// If daily limit reached (if limit is not -1 meaning unlimited)
	if plan.DailyEmailLimit != -1 && dayCount >= plan.DailyEmailLimit {
		return false, nil
	}

	// 4. Check and increment Redis month counter
	monthCount, err := s.redisClient.Get(ctx, monthKey).Int()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("get redis month quota: %w", err)
	}

	// If monthly limit reached (if limit is not -1 meaning unlimited)
	if plan.MonthlyEmailLimit != -1 && monthCount >= plan.MonthlyEmailLimit {
		return false, nil
	}

	// 5. Quota check passed, increment counters
	pipe := s.redisClient.Pipeline()
	pipe.Incr(ctx, dayKey)
	pipe.Expire(ctx, dayKey, 25*time.Hour)
	pipe.Incr(ctx, monthKey)
	pipe.Expire(ctx, monthKey, 32*24*time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("increment redis quota: %w", err)
	}

	// 6. Asynchronously or synchronously update database usage
	// To prevent lock contention on high throughput sending, we update usage.
	// But since this is standard execution, updating DB is safe.
	err = s.subRepo.UpdateUsage(ctx, userID, 1, 1)
	if err != nil {
		// Log error but don't fail the request since Redis already tracked it
		fmt.Printf("Warning: failed to update PostgreSQL usage: %v\n", err)
	}

	return true, nil
}

func (s *quotaService) ResetDailyCounters(ctx context.Context) error {
	// Daily counter is self-expired via TTL, so we do not need to manually delete.
	return nil
}
