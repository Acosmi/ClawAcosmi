// Package handler — Platform billing, dashboard, and log routes.
package handler

import (
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// --- Billing Handlers ---

func (h *PlatformHandler) GetBillingAccount(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)
	var account models.BillingAccount
	if err := db.First(&account, "user_id = ?", userID).Error; err != nil {
		// Create default account
		account = models.BillingAccount{
			UserID:   userID,
			Balance:  decimal.Zero,
			Currency: "USD",
		}
		db.Create(&account)
	}

	c.JSON(http.StatusOK, account)
}

func (h *PlatformHandler) RechargeAccount(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	var req RechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	db := getTenantDB(c)
	// Get or create account
	var account models.BillingAccount
	if err := db.First(&account, "user_id = ?", userID).Error; err != nil {
		account = models.BillingAccount{
			UserID:   userID,
			Balance:  decimal.Zero,
			Currency: "USD",
		}
		db.Create(&account)
	}

	// Update balance
	amount := decimal.NewFromFloat(req.Amount)
	db.Model(&account).Update("balance", gorm.Expr("balance + ?", amount))

	// Create transaction
	description := req.Description
	if description == "" {
		description = "Account recharge"
	}
	tx := models.Transaction{
		UserID:          userID,
		Amount:          amount,
		TransactionType: "recharge",
		Description:     &description,
	}
	db.Create(&tx)

	c.JSON(http.StatusOK, tx)
}

func (h *PlatformHandler) ListTransactions(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	limit := parseIntParam(c, "limit", 20, 1, 100)
	offset := parseIntParam(c, "offset", 0, 0, 10000)

	db := getTenantDB(c)
	var transactions []models.Transaction
	db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&transactions)

	if transactions == nil {
		transactions = []models.Transaction{}
	}
	c.JSON(http.StatusOK, transactions)
}

// --- Dashboard Handlers ---

func (h *PlatformHandler) GetDashboardStats(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)

	// Total requests
	var totalRequests int64
	db.Model(&models.UsageLog{}).Where("user_id = ?", userID).Count(&totalRequests)

	// Average latency
	var avgLatency float64
	db.Model(&models.UsageLog{}).Where("user_id = ?", userID).
		Select("COALESCE(AVG(latency_ms), 0)").Scan(&avgLatency)

	// Error count
	var errorCount int64
	db.Model(&models.UsageLog{}).
		Where("user_id = ? AND status_code >= 400", userID).
		Count(&errorCount)

	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(errorCount) / float64(totalRequests) * 100
	}

	// Active keys
	var activeKeys int64
	db.Model(&models.ApiKey{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&activeKeys)

	// Chart data (last 7 days)
	sevenDaysAgo := time.Now().UTC().AddDate(0, 0, -7)
	type ChartRow struct {
		Date  time.Time `json:"date"`
		Count int64     `json:"count"`
	}
	var chartData []ChartRow
	db.Model(&models.UsageLog{}).
		Select("DATE(timestamp) as date, COUNT(*) as count"). // BUG-V8-12: DATE() 兼容 SQLite/PostgreSQL
		Where("user_id = ? AND timestamp >= ?", userID, sevenDaysAgo).
		Group("date").Order("date").
		Scan(&chartData)

	// Format chart data
	chartItems := make([]map[string]any, len(chartData))
	for i, row := range chartData {
		chartItems[i] = map[string]any{
			"date":     row.Date.Format("2006-01-02"),
			"requests": row.Count,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_requests": totalRequests,
		"avg_latency":    math.Round(avgLatency*100) / 100,
		"error_rate":     math.Round(errorRate*100) / 100,
		"active_keys":    activeKeys,
		"chart_data":     chartItems,
	})
}

// --- Log Handlers ---

func (h *PlatformHandler) GetUsageLogs(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	limit := parseIntParam(c, "limit", 50, 1, 200)
	offset := parseIntParam(c, "offset", 0, 0, 10000)

	db := getTenantDB(c)
	var logs []models.UsageLog
	db.Where("user_id = ?", userID).
		Order("timestamp DESC").
		Limit(limit).Offset(offset).
		Find(&logs)

	if logs == nil {
		logs = []models.UsageLog{}
	}
	c.JSON(http.StatusOK, logs)
}

func (h *PlatformHandler) GetSystemLogs(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	limit := parseIntParam(c, "limit", 50, 1, 200)
	offset := parseIntParam(c, "offset", 0, 0, 10000)
	level := c.Query("level")

	db := getTenantDB(c)
	query := db.Where("user_id = ?", userID)
	if level == "error" {
		query = query.Where("status_code >= 400")
	} else if level == "success" {
		query = query.Where("status_code < 400")
	}

	var logs []models.UsageLog
	query.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&logs)

	if logs == nil {
		logs = []models.UsageLog{}
	}
	c.JSON(http.StatusOK, logs)
}
