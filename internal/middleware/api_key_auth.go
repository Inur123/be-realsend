package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/realsend/be-realsend/internal/repository"
	"github.com/realsend/be-realsend/internal/utils"
)

// APIKeyAuth is a middleware that validates incoming transactional sending requests via API key.
func APIKeyAuth(keyRepo repository.APIKeyRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var rawKey string

		// 1. Check standard Authorization header
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				rawKey = parts[1]
			}
		}

		// 2. Fallback to custom X-API-Key header
		if rawKey == "" {
			rawKey = c.Get("X-API-Key")
		}

		if rawKey == "" {
			return utils.Unauthorized(c, "missing api key. Provide via 'Authorization: Bearer <key>' or 'X-API-Key' header")
		}

		// RealSend API Keys must start with rs_live_
		if !strings.HasPrefix(rawKey, "rs_live_") {
			return utils.Unauthorized(c, "invalid api key format. Must start with 'rs_live_'")
		}

		// 3. Hash raw key via SHA-256
		hashBytes := sha256.Sum256([]byte(rawKey))
		hashHex := hex.EncodeToString(hashBytes[:])

		// 4. Retrieve key metadata from repository
		key, err := keyRepo.GetByHash(c.Context(), hashHex)
		if err != nil {
			return utils.InternalError(c, "failed to verify api key")
		}

		if key == nil || !key.IsActive {
			return utils.Unauthorized(c, "invalid, expired, or revoked api key")
		}

		// 5. Inject key properties into context locals
		c.Locals("api_key_id", key.ID.String())
		c.Locals("user_id", key.UserID.String())
		c.Locals("api_key_scopes", key.Scopes)
		
		if key.DomainID.Valid {
			c.Locals("api_key_domain_id", key.DomainID.UUID.String())
		} else {
			c.Locals("api_key_domain_id", "")
		}

		return c.Next()
	}
}
