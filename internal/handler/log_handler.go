package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/utils"
)

// LogHandler handles email log listing endpoints with filtering.
type LogHandler struct {
	emailRepo repository.EmailRepository
}

// NewLogHandler creates a new LogHandler.
func NewLogHandler(emailRepo repository.EmailRepository) *LogHandler {
	return &LogHandler{emailRepo: emailRepo}
}

// ListLogs handles GET /api/v1/logs?status=sent&page=1&per_page=20&search=...
func (h *LogHandler) ListLogs(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return utils.Unauthorized(c, "unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.BadRequest(c, "invalid user id")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))

	filters := repository.EmailLogFilters{
		Status:    c.Query("status"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		DomainID:  c.Query("domain_id"),
		Search:    c.Query("search"),
		Page:      page,
		PerPage:   perPage,
	}

	logs, total, err := h.emailRepo.ListFiltered(c.Context(), userID, filters)
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
