package utils

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/models"
	"github.com/realsend/be-realsend/internal/repository"
)

// GetAuditMeta extracts IP address, User-Agent, and Cloudflare/Vercel geolocation country or city info.
func GetAuditMeta(c *fiber.Ctx) (string, string, string) {
	ip := c.Get("X-Forwarded-For")
	if ip != "" {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	if ip == "" {
		ip = c.Get("X-Real-IP")
	}
	if ip == "" {
		ip = c.IP()
	}
	if ip == "" {
		ip = "0.0.0.0"
	}

	location := c.Get("CF-IPCity")
	if country := c.Get("CF-IPCountry"); country != "" {
		if location != "" {
			location = location + ", " + country
		} else {
			location = country
		}
	}
	if location == "" {
		location = c.Get("X-Vercel-IP-Country")
	}
	if location == "" {
		location = "Tidak tersedia"
	}

	userAgent := c.Get("User-Agent")
	return ip, userAgent, location
}

// LogAction writes a new audit log record to PostgreSQL.
func LogAction(
	ctx context.Context,
	repo repository.AuditLogRepository,
	c *fiber.Ctx,
	actorID uuid.UUID,
	action string,
	targetType string,
	targetID *uuid.UUID,
	details interface{},
) {
	ip, userAgent, location := GetAuditMeta(c)
	
	var detailsJSON []byte
	if details != nil {
		detailsJSON, _ = json.Marshal(details)
	}

	log := &models.AuditLog{
		ID:         uuid.New(),
		ActorID:    actorID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    detailsJSON,
		IPAddress:  ip,
		UserAgent:  userAgent,
		Location:   location,
		CreatedAt:  time.Now(),
	}
	err := repo.Create(ctx, log)
	if err != nil {
		// Log the error to stdout for debugging
		println("ERROR [LogAction] failed to create audit log:", err.Error())
	}
}
