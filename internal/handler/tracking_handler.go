package handler

import (
	"encoding/base64"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/service"
)

// 1x1 transparent GIF pixel (43 bytes)
var transparentGIF, _ = base64.StdEncoding.DecodeString(
	"R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
)

// TrackingHandler handles open and click tracking endpoints.
type TrackingHandler struct {
	trackingService service.TrackingService
}

// NewTrackingHandler creates a new TrackingHandler.
func NewTrackingHandler(trackingService service.TrackingService) *TrackingHandler {
	return &TrackingHandler{trackingService: trackingService}
}

// TrackOpen handles GET /t/o/:id — serves 1x1 transparent GIF and records open event.
func (h *TrackingHandler) TrackOpen(c *fiber.Ctx) error {
	idStr := c.Params("id")
	emailID, err := uuid.Parse(idStr)
	if err != nil {
		// Return pixel anyway to avoid breaking email display
		c.Set("Content-Type", "image/gif")
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		return c.Send(transparentGIF)
	}

	// Track open asynchronously (don't block pixel delivery)
	go func() {
		_ = h.trackingService.TrackOpen(c.Context(), emailID)
	}()

	c.Set("Content-Type", "image/gif")
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	return c.Send(transparentGIF)
}

// TrackClick handles GET /t/c/:id?url=... — redirects to original URL and records click event.
func (h *TrackingHandler) TrackClick(c *fiber.Ctx) error {
	idStr := c.Params("id")
	originalURL := c.Query("url")

	if originalURL == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing url parameter")
	}

	emailID, err := uuid.Parse(idStr)
	if err != nil {
		// Redirect anyway to avoid broken links
		return c.Redirect(originalURL, fiber.StatusFound)
	}

	// Track click asynchronously
	go func() {
		_ = h.trackingService.TrackClick(c.Context(), emailID)
	}()

	return c.Redirect(originalURL, fiber.StatusFound)
}
