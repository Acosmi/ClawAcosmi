// Package handler — Platform notification and documentation routes.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/uhms/go-api/internal/models"
)

// --- Notification Handlers ---

func (h *PlatformHandler) GetNotificationHistory(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	limit := parseIntParam(c, "limit", 50, 1, 200)

	db := getTenantDB(c)
	var logs []models.NotificationLog
	db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs)

	if logs == nil {
		logs = []models.NotificationLog{}
	}
	c.JSON(http.StatusOK, logs)
}

func (h *PlatformHandler) GetNotificationPreferences(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)
	var prefs models.NotificationPreference
	if err := db.First(&prefs, "user_id = ?", userID).Error; err != nil {
		// Return default preferences
		prefs = models.NotificationPreference{
			UserID:           userID,
			EmailEnabled:     false,
			SMSEnabled:       false,
			BalanceThreshold: decimal.NewFromFloat(1.0),
		}
		db.Create(&prefs)
	}
	c.JSON(http.StatusOK, prefs)
}

func (h *PlatformHandler) UpdateNotificationPreferences(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	db := getTenantDB(c)
	// Get or create
	var prefs models.NotificationPreference
	if err := db.First(&prefs, "user_id = ?", userID).Error; err != nil {
		prefs = models.NotificationPreference{
			UserID:           userID,
			EmailEnabled:     false,
			SMSEnabled:       false,
			BalanceThreshold: decimal.NewFromFloat(1.0),
		}
		db.Create(&prefs)
	}

	// Apply updates
	db.Model(&prefs).Updates(updates)

	// Re-read
	db.First(&prefs, "user_id = ?", userID)
	c.JSON(http.StatusOK, prefs)
}

// ===========================================================================
// Documentation Content (public-facing developer docs)
// ===========================================================================

func (h *PlatformHandler) GetDocsContent(c *gin.Context) {
	db := getTenantDB(c)
	var docs []models.DocEndpoint
	db.Where("is_public = ?", true).
		Order("category ASC, sort_order ASC").
		Find(&docs)

	// Return as map keyed by ID, matching frontend DocsContentResponse
	endpoints := make(map[string]models.DocEndpoint, len(docs))
	for _, d := range docs {
		endpoints[d.ID] = d
	}

	c.JSON(http.StatusOK, gin.H{"endpoints": endpoints})
}
