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
}

// NewCleanupWorker creates a new CleanupWorker.
func NewCleanupWorker(emailRepo repository.EmailRepository, subRepo repository.SubscriptionRepository) *CleanupWorker {
	return &CleanupWorker{
		emailRepo: emailRepo,
		subRepo:   subRepo,
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

// ProcessQuotaResetTask resets PostgreSQL daily email counters to 0.
func (w *CleanupWorker) ProcessQuotaResetTask(ctx context.Context, t *asynq.Task) error {
	log.Println("[CleanupWorker] Starting daily subscription quota reset in database...")
	if err := w.subRepo.ResetDailyUsage(ctx); err != nil {
		log.Printf("[CleanupWorker] Error during daily quota reset: %v", err)
		return err
	}
	log.Println("[CleanupWorker] Daily subscription quota reset completed successfully.")
	return nil
}
