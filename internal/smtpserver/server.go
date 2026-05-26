package smtpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/service"
)

type Backend struct {
	cfg          *config.Config
	apiKeyRepo   repository.APIKeyRepository
	emailService service.EmailService
}

func NewBackend(cfg *config.Config, apiKeyRepo repository.APIKeyRepository, emailService service.EmailService) *Backend {
	return &Backend{
		cfg:          cfg,
		apiKeyRepo:   apiKeyRepo,
		emailService: emailService,
	}
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		backend: b,
		conn:    c,
	}, nil
}

type Session struct {
	backend    *Backend
	conn       *smtp.Conn
	userID     uuid.UUID
	apiKeyID   uuid.UUID
	from       string
	recipients []string
}

func (s *Session) AuthMechanisms() []string {
	return []string{"PLAIN"}
}

func (s *Session) Auth(mech string) (sasl.Server, error) {
	if mech == "PLAIN" {
		return sasl.NewPlainServer(func(identity, username, password string) error {
			return s.authenticate(username, password)
		}), nil
	}
	return nil, smtp.ErrAuthUnsupported
}

func (s *Session) authenticate(username, password string) error {
	if !strings.HasPrefix(password, "rs_live_") {
		return errors.New("invalid api key format")
	}

	hashBytes := sha256.Sum256([]byte(password))
	hashHex := hex.EncodeToString(hashBytes[:])

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key, err := s.backend.apiKeyRepo.GetByHash(ctx, hashHex)
	if err != nil {
		log.Printf("[SMTP Server] Error querying API key: %v", err)
		return errors.New("temporary authentication error")
	}
	if key == nil || !key.IsActive {
		return errors.New("invalid, expired, or inactive api key")
	}

	s.userID = key.UserID
	s.apiKeyID = key.ID
	return nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.recipients = append(s.recipients, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	if s.userID == uuid.Nil {
		return smtp.ErrAuthRequired
	}

	msg, err := mail.ReadMessage(r)
	if err != nil {
		return fmt.Errorf("read mime message: %w", err)
	}

	subject := msg.Header.Get("Subject")
	dec := new(mime.WordDecoder)
	if decodedSubject, err := dec.DecodeHeader(subject); err == nil {
		subject = decodedSubject
	}

	fromHeader := msg.Header.Get("From")
	ccHeader := msg.Header.Get("Cc")
	bccHeader := msg.Header.Get("Bcc")

	fromEmail := s.from
	if fromEmail == "" && fromHeader != "" {
		addr, err := mail.ParseAddress(fromHeader)
		if err == nil {
			fromEmail = addr.Address
		}
	}

	// Parse body recursively to get content and content type
	body, contentType, err := parseBody(msg.Body, msg.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("parse email body: %w", err)
	}

	// Parse CC / BCC
	cc := parseEmailList(ccHeader)
	bcc := parseEmailList(bccHeader)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, recipient := range s.recipients {
		req := &service.SendEmailRequest{
			From:        fromEmail,
			To:          recipient,
			CC:          cc,
			BCC:         bcc,
			Subject:     subject,
			ContentType: contentType,
			Body:        body,
		}

		_, err := s.backend.emailService.SendEmail(ctx, s.userID, s.apiKeyID, req)
		if err != nil {
			log.Printf("[SMTP Server] Failed to send email to %s: %v", recipient, err)
			return fmt.Errorf("failed to process email for %s: %w", recipient, err)
		}
	}

	return nil
}

func (s *Session) Reset() {
	s.from = ""
	s.recipients = nil
}

func (s *Session) Logout() error {
	return nil
}

func parseBody(bodyReader io.Reader, contentTypeHeader string) (string, string, error) {
	mediaType, params, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, bodyReader)
		return buf.String(), "text/plain", nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(bodyReader, params["boundary"])
		var htmlBody, textBody string
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", "", err
			}

			partContentType := part.Header.Get("Content-Type")
			partMediaType, _, _ := mime.ParseMediaType(partContentType)

			if strings.HasPrefix(partMediaType, "multipart/") {
				h, t, err := parseBody(part, partContentType)
				if err == nil {
					if h != "" {
						htmlBody = h
					}
					if t != "" {
						textBody = t
					}
				}
			} else if partMediaType == "text/html" {
				buf := new(bytes.Buffer)
				_, _ = io.Copy(buf, part)
				htmlBody = buf.String()
			} else if partMediaType == "text/plain" || partMediaType == "" {
				buf := new(bytes.Buffer)
				_, _ = io.Copy(buf, part)
				textBody = buf.String()
			}
		}

		if htmlBody != "" {
			return htmlBody, "text/html", nil
		}
		return textBody, "text/plain", nil
	}

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, bodyReader)
	return buf.String(), mediaType, nil
}

func parseEmailList(headerVal string) []string {
	if headerVal == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(headerVal)
	if err != nil {
		parts := strings.Split(headerVal, ",")
		var clean []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				clean = append(clean, trimmed)
			}
		}
		return clean
	}

	var emails []string
	for _, a := range addrs {
		emails = append(emails, a.Address)
	}
	return emails
}

// StartServer runs the SMTP inbound server.
func StartServer(cfg *config.Config, apiKeyRepo repository.APIKeyRepository, emailService service.EmailService) (*smtp.Server, error) {
	backend := NewBackend(cfg, apiKeyRepo, emailService)
	server := smtp.NewServer(backend)

	server.Addr = fmt.Sprintf(":%s", cfg.SMTPInboundPort)
	server.Domain = cfg.SMTPInboundHost
	server.WriteTimeout = 10 * time.Second
	server.ReadTimeout = 10 * time.Second
	server.MaxMessageBytes = 10 * 1024 * 1024 // 10MB
	server.MaxRecipients = 50
	server.AllowInsecureAuth = true

	log.Printf("[SMTP Server] Starting SMTP Inbound Server on port %s...", cfg.SMTPInboundPort)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != smtp.ErrServerClosed {
			log.Printf("[SMTP Server] SMTP inbound server error: %v", err)
		}
	}()

	return server, nil
}
