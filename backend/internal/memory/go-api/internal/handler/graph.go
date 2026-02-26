// Package handler — Knowledge Graph API routes.
// Mirrors Python api/routes/graph.py — graph visualization, entity CRUD, community detection.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
	"github.com/uhms/go-api/internal/services"
)

// GraphHandler handles graph-related HTTP routes.
type GraphHandler struct {
	graphStore *services.GraphStoreService
}

// NewGraphHandler creates a new GraphHandler.
// DB is obtained dynamically from Gin context via TenantDB middleware.
func NewGraphHandler(gs *services.GraphStoreService) *GraphHandler {
	return &GraphHandler{graphStore: gs}
}

// RegisterRoutes registers graph routes on the given router group.
func (h *GraphHandler) RegisterRoutes(rg *gin.RouterGroup) {
	graph := rg.Group("/graph")
	{
		graph.GET("", h.GetGraph)
		graph.GET("/entities", h.ListEntities)
		graph.GET("/entities/:entity_id", h.GetEntity)
		graph.POST("/entities", h.CreateEntity)
		graph.DELETE("/entities/:entity_id", h.DeleteEntity)
		graph.POST("/communities", h.RunCommunityDetection)
		graph.GET("/communities", h.GetCommunities)
		graph.POST("/communities/summary", h.GenerateCommunitySummary)
		graph.GET("/temporal", h.GetTemporalGraph)
		graph.GET("/recent", h.GetRecentEntities)
	}
}

// --- Response types ---

// GraphDataResponse wraps entities and relations for React Flow.
type GraphDataResponse struct {
	Entities  []models.Entity   `json:"entities"`
	Relations []models.Relation `json:"relations"`
}

// --- Request types ---

// CreateEntityRequest mirrors EntityCreate schema.
type CreateEntityRequest struct {
	Name        string         `json:"name" binding:"required"`
	EntityType  string         `json:"entity_type" binding:"required"`
	Description string         `json:"description,omitempty"`
	UserID      string         `json:"user_id" binding:"required"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// --- Handlers ---

// GetGraph handles GET /graph — full knowledge graph for a user.
func (h *GraphHandler) GetGraph(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)
	entities, relations, err := h.graphStore.GetGraph(db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to retrieve graph"})
		return
	}

	c.JSON(http.StatusOK, GraphDataResponse{
		Entities:  entities,
		Relations: relations,
	})
}

// ListEntities handles GET /graph/entities — list entities, optionally filtered by type.
func (h *GraphHandler) ListEntities(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	entityType := c.Query("entity_type")

	db := getTenantDB(c)
	query := db.Where("user_id = ?", userID)
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	query = query.Order("name ASC")

	var entities []models.Entity
	if err := query.Find(&entities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to list entities"})
		return
	}

	if entities == nil {
		entities = []models.Entity{}
	}
	c.JSON(http.StatusOK, entities)
}

// GetEntity handles GET /graph/entities/:entity_id — get an entity by ID.
func (h *GraphHandler) GetEntity(c *gin.Context) {
	entityID, err := uuid.Parse(c.Param("entity_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid entity ID format"})
		return
	}

	db := getTenantDB(c)
	var entity models.Entity
	if err := db.First(&entity, "id = ?", entityID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"detail": "Entity not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to retrieve entity"})
		return
	}

	c.JSON(http.StatusOK, entity)
}

// CreateEntity handles POST /graph/entities — manually create an entity.
func (h *GraphHandler) CreateEntity(c *gin.Context) {
	var req CreateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	entity := &models.Entity{
		Name:       req.Name,
		EntityType: req.EntityType,
		UserID:     req.UserID,
	}
	if req.Description != "" {
		desc := req.Description
		entity.Description = &desc
	}

	db := getTenantDB(c)
	if err := h.graphStore.AddEntity(db, entity); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to create entity"})
		return
	}

	c.JSON(http.StatusCreated, entity)
}

// DeleteEntity handles DELETE /graph/entities/:entity_id — delete entity and relations.
func (h *GraphHandler) DeleteEntity(c *gin.Context) {
	entityID, err := uuid.Parse(c.Param("entity_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid entity ID format"})
		return
	}

	// Check entity exists
	db := getTenantDB(c)
	var entity models.Entity
	if err := db.First(&entity, "id = ?", entityID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"detail": "Entity not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to check entity"})
		return
	}

	if err := h.graphStore.DeleteEntity(db, entityID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to delete entity"})
		return
	}

	c.Status(http.StatusNoContent)
}

// RunCommunityDetection handles POST /graph/communities — Label Propagation.
func (h *GraphHandler) RunCommunityDetection(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)
	communities, err := h.graphStore.RunCommunityDetection(db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Community detection failed"})
		return
	}

	// Convert uuid.UUID keys to strings for JSON
	result := make(map[string]int, len(communities))
	for id, communityID := range communities {
		result[id.String()] = communityID
	}

	c.JSON(http.StatusOK, result)
}

// GetCommunities handles GET /graph/communities — list communities with entities.
func (h *GraphHandler) GetCommunities(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	db := getTenantDB(c)
	communities, err := h.graphStore.GetCommunities(db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to get communities"})
		return
	}
	c.JSON(http.StatusOK, communities)
}

// GenerateCommunitySummary handles POST /graph/communities/summary.
func (h *GraphHandler) GenerateCommunitySummary(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}
	cidStr := c.Query("community_id")
	if cidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "community_id required"})
		return
	}
	cid, err := strconv.Atoi(cidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid community_id"})
		return
	}

	llm := services.GetLLMProvider()
	if llm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "LLM not available"})
		return
	}

	db := getTenantDB(c)
	summary, err := h.graphStore.GenerateCommunitySummary(c.Request.Context(), db, userID, cid, llm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Summary generation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"community_id": cid, "summary": summary})
}

// GetTemporalGraph handles GET /graph/temporal — temporal graph query.
// Supports two modes:
//   - Point-in-time: ?user_id=X&as_of=2026-01-01T00:00:00Z
//   - Time range:    ?user_id=X&from=2026-01-01T00:00:00Z&to=2026-01-07T00:00:00Z
func (h *GraphHandler) GetTemporalGraph(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	asOfStr := c.Query("as_of")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	// Mode 1: as_of point-in-time query
	if asOfStr != "" {
		asOf, err := time.Parse(time.RFC3339, asOfStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid as_of format, use RFC3339 (e.g. 2026-01-01T00:00:00Z)"})
			return
		}
		db := getTenantDB(c)
		result, err := h.graphStore.QueryGraphAsOf(db, userID, asOf)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Temporal query failed"})
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	// Mode 2: from+to range query
	if fromStr != "" && toStr != "" {
		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid from format, use RFC3339"})
			return
		}
		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid to format, use RFC3339"})
			return
		}
		db := getTenantDB(c)
		result, err := h.graphStore.QueryGraphByTimeRange(db, userID, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Temporal range query failed"})
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"detail": "Provide either as_of or both from and to parameters"})
}

// GetRecentEntities handles GET /graph/recent — recently created entities.
// Query params: user_id (required), entity_type (optional), limit (optional, default 10).
func (h *GraphHandler) GetRecentEntities(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		return
	}

	entityType := c.Query("entity_type")
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	db := getTenantDB(c)
	entities, err := h.graphStore.QueryRecentEntities(db, userID, entityType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to query recent entities"})
		return
	}

	if entities == nil {
		entities = []models.Entity{}
	}
	c.JSON(http.StatusOK, entities)
}
