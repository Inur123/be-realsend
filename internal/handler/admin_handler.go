package handler

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

// AdminHandler handles super admin action endpoints.
type AdminHandler struct {
	adminService     service.AdminService
	analyticsService service.AnalyticsService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(adminService service.AdminService, analyticsService service.AnalyticsService) *AdminHandler {
	return &AdminHandler{
		adminService:     adminService,
		analyticsService: analyticsService,
	}
}

func auditMetaFromCtx(c *fiber.Ctx) service.AuditMeta {
	ip := c.Get("X-Forwarded-For")
	if ip != "" {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	if ip == "" {
		ip = c.Get("X-Real-IP")
	}
	if ip == "" {
		ip = c.IP()
	}
	if ip == "" {
		ip = "0.0.0.0"
	}

	location := c.Get("CF-IPCity")
	if country := c.Get("CF-IPCountry"); country != "" {
		if location != "" {
			location = location + ", " + country
		} else {
			location = country
		}
	}
	if location == "" {
		location = c.Get("X-Vercel-IP-Country")
	}
	if location == "" {
		location = "Tidak tersedia"
	}

	return service.AuditMeta{
		IPAddress: ip,
		UserAgent: c.Get("User-Agent"),
		Location:  location,
	}
}

// ListPlans handles GET /api/v1/admin/plans
func (h *AdminHandler) ListPlans(c *fiber.Ctx) error {
	plans, err := h.adminService.ListPlans(c.Context())
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	return utils.Success(c, plans)
}

// CreatePlan handles POST /api/v1/admin/plans
func (h *AdminHandler) CreatePlan(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	var req models.Plan
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := h.adminService.CreatePlan(c.Context(), actorID, &req, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.SuccessCreated(c, req)
}

// UpdatePlan handles PUT /api/v1/admin/plans/:id
func (h *AdminHandler) UpdatePlan(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid plan id")
	}

	var req models.Plan
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}
	req.ID = id

	if err := h.adminService.UpdatePlan(c.Context(), actorID, &req, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, req)
}

// DeletePlan handles DELETE /api/v1/admin/plans/:id
func (h *AdminHandler) DeletePlan(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid plan id")
	}

	if err := h.adminService.DeletePlan(c.Context(), actorID, id, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "plan deleted successfully"})
}

// ListUsers handles GET /api/v1/admin/users
func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	search := c.Query("search")

	users, total, err := h.adminService.ListUsers(c.Context(), page, perPage, search)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return utils.SuccessWithMeta(c, users, &utils.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// SuspendUser handles PUT /api/v1/admin/users/:id/suspend
func (h *AdminHandler) SuspendUser(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&req)

	if err := h.adminService.SuspendUser(c.Context(), actorID, targetUserID, req.Reason, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "user suspended successfully"})
}

// UnsuspendUser handles PUT /api/v1/admin/users/:id/unsuspend
func (h *AdminHandler) UnsuspendUser(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	if err := h.adminService.UnsuspendUser(c.Context(), actorID, targetUserID, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "user unsuspended successfully"})
}

// ChangeUserRole handles PUT /api/v1/admin/users/:id/role
func (h *AdminHandler) ChangeUserRole(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req struct {
		Role models.UserRole `json:"role" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := h.adminService.ChangeUserRole(c.Context(), actorID, targetUserID, req.Role, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "user role updated successfully"})
}

// OverrideUserFeature handles POST /api/v1/admin/users/:id/override
func (h *AdminHandler) OverrideUserFeature(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	var req struct {
		FeatureKey   string `json:"feature_key" validate:"required"`
		Value        string `json:"value" validate:"required"`
		Note         string `json:"note"`
		DurationDays int    `json:"duration_days"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if err := h.adminService.OverrideUserFeature(c.Context(), actorID, targetUserID, req.FeatureKey, req.Value, req.Note, req.DurationDays, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "user feature override set successfully"})
}

// DeleteUserOverride handles DELETE /api/v1/admin/users/:id/override/:featureKey
func (h *AdminHandler) DeleteUserOverride(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	featureKey := c.Params("featureKey")
	if featureKey == "" {
		return utils.BadRequest(c, "missing feature key")
	}

	if err := h.adminService.DeleteUserOverride(c.Context(), actorID, targetUserID, featureKey, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "user feature override deleted successfully"})
}

// DeleteUser handles DELETE /api/v1/admin/users/:id
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	actorIDStr, _ := c.Locals("user_id").(string)
	actorID, _ := uuid.Parse(actorIDStr)

	targetUserID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	if err := h.adminService.DeleteUser(c.Context(), actorID, targetUserID, auditMetaFromCtx(c)); err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{"message": "user deleted successfully"})
}

// GetGlobalOverview handles GET /api/v1/admin/analytics/overview
func (h *AdminHandler) GetGlobalOverview(c *fiber.Ctx) error {
	period := c.Query("period", "30d")

	overview, err := h.analyticsService.GetGlobalOverview(c.Context(), period)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, overview)
}

// GetAuditLogs handles GET /api/v1/admin/audit-logs
func (h *AdminHandler) GetAuditLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))

	logs, total, err := h.adminService.ListAuditLogs(c.Context(), page, perPage)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return utils.SuccessWithMeta(c, logs, &utils.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetAuditLog handles GET /api/v1/admin/audit-logs/:id
func (h *AdminHandler) GetAuditLog(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "invalid audit log id")
	}

	log, err := h.adminService.GetAuditLog(c.Context(), id)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	if log == nil {
		return utils.NotFound(c, "audit log not found")
	}

	return utils.Success(c, log)
}
