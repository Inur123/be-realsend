package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/middleware"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/redis/go-redis/v9"
)

// RegisterRoutes sets up all routes for the API server.
func RegisterRoutes(
	app *fiber.App,
	cfg *config.Config,
	redisClient *redis.Client,
	apiKeyRepo repository.APIKeyRepository,
	authHandler *AuthHandler,
	planHandler *PlanHandler,
	domainHandler *DomainHandler,
	apiKeyHandler *APIKeyHandler,
	emailHandler *EmailHandler,
	trackingHandler *TrackingHandler,
	webhookHandler *WebhookHandler,
	analyticsHandler *AnalyticsHandler,
	logHandler *LogHandler,
	billingHandler *BillingHandler,
	adminHandler *AdminHandler,
) {
	// CORS Middleware
	app.Use(middleware.CORS(cfg))

	// API Group
	api := app.Group("/api/v1")

	// Public Routes
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   "2026-05-22T14:14:00+07:00",
		})
	})

	// Plans routes (public)
	api.Get("/plans", planHandler.ListPlans)
	api.Get("/plans/:id", planHandler.GetPlan)

	// Auth routes (public)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)

	// Auth routes (protected)
	auth.Post("/logout", middleware.Protected(cfg), authHandler.Logout)
	protectedAuth := auth.Group("/me", middleware.Protected(cfg))
	protectedAuth.Get("/", authHandler.Me)
	protectedAuth.Put("/", authHandler.UpdateProfile)
	protectedAuth.Put("/password", authHandler.ChangePassword)

	// Domain routes (protected)
	domains := api.Group("/domains", middleware.Protected(cfg))
	domains.Post("/", domainHandler.AddDomain)
	domains.Get("/", domainHandler.ListDomains)
	domains.Get("/:id", domainHandler.GetDomain)
	domains.Post("/:id/verify", domainHandler.VerifyDomain)
	domains.Delete("/:id", domainHandler.DeleteDomain)

	// API Key routes (protected)
	apiKeys := api.Group("/api-keys", middleware.Protected(cfg))
	apiKeys.Post("/", apiKeyHandler.CreateKey)
	apiKeys.Get("/", apiKeyHandler.ListKeys)
	apiKeys.Get("/:id", apiKeyHandler.GetKey)
	apiKeys.Delete("/:id", apiKeyHandler.RevokeKey)

	// Email sending routes (protected via API Key auth)
	emails := api.Group("/emails", middleware.APIKeyAuth(apiKeyRepo))
	emails.Post("/send", emailHandler.SendEmail)
	emails.Get("/:id", emailHandler.GetEmail)

	// Webhook routes (protected)
	webhooks := api.Group("/webhooks", middleware.Protected(cfg))
	webhooks.Post("/", webhookHandler.CreateWebhook)
	webhooks.Get("/", webhookHandler.ListWebhooks)
	webhooks.Get("/:id", webhookHandler.GetWebhook)
	webhooks.Put("/:id", webhookHandler.UpdateWebhook)
	webhooks.Delete("/:id", webhookHandler.DeleteWebhook)

	// Analytics routes (protected)
	analytics := api.Group("/analytics", middleware.Protected(cfg))
	analytics.Get("/overview", analyticsHandler.GetOverview)
	analytics.Get("/daily", analyticsHandler.GetDailyBreakdown)
	analytics.Get("/domains", analyticsHandler.GetDomainBreakdown)

	// Log routes (protected)
	emailLogs := api.Group("/email-logs", middleware.Protected(cfg))
	emailLogs.Get("/", logHandler.ListLogs)
	emailLogs.Get("/:id", logHandler.GetLog)

	// Billing routes (Public)
	api.Post("/billing/midtrans/notification", billingHandler.HandleNotification)

	// Billing routes (Protected)
	billing := api.Group("/billing", middleware.Protected(cfg))
	billing.Get("/current", billingHandler.GetOverview)
	billing.Get("/invoices", billingHandler.GetPaymentHistory)
	billing.Post("/checkout", billingHandler.CreateTransaction)
	billing.Post("/sync/:order_id", billingHandler.SyncPaymentStatus)
	billing.Get("/client-key", func(c *fiber.Ctx) error {
		c.Locals("midtrans_client_key", cfg.MidtransClientKey)
		return billingHandler.GetClientKey(c)
	})

	// Tracking routes (public at root)
	app.Get("/t/o/:id", trackingHandler.TrackOpen)
	app.Get("/t/c/:id", trackingHandler.TrackClick)

	// Admin routes (protected via JWT + role check)
	admin := api.Group("/admin", middleware.Protected(cfg), middleware.RequireRole("admin", "super_admin"))
	// Plans CRUD
	admin.Get("/plans", adminHandler.ListPlans)
	admin.Post("/plans", adminHandler.CreatePlan)
	admin.Put("/plans/:id", adminHandler.UpdatePlan)
	admin.Delete("/plans/:id", adminHandler.DeletePlan)
	// Users management
	admin.Get("/users", adminHandler.ListUsers)
	admin.Put("/users/:id/suspend", adminHandler.SuspendUser)
	admin.Put("/users/:id/unsuspend", adminHandler.UnsuspendUser)
	admin.Put("/users/:id/role", adminHandler.ChangeUserRole)
	admin.Delete("/users/:id", adminHandler.DeleteUser)
	// User overrides
	admin.Post("/users/:id/override", adminHandler.OverrideUserFeature)
	admin.Delete("/users/:id/override/:featureKey", adminHandler.DeleteUserOverride)
	// Global Analytics
	admin.Get("/analytics/overview", adminHandler.GetGlobalOverview)
	// Audit logs
	admin.Get("/audit-logs", adminHandler.GetAuditLogs)
	admin.Get("/audit-logs/:id", adminHandler.GetAuditLog)
}
