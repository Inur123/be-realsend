package worker

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/repository"
)

const TaskLogsCleanup = "logs:cleanup"
const TaskDailyQuotaReset = "quota:daily_reset"

// CleanupWorker handles the cleanup of old email and webhook logs, as well as daily quota resets.
type CleanupWorker struct {
	emailRepo repository.EmailRepository
	subRepo   repository.SubscriptionRepository
	planRepo  repository.PlanRepository
}

// NewCleanupWorker creates a new CleanupWorker.
func NewCleanupWorker(emailRepo repository.EmailRepository, subRepo repository.SubscriptionRepository, planRepo repository.PlanRepository) *CleanupWorker {
	return &CleanupWorker{
		emailRepo: emailRepo,
		subRepo:   subRepo,
		planRepo:  planRepo,
	}
}

// ProcessTask runs the log cleanup query.
func (w *CleanupWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	log.Println("[CleanupWorker] Starting logs cleanup process...")
	emailsDeleted, webhooksDeleted, err := w.emailRepo.CleanupOldLogs(ctx)
	if err != nil {
		log.Printf("[CleanupWorker] Error during logs cleanup: %v", err)
		return err
	}
	log.Printf("[CleanupWorker] Cleanup completed successfully: deleted %d email logs and %d webhook logs", emailsDeleted, webhooksDeleted)
	return nil
}

// ProcessQuotaResetTask resets due quota windows and downgrades expired paid subscriptions.
func (w *CleanupWorker) ProcessQuotaResetTask(ctx context.Context, t *asynq.Task) error {
	log.Println("[CleanupWorker] Starting subscription quota/expiry maintenance...")
	if err := w.subRepo.ResetDueUsage(ctx); err != nil {
		log.Printf("[CleanupWorker] Error during due quota reset: %v", err)
		return err
	}

	freePlan, err := w.planRepo.GetBySlug(ctx, "free")
	if err != nil {
		log.Printf("[CleanupWorker] Error loading free plan: %v", err)
		return err
	}
	if freePlan == nil {
		log.Println("[CleanupWorker] Free plan not found; skipping subscription expiry")
		return nil
	}

	expiredCount, err := w.subRepo.ExpireEndedPaidSubscriptions(ctx, freePlan.ID)
	if err != nil {
		log.Printf("[CleanupWorker] Error expiring subscriptions: %v", err)
		return err
	}

	log.Printf("[CleanupWorker] Subscription maintenance completed successfully; downgraded %d expired subscriptions", expiredCount)
	return nil
}
