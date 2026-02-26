// Package middleware — Rate limiting middleware.
// Uses ratelimit.Limiter (standalone package, no import cycle) for
// per-API-key Token Bucket limiting backed by Redis.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/cache"
	"github.com/uhms/go-api/internal/ratelimit"
)

var defaultLimiter *ratelimit.Limiter

// RateLimit returns a Gin middleware that enforces per-key rate limiting.
// Limits: 120 requests/minute per API key; fail-open if Redis is unavailable.
func RateLimit() gin.HandlerFunc {
	if defaultLimiter == nil {
		// fail-open = true: if Redis is down, allow requests rather than blocking
		defaultLimiter = ratelimit.New(cache.GetClient(), true)
	}

	return func(c *gin.Context) {
		// Only limit authenticated API key requests (user_id set by Billing middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		key := "rate:user:" + userID.(string)
		result := defaultLimiter.Check(c.Request.Context(), key, 120, 60)

		// Set standard rate limit headers on every response
		for k, v := range result.Headers() {
			c.Header(k, v)
		}

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"detail": "Rate limit exceeded. Please slow down.",
				"error":  "too_many_requests",
			})
			return
		}

		c.Next()
	}
}
