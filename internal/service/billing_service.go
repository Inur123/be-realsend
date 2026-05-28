package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

// CreateTransactionRequest is the request body for creating a billing transaction.
type CreateTransactionRequest struct {
	PlanID       uuid.UUID `json:"plan_id" validate:"required"`
	BillingCycle string    `json:"billing_cycle" validate:"required,oneof=monthly yearly"`
}

// CreateTransactionResponse is returned after creating a Snap transaction.
type CreateTransactionResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
	OrderID     string `json:"order_id"`
}

// MidtransNotification maps the JSON body Midtrans sends to our webhook.
type MidtransNotification struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	StatusMessage     string `json:"status_message"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	Currency          string `json:"currency"`
}

// BillingOverview returns billing overview for current user.
type BillingOverview struct {
	User           *models.User         `json:"user"`
	Subscription   *models.Subscription `json:"subscription,omitempty"`
	LatestPayment  *models.Payment      `json:"latest_payment,omitempty"`
	RecentInvoices []*models.Payment    `json:"recent_invoices,omitempty"`
}

// BillingService handles Midtrans payment integration.
type BillingService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, req CreateTransactionRequest) (*CreateTransactionResponse, error)
	HandleNotification(ctx context.Context, notif MidtransNotification) error
	SyncPaymentStatus(ctx context.Context, userID uuid.UUID, orderID string) (*models.Payment, error)
	GetPaymentHistory(ctx context.Context, userID uuid.UUID, page, perPage int) ([]*models.Payment, int64, error)
	GetCurrentSubscription(ctx context.Context, userID uuid.UUID) (*SubscriptionInfo, error)
	GetOverview(ctx context.Context, userID uuid.UUID) (*BillingOverview, error)
}

// SubscriptionInfo returns current subscription details for billing page.
type SubscriptionInfo struct {
	Subscription *models.Subscription `json:"subscription"`
	Plan         *models.Plan         `json:"plan"`
}

type billingService struct {
	cfg         *config.Config
	db          *pgxpool.Pool
	snapClient  snap.Client
	coreClient  coreapi.Client
	planRepo    repository.PlanRepository
	subRepo     repository.SubscriptionRepository
	userRepo    repository.UserRepository
	paymentRepo repository.PaymentRepository
}

func NewBillingService(
	cfg *config.Config,
	db *pgxpool.Pool,
	planRepo repository.PlanRepository,
	subRepo repository.SubscriptionRepository,
	userRepo repository.UserRepository,
	paymentRepo repository.PaymentRepository,
) BillingService {
	var s snap.Client
	var env midtrans.EnvironmentType
	if cfg.MidtransIsProduction {
		env = midtrans.Production
	} else {
		env = midtrans.Sandbox
	}
	s.New(cfg.MidtransServerKey, env)
	var c coreapi.Client
	c.New(cfg.MidtransServerKey, env)

	return &billingService{
		cfg:         cfg,
		db:          db,
		snapClient:  s,
		coreClient:  c,
		planRepo:    planRepo,
		subRepo:     subRepo,
		userRepo:    userRepo,
		paymentRepo: paymentRepo,
	}
}

func (s *billingService) CreateTransaction(ctx context.Context, userID uuid.UUID, req CreateTransactionRequest) (*CreateTransactionResponse, error) {
	// 0. Reuse existing pending payment if it exists to avoid duplicate invoices
	var existingOrderID string
	var existingRedirectURL string
	err := s.db.QueryRow(ctx, `
		SELECT external_id, COALESCE(invoice_url, '') 
		FROM payments 
		WHERE user_id = $1 AND status = 'pending' 
		ORDER BY created_at DESC 
		LIMIT 1
	`, userID).Scan(&existingOrderID, &existingRedirectURL)
	if err == nil && existingOrderID != "" && existingRedirectURL != "" {
		return &CreateTransactionResponse{
			Token:       "",
			RedirectURL: existingRedirectURL,
			OrderID:     existingOrderID,
		}, nil
	}

	// 1. Get plan
	plan, err := s.planRepo.GetByID(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	if plan == nil {
		return nil, fmt.Errorf("plan not found")
	}
	if !plan.IsActive || !plan.IsPublic {
		return nil, fmt.Errorf("plan is not available for checkout")
	}
	if plan.PriceMonthlyIDR <= 0 && plan.PriceYearlyIDR <= 0 {
		return nil, fmt.Errorf("free plan does not require checkout")
	}

	// 2. Determine amount based on billing cycle
	var amount int
	switch req.BillingCycle {
	case "monthly":
		amount = plan.PriceMonthlyIDR
	case "yearly":
		amount = plan.PriceYearlyIDR
	default:
		return nil, fmt.Errorf("invalid billing cycle: %s", req.BillingCycle)
	}

	if amount <= 0 {
		return nil, fmt.Errorf("plan %s has no price for %s billing", plan.Name, req.BillingCycle)
	}

	// 3. Get user info
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// 4. Generate unique order ID
	orderID := fmt.Sprintf("RS-%s-%d", userID.String()[:8], time.Now().UnixMilli())

	// 5. Create payment record in DB (status=pending)
	paymentID := uuid.New()
	sub, _ := s.subRepo.GetByUserID(ctx, userID)
	var subID *uuid.UUID
	overageAmount := 0
	if sub != nil {
		subID = &sub.ID
		currentPlan, err := s.planRepo.GetByID(ctx, sub.PlanID)
		if err == nil && currentPlan != nil && currentPlan.OveragePer1kIDR > 0 {
			overageEmails := sub.EmailsSentThisMonth - currentPlan.MonthlyEmailLimit
			if overageEmails > 0 {
				blocks := (overageEmails + 999) / 1000
				overageAmount = blocks * currentPlan.OveragePer1kIDR
			}
		}
	}

	totalAmount := amount + overageAmount
	invoiceNumber := fmt.Sprintf("INV-%s", strings.ToUpper(orderID))

	payment := &models.Payment{
		ID:             paymentID,
		UserID:         userID,
		SubscriptionID: subID,
		PlanID:         &req.PlanID,
		BillingCycle:   req.BillingCycle,
		AmountIDR:      totalAmount,
		PaymentMethod:  "midtrans",
		ExternalID:     orderID,
		Status:         models.PaymentPending,
		InvoiceNumber:  invoiceNumber,
		CreatedAt:      time.Now(),
	}

	if err := s.paymentRepo.Create(ctx, nil, payment); err != nil {
		return nil, fmt.Errorf("create payment record: %w", err)
	}

	// 6. Create Snap transaction
	items := []midtrans.ItemDetails{
		{
			ID:    plan.ID.String(),
			Name:  fmt.Sprintf("%s Plan (%s)", plan.Name, req.BillingCycle),
			Price: int64(amount),
			Qty:   1,
		},
	}
	if overageAmount > 0 {
		items = append(items, midtrans.ItemDetails{
			ID:    "overage",
			Name:  "Biaya Kelebihan Email (Overage)",
			Price: int64(overageAmount),
			Qty:   1,
		})
	}

	snapReq := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(totalAmount),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: user.FullName,
			Email: user.Email,
		},
		Items:           &items,
		EnabledPayments: snap.AllSnapPaymentType,
		Callbacks: &snap.Callbacks{
			Finish: s.cfg.MidtransFinishURL,
		},
	}

	snapResp, errSnap := s.snapClient.CreateTransaction(snapReq)
	if errSnap != nil {
		// Clean up: delete the pending payment
		log.Printf("Midtrans Snap error: %v", errSnap)
		return nil, fmt.Errorf("midtrans create transaction failed: %v", errSnap)
	}

	// Update payment record to save the redirect URL
	if err := s.paymentRepo.UpdateStatusByExternalID(ctx, nil, orderID, models.PaymentPending, nil, snapResp.RedirectURL); err != nil {
		log.Printf("Failed to save redirect URL to payment %s: %v", orderID, err)
	}

	return &CreateTransactionResponse{
		Token:       snapResp.Token,
		RedirectURL: snapResp.RedirectURL,
		OrderID:     orderID,
	}, nil
}

func (s *billingService) HandleNotification(ctx context.Context, notif MidtransNotification) error {
	// 1. Verify signature
	rawSignature := notif.OrderID + notif.StatusCode + notif.GrossAmount + s.cfg.MidtransServerKey
	hash := sha512.Sum512([]byte(rawSignature))
	expectedSignature := hex.EncodeToString(hash[:])

	if expectedSignature != notif.SignatureKey {
		log.Printf("Invalid Midtrans signature for order %s", notif.OrderID)
		return fmt.Errorf("invalid signature")
	}

	// 2. Get payment by order ID
	payment, err := s.paymentRepo.GetByExternalID(ctx, notif.OrderID)
	if err != nil {
		return fmt.Errorf("get payment by external id: %w", err)
	}
	if payment == nil {
		log.Printf("Payment not found for order %s", notif.OrderID)
		return fmt.Errorf("payment not found for order: %s", notif.OrderID)
	}

	// 3. Determine new payment status
	var newStatus models.PaymentStatus
	var paidAt *time.Time

	switch notif.TransactionStatus {
	case "capture":
		if notif.FraudStatus == "accept" {
			newStatus = models.PaymentPaid
			now := time.Now()
			paidAt = &now
		} else {
			newStatus = models.PaymentPending
		}
	case "settlement":
		newStatus = models.PaymentPaid
		now := time.Now()
		paidAt = &now
	case "pending":
		newStatus = models.PaymentPending
	case "deny", "cancel":
		newStatus = models.PaymentFailed
	case "expire":
		newStatus = models.PaymentExpired
	case "refund", "partial_refund":
		newStatus = models.PaymentRefunded
	default:
		log.Printf("Unknown Midtrans status: %s for order %s", notif.TransactionStatus, notif.OrderID)
		return nil
	}

	// 4. Update payment status
	if payment.Status == models.PaymentPaid && newStatus == models.PaymentPaid {
		log.Printf("Payment %s already paid; skipping duplicate subscription activation", notif.OrderID)
		return nil
	}

	if err := s.paymentRepo.UpdateStatusByExternalID(ctx, nil, notif.OrderID, newStatus, paidAt, ""); err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment %s status updated to %s (midtrans status: %s)", notif.OrderID, newStatus, notif.TransactionStatus)

	// 5. If paid → activate/upgrade subscription
	if newStatus == models.PaymentPaid && payment.PlanID != nil {
		if err := s.activateSubscription(ctx, payment); err != nil {
			log.Printf("Error activating subscription for payment %s: %v", notif.OrderID, err)
			return fmt.Errorf("activate subscription: %w", err)
		}
		log.Printf("Subscription activated for user %s, plan %s", payment.UserID, payment.PlanID)
	}

	return nil
}

func (s *billingService) SyncPaymentStatus(ctx context.Context, userID uuid.UUID, orderID string) (*models.Payment, error) {
	payment, err := s.paymentRepo.GetByExternalID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get payment by external id: %w", err)
	}
	if payment == nil {
		return nil, fmt.Errorf("payment not found for order: %s", orderID)
	}
	if payment.UserID != userID {
		return nil, fmt.Errorf("payment does not belong to current user")
	}

	statusResp, midErr := s.coreClient.CheckTransaction(orderID)
	if midErr != nil {
		return nil, fmt.Errorf("midtrans status check failed: %v", midErr)
	}

	signatureKey := statusResp.SignatureKey
	if signatureKey == "" {
		rawSignature := statusResp.OrderID + statusResp.StatusCode + statusResp.GrossAmount + s.cfg.MidtransServerKey
		hash := sha512.Sum512([]byte(rawSignature))
		signatureKey = hex.EncodeToString(hash[:])
	}

	notif := MidtransNotification{
		TransactionTime:   statusResp.TransactionTime,
		TransactionStatus: statusResp.TransactionStatus,
		TransactionID:     statusResp.TransactionID,
		StatusMessage:     statusResp.StatusMessage,
		StatusCode:        statusResp.StatusCode,
		SignatureKey:      signatureKey,
		PaymentType:       statusResp.PaymentType,
		OrderID:           statusResp.OrderID,
		GrossAmount:       statusResp.GrossAmount,
		FraudStatus:       statusResp.FraudStatus,
		Currency:          statusResp.Currency,
	}

	if err := s.HandleNotification(ctx, notif); err != nil {
		return nil, err
	}

	updatedPayment, err := s.paymentRepo.GetByExternalID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get updated payment: %w", err)
	}
	return updatedPayment, nil
}

func (s *billingService) activateSubscription(ctx context.Context, payment *models.Payment) error {
	// Use DB transaction for atomicity
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	currentSub, err := s.subRepo.GetByUserID(ctx, payment.UserID)
	if err != nil {
		return fmt.Errorf("get current subscription: %w", err)
	}

	// Renewals of the same paid plan extend from the current expiry when it is
	// still in the future. Upgrades/downgrades start a fresh billing period.
	base := now
	if currentSub != nil && currentSub.PlanID == *payment.PlanID && currentSub.ExpiresAt != nil && currentSub.ExpiresAt.After(now) {
		base = *currentSub.ExpiresAt
	}

	// Calculate expiry based on billing cycle
	var expiresAt time.Time
	switch payment.BillingCycle {
	case "yearly":
		expiresAt = base.AddDate(1, 0, 0)
	default: // monthly
		expiresAt = base.AddDate(0, 1, 0)
	}

	// Update subscription plan
	if err := s.subRepo.UpdatePlanByUserID(ctx, tx, payment.UserID, *payment.PlanID, payment.BillingCycle, payment.AmountIDR, "midtrans", &expiresAt); err != nil {
		return fmt.Errorf("update subscription plan: %w", err)
	}

	// Update user's current_plan_id in the same transaction.
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET current_plan_id = $1,
		    updated_at = NOW()
		WHERE id = $2
	`, *payment.PlanID, payment.UserID); err != nil {
		return fmt.Errorf("update user plan: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *billingService) GetPaymentHistory(ctx context.Context, userID uuid.UUID, page, perPage int) ([]*models.Payment, int64, error) {
	offset := (page - 1) * perPage
	return s.paymentRepo.ListByUserID(ctx, userID, perPage, offset)
}

func (s *billingService) GetCurrentSubscription(ctx context.Context, userID uuid.UUID) (*SubscriptionInfo, error) {
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	if sub == nil {
		return nil, nil
	}

	plan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}

	return &SubscriptionInfo{
		Subscription: sub,
		Plan:         plan,
	}, nil
}

func (s *billingService) GetOverview(ctx context.Context, userID uuid.UUID) (*BillingOverview, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	// Fetch recent payments (e.g. limit 5)
	payments, _, err := s.paymentRepo.ListByUserID(ctx, userID, 5, 0)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}

	var latestPayment *models.Payment
	if len(payments) > 0 {
		latestPayment = payments[0]
	}

	return &BillingOverview{
		User:           user,
		Subscription:   sub,
		LatestPayment:  latestPayment,
		RecentInvoices: payments,
	}, nil
}
