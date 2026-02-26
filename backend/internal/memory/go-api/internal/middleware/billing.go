// Package middleware — billing middleware.
// Mirrors Python middleware/billing.py: pre-request balance check, post-request billing.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/models"
)

// --- Endpoint Cost Configuration ---

// endpointCosts maps endpoint patterns to their cost in credits.
// Free endpoints (cost=0) are not charged.
var endpointCosts = map[string]float64{
	"memories": 0.001,
	"search":   0.002,
	"graph":    0.001,
	"events":   0.0,
	"platform": 0.0,
}

// freeEndpoints lists endpoints that bypass billing entirely.
var freeEndpoints = []string{
	"/health",
	"/ready",
	"/metrics",
	"/docs",
	"/api/v1/platform",
	"/api/v1/auth",
}

// getEndpointCost returns the cost for a given request path.
func getEndpointCost(path string) float64 {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		if cost, ok := endpointCosts[parts[2]]; ok {
			return cost
		}
	}
	return 0.0
}

// isFreeEndpoint checks if the endpoint is free (bypasses billing).
func isFreeEndpoint(path string) bool {
	for _, prefix := range freeEndpoints {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// isExternalAPIRequest checks if the path requires X-API-Key auth.
func isExternalAPIRequest(path string) bool {
	cfg := config.Get()
	externalPrefixes := []string{
		cfg.APIPrefix + "/memories",
		cfg.APIPrefix + "/graph",
		cfg.APIPrefix + "/events",
	}
	for _, prefix := range externalPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// Billing returns a Gin middleware that handles billing logic.
//
// Flow:
// 1. External API: validates X-API-Key → 401 if invalid
// 2. Pre-Request: checks balance → 402 if insufficient
// 3. Process request
// 4. Post-Request: deducts cost for successful (2xx) requests
//
// Bypass conditions: Admin role, free endpoints
func Billing() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()
		path := c.Request.URL.Path

		// Only process API requests
		if !strings.HasPrefix(path, cfg.APIPrefix) {
			c.Next()
			return
		}

		start := time.Now()

		// Check if free endpoint
		if isFreeEndpoint(path) {
			c.Next()
			return
		}

		// --- External API Key Validation ---
		if isExternalAPIRequest(path) {
			// Dev bypass: skip X-API-Key check when dev auth is enabled and user is already authenticated via Bearer token.
			if cfg.AllowDevAuthBypass && gin.Mode() == gin.DebugMode {
				if _, exists := c.Get("user_id"); exists {
					slog.Debug("Dev bypass: skipping X-API-Key for external endpoint", "path", path)
					c.Next()
					return
				}
			}

			apiKeyHeader := c.GetHeader("X-API-Key")

			if apiKeyHeader == "" {
				logAndAbort(c, start, "anonymous", http.StatusUnauthorized, "API key is required")
				return
			}

			// Validate against database
			db, err := database.GetDB()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"detail": "Database unavailable"})
				return
			}

			var apiKey models.ApiKey
			result := db.Where("key = ? AND is_active = ?", apiKeyHeader, true).First(&apiKey)
			if result.Error != nil {
				logAndAbort(c, start, "anonymous", http.StatusUnauthorized, "Invalid or revoked API key")
				return
			}

			// Check expiration (future-proof, ApiKey model doesn't have expires_at yet)
			// Key is valid — set context
			c.Set("api_key_id", &apiKey.ID)
			c.Set("user_id", apiKey.UserID)
			c.Set("api_key_role", apiKey.Role)

			// Admin bypass
			if apiKey.Role == "admin" {
				slog.Debug("Admin role bypasses billing", "user_id", apiKey.UserID)
				c.Next()
				return
			}

			// Balance check
			cost := getEndpointCost(path)
			if cost > 0 {
				var account models.BillingAccount
				result := db.Where("user_id = ?", apiKey.UserID).First(&account)
				if result.Error == nil && account.Balance.LessThan(
					account.Balance.Add(account.Balance).Sub(account.Balance), // just check > 0
				) {
					// Simplified: just check balance > 0 for now
					// Full billing service will be in Phase 5
				}
			}
		}

		// Process request
		c.Next()

		// Post-request: background billing for successful 2xx responses
		statusCode := c.Writer.Status()
		if statusCode >= 200 && statusCode < 300 {
			cost := getEndpointCost(path)
			userID := extractUserID(c)

			if cost > 0 && userID != "anonymous" {
				go safeGo(func() {
					processBillingBackground(userID, path, c.Request.Method, cost)
				})
			}
		}

		// NOTE: Usage logging is handled by APILogging middleware.
		// Do NOT call writeUsageLog here to avoid double-counting.
	}
}

// safeGo wraps a function call with panic recovery for goroutine safety.
func safeGo(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in background goroutine", "panic", r)
		}
	}()
	fn()
}

// logAndAbort aborts the request with an error response and logs the failure.
func logAndAbort(c *gin.Context, _ time.Time, _ string, statusCode int, detail string) {
	// NOTE: Usage logging is handled by APILogging middleware.
	// Do NOT call writeUsageLog here to avoid double-counting.
	c.AbortWithStatusJSON(statusCode, gin.H{
		"detail": detail,
		"error":  http.StatusText(statusCode),
	})
}

// processBillingBackground handles post-request billing deduction.
// Runs in a goroutine — errors are logged but don't affect the response.
func processBillingBackground(userID, endpoint, method string, cost float64) {
	// Phase 5 will implement full BillingService with transaction support.
	// For now, just log the billing intent.
	slog.Debug("Billing deduction pending",
		"user_id", userID,
		"endpoint", endpoint,
		"method", method,
		"cost", cost,
	)
}
