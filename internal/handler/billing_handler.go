package handler

import (
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/service"
	"github.com/realsend/be-realsend/internal/utils"
)

type BillingHandler struct {
	billingService service.BillingService
}

func NewBillingHandler(billingService service.BillingService) *BillingHandler {
	return &BillingHandler{billingService: billingService}
}

// CreateTransaction creates a Midtrans Snap payment transaction.
// POST /api/v1/billing/create-transaction
func (h *BillingHandler) CreateTransaction(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.Unauthorized(c, "invalid user id")
	}

	var req service.CreateTransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "invalid request body")
	}

	if req.PlanID == uuid.Nil {
		return utils.BadRequest(c, "plan_id is required")
	}
	if req.BillingCycle != "monthly" && req.BillingCycle != "yearly" {
		return utils.BadRequest(c, "billing_cycle must be 'monthly' or 'yearly'")
	}

	result, err := h.billingService.CreateTransaction(c.Context(), userID, req)
	if err != nil {
		log.Printf("Error creating billing transaction: %v", err)
		return utils.InternalError(c, err.Error())
	}

	return utils.SuccessCreated(c, result)
}

// HandleNotification processes Midtrans webhook notifications.
// POST /api/v1/billing/notification (public - called by Midtrans)
func (h *BillingHandler) HandleNotification(c *fiber.Ctx) error {
	var notif service.MidtransNotification
	if err := c.BodyParser(&notif); err != nil {
		log.Printf("Error parsing Midtrans notification: %v", err)
		return utils.BadRequest(c, "invalid notification body")
	}

	log.Printf("Midtrans notification received: order=%s status=%s", notif.OrderID, notif.TransactionStatus)

	if err := h.billingService.HandleNotification(c.Context(), notif); err != nil {
		log.Printf("Error handling Midtrans notification: %v", err)
		// Return 200 to Midtrans even on error to prevent retries for invalid signatures
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// SyncPaymentStatus checks Midtrans transaction status for a user's order and
// applies the same activation flow as the webhook.
func (h *BillingHandler) SyncPaymentStatus(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.Unauthorized(c, "invalid user id")
	}

	orderID := strings.TrimSpace(c.Params("order_id"))
	if orderID == "" {
		return utils.BadRequest(c, "order_id is required")
	}

	payment, err := h.billingService.SyncPaymentStatus(c.Context(), userID, orderID)
	if err != nil {
		log.Printf("Error syncing payment status: %v", err)
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, payment)
}

// GetPaymentHistory returns paginated payment history for the current user.
// GET /api/v1/billing/payments
func (h *BillingHandler) GetPaymentHistory(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.Unauthorized(c, "invalid user id")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 10
	}

	payments, total, err := h.billingService.GetPaymentHistory(c.Context(), userID, page, perPage)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return utils.SuccessWithMeta(c, payments, &utils.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetCurrentSubscription returns the current subscription and plan info.
// GET /api/v1/billing/subscription
func (h *BillingHandler) GetCurrentSubscription(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.Unauthorized(c, "invalid user id")
	}

	info, err := h.billingService.GetCurrentSubscription(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}
	if info == nil {
		return utils.NotFound(c, "no active subscription")
	}

	return utils.Success(c, info)
}

// GetOverview returns the billing overview for the current user.
// GET /api/v1/billing/current
func (h *BillingHandler) GetOverview(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return utils.Unauthorized(c, "invalid user id")
	}

	overview, err := h.billingService.GetOverview(c.Context(), userID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, overview)
}

// GetClientKey returns the Midtrans client key for frontend Snap.js.
// GET /api/v1/billing/client-key
func (h *BillingHandler) GetClientKey(c *fiber.Ctx) error {
	// This will be injected via handler initialization
	return utils.Success(c, fiber.Map{"client_key": c.Locals("midtrans_client_key")})
}
