// Package handler — 租户记忆存储模式配置管理 API。
// 允许租户在 vector / fs / hybrid 三种永久记忆存储方式之间切换。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/models"
	"github.com/uhms/go-api/internal/services"
)

// MemoryConfigHandler 处理租户记忆存储模式配置相关路由。
type MemoryConfigHandler struct{}

// NewMemoryConfigHandler creates a new MemoryConfigHandler.
func NewMemoryConfigHandler() *MemoryConfigHandler {
	return &MemoryConfigHandler{}
}

// RegisterRoutes 注册记忆配置路由。
// 仅 admin/owner 角色可操作存储模式配置。
func (h *MemoryConfigHandler) RegisterRoutes(rg *gin.RouterGroup) {
	memConfig := rg.Group("/platform/memory-config")
	memConfig.Use(middleware.RequireRole("admin", "owner"))
	{
		memConfig.GET("", h.GetMemoryConfig)
		memConfig.PUT("", h.UpdateMemoryConfig)
	}
}

// --- Request types ---

// UpdateMemoryConfigRequest 更新记忆存储模式请求。
type UpdateMemoryConfigRequest struct {
	MemoryStorageMode string `json:"memory_storage_mode" binding:"required,oneof=vector fs hybrid"`
}

// --- Handlers ---

// GetMemoryConfig 获取当前租户的记忆存储模式配置。
// GET /platform/memory-config
func (h *MemoryConfigHandler) GetMemoryConfig(c *gin.Context) {
	tenantID := getUserID(c)
	if tenantID == "" {
		return
	}

	db := getTenantDB(c)
	var config models.TenantMemoryConfig
	if err := db.Where("tenant_id = ?", tenantID).First(&config).Error; err != nil {
		// 未配置时返回默认值
		c.JSON(http.StatusOK, gin.H{
			"tenant_id":           tenantID,
			"memory_storage_mode": services.DefaultStorageMode,
			"configured":          false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_id":           config.TenantID,
		"memory_storage_mode": config.MemoryStorageMode,
		"configured":          true,
		"created_at":          config.CreatedAt,
		"updated_at":          config.UpdatedAt,
	})
}

// UpdateMemoryConfig 更新租户的记忆存储模式。
// PUT /platform/memory-config
func (h *MemoryConfigHandler) UpdateMemoryConfig(c *gin.Context) {
	tenantID := getUserID(c)
	if tenantID == "" {
		return
	}

	var req UpdateMemoryConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request: " + err.Error()})
		return
	}

	config := models.TenantMemoryConfig{
		TenantID:          tenantID,
		MemoryStorageMode: req.MemoryStorageMode,
	}

	db := getTenantDB(c)
	// Upsert: 存在则更新，不存在则创建
	result := db.Where("tenant_id = ?", tenantID).Assign(config).FirstOrCreate(&config)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to save memory config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "Memory storage mode updated",
		"tenant_id":           config.TenantID,
		"memory_storage_mode": config.MemoryStorageMode,
	})
}
