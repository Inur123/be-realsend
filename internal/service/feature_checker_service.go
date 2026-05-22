package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/realsend/be-realsend/internal/repository"
)

// FeatureCheckerService checks whether a specific feature is enabled for a user,
// considering plan features and admin-level per-user overrides.
type FeatureCheckerService interface {
	HasFeature(ctx context.Context, userID uuid.UUID, featureKey string) (bool, error)
}

type featureCheckerService struct {
	redisClient *redis.Client
	subRepo     repository.SubscriptionRepository
	planRepo    repository.PlanRepository
}

// NewFeatureCheckerService creates a new FeatureCheckerService.
func NewFeatureCheckerService(
	redisClient *redis.Client,
	subRepo repository.SubscriptionRepository,
	planRepo repository.PlanRepository,
) FeatureCheckerService {
	return &featureCheckerService{
		redisClient: redisClient,
		subRepo:     subRepo,
		planRepo:    planRepo,
	}
}

// HasFeature returns true if the user's active plan includes the given featureKey.
// Lookup order: Redis cache → plan_features table.
// Admin overrides (user_plan_overrides) are handled at the repository layer in the future.
func (s *featureCheckerService) HasFeature(ctx context.Context, userID uuid.UUID, featureKey string) (bool, error) {
	// 1. Try Redis cache first (TTL = 5 minutes)
	cacheKey := fmt.Sprintf("feature:%s:%s", userID.String(), featureKey)
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		return cached == "1", nil
	}
	if err != redis.Nil {
		// Redis error, fall through to DB lookup
		fmt.Printf("Warning: redis feature cache read error: %v\n", err)
	}

	// 1b. Check admin overrides first
	overrides, err := s.subRepo.GetOverrides(ctx, userID)
	if err == nil {
		for _, o := range overrides {
			if o.FeatureKey == featureKey {
				enabled := o.OverrideValue == "true" || o.OverrideValue == "1"
				val := "0"
				if enabled {
					val = "1"
				}
				_ = s.redisClient.Set(ctx, cacheKey, val, 5*time.Minute).Err()
				return enabled, nil
			}
		}
	}

	// 2. Get active subscription
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get subscription: %w", err)
	}
	if sub == nil || sub.Status != "active" {
		return false, nil
	}

	// 3. Get plan features from DB
	features, err := s.planRepo.GetFeatures(ctx, sub.PlanID)
	if err != nil {
		return false, fmt.Errorf("get plan features: %w", err)
	}

	// 4. Check feature membership
	enabled := false
	for _, f := range features {
		if f == featureKey {
			enabled = true
			break
		}
	}

	// 5. Cache result for 5 minutes
	val := "0"
	if enabled {
		val = "1"
	}
	if setErr := s.redisClient.Set(ctx, cacheKey, val, 5*time.Minute).Err(); setErr != nil {
		fmt.Printf("Warning: redis feature cache write error: %v\n", setErr)
	}

	return enabled, nil
}
