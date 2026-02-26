// Package handler — Platform management routes.
// Mirrors Python api/routes/platform/ — API keys, billing, dashboard, usage logs.
// All routes use JWT-based auth (current user from token).
//
// Handler implementations are split across domain-specific files:
//   - platform_api_keys.go       — API key CRUD and validation
//   - platform_billing.go        — billing, dashboard stats, usage/system logs
//   - platform_notifications.go  — notifications, preferences, developer docs
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// PlatformHandler handles platform management routes.
type PlatformHandler struct{}

// NewPlatformHandler creates a new PlatformHandler.
// DB is obtained dynamically from Gin context via TenantDB middleware.
func NewPlatformHandler() *PlatformHandler {
	return &PlatformHandler{}
}

// RegisterRoutes registers platform routes.
func (h *PlatformHandler) RegisterRoutes(rg *gin.RouterGroup) {
	platform := rg.Group("/platform")
	{
		// API Keys
		platform.POST("/api-keys", h.CreateAPIKey)
		platform.GET("/api-keys", h.ListAPIKeys)
		platform.DELETE("/api-keys/:key_id", h.RevokeAPIKey)
		platform.POST("/api-keys/validate", h.ValidateAPIKey)

		// Billing
		platform.GET("/billing/account", h.GetBillingAccount)
		platform.POST("/billing/recharge", h.RechargeAccount)
		platform.GET("/billing/transactions", h.ListTransactions)

		// Dashboard
		platform.GET("/dashboard/stats", h.GetDashboardStats)

		// Logs
		platform.GET("/usage-logs", h.GetUsageLogs)
		platform.GET("/system-logs", h.GetSystemLogs)

		// Notifications
		platform.GET("/notifications/history", h.GetNotificationHistory)
		platform.GET("/notifications/preferences", h.GetNotificationPreferences)
		platform.PATCH("/notifications/preferences", h.UpdateNotificationPreferences)

		// Documentation
		platform.GET("/docs/content", h.GetDocsContent)
	}
}

// --- Request types ---

// CreateAPIKeyRequest mirrors ApiKeyCreate schema.
type CreateAPIKeyRequest struct {
	Name           string   `json:"name" binding:"required"`
	AllowedIPs     []string `json:"allowed_ips,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// RechargeRequest mirrors TransactionCreate schema.
type RechargeRequest struct {
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Description string  `json:"description,omitempty"`
}

// --- Helpers ---

func parseIntParam(c *gin.Context, name string, def, min, max int) int {
	if v := c.Query(name); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed < min {
				return min
			}
			if parsed > max {
				return max
			}
			return parsed
		}
	}
	return def
}
