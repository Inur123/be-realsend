package worker

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
)

// EmailWorker processes email send tasks from the Asynq queue.
type EmailWorker struct {
	emailRepo  repository.EmailRepository
	domainRepo repository.DomainRepository
	cfg        *config.Config
	webhookSvc service.WebhookService
}

// NewEmailWorker creates a new email worker handler.
func NewEmailWorker(
	emailRepo repository.EmailRepository,
	domainRepo repository.DomainRepository,
	cfg *config.Config,
	webhookSvc service.WebhookService,
) *EmailWorker {
	return &EmailWorker{
		emailRepo:  emailRepo,
		domainRepo: domainRepo,
		cfg:        cfg,
		webhookSvc: webhookSvc,
	}
}

// ProcessTask handles the email:send task from Asynq.
func (w *EmailWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload service.EmailSendPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal email payload: %v", err)
	}

	log.Printf("[Worker] Processing email task: %s", payload.EmailLogID.String())

	// 1. Fetch email log
	emailLog, err := w.emailRepo.GetByID(ctx, payload.EmailLogID)
	if err != nil {
		return fmt.Errorf("get email log: %v", err)
	}
	if emailLog == nil {
		log.Printf("[Worker] Email log %s not found, skipping", payload.EmailLogID)
		return nil
	}

	// 2. Update status to processing
	if err := w.emailRepo.UpdateStatus(ctx, emailLog.ID, models.StatusProcessing, "", ""); err != nil {
		log.Printf("[Worker] Failed to update status to processing: %v", err)
	}

	// 3. Get domain for DKIM signing
	var domain *models.Domain
	if emailLog.DomainID.Valid {
		domain, err = w.domainRepo.GetByID(ctx, emailLog.DomainID.UUID)
		if err != nil {
			w.failEmail(ctx, emailLog, "failed to load domain: "+err.Error())
			return fmt.Errorf("get domain: %v", err)
		}
	}

	// 4. Build MIME message
	mimeMsg := w.buildMIMEMessage(emailLog)

	// 5. DKIM sign if domain has private key
	if domain != nil && domain.DKIMPrivateKey != "" && domain.DKIMSelector != "" {
		signedMsg, err := w.signDKIM(mimeMsg, domain)
		if err != nil {
			log.Printf("[Worker] DKIM signing failed for %s, sending without signature: %v", emailLog.ID, err)
			// Continue sending without DKIM signature
		} else {
			mimeMsg = signedMsg
		}
	}

	// 6. Send via SMTP
	smtpAddr := fmt.Sprintf("%s:%s", w.cfg.SMTPHost, w.cfg.SMTPPort)
	
	var auth smtp.Auth
	if w.cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", w.cfg.SMTPUsername, w.cfg.SMTPPassword, w.cfg.SMTPHost)
	}

	recipients := []string{emailLog.ToAddress}
	for _, cc := range emailLog.CCAddresses {
		if cc != "" {
			recipients = append(recipients, cc)
		}
	}
	for _, bcc := range emailLog.BCCAddresses {
		if bcc != "" {
			recipients = append(recipients, bcc)
		}
	}

	err = smtp.SendMail(smtpAddr, auth, emailLog.FromAddress, recipients, []byte(mimeMsg))
	if err != nil {
		w.failEmail(ctx, emailLog, "SMTP send failed: "+err.Error())
		return fmt.Errorf("smtp send: %v", err)
	}

	// 7. Update status to sent
	smtpMessageID := fmt.Sprintf("<%s@%s>", emailLog.ID.String(), extractDomain(emailLog.FromAddress))
	if err := w.emailRepo.UpdateStatus(ctx, emailLog.ID, models.StatusSent, smtpMessageID, "250 OK"); err != nil {
		log.Printf("[Worker] Failed to update status to sent: %v", err)
	}

	// Dispatch webhook event email.sent
	if w.webhookSvc != nil {
		_ = w.webhookSvc.DispatchEvent(ctx, emailLog.UserID, "email.sent", map[string]interface{}{
			"email_id":     emailLog.ID.String(),
			"to_address":   emailLog.ToAddress,
			"from_address": emailLog.FromAddress,
			"subject":      emailLog.Subject,
			"status":       "sent",
			"message_id":   smtpMessageID,
		})
	}

	log.Printf("[Worker] Email %s sent successfully to %s", emailLog.ID, emailLog.ToAddress)
	return nil
}

// buildMIMEMessage constructs a proper MIME email message.
func (w *EmailWorker) buildMIMEMessage(e *models.EmailLog) string {
	var sb strings.Builder

	msgID := fmt.Sprintf("<%s@%s>", e.ID.String(), extractDomain(e.FromAddress))
	
	sb.WriteString(fmt.Sprintf("Message-ID: %s\r\n", msgID))
	sb.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	sb.WriteString(fmt.Sprintf("From: %s\r\n", e.FromAddress))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", e.ToAddress))
	
	if len(e.CCAddresses) > 0 {
		sb.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(e.CCAddresses, ", ")))
	}
	
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", e.Subject))
	sb.WriteString(fmt.Sprintf("MIME-Version: 1.0\r\n"))
	sb.WriteString(fmt.Sprintf("Content-Type: %s; charset=utf-8\r\n", e.ContentType))
	sb.WriteString("X-Mailer: RealSend/1.0\r\n")

	// Add custom headers from metadata
	if e.Headers != nil {
		var headers map[string]string
		if err := json.Unmarshal(e.Headers, &headers); err == nil {
			for k, v := range headers {
				sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
			}
		}
	}

	sb.WriteString("\r\n") // End of headers

	// Body — we store the body in metadata for now
	if e.Metadata != nil {
		var meta map[string]string
		if err := json.Unmarshal(e.Metadata, &meta); err == nil {
			if body, ok := meta["body"]; ok {
				sb.WriteString(body)
			}
		}
	}

	return sb.String()
}

// signDKIM performs a simplified DKIM signature on the email message.
func (w *EmailWorker) signDKIM(message string, domain *models.Domain) (string, error) {
	// Parse PEM private key
	block, _ := pem.Decode([]byte(domain.DKIMPrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block for DKIM key")
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse DKIM private key: %v", err)
	}
	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("DKIM private key is not an RSA private key")
	}

	// Create DKIM-Signature header
	timestamp := time.Now().Unix()
	
	// Hash the message body
	bodyHash := sha256.Sum256([]byte(getBody(message)))
	bodyHashB64 := base64.StdEncoding.EncodeToString(bodyHash[:])

	dkimHeader := fmt.Sprintf(
		"DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=%s; s=%s; t=%d; "+
			"bh=%s; h=From:To:Subject:Date:Message-ID; b=",
		domain.DomainName,
		domain.DKIMSelector,
		timestamp,
		bodyHashB64,
	)

	// Hash the headers + DKIM-Signature for signing
	headerHash := sha256.Sum256([]byte(getHeaders(message) + dkimHeader))
	
	// Sign with RSA-SHA256
	signature, err := rsa.SignPKCS1v15(nil, privateKey, crypto.SHA256, headerHash[:])
	if err != nil {
		return "", fmt.Errorf("sign DKIM: %v", err)
	}

	sigB64 := base64.StdEncoding.EncodeToString(signature)
	dkimHeader += sigB64 + "\r\n"

	// Prepend DKIM header to message
	return dkimHeader + message, nil
}

func (w *EmailWorker) failEmail(ctx context.Context, e *models.EmailLog, reason string) {
	if err := w.emailRepo.UpdateStatus(ctx, e.ID, models.StatusFailed, "", reason); err != nil {
		log.Printf("[Worker] Failed to mark email %s as failed: %v", e.ID, err)
	}
	if w.webhookSvc != nil {
		_ = w.webhookSvc.DispatchEvent(ctx, e.UserID, "email.failed", map[string]interface{}{
			"email_id":     e.ID.String(),
			"to_address":   e.ToAddress,
			"from_address": e.FromAddress,
			"subject":      e.Subject,
			"status":       "failed",
			"error":        reason,
		})
	}
}

// extractDomain gets domain part from email address.
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return "localhost"
}

// getHeaders returns the header portion of a MIME message (before double CRLF).
func getHeaders(msg string) string {
	idx := strings.Index(msg, "\r\n\r\n")
	if idx >= 0 {
		return msg[:idx+2] // include trailing \r\n
	}
	return msg
}

// getBody returns the body portion of a MIME message (after double CRLF).
func getBody(msg string) string {
	idx := strings.Index(msg, "\r\n\r\n")
	if idx >= 0 {
		return msg[idx+4:]
	}
	return ""
}
