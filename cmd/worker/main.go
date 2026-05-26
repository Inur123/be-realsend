package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/database"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/worker"
)

func main() {
	// 1. Load configuration
	cfg := config.Load()
	log.Printf("Starting background worker in %s mode...", cfg.AppEnv)

	// 2. Connect to PostgreSQL
	log.Printf("Connecting to PostgreSQL at %s:%s...", cfg.DBHost, cfg.DBPort)
	dbPool, err := database.NewPostgres(cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer dbPool.Close()
	log.Println("PostgreSQL connected successfully.")

	// 3. Initialize repositories needed by the worker
	emailRepo := repository.NewEmailRepository(dbPool)
	domainRepo := repository.NewDomainRepository(dbPool)
	planRepo := repository.NewPlanRepository(dbPool)
	subRepo := repository.NewSubscriptionRepository(dbPool)
	userRepo := repository.NewUserRepository(dbPool)
	webhookRepo := repository.NewWebhookRepository(dbPool)

	// 3b. Initialize Asynq Client (for webhook dispatch)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer asynqClient.Close()

	// 3c. Initialize services
	webhookSvc := service.NewWebhookService(webhookRepo, asynqClient, planRepo, subRepo, userRepo)

	// 4. Create Asynq server
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"mail_priority": 6,
				"default":       3,
				"mail_low":      1,
			},
		},
	)

	// 5. Create worker handlers
	emailWorker := worker.NewEmailWorker(emailRepo, domainRepo, cfg, webhookSvc)
	webhookWorker := worker.NewWebhookWorker(webhookRepo)
	cleanupWorker := worker.NewCleanupWorker(emailRepo, subRepo)

	// 5b. Start Asynq Scheduler for periodic tasks
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		},
		&asynq.SchedulerOpts{
			Location: time.UTC,
		},
	)

	// Schedule log cleanup daily at 2:00 AM UTC (queued into mail_low)
	if _, err := scheduler.Register("0 2 * * *", asynq.NewTask(worker.TaskLogsCleanup, nil, asynq.Queue("mail_low"))); err != nil {
		log.Fatalf("Failed to register daily log cleanup task: %v", err)
	}

	// Schedule daily quota reset in PostgreSQL at 12:00 AM UTC (queued into mail_low)
	if _, err := scheduler.Register("0 0 * * *", asynq.NewTask(worker.TaskDailyQuotaReset, nil, asynq.Queue("mail_low"))); err != nil {
		log.Fatalf("Failed to register daily quota reset task: %v", err)
	}

	// 6. Register task handlers
	mux := asynq.NewServeMux()
	mux.HandleFunc(service.TaskSendEmail, emailWorker.ProcessTask)
	mux.HandleFunc(service.TaskDispatchWebhook, webhookWorker.ProcessTask)
	mux.HandleFunc(worker.TaskLogsCleanup, cleanupWorker.ProcessTask)
	mux.HandleFunc(worker.TaskDailyQuotaReset, cleanupWorker.ProcessQuotaResetTask)

	// 7. Graceful shutdown signal handling
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("Worker processes are ready. Subscribed to task queues: [mail_priority, default, mail_low]")
		if err := srv.Run(mux); err != nil {
			log.Printf("Asynq server error: %v", err)
		}
	}()

	go func() {
		log.Println("Asynq Scheduler is starting...")
		if err := scheduler.Run(); err != nil {
			log.Printf("Asynq Scheduler error: %v", err)
		}
	}()

	log.Println("Worker running...")
	sig := <-stopChan
	log.Printf("Signal %v received, stopping worker...", sig)

	scheduler.Shutdown()
	srv.Shutdown()
	log.Println("Worker stopped successfully.")
}
