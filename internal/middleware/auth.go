package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/realsend/be-realsend/internal/config"
	"github.com/realsend/be-realsend/internal/utils"
)

// Protected handles JWT authentication and validation.
func Protected(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return utils.Unauthorized(c, "missing authorization header")
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return utils.Unauthorized(c, "invalid authorization header format")
		}

		tokenString := parts[1]
		claims, err := utils.ValidateToken(cfg.JWTSecret, tokenString)
		if err != nil {
			return utils.Unauthorized(c, "invalid or expired token")
		}

		// Inject user_id and role into context
		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// RequireRole restricts access to users with specific roles.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("role").(string)
		if !ok {
			return utils.Forbidden(c, "access denied")
		}

		for _, role := range roles {
			if userRole == role {
				return c.Next()
			}
		}

		return utils.Forbidden(c, "insufficient permissions")
	}
}
