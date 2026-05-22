package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/realsend/be-realsend/internal/config"
)

// CORS returns the CORS configurations for the API server.
func CORS(cfg *config.Config) fiber.Handler {
	origins := strings.Split(cfg.CORSOrigins, ",")
	originsMap := make(map[string]bool)
	for _, o := range origins {
		originsMap[strings.TrimSpace(o)] = true
	}

	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		
		// If origin is allowed or in development mode, set CORS headers
		if originsMap[origin] || cfg.IsDevelopment() {
			c.Set("Access-Control-Allow-Origin", origin)
		} else if cfg.CORSOrigins == "*" {
			c.Set("Access-Control-Allow-Origin", "*")
		}

		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Max-Age", "86400")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}
