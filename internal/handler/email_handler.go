package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

// EmailHandler handles transactional email sending endpoints.
type EmailHandler struct {
	emailService service.EmailService
}

// NewEmailHandler creates a new EmailHandler.
func NewEmailHandler(emailService service.EmailService) *EmailHandler {
	return &EmailHandler{emailService: emailService}
}

// SendEmail handles POST /api/v1/emails/send
func (h *EmailHandler) SendEmail(c *fiber.Ctx) error {
	// user_id and api_key_id are injected by APIKeyAuth middleware
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	apiKeyIDStr, _ := c.Locals("api_key_id").(string)
	apiKeyID, err := uuid.Parse(apiKeyIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid api key id")
	}

	var req service.SendEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	emailLog, err := h.emailService.SendEmail(c.Context(), userID, apiKeyID, &req)
	if err != nil {
		// Differentiate between client errors and server errors
		errMsg := err.Error()
		if contains(errMsg, "not found", "not verified", "suppression", "quota exceeded", "invalid") {
			return utils.BadRequest(c, errMsg)
		}
		return utils.InternalError(c, errMsg)
	}

	return utils.SuccessAccepted(c, fiber.Map{
		"id":           emailLog.ID,
		"status":       emailLog.Status,
		"from_address": emailLog.FromAddress,
		"to_address":   emailLog.ToAddress,
		"subject":      emailLog.Subject,
		"queued_at":    emailLog.QueuedAt,
		"message":      "email queued for delivery",
	})
}

// GetEmail handles GET /api/v1/emails/:id
func (h *EmailHandler) GetEmail(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest(c, "invalid email id format")
	}

	email, err := h.emailService.GetEmail(c.Context(), id, userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	if email == nil {
		return utils.NotFound(c, "email not found")
	}

	return utils.Success(c, email)
}

// contains checks if s contains any of the substrings.
func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
