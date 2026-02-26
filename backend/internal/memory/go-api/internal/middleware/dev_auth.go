// Package middleware — dev auth bypass.
// Mirrors Python's ALLOW_DEV_AUTH_BYPASS functionality.
// Parses "Authorization: Bearer dev_{user_id}_{role}" tokens for local development.
package middleware

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/config"
)

// DevAuthBypass returns a Gin middleware that parses dev tokens
// when ALLOW_DEV_AUTH_BYPASS=true.
//
// Token format: "Bearer dev_{user_id}_{role}"
// Example: "Bearer dev_user_admin_001_admin"
//
// This middleware runs BEFORE billing. It sets:
//   - "user_id" in context
//   - "role" in context
func DevAuthBypass() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		if !cfg.AllowDevAuthBypass {
			c.Next()
			return
		}

		// Already authenticated by a prior middleware
		if _, exists := c.Get("user_id"); exists {
			c.Next()
			return
		}

		// Track 1: "Authorization: Bearer dev_{user_id}_{role}"
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer dev_") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			parts := strings.TrimPrefix(token, "dev_")
			lastUnderscore := strings.LastIndex(parts, "_")
			if lastUnderscore > 0 {
				userID := parts[:lastUnderscore]
				role := parts[lastUnderscore+1:]
				c.Set("user_id", userID)
				c.Set("role", role)
				c.Set("dev_auth", true)
				slog.Debug("Dev auth bypass (Bearer)", "user_id", userID, "role", role)
				c.Next()
				return
			}
		}

		// Track 2: "X-User-Session: {\"user_id\":\"...\",\"role\":\"...\"}"
		sessionHeader := c.GetHeader("X-User-Session")
		if sessionHeader != "" {
			var session struct {
				UserID string `json:"user_id"`
				Role   string `json:"role"`
				Email  string `json:"email"`
			}
			if err := json.Unmarshal([]byte(sessionHeader), &session); err == nil && session.UserID != "" {
				c.Set("user_id", session.UserID)
				if session.Role != "" {
					c.Set("role", session.Role)
				}
				c.Set("dev_auth", true)
				slog.Debug("Dev auth bypass (X-User-Session)", "user_id", session.UserID, "role", session.Role)
				c.Next()
				return
			}
		}

		// Track 3: query param "user_id" — for SSE/EventSource which cannot send HTTP headers.
		// Only enabled in dev mode, safe because AllowDevAuthBypass is already checked above.
		if uid := c.Query("user_id"); uid != "" {
			c.Set("user_id", uid)
			c.Set("role", "user")
			c.Set("dev_auth", true)
			slog.Debug("Dev auth bypass (query param)", "user_id", uid)
			c.Next()
			return
		}

		c.Next()
	}
}
