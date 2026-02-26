// Package handler — Admin user management routes.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/uhms/go-api/internal/models"
)

// ===========================================================================
// User Management
// ===========================================================================

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page := parseIntParam(c, "page", 1, 1, 1000)
	pageSize := parseIntParam(c, "page_size", 20, 1, 100)
	offset := (page - 1) * pageSize

	// Get distinct user_ids
	var userIDs []string
	getTenantDB(c).Model(&models.ApiKey{}).
		Distinct("user_id").
		Order("user_id ASC").
		Offset(offset).Limit(pageSize).
		Pluck("user_id", &userIDs)

	// Total users
	var total int64
	getTenantDB(c).Model(&models.ApiKey{}).Distinct("user_id").Count(&total)

	// Build user summaries
	users := make([]gin.H, 0, len(userIDs))
	for _, uid := range userIDs {
		var keyCount int64
		getTenantDB(c).Model(&models.ApiKey{}).Where("user_id = ?", uid).Count(&keyCount)

		var activeKeys int64
		getTenantDB(c).Model(&models.ApiKey{}).Where("user_id = ? AND is_active = ?", uid, true).Count(&activeKeys)

		isBanned := keyCount > 0 && activeKeys == 0

		var balance decimal.Decimal
		getTenantDB(c).Model(&models.BillingAccount{}).Where("user_id = ?", uid).
			Select("COALESCE(balance, 0)").Scan(&balance)

		var requestCount int64
		getTenantDB(c).Model(&models.UsageLog{}).Where("user_id = ?", uid).Count(&requestCount)

		bal, _ := balance.Float64()
		users = append(users, gin.H{
			"user_id":        uid,
			"api_key_count":  keyCount,
			"balance":        bal,
			"total_requests": requestCount,
			"is_banned":      isBanned,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"users":     users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	userID := c.Param("user_id")

	var count int64
	getTenantDB(c).Model(&models.ApiKey{}).Where("user_id = ?", userID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
		return
	}

	result := getTenantDB(c).Model(&models.ApiKey{}).Where("user_id = ?", userID).
		Update("is_active", false)

	c.JSON(http.StatusOK, gin.H{
		"user_id":             userID,
		"is_banned":           true,
		"disabled_keys_count": result.RowsAffected,
		"message":             "用户已禁用",
	})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	userID := c.Param("user_id")

	var count int64
	getTenantDB(c).Model(&models.ApiKey{}).Where("user_id = ?", userID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
		return
	}

	result := getTenantDB(c).Model(&models.ApiKey{}).Where("user_id = ?", userID).
		Update("is_active", true)

	c.JSON(http.StatusOK, gin.H{
		"user_id":             userID,
		"is_banned":           false,
		"disabled_keys_count": result.RowsAffected,
		"message":             "用户已解禁",
	})
}
