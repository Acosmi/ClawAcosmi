// Package handler — 租户数据库配置管理 API。
// 允许租户通过开放平台 API 配置自己的数据库连接。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/services"
)

// DBConfigHandler 处理租户数据库配置相关路由。
type DBConfigHandler struct {
	router *database.TenantDBRouter
}

// NewDBConfigHandler creates a new DBConfigHandler.
func NewDBConfigHandler(router *database.TenantDBRouter) *DBConfigHandler {
	return &DBConfigHandler{router: router}
}

// RegisterRoutes 注册数据库配置路由。
// P2-2: 仅 admin/owner 角色可操作数据库配置。
func (h *DBConfigHandler) RegisterRoutes(rg *gin.RouterGroup) {
	dbConfig := rg.Group("/platform/db-config")
	dbConfig.Use(middleware.RequireRole("admin", "owner"))
	{
		dbConfig.POST("", h.SetDBConfig)
		dbConfig.GET("", h.GetDBConfig)
		dbConfig.DELETE("", h.DeleteDBConfig)
		dbConfig.POST("/test", h.TestDBConnection)
		dbConfig.GET("/schema-status", h.GetSchemaStatus) // P3-2
	}
}

// --- Request types ---

// SetDBConfigRequest 设置数据库配置请求。
type SetDBConfigRequest struct {
	DBType   string `json:"db_type" binding:"required,oneof=postgres mysql sqlite"`
	DSN      string `json:"dsn" binding:"required"` // 明文 DSN，存储时加密
	MaxConns int    `json:"max_conns,omitempty"`    // 默认 10
}

// TestDBConnectionRequest 测试数据库连接请求。
type TestDBConnectionRequest struct {
	DBType string `json:"db_type" binding:"required,oneof=postgres mysql sqlite"`
	DSN    string `json:"dsn" binding:"required"`
}

// --- Handlers ---

// SetDBConfig 设置或更新租户的数据库配置。
// POST /platform/db-config
func (h *DBConfigHandler) SetDBConfig(c *gin.Context) {
	tenantID := getUserID(c)
	if tenantID == "" {
		return
	}

	var req SetDBConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request: " + err.Error()})
		return
	}

	// 加密 DSN — 无密钥时拒绝保存（P2-1 安全加固）
	encryptedDSN, err := services.EncryptConfigValue(req.DSN)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"detail": "CONFIG_ENCRYPTION_KEY not configured, cannot save DSN securely",
		})
		return
	}

	maxConns := req.MaxConns
	if maxConns <= 0 {
		maxConns = 10
	}

	config := database.TenantDBConfig{
		TenantID: tenantID,
		DBType:   req.DBType,
		DSN:      encryptedDSN,
		MaxConns: maxConns,
		Enabled:  true,
	}

	db := getTenantDB(c)
	// Upsert
	result := db.Where("tenant_id = ?", tenantID).Assign(config).FirstOrCreate(&config)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to save DB config"})
		return
	}

	// 清除连接缓存以便下次使用新配置
	h.router.InvalidateCache(tenantID)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Database configuration saved",
		"tenant_id": tenantID,
		"db_type":   config.DBType,
		"max_conns": config.MaxConns,
		"enabled":   config.Enabled,
	})
}

// GetDBConfig 获取租户的数据库配置（不返回 DSN）。
// GET /platform/db-config
func (h *DBConfigHandler) GetDBConfig(c *gin.Context) {
	tenantID := getUserID(c)
	if tenantID == "" {
		return
	}

	db := getTenantDB(c)
	var config database.TenantDBConfig
	if err := db.Where("tenant_id = ?", tenantID).First(&config).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
			"message":    "Using default database",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configured": true,
		"tenant_id":  config.TenantID,
		"db_type":    config.DBType,
		"max_conns":  config.MaxConns,
		"enabled":    config.Enabled,
		"created_at": config.CreatedAt,
		"updated_at": config.UpdatedAt,
	})
}

// DeleteDBConfig 删除租户的数据库配置，回退到默认数据库。
// DELETE /platform/db-config
func (h *DBConfigHandler) DeleteDBConfig(c *gin.Context) {
	tenantID := getUserID(c)
	if tenantID == "" {
		return
	}

	db := getTenantDB(c)
	result := db.Where("tenant_id = ?", tenantID).Delete(&database.TenantDBConfig{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to delete DB config"})
		return
	}

	h.router.InvalidateCache(tenantID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Database configuration deleted, using default database",
	})
}

// TestDBConnection 测试数据库连接。
// POST /platform/db-config/test
func (h *DBConfigHandler) TestDBConnection(c *gin.Context) {
	var req TestDBConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request: " + err.Error()})
		return
	}

	// P2-3: SSRF 防护 — 禁止连接内网地址
	if err := middleware.ValidateDSNSafety(req.DSN); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	config := database.TenantDBConfig{
		DBType: req.DBType,
		DSN:    req.DSN,
	}

	if err := h.router.TestConnection(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Database connection successful",
	})
}

// GetSchemaStatus 返回当前租户 DB 的 schema 版本状态。
// GET /platform/db-config/schema-status
func (h *DBConfigHandler) GetSchemaStatus(c *gin.Context) {
	tenantID := getUserID(c)
	if tenantID == "" {
		return
	}

	db, err := h.router.GetDB(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"detail": "Failed to get tenant DB",
		})
		return
	}

	status, err := database.GetSchemaStatus(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"detail": "Failed to query schema status: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_id": tenantID,
		"schema":    status,
	})
}
