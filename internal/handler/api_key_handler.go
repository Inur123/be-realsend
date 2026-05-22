package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type APIKeyHandler struct {
	keyService service.APIKeyService
}

func NewAPIKeyHandler(keyService service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{keyService: keyService}
}

type createAPIKeyRequest struct {
	Name     string  `json:"name" validate:"required,min=2,max=100"`
	DomainID *string `json:"domain_id" validate:"omitempty,uuid"`
}

// CreateKey handles token generation and metadata insertion.
func (h *APIKeyHandler) CreateKey(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req createAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	var parsedDomainID *uuid.UUID
	if req.DomainID != nil && *req.DomainID != "" {
		dID, err := uuid.Parse(*req.DomainID)
		if err != nil {
			return utils.BadRequest(c, "invalid domain id format")
		}
		parsedDomainID = &dID
	}

	rawKey, keyMeta, err := h.keyService.CreateKey(c.Context(), userID, req.Name, parsedDomainID)
	if err != nil {
		return utils.BadRequest(c, err.Error())
	}

	// Important: Return BOTH rawKey (the secret revealed exactly once) and stored keyMeta
	return utils.SuccessCreated(c, fiber.Map{
		"token":    rawKey,
		"metadata": keyMeta,
	})
}

// ListKeys returns all active key records for the dashboard.
func (h *APIKeyHandler) ListKeys(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	keys, err := h.keyService.ListKeys(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, keys)
}

// RevokeKey deletes/invalidates an existing key.
func (h *APIKeyHandler) RevokeKey(c *fiber.Ctx) error {
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
		return utils.BadRequest(c, "invalid api key id")
	}

	// Verify owner first
	key, err := h.keyService.GetKey(c.Context(), id)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}
	if key.UserID != userID {
		return utils.Forbidden(c, "you do not have permission to revoke this api key")
	}

	err = h.keyService.RevokeKey(c.Context(), id)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{
		"message": "api key revoked successfully",
	})
}
