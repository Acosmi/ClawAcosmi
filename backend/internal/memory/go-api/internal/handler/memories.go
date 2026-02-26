// Package handler — Memory CRUD API routes.
// Mirrors Python api/routes/memories.py — all routes require API key auth.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/models"
	"github.com/uhms/go-api/internal/services"
)

// MemoryHandler handles memory-related HTTP routes.
type MemoryHandler struct {
	manager           *services.MemoryManager
	treeManager       *services.TreeManager
	archiver          *services.MemoryArchiver
	imaginationEngine *services.ImaginationEngine

	// ProMem per-user cooldown: prevents repeated archiving within 5 minutes.
	archiveCooldownMu sync.Mutex
	archiveCooldown   map[string]time.Time
}

// MemoryHandlerOption configures optional dependencies on MemoryHandler.
type MemoryHandlerOption func(*MemoryHandler)

// WithImaginationEngine injects a DI-managed ImaginationEngine.
func WithImaginationEngine(e *services.ImaginationEngine) MemoryHandlerOption {
	return func(h *MemoryHandler) { h.imaginationEngine = e }
}

// WithMemoryArchiver injects a DI-managed MemoryArchiver.
func WithMemoryArchiver(a *services.MemoryArchiver) MemoryHandlerOption {
	return func(h *MemoryHandler) { h.archiver = a }
}

// NewMemoryHandler creates a new MemoryHandler.
// DB is obtained dynamically from Gin context via TenantDB middleware.
func NewMemoryHandler(manager *services.MemoryManager, opts ...MemoryHandlerOption) *MemoryHandler {
	h := &MemoryHandler{
		manager:         manager,
		treeManager:     services.GetTreeManager(),
		archiveCooldown: make(map[string]time.Time),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers memory routes on the given router group.
func (h *MemoryHandler) RegisterRoutes(rg *gin.RouterGroup) {
	memories := rg.Group("/memories")
	{
		memories.POST("", h.CreateMemory)
		memories.GET("", h.ListMemories)
		memories.GET("/search", h.SearchMemories)
		memories.GET("/search-trace", h.SearchMemoriesWithTrace)
		memories.GET("/tree", h.GetMemoryTree)
		memories.GET("/tree/search", h.SearchMemoryTree)
		memories.GET("/tree/stats", h.GetTreeStats)
		memories.GET("/:memory_id", h.GetMemory)
		memories.GET("/:memory_id/detail", h.GetMemoryDetail) // Phase 3: L2 按需展开
		memories.DELETE("/:memory_id", h.DeleteMemory)
		memories.POST("/reflect", h.TriggerReflection)
		// L1.1 永久记忆路由
		memories.GET("/permanent", h.ListPermanentMemories)
		memories.POST("/:memory_id/pin", h.PinMemory)
		memories.DELETE("/:memory_id/pin", h.UnpinMemory)
		// L4 想象记忆路由
		memories.POST("/imagine", h.TriggerImagination)
		memories.GET("/imaginations", h.ListImaginations)
		// CommitSession 触发路由
		memories.POST("/commit-session", h.CommitSession)
	}
}

// --- Request types ---

// CreateMemoryRequest mirrors MemoryCreate schema.
type CreateMemoryRequest struct {
	Content    string         `json:"content" binding:"required"`
	UserID     string         `json:"user_id" binding:"required"`
	MemoryType string         `json:"memory_type,omitempty"`
	Category   string         `json:"category,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	EventTime  *time.Time     `json:"event_time,omitempty"` // Bi-temporal: 显式事件时间
}

// --- Handlers ---

// ListMemories handles GET /memories — list memories for a user with pagination.
// Query params: user_id (required), limit (1-200, default 50), offset (default 0),
// memory_type (optional filter).
func (h *MemoryHandler) ListMemories(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed >= 1 && parsed <= 200 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	memoryType := c.Query("memory_type")

	db := getTenantDB(c)
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Database unavailable"})
		return
	}

	query := db.Model(&models.Memory{}).Where("user_id = ?", userID)
	if memoryType != "" {
		query = query.Where("memory_type = ?", memoryType)
	}

	var memories []models.Memory
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&memories).Error; err != nil {
		slog.Error("ListMemories: DB query failed", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to list memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"memories": memories,
		"limit":    limit,
		"offset":   offset,
	})
}

// CreateMemory handles POST /memories — add a new memory.
func (h *MemoryHandler) CreateMemory(c *gin.Context) {
	var req CreateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	if req.MemoryType == "" {
		req.MemoryType = "observation"
	}

	// Inject explicit category into metadata so classifyCategory picks it up
	if req.Category != "" {
		if req.Metadata == nil {
			req.Metadata = map[string]any{}
		}
		req.Metadata["category"] = req.Category
	}

	// Bi-temporal: 将显式 event_time 注入 metadata 供 AddMemory 读取
	if req.EventTime != nil {
		if req.Metadata == nil {
			req.Metadata = map[string]any{}
		}
		req.Metadata["event_time"] = req.EventTime.Format(time.RFC3339)
	}

	db := getTenantDB(c)
	memory, err := h.manager.AddMemory(
		c.Request.Context(),
		db,
		req.Content,
		req.UserID,
		req.MemoryType,
		nil, // importanceScore — let LLM auto-score
		req.Metadata,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to create memory"})
		return
	}

	c.JSON(http.StatusCreated, memory)
}

// SearchMemories handles GET /memories/search — hybrid vector search.
func (h *MemoryHandler) SearchMemories(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "query parameter is required"})
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	limit := 5
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed >= 1 && parsed <= 50 {
			limit = parsed
		}
	}

	minImportance := 0.0
	if mi := c.Query("min_importance"); mi != "" {
		if parsed, err := strconv.ParseFloat(mi, 64); err == nil && parsed >= 0 && parsed <= 1 {
			minImportance = parsed
		}
	}

	memoryTypes := c.QueryArray("memory_types")
	category := c.Query("category")

	// Bi-temporal: 解析 event_from / event_to（RFC3339 格式）
	var eventFrom, eventTo *time.Time
	if ef := c.Query("event_from"); ef != "" {
		if parsed, err := time.Parse(time.RFC3339, ef); err == nil {
			eventFrom = &parsed
		}
	}
	if et := c.Query("event_to"); et != "" {
		if parsed, err := time.Parse(time.RFC3339, et); err == nil {
			eventTo = &parsed
		}
	}

	db := getTenantDB(c)
	results, err := h.manager.SearchMemories(
		c.Request.Context(),
		db,
		query, userID, limit, memoryTypes, minImportance, category, eventFrom, eventTo,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Search failed"})
		return
	}

	if results == nil {
		results = []services.VectorSearchResult{}
	}
	c.JSON(http.StatusOK, results)
}

// SearchMemoriesWithTrace handles GET /memories/search-trace — returns search trace for visualization.
// Phase 4: Visualized Retrieval Trajectory.
func (h *MemoryHandler) SearchMemoriesWithTrace(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "query parameter is required"})
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed >= 1 && parsed <= 50 {
			limit = parsed
		}
	}

	trace, err := h.manager.SearchMemoriesWithTrace(
		c.Request.Context(),
		userID, query, limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Search trace failed"})
		return
	}

	c.JSON(http.StatusOK, trace)
}

// GetMemory handles GET /memories/:memory_id — get a memory by ID.
func (h *MemoryHandler) GetMemory(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID format"})
		return
	}

	db := getTenantDB(c)
	memory, err := h.manager.GetMemory(db, memoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to retrieve memory"})
		return
	}
	if memory == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Memory not found"})
		return
	}

	c.JSON(http.StatusOK, memory)
}

// GetMemoryDetail handles GET /memories/:memory_id/detail?level=2&user_id=xxx
// Phase 3: Returns a specific tier level (L0/L1/L2) for progressive loading.
// Imagination memories are blocked from expanding beyond L0.
func (h *MemoryHandler) GetMemoryDetail(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID format"})
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	level := 2 // default to L2 (full content)
	if l := c.Query("level"); l != "" {
		if parsed, parseErr := strconv.Atoi(l); parseErr == nil && parsed >= 0 && parsed <= 2 {
			level = parsed
		}
	}

	// Look up memory from DB to get type and category.
	db := getTenantDB(c)
	memory, err := h.manager.GetMemory(db, memoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to retrieve memory"})
		return
	}
	if memory == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Memory not found"})
		return
	}

	tenantID := middleware.TenantFromCtx(c.Request.Context())

	result, err := h.manager.GetMemoryDetail(
		tenantID, userID, memoryID.String(),
		memory.MemoryType, memory.Category,
		level,
	)
	if err != nil {
		if errors.Is(err, services.ErrImagineL2Blocked) {
			c.JSON(http.StatusForbidden, gin.H{
				"detail": "Imagination memories cannot be expanded beyond L0",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to read memory detail"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteMemory handles DELETE /memories/:memory_id — delete a memory.
func (h *MemoryHandler) DeleteMemory(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID format"})
		return
	}

	// Verify memory exists
	db := getTenantDB(c)
	memory, err := h.manager.GetMemory(db, memoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to check memory"})
		return
	}
	if memory == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Memory not found"})
		return
	}

	if err := h.manager.DeleteMemory(c.Request.Context(), db, memoryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to delete memory"})
		return
	}

	c.Status(http.StatusNoContent)
}

// TriggerReflection handles POST /memories/reflect — manual reflection.
func (h *MemoryHandler) TriggerReflection(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	db := getTenantDB(c)
	reflection, err := h.manager.TriggerReflection(c.Request.Context(), db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Reflection failed"})
		return
	}

	if reflection == nil {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusOK, reflection)
}

// ============================================================================
// MemTree API Handlers
// ============================================================================

// GetMemoryTree returns the tree structure for a user.
// GET /memories/tree?user_id=xxx&category=xxx&node_id=xxx
func (h *MemoryHandler) GetMemoryTree(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		userID = "default"
	}
	category := c.Query("category")

	var nodeID *uuid.UUID
	if nid := c.Query("node_id"); nid != "" {
		parsed, err := uuid.Parse(nid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid node_id"})
			return
		}
		nodeID = &parsed
	}

	db := getTenantDB(c)
	nodes, err := h.treeManager.GetSubtree(db, userID, nodeID, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to get tree"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// SearchMemoryTree performs collapsed tree retrieval.
// GET /memories/tree/search?query=xxx&user_id=xxx&category=xxx&limit=5
func (h *MemoryHandler) SearchMemoryTree(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "query is required"})
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		userID = "default"
	}
	category := c.Query("category")

	limit := 5
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	db := getTenantDB(c)
	results, err := h.treeManager.CollapsedTreeRetrieve(
		c.Request.Context(), db, query, userID, limit, category,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Tree search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
		"query":   query,
	})
}

// GetTreeStats returns statistics about a user's memory tree.
// GET /memories/tree/stats?user_id=xxx
func (h *MemoryHandler) GetTreeStats(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		userID = "default"
	}

	db := getTenantDB(c)
	stats := h.treeManager.GetTreeStats(db, userID)
	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// L1.1 永久记忆 API Handlers
// ============================================================================

// getArchiver 懒初始化 MemoryArchiver（避免修改外部初始化链路）。
func (h *MemoryHandler) getArchiver(c *gin.Context) *services.MemoryArchiver {
	if h.archiver == nil {
		h.archiver = services.NewMemoryArchiver(
			getTenantDB(c),
			services.GetLLMProvider(),
			services.GetVectorStore(),
		)
	}
	return h.archiver
}

// ListPermanentMemories handles GET /memories/permanent — list user's permanent memories.
func (h *MemoryHandler) ListPermanentMemories(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed >= 1 {
			page = parsed
		}
	}
	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed >= 1 && parsed <= 100 {
			pageSize = parsed
		}
	}

	db := getTenantDB(c)
	archiver := h.getArchiver(c)
	memories, total, err := archiver.GetPermanentMemories(db, userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to list permanent memories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"memories":  memories,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// PinMemory handles POST /memories/:memory_id/pin — mark memory as permanent.
func (h *MemoryHandler) PinMemory(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID format"})
		return
	}

	db := getTenantDB(c)
	archiver := h.getArchiver(c)
	if err := archiver.PinMemory(db, memoryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to pin memory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Memory pinned as permanent"})
}

// UnpinMemory handles DELETE /memories/:memory_id/pin — remove permanent mark.
func (h *MemoryHandler) UnpinMemory(c *gin.Context) {
	memoryID, err := uuid.Parse(c.Param("memory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid memory ID format"})
		return
	}

	db := getTenantDB(c)
	archiver := h.getArchiver(c)
	if err := archiver.UnpinMemory(db, memoryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to unpin memory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Memory unpinned"})
}

// ============================================================================
// L4 想象记忆 API Handlers
// ============================================================================

// getImaginationEngine 懒初始化 ImaginationEngine。
func (h *MemoryHandler) getImaginationEngine() *services.ImaginationEngine {
	if h.imaginationEngine == nil {
		h.imaginationEngine = services.NewImaginationEngine(
			services.GetGraphStore(),
			services.GetVectorStore(),
			services.GetLLMProvider(),
			services.GetWebSearchProvider(),
		)
	}
	return h.imaginationEngine
}

// TriggerImagination handles POST /memories/imagine — manually trigger imagination generation.
func (h *MemoryHandler) TriggerImagination(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	db := getTenantDB(c)
	engine := h.getImaginationEngine()
	memories, err := engine.Run(c.Request.Context(), db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Imagination generation failed", "error": err.Error()})
		return
	}

	if memories == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":   "No trending entities found for imagination",
			"generated": 0,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Imagination memories generated",
		"generated": len(memories),
		"memories":  memories,
	})
}

// ListImaginations handles GET /memories/imaginations — list user's imagination memories.
func (h *MemoryHandler) ListImaginations(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "user_id parameter is required"})
		return
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed >= 1 {
			page = parsed
		}
	}
	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed >= 1 && parsed <= 100 {
			pageSize = parsed
		}
	}

	db := getTenantDB(c)
	result, err := services.ListImaginations(db, userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Failed to list imaginations"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ============================================================================
// CommitSession API Handler (DEF-MEMFS-02)
// ============================================================================

// CommitSessionRequest 是 POST /memories/commit-session 的请求体。
type CommitSessionRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	UserID    string `json:"user_id"    binding:"required"`
	Messages  string `json:"messages"   binding:"required"`
}

// CommitSession handles POST /memories/commit-session —
// 触发会话提交管线（对话压缩 → 记忆提取 → 去重 → 写入）。
// 返回 202 Accepted，后台异步处理，结果通过 SSE 事件推送。
func (h *MemoryHandler) CommitSession(c *gin.Context) {
	var req CommitSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "session_id, user_id and messages are required"})
		return
	}

	// 通过 SSE 通知前端会话提交开始
	EmitSessionCommitting(req.UserID, req.SessionID)

	// 异步触发 CommitSession 管线
	resultCh := h.manager.CommitSession(
		c.Request.Context(),
		req.SessionID,
		req.UserID,
		req.Messages,
	)

	// 后台 goroutine 等待结果并通过 SSE 推送
	go func() {
		result := <-resultCh
		if result != nil {
			EmitSessionCommitted(
				req.UserID, req.SessionID,
				result.ExtractedCount, result.CreatedCount, result.MergedCount,
			)
			slog.Info("会话提交完成",
				"session_id", req.SessionID,
				"user_id", req.UserID,
				"extracted", result.ExtractedCount,
				"created", result.CreatedCount,
				"merged", result.MergedCount,
			)

			// L1.1 ProMem: 自动归档（仅在有提取结果且 archiver 可用时触发）
			if result.ExtractedCount > 0 && h.archiver != nil {
				h.triggerProMemArchive(req.UserID, req.Messages)
			}
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Session commit started",
		"session_id": req.SessionID,
	})
}

// triggerProMemArchive runs MemoryArchiver.ArchiveSession in background with
// per-user 5-minute cooldown to prevent repeated archiving.
func (h *MemoryHandler) triggerProMemArchive(userID, messages string) {
	h.archiveCooldownMu.Lock()
	if last, ok := h.archiveCooldown[userID]; ok && time.Since(last) < 5*time.Minute {
		h.archiveCooldownMu.Unlock()
		slog.Debug("ProMem 归档冷却中，跳过", "user_id", userID)
		return
	}
	h.archiveCooldown[userID] = time.Now()
	h.archiveCooldownMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := h.archiver.ArchiveSession(ctx, userID, messages); err != nil {
			slog.Warn("ProMem 自动归档失败", "user_id", userID, "error", err)
		} else {
			slog.Info("ProMem 自动归档完成", "user_id", userID)
		}
	}()
}
