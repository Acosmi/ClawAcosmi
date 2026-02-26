// Package algo — API Key middleware for algorithm endpoints.
// Ensures only authorized local-proxy clients can call /algo/* endpoints.
package algo

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyConfig holds configuration for the API key middleware.
type APIKeyConfig struct {
	// Keys is the set of valid API keys. If empty, middleware is disabled (dev mode).
	Keys []string
	// Header is the HTTP header to check (default: "X-API-Key").
	Header string
}

// APIKeyAuth creates a Gin middleware that validates API keys for algo endpoints.
// In production, this protects cloud algo APIs so only authorized local-proxy instances can call them.
// If no keys are configured, the middleware is permissive (development mode).
func APIKeyAuth(cfg APIKeyConfig) gin.HandlerFunc {
	if cfg.Header == "" {
		cfg.Header = "X-API-Key"
	}

	// Build a set for O(1) lookup
	validKeys := make(map[string]bool, len(cfg.Keys))
	for _, k := range cfg.Keys {
		if k != "" {
			validKeys[k] = true
		}
	}

	return func(c *gin.Context) {
		// If no keys configured, skip validation (dev mode)
		if len(validKeys) == 0 {
			c.Next()
			return
		}

		// Check X-API-Key header
		apiKey := c.GetHeader(cfg.Header)

		// Also check Authorization: Bearer <key>
		if apiKey == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key. Provide via X-API-Key header or Authorization: Bearer <key>.",
			})
			return
		}

		// Constant-time comparison to prevent timing attacks
		valid := false
		for key := range validKeys {
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
				valid = true
				break
			}
		}

		if !valid {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Invalid API key.",
			})
			return
		}

		c.Next()
	}
}
