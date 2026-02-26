// Package middleware — 租户数据库路由中间件。
// 从请求中提取 tenant_id，通过 TenantDBRouter 获取对应 DB 连接，
// 写入 Gin Context 供下游 handler 使用。
package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/database"
)

// ContextKeyTenantDB 是 Gin Context 中存放租户 DB 的 key。
const ContextKeyTenantDB = "tenant_db"

// TenantDB 返回一个 Gin 中间件，根据 tenant_id 动态注入对应的 *gorm.DB。
//
// 提取优先级:
//  1. context 中的 "user_id"（由 auth 中间件设置）
//  2. 请求头 "X-Tenant-ID"
//
// 如果 tenant_id 为空，使用默认 DB（router.GetDB 的行为）。
func TenantDB(router *database.TenantDBRouter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先从 auth context 获取 user_id 作为 tenant_id
		tenantID := ""
		if uid, exists := c.Get("user_id"); exists {
			if s, ok := uid.(string); ok {
				tenantID = s
			}
		}
		// 备选: 请求头
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		db, err := router.GetDB(c.Request.Context(), tenantID)
		if err != nil {
			slog.Error("TenantDB middleware: failed to get DB",
				"tenant_id", tenantID, "error", err)
			// GetDB 内部已有 fallback，此处不应到达
		}

		c.Set(ContextKeyTenantDB, db)

		// 将 tenant_id 写入 request context，供 service 层 TenantFromCtx() 读取
		if tenantID != "" {
			ctx := WithTenantID(c.Request.Context(), tenantID)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}
