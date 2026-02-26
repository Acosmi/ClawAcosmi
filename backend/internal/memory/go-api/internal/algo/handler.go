// Package algo — HTTP handler for the Algorithm API.
// Registers Gin routes under /api/v1/algo/*.
package algo

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes the Algorithm API via HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Algorithm API handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all /algo/* endpoints on the given Gin router group.
// All endpoints are protected by the parent router group's middleware (OAuth/API Key).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	algo := rg.Group("/algo")
	{
		algo.POST("/embed", h.handleEmbed)
		algo.POST("/classify", h.handleClassify)
		algo.POST("/rank", h.handleRank)
		algo.POST("/reflect", h.handleReflect)
		algo.POST("/extract", h.handleExtract)
		algo.GET("/health", h.handleHealth)
	}
}

// handleEmbed handles POST /algo/embed — generates vector embeddings.
func (h *Handler) handleEmbed(c *gin.Context) {
	var req EmbedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.svc.Embed(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleClassify handles POST /algo/classify — NLP classification + importance scoring.
func (h *Handler) handleClassify(c *gin.Context) {
	var req ClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.svc.Classify(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleRank handles POST /algo/rank — semantic reranking.
func (h *Handler) handleRank(c *gin.Context) {
	var req RankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.svc.Rank(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleReflect handles POST /algo/reflect — reflection generation.
func (h *Handler) handleReflect(c *gin.Context) {
	var req ReflectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.svc.Reflect(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleExtract handles POST /algo/extract — entity/relation extraction.
func (h *Handler) handleExtract(c *gin.Context) {
	var req ExtractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.svc.Extract(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleHealth handles GET /algo/health — check algorithm services status.
func (h *Handler) handleHealth(c *gin.Context) {
	resp := h.svc.Health()
	status := http.StatusOK
	if resp.Status == "degraded" {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
}
