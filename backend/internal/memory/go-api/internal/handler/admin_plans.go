// Package handler — Admin plan/memory management (Kanban) routes.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/models"
)

// ===========================================================================
// Plan/Memory Management (Kanban)
// ===========================================================================

func (h *AdminHandler) GetPlans(c *gin.Context) {
	var plans []models.Memory
	getTenantDB(c).Where("memory_type = ?", "plan").
		Order("created_at DESC").Limit(100).
		Find(&plans)

	result := make([]gin.H, len(plans))
	for i, p := range plans {
		result[i] = gin.H{
			"id":               p.ID.String(),
			"content":          p.Content,
			"user_id":          p.UserID,
			"importance_score": p.ImportanceScore,
			"created_at":       p.CreatedAt,
			"metadata":         p.Metadata,
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) UpdatePlanStatus(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	validStatuses := map[string]bool{"todo": true, "in_progress": true, "done": true, "cancelled": true}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid status"})
		return
	}

	var memory models.Memory
	if err := getTenantDB(c).First(&memory, "id = ?", memoryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Memory not found"})
		return
	}
	if memory.MemoryType != "plan" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Only plan-type memories can have their status updated"})
		return
	}

	// Update metadata
	meta := make(map[string]any)
	if memory.Metadata != nil {
		meta = *memory.Metadata
	}
	meta["status"] = req.Status

	getTenantDB(c).Model(&memory).Update("metadata", meta)

	c.JSON(http.StatusOK, gin.H{
		"id":      memoryID.String(),
		"status":  req.Status,
		"message": "Plan status updated",
	})
}

func (h *AdminHandler) UpdatePlan(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID"})
		return
	}

	var req struct {
		Content     *string        `json:"content"`
		PlanDetails map[string]any `json:"plan_details"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	var memory models.Memory
	if err := getTenantDB(c).First(&memory, "id = ?", memoryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Memory not found"})
		return
	}

	if req.Content != nil {
		getTenantDB(c).Model(&memory).Update("content", *req.Content)
	}

	if req.PlanDetails != nil {
		meta := make(map[string]any)
		if memory.Metadata != nil {
			meta = *memory.Metadata
		}
		for k, v := range req.PlanDetails {
			if v != nil {
				meta[k] = v
			} else {
				delete(meta, k)
			}
		}
		getTenantDB(c).Model(&memory).Update("metadata", meta)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      memoryID.String(),
		"message": "Plan updated successfully",
	})
}

func (h *AdminHandler) DeletePlan(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID"})
		return
	}

	result := getTenantDB(c).Where("id = ?", memoryID).Delete(&models.Memory{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Memory not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      memoryID.String(),
		"message": "Plan deleted successfully",
	})
}
