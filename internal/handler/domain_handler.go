package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type DomainHandler struct {
	domainService service.DomainService
	auditRepo     repository.AuditLogRepository
}

func NewDomainHandler(domainService service.DomainService, auditRepo repository.AuditLogRepository) *DomainHandler {
	return &DomainHandler{
		domainService: domainService,
		auditRepo:     auditRepo,
	}
}

type addDomainRequest struct {
	DomainName string `json:"domain_name" validate:"required"`
}

// AddDomain handles creating a new domain record.
func (h *DomainHandler) AddDomain(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req addDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.UnprocessableEntity(c, err.Error())
	}

	domain, records, err := h.domainService.AddDomain(c.Context(), userID, req.DomainName)
	if err != nil {
		return utils.Conflict(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "domain.created", "domain", &domain.ID, map[string]string{"domain_name": domain.DomainName})

	return utils.SuccessCreated(c, fiber.Map{
		"domain":  domain,
		"records": records,
	})
}

// ListDomains returns registered domains for the current user.
func (h *DomainHandler) ListDomains(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	domains, err := h.domainService.ListDomains(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, domains)
}

// GetDomain returns records and verification details of a specific domain.
func (h *DomainHandler) GetDomain(c *fiber.Ctx) error {
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
		return utils.BadRequest(c, "invalid domain id")
	}

	domain, records, err := h.domainService.GetDomain(c.Context(), id)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}

	// Security: Verify that the domain belongs to the logged-in user
	if domain.UserID != userID {
		return utils.Forbidden(c, "you do not have permission to access this domain")
	}

	return utils.Success(c, fiber.Map{
		"domain":  domain,
		"records": records,
	})
}

// VerifyDomain triggers the live DNS TXT lookup verify loop.
func (h *DomainHandler) VerifyDomain(c *fiber.Ctx) error {
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
		return utils.BadRequest(c, "invalid domain id")
	}

	// Verify owner first
	domain, _, err := h.domainService.GetDomain(c.Context(), id)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}
	if domain.UserID != userID {
		return utils.Forbidden(c, "you do not have permission to verify this domain")
	}

	updated, err := h.domainService.VerifyDomain(c.Context(), id)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "domain.verified", "domain", &id, map[string]string{"domain_name": updated.DomainName, "status": string(updated.Status)})

	return utils.Success(c, updated)
}

// DeleteDomain deletes a registered domain.
func (h *DomainHandler) DeleteDomain(c *fiber.Ctx) error {
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
		return utils.BadRequest(c, "invalid domain id")
	}

	// Check domain owner
	domain, _, err := h.domainService.GetDomain(c.Context(), id)
	if err != nil {
		return utils.NotFound(c, err.Error())
	}
	if domain.UserID != userID {
		return utils.Forbidden(c, "you do not have permission to delete this domain")
	}

	err = h.domainService.DeleteDomain(c.Context(), id)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	// Audit log
	utils.LogAction(c.Context(), h.auditRepo, c, userID, "domain.deleted", "domain", &id, map[string]string{"domain_name": domain.DomainName})

	return utils.Success(c, fiber.Map{
		"message": "domain deleted successfully",
	})
}
