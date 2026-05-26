package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

// Asynq task type for webhook dispatching.
const TaskDispatchWebhook = "webhook:dispatch"

// WebhookDispatchPayload is the JSON payload enqueued into the Asynq task queue.
type WebhookDispatchPayload struct {
	WebhookID  uuid.UUID              `json:"webhook_id"`
	EventType  string                 `json:"event_type"`
	Payload    map[string]interface{} `json:"payload"`
	EmailLogID string                 `json:"email_log_id,omitempty"`
}

// WebhookService handles webhook configuration CRUD and event dispatching.
type WebhookService interface {
	CreateWebhook(ctx context.Context, userID uuid.UUID, url string, events []string) (*models.Webhook, error)
	GetWebhook(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Webhook, error)
	ListWebhooks(ctx context.Context, userID uuid.UUID) ([]*models.Webhook, error)
	UpdateWebhook(ctx context.Context, id uuid.UUID, userID uuid.UUID, url string, events []string, isActive bool) (*models.Webhook, error)
	DeleteWebhook(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetWebhookWithLogs(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Webhook, []*models.WebhookLog, error)
	DispatchEvent(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error
}

type webhookService struct {
	webhookRepo repository.WebhookRepository
	asynqClient *asynq.Client
	planRepo    repository.PlanRepository
	subRepo     repository.SubscriptionRepository
	userRepo    repository.UserRepository
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(
	webhookRepo repository.WebhookRepository,
	asynqClient *asynq.Client,
	planRepo repository.PlanRepository,
	subRepo repository.SubscriptionRepository,
	userRepo repository.UserRepository,
) WebhookService {
	return &webhookService{
		webhookRepo: webhookRepo,
		asynqClient: asynqClient,
		planRepo:    planRepo,
		subRepo:     subRepo,
		userRepo:    userRepo,
	}
}

// GenerateWebhookSecret creates a random secret for HMAC signing.
func GenerateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SignPayload signs a JSON payload with HMAC-SHA256.
func SignPayload(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *webhookService) CreateWebhook(ctx context.Context, userID uuid.UUID, url string, events []string) (*models.Webhook, error) {
	// Super admin bypasses all plan quota checks.
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && user != nil && user.Role == models.RoleSuperAdmin {
		return s.createWebhookWithoutQuotaCheck(ctx, userID, url, events)
	}

	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user subscription: %w", err)
	}
	if sub == nil || sub.Status != models.SubscriptionActive {
		return nil, fmt.Errorf("user does not have an active subscription")
	}

	plan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return nil, fmt.Errorf("get plan details: %w", err)
	}
	if plan == nil {
		return nil, fmt.Errorf("plan not found")
	}

	if plan.MaxWebhooks != -1 {
		webhooks, err := s.webhookRepo.ListByUserID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("count webhooks: %w", err)
		}
		if len(webhooks) >= plan.MaxWebhooks {
			return nil, fmt.Errorf("paket %s hanya mendukung maksimal %d webhook. silakan upgrade untuk menambah webhook", plan.Name, plan.MaxWebhooks)
		}
	}

	return s.createWebhookWithoutQuotaCheck(ctx, userID, url, events)
}

func (s *webhookService) createWebhookWithoutQuotaCheck(ctx context.Context, userID uuid.UUID, url string, events []string) (*models.Webhook, error) {
	secret, err := GenerateWebhookSecret()
	if err != nil {
		return nil, fmt.Errorf("generate webhook secret: %w", err)
	}

	now := time.Now()
	webhook := &models.Webhook{
		ID:        uuid.New(),
		UserID:    userID,
		URL:       url,
		Secret:    secret,
		Events:    events,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.webhookRepo.Create(ctx, webhook); err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return webhook, nil
}

func (s *webhookService) GetWebhook(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Webhook, error) {
	w, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil || w.UserID != userID {
		return nil, nil
	}
	return w, nil
}

func (s *webhookService) ListWebhooks(ctx context.Context, userID uuid.UUID) ([]*models.Webhook, error) {
	return s.webhookRepo.ListByUserID(ctx, userID)
}

func (s *webhookService) UpdateWebhook(ctx context.Context, id uuid.UUID, userID uuid.UUID, url string, events []string, isActive bool) (*models.Webhook, error) {
	w, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil || w.UserID != userID {
		return nil, fmt.Errorf("webhook not found")
	}

	w.URL = url
	w.Events = events
	w.IsActive = isActive

	if err := s.webhookRepo.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("update webhook: %w", err)
	}

	return s.webhookRepo.GetByID(ctx, id)
}

func (s *webhookService) DeleteWebhook(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	w, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if w == nil || w.UserID != userID {
		return fmt.Errorf("webhook not found")
	}
	return s.webhookRepo.Delete(ctx, id)
}

func (s *webhookService) GetWebhookWithLogs(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Webhook, []*models.WebhookLog, error) {
	w, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if w == nil || w.UserID != userID {
		return nil, nil, nil
	}

	logs, err := s.webhookRepo.ListLogsByWebhookID(ctx, id, 20)
	if err != nil {
		return nil, nil, fmt.Errorf("list webhook logs: %w", err)
	}

	return w, logs, nil
}

func (s *webhookService) DispatchEvent(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error {
	// Find all active webhooks subscribed to this event
	webhooks, err := s.webhookRepo.ListByUserAndEvent(ctx, userID, eventType)
	if err != nil {
		return fmt.Errorf("list webhooks for dispatch: %w", err)
	}

	// Add standard fields to payload
	payload["event"] = eventType
	payload["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	for _, w := range webhooks {
		// Enqueue async dispatch task per webhook
		taskPayload := WebhookDispatchPayload{
			WebhookID: w.ID,
			EventType: eventType,
			Payload:   payload,
		}
		if emailID, ok := payload["email_id"].(string); ok {
			taskPayload.EmailLogID = emailID
		}

		data, err := json.Marshal(taskPayload)
		if err != nil {
			continue
		}

		task := asynq.NewTask(TaskDispatchWebhook, data, asynq.MaxRetry(3), asynq.Queue("default"))
		if _, err := s.asynqClient.Enqueue(task); err != nil {
			fmt.Printf("Warning: failed to enqueue webhook dispatch: %v\n", err)
		}
	}

	return nil
}
