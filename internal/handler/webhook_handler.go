package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

// WebhookHandler handles webhook configuration endpoints.
type WebhookHandler struct {
	webhookService service.WebhookService
	auditRepo      repository.AuditLogRepository
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(webhookService service.WebhookService, auditRepo repository.AuditLogRepository) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		auditRepo:      auditRepo,
	}
}

type createWebhookRequest struct {
	URL    string   `json:"url" validate:"required,url"`
	Events []string `json:"events" validate:"required,min=1"`
}

type updateWebhookRequest struct {
	URL      string   `json:"url" validate:"required,url"`
	Events   []string `json:"events" validate:"required,min=1"`
	IsActive bool     `json:"is_active"`
}

// CreateWebhook handles POST /api/v1/webhooks
func (h *WebhookHandler) CreateWebhook(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req createWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	webhook, err := h.webhookService.CreateWebhook(c.Context(), userID, req.URL, req.Events)
	if err != nil {
		return utils.Conflict(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "webhook.created", "webhook", &webhook.ID, map[string]interface{}{"url": webhook.URL, "events": webhook.Events})

	return utils.SuccessCreated(c, webhook)
}

// ListWebhooks handles GET /api/v1/webhooks
func (h *WebhookHandler) ListWebhooks(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	webhooks, err := h.webhookService.ListWebhooks(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, webhooks)
}

// GetWebhook handles GET /api/v1/webhooks/:id
func (h *WebhookHandler) GetWebhook(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid webhook id")
	}

	webhook, logs, err := h.webhookService.GetWebhookWithLogs(c.Context(), id, userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	if webhook == nil {
		return utils.NotFound(c, "webhook not found")
	}

	return utils.Success(c, fiber.Map{
		"webhook": webhook,
		"logs":    logs,
	})
}

// UpdateWebhook handles PUT /api/v1/webhooks/:id
func (h *WebhookHandler) UpdateWebhook(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid webhook id")
	}

	var req updateWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}
	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	webhook, err := h.webhookService.UpdateWebhook(c.Context(), id, userID, req.URL, req.Events, req.IsActive)
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "webhook.updated", "webhook", &id, map[string]interface{}{"url": webhook.URL, "events": webhook.Events, "is_active": webhook.IsActive})

	return utils.Success(c, webhook)
}

// DeleteWebhook handles DELETE /api/v1/webhooks/:id
func (h *WebhookHandler) DeleteWebhook(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid webhook id")
	}

	if err := h.webhookService.DeleteWebhook(c.Context(), id, userID); err != nil {
		return utils.BadRequest(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "webhook.deleted", "webhook", &id, nil)

	return utils.Success(c, fiber.Map{"message": "webhook deleted successfully"})
}
