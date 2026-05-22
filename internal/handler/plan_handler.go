package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type PlanHandler struct {
	planService service.PlanService
}

func NewPlanHandler(planService service.PlanService) *PlanHandler {
	return &PlanHandler{planService: planService}
}

// ListPlans handles fetching active public pricing plans.
func (h *PlanHandler) ListPlans(c *fiber.Ctx) error {
	plans, err := h.planService.GetPublicPlans(c.Context())
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	return utils.Success(c, plans)
}

// GetPlan handles fetching details of a specific plan.
func (h *PlanHandler) GetPlan(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest(c, "invalid plan id")
	}

	plan, err := h.planService.GetPlanByID(c.Context(), id)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	if plan == nil {
		return utils.NotFound(c, "plan not found")
	}

	return utils.Success(c, plan)
}
