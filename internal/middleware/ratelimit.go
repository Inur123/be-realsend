package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/realsend/be-realsend/internal/utils"
)

// RateLimiter returns a Redis-based rate limiting middleware.
// For now, this is a simple IP-based rate limiter for general routes.
// A client can make up to `limit` requests per `window` duration.
func RateLimiter(redisClient *redis.Client, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Bypass for testing if redis is nil
		if redisClient == nil {
			return c.Next()
		}

		ip := c.IP()
		key := fmt.Sprintf("rl:ip:%s:%d", ip, time.Now().Unix() / int64(window.Seconds()))

		ctx := context.Background()
		count, err := redisClient.Incr(ctx, key).Result()
		if err != nil {
			// Fail open on Redis error so we don't block users if Redis goes down, but log it
			fmt.Printf("Rate limit Redis error: %v\n", err)
			return c.Next()
		}

		if count == 1 {
			redisClient.Expire(ctx, key, window * 2)
		}

		if count > int64(limit) {
			return utils.TooManyRequests(c, "too many requests, please try again later")
		}

		return c.Next()
	}
}
