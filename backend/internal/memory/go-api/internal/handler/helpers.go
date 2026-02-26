// Package handler — 共享辅助函数。
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/middleware"
)

// getTenantDB 从 Gin context 获取当前请求的租户数据库连接。
// 由 middleware.TenantDB 中间件注入。
func getTenantDB(c *gin.Context) *gorm.DB {
	if db, exists := c.Get(middleware.ContextKeyTenantDB); exists {
		if gormDB, ok := db.(*gorm.DB); ok {
			return gormDB
		}
	}
	slog.Error("getTenantDB: tenant_db not found in context, this should not happen")
	return nil
}

// getUserID extracts user_id using a consistent priority:
//  1. Gin context "user_id" — set by DevAuthBypass / OAuth middleware
//  2. Query parameter "user_id" — backward compatibility / external API
//  3. Returns "" and sends 401 if neither is available
//
// All handlers that need user_id should call this instead of c.Query("user_id").
func getUserID(c *gin.Context) string {
	// From auth middleware (DevAuthBypass / OAuth / Billing)
	if uid, exists := c.Get("user_id"); exists {
		if s, ok := uid.(string); ok && s != "" {
			return s
		}
	}
	// Backward-compatible fallback: query param
	if uid := c.Query("user_id"); uid != "" {
		return uid
	}
	c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
	return ""
}
