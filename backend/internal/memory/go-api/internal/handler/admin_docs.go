// Package handler — Admin documentation CMS routes.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/models"
)

// ===========================================================================
// Documentation CMS
// ===========================================================================

func (h *AdminHandler) ListDocs(c *gin.Context) {
	var docs []models.DocEndpoint
	getTenantDB(c).Order("category ASC, sort_order ASC, name ASC").Find(&docs)

	// Group by category
	groupsMap := make(map[string][]models.DocEndpoint)
	var categoryOrder []string
	seen := make(map[string]bool)
	for _, d := range docs {
		if !seen[d.Category] {
			categoryOrder = append(categoryOrder, d.Category)
			seen[d.Category] = true
		}
		groupsMap[d.Category] = append(groupsMap[d.Category], d)
	}

	groups := make([]gin.H, 0, len(categoryOrder))
	for _, cat := range categoryOrder {
		groups = append(groups, gin.H{
			"category":  cat,
			"endpoints": groupsMap[cat],
		})
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups, "total": len(docs)})
}

func (h *AdminHandler) GetDoc(c *gin.Context) {
	id := c.Param("id")
	var doc models.DocEndpoint
	if err := getTenantDB(c).First(&doc, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Document not found"})
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *AdminHandler) UpdateDoc(c *gin.Context) {
	id := c.Param("id")
	var doc models.DocEndpoint
	if err := getTenantDB(c).First(&doc, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Document not found"})
		return
	}

	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	getTenantDB(c).Model(&doc).Updates(updates)
	getTenantDB(c).First(&doc, "id = ?", id)
	c.JSON(http.StatusOK, doc)
}

func (h *AdminHandler) SyncDocs(c *gin.Context) {
	// Placeholder — actual sync from OpenAPI spec is done by Python script
	c.JSON(http.StatusOK, gin.H{
		"created": 0,
		"updated": 0,
		"message": "Sync not yet implemented in Go backend. Use Python sync script.",
	})
}
