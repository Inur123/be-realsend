package worker

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/repository"
)

const TaskLogsCleanup = "logs:cleanup"

// CleanupWorker handles the cleanup of old email and webhook logs.
type CleanupWorker struct {
	emailRepo repository.EmailRepository
}

// NewCleanupWorker creates a new CleanupWorker.
func NewCleanupWorker(emailRepo repository.EmailRepository) *CleanupWorker {
	return &CleanupWorker{
		emailRepo: emailRepo,
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
