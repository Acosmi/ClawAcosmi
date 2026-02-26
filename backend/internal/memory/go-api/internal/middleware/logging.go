// Package middleware provides Gin middleware for logging and billing.
// logging.go mirrors Python middleware/logging.py.
package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/models"
)

// internalPrefixes lists endpoints that should NOT be logged to UsageLog.
// These are internal management, platform UI, and long-lived connections.
// Only external API calls (memories, graph, search) should be metered.
var internalPrefixes = []string{
	"/api/v1/platform",
	"/api/v1/auth",
	"/api/v1/admin",
	"/api/v1/events/stream",
	"/api/v1/db-config",
	"/api/v1/memory-config",
	"/api/v1/oauth",
	"/api/v1/mcp",
}

// isInternalEndpoint checks if the endpoint is an internal/management route
// that should be excluded from usage logging.
func isInternalEndpoint(path string) bool {
	for _, prefix := range internalPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// APILogging returns a Gin middleware that logs external /api/v1/* requests
// to the usage_logs database table.
//
// Features:
// - Records endpoint, method, status code, latency
// - Background DB write via goroutine (non-blocking)
// - Safe handling of SSE/streaming responses
// - Extracts user_id from context (set by auth middleware)
// - Excludes internal/management endpoints (platform, auth, admin, SSE, etc.)
func APILogging() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()
		path := c.Request.URL.Path

		// Only log API requests
		if !strings.HasPrefix(path, cfg.APIPrefix) {
			c.Next()
			return
		}

		// Skip internal/management endpoints — these are not billable
		if isInternalEndpoint(path) {
			c.Next()
			return
		}

		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latencyMS := int(time.Since(start).Milliseconds())

		// Extract user info from context (set by auth middleware)
		userID := extractUserID(c)
		apiKeyID := extractAPIKeyID(c)

		// Extract error if any
		var errMsg *string
		if len(c.Errors) > 0 {
			msg := c.Errors.String()
			errMsg = &msg
		}

		// Background DB write — non-blocking
		go writeUsageLog(
			userID,
			path,
			c.Request.Method,
			c.Writer.Status(),
			latencyMS,
			apiKeyID,
			errMsg,
		)
	}
}

// writeUsageLog writes an API usage log entry to the database.
// Runs in a goroutine — errors are logged but do not affect the response.
func writeUsageLog(
	userID, endpoint, method string,
	statusCode, latencyMS int,
	apiKeyID *uuid.UUID,
	errorMessage *string,
) {
	db, err := database.GetDB()
	if err != nil {
		slog.Error("Failed to get DB for usage log", "error", err)
		return
	}

	log := models.UsageLog{
		UserID:       userID,
		Endpoint:     endpoint,
		Method:       method,
		StatusCode:   statusCode,
		LatencyMS:    latencyMS,
		ApiKeyID:     apiKeyID,
		ErrorMessage: errorMessage,
	}

	if result := db.Create(&log); result.Error != nil {
		slog.Error("Failed to write usage log", "error", result.Error)
	} else {
		slog.Debug("Usage log recorded", "method", method, "endpoint", endpoint, "status", statusCode)
	}
}

// --- Helper extraction functions ---

func extractUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok && id != "" {
			return id
		}
	}
	return "anonymous"
}

func extractAPIKeyID(c *gin.Context) *uuid.UUID {
	if keyID, exists := c.Get("api_key_id"); exists {
		if id, ok := keyID.(*uuid.UUID); ok {
			return id
		}
	}
	return nil
}

// isStreamingResponse detects SSE/streaming responses.
func isStreamingResponse(c *gin.Context) bool {
	ct := c.Writer.Header().Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return true
	}
	if strings.Contains(c.Request.URL.Path, "/stream") {
		return true
	}
	return false
}
