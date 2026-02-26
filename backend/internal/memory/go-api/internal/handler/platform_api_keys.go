// Package handler — Platform API key management routes.
package handler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/models"
)

// --- API Key Handlers ---

func (h *PlatformHandler) CreateAPIKey(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	// Generate secure key
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to generate key"})
		return
	}
	key := "sk-uhms-" + base64.URLEncoding.EncodeToString(randomBytes)

	apiKey := &models.ApiKey{
		Key:    key,
		Name:   req.Name,
		UserID: userID,
	}

	db := getTenantDB(c)
	if err := db.Create(apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, apiKey)
}

func (h *PlatformHandler) ListAPIKeys(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)
	var keys []models.ApiKey
	if err := db.Where("user_id = ? AND is_active = ?", userID, true).Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to list API keys"})
		return
	}

	if keys == nil {
		keys = []models.ApiKey{}
	}
	c.JSON(http.StatusOK, keys)
}

func (h *PlatformHandler) RevokeAPIKey(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	keyID, err := uuid.Parse(c.Param("key_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid key ID"})
		return
	}

	db := getTenantDB(c)
	var key models.ApiKey
	if err := db.First(&key, "id = ?", keyID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "API key not found"})
		return
	}

	// Verify ownership (allow admin bypass)
	role, _ := c.Get("role")
	if key.UserID != userID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"detail": "无权删除此 API Key"})
		return
	}

	// Clear usage_logs references to avoid FK violation
	db.Model(&models.UsageLog{}).Where("api_key_id = ?", keyID).
		Update("api_key_id", nil)

	if err := db.Delete(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to revoke key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

func (h *PlatformHandler) ValidateAPIKey(c *gin.Context) {
	apiKeyStr := c.GetHeader("X-API-Key")
	if apiKeyStr == "" || len(apiKeyStr) < 8 || apiKeyStr[:8] != "sk-uhms-" {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid API key format"})
		return
	}

	db := getTenantDB(c)
	var key models.ApiKey
	if err := db.First(&key, "key = ?", apiKeyStr).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid or revoked API key"})
		return
	}

	if !key.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "API key has been revoked"})
		return
	}

	// Update last_used_at
	now := time.Now().UTC()
	db.Model(&key).Update("last_used_at", now)

	// Get balance
	var account models.BillingAccount
	balance := 0.0
	currency := "USD"
	if err := db.First(&account, "user_id = ?", key.UserID).Error; err == nil {
		bal, _ := account.Balance.Float64()
		balance = bal
		currency = account.Currency
	}

	// Balance warnings
	var warning *string
	if balance <= 0 {
		w := "余额不足，部分功能将受限。请及时充值。"
		warning = &w
	} else if balance < 1.0 {
		w := "余额较低，建议尽快充值。"
		warning = &w
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":           true,
		"key_name":        key.Name,
		"user_id":         key.UserID,
		"is_active":       key.IsActive,
		"balance":         balance,
		"currency":        currency,
		"balance_warning": warning,
		"created_at":      key.CreatedAt,
		"last_used_at":    key.LastUsedAt,
	})
}
