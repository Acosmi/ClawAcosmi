// Package handler — Admin financial statistics routes.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/models"
)

// ===========================================================================
// Financial Stats
// ===========================================================================

func (h *AdminHandler) GetFinancialStats(c *gin.Context) {
	days := parseIntParam(c, "days", 30, 1, 90)
	startDate := time.Now().UTC().AddDate(0, 0, -days)

	// Total revenue (deductions as positive)
	var totalRevenue float64
	getTenantDB(c).Model(&models.Transaction{}).
		Where("created_at >= ? AND transaction_type = ?", startDate, "deduction").
		Select("COALESCE(SUM(ABS(amount)), 0)").Scan(&totalRevenue)

	// Total transactions
	var totalTransactions int64
	getTenantDB(c).Model(&models.Transaction{}).Where("created_at >= ?", startDate).Count(&totalTransactions)

	// Total users
	var totalUsers int64
	getTenantDB(c).Model(&models.ApiKey{}).Distinct("user_id").Count(&totalUsers)

	// Average balance
	var avgBalance float64
	getTenantDB(c).Model(&models.BillingAccount{}).Select("COALESCE(AVG(balance), 0)").Scan(&avgBalance)

	// Daily revenue
	type DailyRow struct {
		Date    time.Time `json:"date"`
		Revenue float64   `json:"revenue"`
		Count   int64     `json:"count"`
	}
	var dailyData []DailyRow
	getTenantDB(c).Model(&models.Transaction{}).
		Select("DATE(created_at) as date, COALESCE(SUM(CASE WHEN transaction_type='deduction' THEN ABS(amount) ELSE 0 END), 0) as revenue, COUNT(*) as count"). // BUG-V8-12: DATE() 兼容 SQLite
		Where("created_at >= ?", startDate).
		Group("date").Order("date").
		Scan(&dailyData)

	dailyRevenue := make([]gin.H, len(dailyData))
	for i, d := range dailyData {
		dailyRevenue[i] = gin.H{
			"date":              d.Date.Format("2006-01-02"),
			"revenue":           d.Revenue,
			"transaction_count": d.Count,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_revenue":      totalRevenue,
		"total_transactions": totalTransactions,
		"total_users":        totalUsers,
		"average_balance":    avgBalance,
		"daily_revenue":      dailyRevenue,
		"period_days":        days,
	})
}
