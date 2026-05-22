package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

// Asynq task type for email sending
const TaskSendEmail = "email:send"

// EmailSendPayload is the JSON payload enqueued into the Asynq task queue.
type EmailSendPayload struct {
	EmailLogID uuid.UUID `json:"email_log_id"`
}

// SendEmailRequest is the incoming request body for POST /api/v1/emails/send.
type SendEmailRequest struct {
	From        string            `json:"from" validate:"required,email"`
	To          string            `json:"to" validate:"required,email"`
	CC          []string          `json:"cc,omitempty"`
	BCC         []string          `json:"bcc,omitempty"`
	Subject     string            `json:"subject" validate:"required,min=1,max=998"`
	ContentType string            `json:"content_type" validate:"omitempty,oneof=text/plain text/html"`
	Body        string            `json:"body" validate:"required"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// EmailService orchestrates the transactional email sending flow.
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, apiKeyID uuid.UUID, req *SendEmailRequest) (*models.EmailLog, error)
	GetEmail(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.EmailLog, error)
}

type emailService struct {
	emailRepo       repository.EmailRepository
	domainRepo      repository.DomainRepository
	suppressionRepo repository.SuppressionRepository
	quotaService    QuotaService
	asynqClient     *asynq.Client
	cfg             *config.Config
	featureChecker  FeatureCheckerService
	trackingService TrackingService
}

// NewEmailService creates a new EmailService.
func NewEmailService(
	emailRepo repository.EmailRepository,
	domainRepo repository.DomainRepository,
	suppressionRepo repository.SuppressionRepository,
	quotaService QuotaService,
	asynqClient *asynq.Client,
	cfg *config.Config,
	featureChecker FeatureCheckerService,
	trackingService TrackingService,
) EmailService {
	return &emailService{
		emailRepo:       emailRepo,
		domainRepo:      domainRepo,
		suppressionRepo: suppressionRepo,
		quotaService:    quotaService,
		asynqClient:     asynqClient,
		cfg:             cfg,
		featureChecker:  featureChecker,
		trackingService: trackingService,
	}
}

func (s *emailService) SendEmail(ctx context.Context, userID uuid.UUID, apiKeyID uuid.UUID, req *SendEmailRequest) (*models.EmailLog, error) {
	// 1. Extract domain from "from" address
	parts := strings.Split(req.From, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid from address format")
	}
	senderDomain := parts[1]

	// 2. Lookup domain and verify it belongs to the user and is verified
	domain, err := s.domainRepo.GetByDomainName(ctx, userID, senderDomain)
	if err != nil {
		return nil, fmt.Errorf("lookup sender domain: %w", err)
	}
	if domain == nil {
		return nil, fmt.Errorf("sender domain '%s' not found. Register it first at /api/v1/domains", senderDomain)
	}
	if domain.Status != models.DomainVerified {
		return nil, fmt.Errorf("sender domain '%s' is not verified (status: %s). Verify DNS records first", senderDomain, domain.Status)
	}

	// 3. Check suppression list for the recipient
	suppressed, err := s.suppressionRepo.IsSuppressed(ctx, userID, req.To)
	if err != nil {
		return nil, fmt.Errorf("check suppression list: %w", err)
	}
	if suppressed {
		return nil, fmt.Errorf("recipient '%s' is on the suppression list", req.To)
	}

	// 4. Check and consume quota
	allowed, err := s.quotaService.CheckAndIncrement(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check quota: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("email quota exceeded. Upgrade your plan or wait for quota reset")
	}

	// 5. Default content type
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/html"
	}

	emailID := uuid.New()
	body := req.Body

	// 5b. Handle Tracking if text/html
	if contentType == "text/html" {
		if hasOpenTrack, _ := s.featureChecker.HasFeature(ctx, userID, "open_tracking"); hasOpenTrack {
			body = s.trackingService.InjectTrackingPixel(body, emailID, s.cfg.TrackingBaseURL)
		}
		if hasClickTrack, _ := s.featureChecker.HasFeature(ctx, userID, "click_tracking"); hasClickTrack {
			body = s.trackingService.RewriteURLs(body, emailID, s.cfg.TrackingBaseURL)
		}
	}

	// 6. Serialize metadata and headers to JSON bytes
	// Include the body in metadata for the worker to access
	metaMap := make(map[string]string)
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			metaMap[k] = v
		}
	}
	metaMap["body"] = body
	metadataBytes, _ := json.Marshal(metaMap)

	var headersBytes []byte
	if req.Headers != nil {
		headersBytes, _ = json.Marshal(req.Headers)
	}

	// 7. Create email log entry with status=queued
	now := time.Now()
	emailLog := &models.EmailLog{
		ID:          emailID,
		UserID:      userID,
		APIKeyID:    uuid.NullUUID{UUID: apiKeyID, Valid: true},
		DomainID:    uuid.NullUUID{UUID: domain.ID, Valid: true},
		FromAddress: req.From,
		ToAddress:   req.To,
		CCAddresses: req.CC,
		BCCAddresses: req.BCC,
		Subject:     req.Subject,
		ContentType: contentType,
		Status:      models.StatusQueued,
		BounceType:  models.BounceNone,
		Tags:        req.Tags,
		Metadata:    metadataBytes,
		Headers:     headersBytes,
		QueuedAt:    now,
		CreatedAt:   now,
	}

	if err := s.emailRepo.Create(ctx, emailLog); err != nil {
		return nil, fmt.Errorf("create email log: %w", err)
	}

	// 8. Enqueue Asynq task
	payload, err := json.Marshal(EmailSendPayload{
		EmailLogID: emailLog.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal task payload: %w", err)
	}

	task := asynq.NewTask(TaskSendEmail, payload, asynq.MaxRetry(3), asynq.Queue("mail_priority"))
	if _, err := s.asynqClient.Enqueue(task); err != nil {
		// Update status to failed since we couldn't enqueue
		_ = s.emailRepo.UpdateStatus(ctx, emailLog.ID, models.StatusFailed, "", "failed to enqueue task: "+err.Error())
		return nil, fmt.Errorf("enqueue email task: %w", err)
	}

	return emailLog, nil
}

func (s *emailService) GetEmail(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.EmailLog, error) {
	email, err := s.emailRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get email: %w", err)
	}
	if email == nil {
		return nil, nil
	}

	// Verify ownership
	if email.UserID != userID {
		return nil, nil
	}

	// Populate helper string fields for nullable SQL columns
	if email.BounceReason.Valid {
		email.BounceReasonStr = email.BounceReason.String
	}
	if email.SMTPMessageID.Valid {
		email.SMTPMessageIDStr = email.SMTPMessageID.String
	}
	if email.SMTPResponse.Valid {
		email.SMTPResponseStr = email.SMTPResponse.String
	}
	if email.SentAt.Valid {
		email.SentAtStr = email.SentAt.Time.Format(time.RFC3339)
	}

	// Build safe response (hide internal bounce_reason sql.NullString, show clean string)
	_ = sql.NullString{} // keep import

	return email, nil
}
