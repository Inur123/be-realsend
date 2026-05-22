package service

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
)

// TrackingService handles open and click tracking operations.
type TrackingService interface {
	TrackOpen(ctx context.Context, emailID uuid.UUID) error
	TrackClick(ctx context.Context, emailID uuid.UUID) error
	InjectTrackingPixel(html string, emailID uuid.UUID, baseURL string) string
	RewriteURLs(html string, emailID uuid.UUID, baseURL string) string
}

type trackingService struct {
	emailRepo      repository.EmailRepository
	webhookService WebhookService
}

// NewTrackingService creates a new TrackingService.
func NewTrackingService(emailRepo repository.EmailRepository, webhookService WebhookService) TrackingService {
	return &trackingService{
		emailRepo:      emailRepo,
		webhookService: webhookService,
	}
}

func (s *trackingService) TrackOpen(ctx context.Context, emailID uuid.UUID) error {
	// 1. Increment open count in DB
	if err := s.emailRepo.IncrementOpenCount(ctx, emailID); err != nil {
		return fmt.Errorf("increment open count: %w", err)
	}

	// 2. Get email log to find user_id for webhook dispatch
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil || email == nil {
		return nil // silently fail for tracking
	}

	// 3. Dispatch webhook event (async, best-effort)
	if s.webhookService != nil {
		_ = s.webhookService.DispatchEvent(ctx, email.UserID, "email.opened", map[string]interface{}{
			"email_id":     emailID.String(),
			"to_address":   email.ToAddress,
			"from_address": email.FromAddress,
			"subject":      email.Subject,
			"opened_count": email.OpenedCount + 1,
		})
	}

	return nil
}

func (s *trackingService) TrackClick(ctx context.Context, emailID uuid.UUID) error {
	// 1. Increment click count in DB
	if err := s.emailRepo.IncrementClickCount(ctx, emailID); err != nil {
		return fmt.Errorf("increment click count: %w", err)
	}

	// 2. Get email log for webhook
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil || email == nil {
		return nil
	}

	// 3. Dispatch webhook event
	if s.webhookService != nil {
		_ = s.webhookService.DispatchEvent(ctx, email.UserID, "email.clicked", map[string]interface{}{
			"email_id":      emailID.String(),
			"to_address":    email.ToAddress,
			"from_address":  email.FromAddress,
			"subject":       email.Subject,
			"clicked_count": email.ClickedCount + 1,
		})
	}

	return nil
}

// InjectTrackingPixel inserts a 1x1 transparent pixel before </body> in HTML emails.
func (s *trackingService) InjectTrackingPixel(html string, emailID uuid.UUID, baseURL string) string {
	pixelURL := fmt.Sprintf("%s/t/o/%s", strings.TrimRight(baseURL, "/"), emailID.String())
	pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" alt="" style="display:none;border:0;" />`, pixelURL)

	// Insert before </body> if present
	bodyCloseIdx := strings.LastIndex(strings.ToLower(html), "</body>")
	if bodyCloseIdx >= 0 {
		return html[:bodyCloseIdx] + pixel + html[bodyCloseIdx:]
	}
	// Otherwise append at the end
	return html + pixel
}

// hrefRegex matches href="..." attributes in anchor tags.
var hrefRegex = regexp.MustCompile(`(<a\s[^>]*href\s*=\s*")([^"]+)(")`)

// RewriteURLs replaces all <a href="..."> links with tracking redirect URLs.
func (s *trackingService) RewriteURLs(html string, emailID uuid.UUID, baseURL string) string {
	trackBase := fmt.Sprintf("%s/t/c/%s", strings.TrimRight(baseURL, "/"), emailID.String())

	return hrefRegex.ReplaceAllStringFunc(html, func(match string) string {
		parts := hrefRegex.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		originalURL := parts[2]

		// Skip mailto:, tel:, and # links
		if strings.HasPrefix(originalURL, "mailto:") ||
			strings.HasPrefix(originalURL, "tel:") ||
			strings.HasPrefix(originalURL, "#") {
			return match
		}

		encodedURL := url.QueryEscape(originalURL)
		return parts[1] + trackBase + "?url=" + encodedURL + parts[3]
	})
}
