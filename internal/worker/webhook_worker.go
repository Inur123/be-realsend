package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
)

// WebhookWorker processes webhook dispatch tasks from the Asynq queue.
type WebhookWorker struct {
	webhookRepo repository.WebhookRepository
	httpClient  *http.Client
}

// NewWebhookWorker creates a new webhook worker handler.
func NewWebhookWorker(webhookRepo repository.WebhookRepository) *WebhookWorker {
	return &WebhookWorker{
		webhookRepo: webhookRepo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ProcessTask handles the webhook:dispatch task from Asynq.
func (w *WebhookWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload service.WebhookDispatchPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal webhook payload: %v", err)
	}

	log.Printf("[WebhookWorker] Dispatching %s to webhook %s", payload.EventType, payload.WebhookID)

	// 1. Get webhook config (need URL and secret)
	webhook, err := w.webhookRepo.GetByID(ctx, payload.WebhookID)
	if err != nil {
		return fmt.Errorf("get webhook: %v", err)
	}
	if webhook == nil || !webhook.IsActive {
		log.Printf("[WebhookWorker] Webhook %s not found or inactive, skipping", payload.WebhookID)
		return nil
	}

	// 2. Marshal the payload
	payloadJSON, err := json.Marshal(payload.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %v", err)
	}

	// 3. Sign with HMAC-SHA256
	signature := service.SignPayload(webhook.Secret, payloadJSON)

	// 4. Send HTTP POST
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(payloadJSON))
	if err != nil {
		w.logFailure(ctx, webhook, payload, 0, "failed to create request: "+err.Error())
		return fmt.Errorf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", payload.EventType)
	req.Header.Set("X-Webhook-ID", payload.WebhookID.String())
	req.Header.Set("User-Agent", "RealSend-Webhook/1.0")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logFailure(ctx, webhook, payload, 0, "http error: "+err.Error())
		return fmt.Errorf("http post: %v", err)
	}
	defer resp.Body.Close()

	// Read response body (limit to 1KB)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	responseBody := string(bodyBytes)

	// 5. Check response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success
		w.logSuccess(ctx, webhook, payload, resp.StatusCode, responseBody)
		log.Printf("[WebhookWorker] Webhook %s dispatched successfully (status %d)", payload.WebhookID, resp.StatusCode)
		return nil
	}

	// Failure
	w.logFailure(ctx, webhook, payload, resp.StatusCode, responseBody)

	// Auto-deactivate if too many failures
	if webhook.FailureCount >= 10 {
		_ = w.webhookRepo.Deactivate(ctx, webhook.ID)
		log.Printf("[WebhookWorker] Webhook %s auto-deactivated after %d failures", webhook.ID, webhook.FailureCount)
	}

	return fmt.Errorf("webhook returned status %d", resp.StatusCode)
}

func (w *WebhookWorker) logSuccess(ctx context.Context, webhook *models.Webhook, payload service.WebhookDispatchPayload, statusCode int, responseBody string) {
	payloadJSON, _ := json.Marshal(payload.Payload)
	emailLogID := uuid.NullUUID{}
	if payload.EmailLogID != "" {
		if parsed, err := uuid.Parse(payload.EmailLogID); err == nil {
			emailLogID = uuid.NullUUID{UUID: parsed, Valid: true}
		}
	}

	wLog := &models.WebhookLog{
		ID:             uuid.New(),
		WebhookID:      webhook.ID,
		EmailLogID:     emailLogID,
		EventType:      payload.EventType,
		Payload:        payloadJSON,
		ResponseStatus: sql.NullInt32{Int32: int32(statusCode), Valid: true},
		ResponseBody:   sql.NullString{String: responseBody, Valid: true},
		Attempts:       1,
		Success:        true,
		CreatedAt:      time.Now(),
	}
	_ = w.webhookRepo.CreateLog(ctx, wLog)
	_ = w.webhookRepo.UpdateLastTriggered(ctx, webhook.ID)
}

func (w *WebhookWorker) logFailure(ctx context.Context, webhook *models.Webhook, payload service.WebhookDispatchPayload, statusCode int, responseBody string) {
	payloadJSON, _ := json.Marshal(payload.Payload)
	emailLogID := uuid.NullUUID{}
	if payload.EmailLogID != "" {
		if parsed, err := uuid.Parse(payload.EmailLogID); err == nil {
			emailLogID = uuid.NullUUID{UUID: parsed, Valid: true}
		}
	}

	wLog := &models.WebhookLog{
		ID:             uuid.New(),
		WebhookID:      webhook.ID,
		EmailLogID:     emailLogID,
		EventType:      payload.EventType,
		Payload:        payloadJSON,
		ResponseStatus: sql.NullInt32{Int32: int32(statusCode), Valid: statusCode > 0},
		ResponseBody:   sql.NullString{String: responseBody, Valid: true},
		Attempts:       1,
		Success:        false,
		CreatedAt:      time.Now(),
	}
	_ = w.webhookRepo.CreateLog(ctx, wLog)
	_ = w.webhookRepo.IncrementFailureCount(ctx, webhook.ID)
}
