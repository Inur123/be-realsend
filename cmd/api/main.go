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
	_ "github.com/realsend/be-realsend/cmd/api/docs"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/database"
	"github.com/realsend/be-realsend/internal/handler"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/smtpserver"
)

// @title RealSend API
// @version 1.0
// @description Platform Email Transaksional & SMTP Delivery Profesional Indonesia.
// @termsOfService https://realsend.web.id/legal/sla

// @contact.name RealSend Support
// @contact.url https://realsend.web.id
// @contact.email support@realsend.web.id

// @host localhost:3001
// @BasePath /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API Key untuk integrasi aplikasi & kirim email

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Format: "Bearer [JWT_TOKEN]" untuk akses API dashboard

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
	paymentRepo := repository.NewPaymentRepository(dbPool)

	// 6. Initialize services
	authService := service.NewAuthService(cfg, dbPool, userRepo, subRepo, planRepo)
	planService := service.NewPlanService(planRepo)
	dnsService := service.NewDNSService()
	domainService := service.NewDomainService(domainRepo, planRepo, subRepo, userRepo, dnsService)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, domainRepo, planRepo, subRepo, userRepo)
	quotaService := service.NewQuotaService(redisClient, subRepo, planRepo, userRepo)
	webhookService := service.NewWebhookService(webhookRepo, asynqClient, planRepo, subRepo, userRepo)
	trackingService := service.NewTrackingService(emailRepo, webhookService)
	featureChecker := service.NewFeatureCheckerService(redisClient, subRepo, planRepo, userRepo)
	emailService := service.NewEmailService(emailRepo, domainRepo, suppressionRepo, planRepo, subRepo, quotaService, asynqClient, cfg, featureChecker, trackingService)
	analyticsService := service.NewAnalyticsService(emailRepo)
	adminService := service.NewAdminService(userRepo, planRepo, subRepo, auditLogRepo, paymentRepo)
	billingService := service.NewBillingService(cfg, dbPool, planRepo, subRepo, userRepo, paymentRepo)

	// 7. Initialize handlers
	authHandler := handler.NewAuthHandler(authService, auditLogRepo)
	planHandler := handler.NewPlanHandler(planService)
	domainHandler := handler.NewDomainHandler(domainService, auditLogRepo)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService, auditLogRepo)
	emailHandler := handler.NewEmailHandler(emailService)
	trackingHandler := handler.NewTrackingHandler(trackingService)
	webhookHandler := handler.NewWebhookHandler(webhookService, auditLogRepo)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService)
	logHandler := handler.NewLogHandler(emailRepo)
	billingHandler := handler.NewBillingHandler(billingService)
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
		billingHandler,
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

	// Start SMTP Inbound Server
	smtpServer, err := smtpserver.StartServer(cfg, apiKeyRepo, emailService)
	if err != nil {
		log.Fatalf("Failed to start SMTP inbound server: %v", err)
	}

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

	// Shutdown SMTP Inbound Server
	if err := smtpServer.Close(); err != nil {
		log.Printf("SMTP server shutdown error: %v", err)
	} else {
		log.Println("SMTP server shut down successfully.")
	}

	// Shutdown Fiber API Server
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Fiber shutdown error: %v", err)
	}

	log.Println("API server shut down successfully.")
}
