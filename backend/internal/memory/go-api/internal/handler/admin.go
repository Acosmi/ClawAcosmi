// Package handler — Admin management routes.
// Mirrors Python api/routes/admin/ — financial stats, system config, user management, plan Kanban.
// All routes require admin authentication.
//
// Handler implementations are split across domain-specific files:
//   - admin_financial.go  — financial statistics
//   - admin_config.go     — system config and config CRUD
//   - admin_users.go      — user management (ban/unban)
//   - admin_plans.go      — plan/memory Kanban management
//   - admin_docs.go       — documentation CMS
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/services"
)

// AdminHandler handles admin-only routes.
type AdminHandler struct {
	configService *services.DynamicConfigService
}

// NewAdminHandler creates a new AdminHandler.
// DB is obtained dynamically from Gin context via TenantDB middleware.
func NewAdminHandler(cs *services.DynamicConfigService) *AdminHandler {
	return &AdminHandler{configService: cs}
}

// RegisterRoutes registers all admin routes.
func (h *AdminHandler) RegisterRoutes(rg *gin.RouterGroup) {
	admin := rg.Group("/admin")
	{
		// Financial Stats
		admin.GET("/stats/financial", h.GetFinancialStats)

		// System Config
		admin.GET("/system/config", h.GetSystemConfig)
		admin.PATCH("/system/config", h.UpdateSystemConfig)
		admin.POST("/system/config/reload", h.ReloadServices)

		// Config Management
		admin.GET("/configs", h.ListConfigs)
		admin.PATCH("/configs", h.BatchUpdateConfigs)
		admin.POST("/configs/refresh", h.RefreshConfigs)
		admin.GET("/configs/groups", h.ListConfigGroups)
		admin.GET("/configs/:key", h.GetConfigValue)
		admin.PUT("/configs/:key", h.UpdateSingleConfig)
		admin.POST("/configs/initialize", h.InitializeDefaults)

		// User Management
		admin.GET("/users", h.ListUsers)
		admin.POST("/users/:user_id/ban", h.BanUser)
		admin.POST("/users/:user_id/unban", h.UnbanUser)

		// Plan/Memory Management (Kanban)
		admin.GET("/memories/plans", h.GetPlans)
		admin.PATCH("/memories/:memory_id/status", h.UpdatePlanStatus)
		admin.PATCH("/memories/:memory_id", h.UpdatePlan)
		admin.DELETE("/memories/:memory_id", h.DeletePlan)

		// Documentation CMS
		admin.GET("/docs", h.ListDocs)
		admin.GET("/docs/:id", h.GetDoc)
		admin.PATCH("/docs/:id", h.UpdateDoc)
		admin.POST("/docs/sync", h.SyncDocs)
	}
}
