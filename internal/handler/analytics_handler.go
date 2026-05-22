package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

// AnalyticsHandler handles analytics endpoints.
type AnalyticsHandler struct {
	analyticsService service.AnalyticsService
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(analyticsService service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService}
}

// GetOverview handles GET /api/v1/analytics/overview?period=30d
func (h *AnalyticsHandler) GetOverview(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	period := c.Query("period", "30d")

	overview, err := h.analyticsService.GetOverview(c.Context(), userID, period)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, overview)
}

// GetDailyBreakdown handles GET /api/v1/analytics/daily?start_date=...&end_date=...
func (h *AnalyticsHandler) GetDailyBreakdown(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	stats, err := h.analyticsService.GetDailyBreakdown(c.Context(), userID, startDate, endDate)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, stats)
}

// GetDomainBreakdown handles GET /api/v1/analytics/domains
func (h *AnalyticsHandler) GetDomainBreakdown(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	stats, err := h.analyticsService.GetDomainBreakdown(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, stats)
}
