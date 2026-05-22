package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/database"
	"github.com/realsend/be-realsend/internal/handler"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
)

func main() {
	// 1. Load configuration
	cfg := config.Load()

	// 2. Connect to PostgreSQL
	log.Printf("Connecting to PostgreSQL at %s:%s...", cfg.DBHost, cfg.DBPort)
	dbPool, err := database.NewPostgres(cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer dbPool.Close()
	log.Println("PostgreSQL connected successfully.")

	// 3. Connect to Redis
	log.Printf("Connecting to Redis at %s...", cfg.RedisAddr)
	redisClient, err := database.NewRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	defer redisClient.Close()
	log.Println("Redis connected successfully.")

	// 4. Initialize Asynq Client (for enqueuing background tasks)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer asynqClient.Close()
	log.Println("Asynq client initialized.")

	// 5. Initialize repositories
	userRepo := repository.NewUserRepository(dbPool)
	planRepo := repository.NewPlanRepository(dbPool)
	subRepo := repository.NewSubscriptionRepository(dbPool)
	domainRepo := repository.NewDomainRepository(dbPool)
	apiKeyRepo := repository.NewAPIKeyRepository(dbPool)
	emailRepo := repository.NewEmailRepository(dbPool)
	suppressionRepo := repository.NewSuppressionRepository(dbPool)
	webhookRepo := repository.NewWebhookRepository(dbPool)
	auditLogRepo := repository.NewAuditLogRepository(dbPool)

	// 6. Initialize services
	authService := service.NewAuthService(cfg, dbPool, userRepo, subRepo, planRepo)
	planService := service.NewPlanService(planRepo)
	dnsService := service.NewDNSService()
	domainService := service.NewDomainService(domainRepo, dnsService)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, domainRepo)
	quotaService := service.NewQuotaService(redisClient, subRepo, planRepo)
	webhookService := service.NewWebhookService(webhookRepo, asynqClient)
	trackingService := service.NewTrackingService(emailRepo, webhookService)
	featureChecker := service.NewFeatureCheckerService(redisClient, subRepo, planRepo)
	emailService := service.NewEmailService(emailRepo, domainRepo, suppressionRepo, quotaService, asynqClient, cfg, featureChecker, trackingService)
	analyticsService := service.NewAnalyticsService(emailRepo)
	adminService := service.NewAdminService(userRepo, planRepo, subRepo, auditLogRepo)

	// 7. Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	planHandler := handler.NewPlanHandler(planService)
	domainHandler := handler.NewDomainHandler(domainService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)
	emailHandler := handler.NewEmailHandler(emailService)
	trackingHandler := handler.NewTrackingHandler(trackingService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService)
	logHandler := handler.NewLogHandler(emailRepo)
	adminHandler := handler.NewAdminHandler(adminService, analyticsService)

	// 8. Setup Fiber App
	app := fiber.New(fiber.Config{
		AppName:      "RealSend API Server",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// Global Logger middleware
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Register routing
	handler.RegisterRoutes(
		app,
		cfg,
		redisClient,
		apiKeyRepo,
		authHandler,
		planHandler,
		domainHandler,
		apiKeyHandler,
		emailHandler,
		trackingHandler,
		webhookHandler,
		analyticsHandler,
		logHandler,
		adminHandler,
	)

	// Custom 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    fiber.StatusNotFound,
				"message": fmt.Sprintf("cannot %s %s", c.Method(), c.Path()),
			},
		})
	})

	// 9. Graceful Shutdown setup
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		port := cfg.AppPort
		log.Printf("Starting API server on port %s in %s mode...", port, cfg.AppEnv)
		if err := app.Listen(fmt.Sprintf(":%s", port)); err != nil {
			log.Printf("Server listen error: %v", err)
		}
	}()

	// Block until shutdown signal received
	sig := <-shutdownChan
	log.Printf("Signal %v received, shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Fiber shutdown error: %v", err)
	}

	log.Println("API server shut down successfully.")
}

