// Package services — Memory Manager orchestration service.
// Mirrors Python services/memory_manager.py — coordinates Vector Store, Graph Store, and LLM.
package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/models"
)

// MemoryType constants — classic types.
const (
	MemoryTypeObservation = "observation"
	MemoryTypeReflection  = "reflection"
	MemoryTypeDialogue    = "dialogue"
	MemoryTypePlan        = "plan"
)

// Extended MemoryType constants — cognitive classification.
const (
	MemoryTypeEpisodic    = "episodic"    // 情景记忆：事件、对话、观察
	MemoryTypeSemantic    = "semantic"    // 语义记忆：事实、概念、反思
	MemoryTypeProcedural  = "procedural"  // 程序记忆：技能、计划、流程
	MemoryTypePermanent   = "permanent"   // L1.1 永久记忆：冷归档，豁免衰减
	MemoryTypeImagination = "imagination" // L4 想象记忆：前瞻性预测（预留）
)

// ProtectedMemoryTypes 受保护的记忆类型，不参与时间衰减和自动归档。
var ProtectedMemoryTypes = []string{
	MemoryTypePermanent,
	MemoryTypeImagination,
}

// legacyTypeMap maps classic types to cognitive equivalents (backward compat).
var legacyTypeMap = map[string]string{
	"observation": MemoryTypeEpisodic,
	"dialogue":    MemoryTypeEpisodic,
	"reflection":  MemoryTypeSemantic,
	"plan":        MemoryTypeProcedural,
}

// NormalizeMemoryType converts legacy types to cognitive types.
// Unknown types are passed through unchanged for forward compatibility.
func NormalizeMemoryType(t string) string {
	if mapped, ok := legacyTypeMap[t]; ok {
		return mapped
	}
	return t
}

// MemoryCategory constants — semantic classification of memory content.
const (
	CategoryPreference   = "preference"   // 偏好
	CategoryHabit        = "habit"        // 习惯
	CategoryProfile      = "profile"      // 个人信息
	CategorySkill        = "skill"        // 技能/知识
	CategoryRelationship = "relationship" // 关系
	CategoryEvent        = "event"        // 事件
	CategoryOpinion      = "opinion"      // 观点
	CategoryFact         = "fact"         // 事实
	CategoryGoal         = "goal"         // 目标
	CategoryTask         = "task"         // 任务
	CategoryReminder     = "reminder"     // 提醒
	CategoryInsight      = "insight"      // 洞察
	CategorySummary      = "summary"      // 总结
)

// validCategories is the set of allowed category values.
var validCategories = map[string]bool{
	CategoryPreference: true, CategoryHabit: true, CategoryProfile: true,
	CategorySkill: true, CategoryRelationship: true, CategoryEvent: true,
	CategoryOpinion: true, CategoryFact: true, CategoryGoal: true,
	CategoryTask: true, CategoryReminder: true, CategoryInsight: true,
	CategorySummary: true,
}

// MemoryManager is the core orchestration service for memory operations.
// Coordinates VectorStore, GraphStore, FSStore, and LLMClient.
type MemoryManager struct {
	vectorStore  *VectorStoreService
	graphStore   *GraphStoreService
	treeManager  *TreeManager
	llmClient    LLMProvider
	fsStore      *FSStoreService   // 文件系统存储（可选，Rust VFS）
	storageMode  string            // "vector" | "fs" | "hybrid"
	committer    *SessionCommitter // 会话提交器（可选）
	vfsSemIdx    *VFSSemanticIndex // Phase 5: VFS 语义索引（可选）
	tieredLoader *TieredLoader     // Phase 2: 分层加载器（可选）
	cache        *MemoryCache      // read-through search cache（可选）
}

// NewMemoryManager creates a new MemoryManager with injected dependencies.
func NewMemoryManager(vs *VectorStoreService, gs *GraphStoreService, llm LLMProvider) *MemoryManager {
	return &MemoryManager{
		vectorStore: vs,
		graphStore:  gs,
		treeManager: GetTreeManager(),
		llmClient:   llm,
		storageMode: StorageModeVector, // 默认向量模式
	}
}

// WithFSStore 挂载文件系统存储并设置存储模式。
func (m *MemoryManager) WithFSStore(fs *FSStoreService, mode string) *MemoryManager {
	m.fsStore = fs
	if mode != "" {
		m.storageMode = mode
	}
	return m
}

// WithVFSSemanticIndex 挂载 VFS 语义索引服务 (Phase 5)。
// 当设置后，VFS 写入时同步建立 segment 语义索引，搜索时使用语义搜索替代关键词搜索。
func (m *MemoryManager) WithVFSSemanticIndex(idx *VFSSemanticIndex) *MemoryManager {
	m.vfsSemIdx = idx
	return m
}

// WithSessionCommitter 挂载会话提交器。
func (m *MemoryManager) WithSessionCommitter(sc *SessionCommitter) *MemoryManager {
	m.committer = sc
	return m
}

// WithTieredLoader 挂载分层加载器 (Phase 2: L0→LLM→L1 渐进式加载)。
// 设置后 SearchMemories 将自动执行 LLM L0 筛选 + L1 按需加载。
func (m *MemoryManager) WithTieredLoader(tl *TieredLoader) *MemoryManager {
	m.tieredLoader = tl
	return m
}

// WithCache 挂载 read-through 搜索缓存。
// 设置后 SearchMemories 将在 Redis/本地缓存中优先查找，miss 时才执行向量搜索并回填缓存。
func (m *MemoryManager) WithCache(c *MemoryCache) *MemoryManager {
	m.cache = c
	return m
}

// searchCacheKeyFor derives a stable, fixed-length cache key from all search parameters.
// Using MD5 keeps Redis keys short regardless of query length.
func searchCacheKeyFor(query, userID string, limit int, types []string, minImportance float64, category string, eventFrom, eventTo *time.Time) string {
	from, to := "", ""
	if eventFrom != nil {
		from = eventFrom.Format(time.RFC3339)
	}
	if eventTo != nil {
		to = eventTo.Format(time.RFC3339)
	}
	raw := fmt.Sprintf("sm|%s|%s|%d|%s|%.2f|%s|%s|%s",
		query, userID, limit, strings.Join(types, ","), minImportance, category, from, to)
	h := md5.Sum([]byte(raw))
	return fmt.Sprintf("sm:%x", h)
}

// CommitSession 异步执行会话提交（对话压缩 → 记忆提取 → 去重 → 写入）。
// 不阻塞调用方，结果通过 channel 返回。
func (m *MemoryManager) CommitSession(
	ctx context.Context,
	sessionID, userID string,
	messages string,
) <-chan *SessionCommitResult {
	ch := make(chan *SessionCommitResult, 1)
	go func() {
		defer close(ch)
		if m.committer == nil {
			slog.Warn("SessionCommitter 未初始化，跳过会话提交")
			ch <- &SessionCommitResult{}
			return
		}
		result, err := m.committer.Commit(ctx, sessionID, userID, messages)
		if err != nil {
			slog.Error("会话提交失败", "error", err, "session_id", sessionID)
			ch <- &SessionCommitResult{}
			return
		}
		ch <- result
	}()
	return ch
}

// SearchMemoriesWithTrace 执行带完整检索轨迹的搜索（阶段四：可视化检索轨迹）。
// 仅在 fs/hybrid 模式下生效，vector-only 模式返回空轨迹。
func (m *MemoryManager) SearchMemoriesWithTrace(
	ctx context.Context,
	userID, query string,
	limit int,
) (*SearchTrace, error) {
	if query == "" {
		return nil, errors.New("query is required")
	}
	if limit <= 0 {
		limit = 10
	}

	// Only available when fsStore is configured
	if m.fsStore == nil {
		return &SearchTrace{
			Query:    query,
			Keywords: []string{},
			Steps:    []TraceStep{},
			Hits:     []SearchHit{},
		}, nil
	}

	tenantID := middleware.TenantFromCtx(ctx)
	return m.fsStore.SearchMemoriesWithTrace(tenantID, userID, query, limit)
}

// AddMemory adds a new memory to the system.
// Orchestrates: category classification → DB insert → vector store → entity extraction → reflection check.
func (m *MemoryManager) AddMemory(
	ctx context.Context,
	db *gorm.DB,
	content, userID string,
	memoryType string,
	importanceScore *float64,
	metadata map[string]any,
) (*models.Memory, error) {
	if content == "" {
		return nil, errors.New("content is required")
	}
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	// Default memory type
	if memoryType == "" {
		memoryType = MemoryTypeObservation
	}

	// Default importance score
	score := 0.5
	if importanceScore != nil {
		score = *importanceScore
	}

	// Auto-classify category based on content and memory type
	category := m.classifyCategory(ctx, content, memoryType, metadata)

	// 0. 事实管线：Extract → Dedupe → Merge（mem0 风格）
	facts, _ := ExtractFacts(ctx, m.llmClient, content)
	if len(facts) > 0 {
		dedupResult := DeduplicateFacts(ctx, facts, userID, m.vectorStore, 0.85)

		// 合并冲突事实
		if len(dedupResult.UpdatePairs) > 0 {
			merged := MergeFacts(ctx, m.llmClient, dedupResult.UpdatePairs)
			for _, pair := range merged {
				// 更新已有记忆内容
				if pair.ExistingID != "" {
					targetUUID, err := uuid.Parse(pair.ExistingID)
					if err == nil {
						if dbErr := db.Model(&models.Memory{}).Where("id = ?", targetUUID).
							Update("content", pair.NewFact.Content).Error; dbErr != nil {
							slog.Warn("事实合并 DB 更新失败", "id", targetUUID, "error", dbErr)
						}
						// 同步向量
						if m.vectorStore != nil {
							go func(id uuid.UUID, c string) {
								_ = m.vectorStore.AddMemory(context.Background(), id, c, userID, memoryType, score, nil)
							}(targetUUID, pair.NewFact.Content)
						}
					}
				}
			}
		}

		// 如果所有事实都已存在（全部跳过或合并），无需插入新记忆
		if len(dedupResult.NewFacts) == 0 {
			slog.Info("事实管线：所有事实已存在，跳过插入",
				"skipped", dedupResult.SkippedCount,
				"merged", len(dedupResult.UpdatePairs),
			)
			// 返回一个占位记忆对象供调用者使用
			var existing models.Memory
			db.Where("user_id = ?", userID).Order("created_at DESC").First(&existing)
			if existing.ID != uuid.Nil {
				return &existing, nil
			}
		}

		// 使用第一个新事实替换原始内容（更精确的事实表述）
		if len(dedupResult.NewFacts) > 0 && dedupResult.NewFacts[0].Content != content {
			content = dedupResult.NewFacts[0].Content
			if validCategories[dedupResult.NewFacts[0].Category] {
				category = dedupResult.NewFacts[0].Category
			}
		}
	}

	// Bi-temporal: 事件时间抽取
	// 优先级: 1) metadata 显式传入 → 2) LLM 自动抽取 → 3) nil（以 ingested_at 为准）
	var eventTime *time.Time
	if metadata != nil {
		if et, ok := metadata["event_time"].(string); ok && et != "" {
			if parsed, parseErr := parseFlexibleTime(et); parseErr == nil {
				eventTime = &parsed
			}
		}
	}
	if eventTime == nil {
		eventTime, _ = ExtractEventTime(ctx, m.llmClient, content)
	}

	// 1. Create memory record in PostgreSQL
	memory := &models.Memory{
		Content:         content,
		UserID:          userID,
		MemoryType:      memoryType,
		Category:        category,
		ImportanceScore: score,
		EventTime:       eventTime,
	}
	if metadata != nil {
		memory.Metadata = &metadata
	}

	if err := db.Create(memory).Error; err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}

	slog.Info("Memory created",
		"id", memory.ID,
		"user_id", userID,
		"type", memoryType,
		"category", category,
		"event_time", eventTime,
	)

	// Warm single-memory cache entry so GetMemory hits cache on first read.
	if m.cache != nil {
		cacheData := map[string]any{
			"id":               memory.ID.String(),
			"content":          memory.Content,
			"user_id":          memory.UserID,
			"memory_type":      memory.MemoryType,
			"category":         memory.Category,
			"importance_score": memory.ImportanceScore,
		}
		go m.cache.SetMemory(context.Background(), memory.ID.String(), cacheData)
	}

	// 2. Add to vector store (async — non-blocking)
	// 将 event_time 注入 metadata 以便搜索时 payload filter
	vsMetadata := metadata
	if eventTime != nil {
		if vsMetadata == nil {
			vsMetadata = map[string]any{}
		}
		vsMetadata["event_time"] = eventTime.Format(time.RFC3339)
	}
	go func() {
		if err := m.vectorStore.AddMemory(context.Background(), memory.ID, content, userID, memoryType, score, vsMetadata); err != nil {
			slog.Error("Failed to add memory to vector store", "error", err, "id", memory.ID)
		}
	}()

	// 3. Extract entities and store in graph (async)
	// Bi-temporal: 将 memory.EventTime 传播到新建实体
	go func() {
		if err := m.extractAndStoreEntities(context.Background(), db, content, userID, memory.ID, eventTime); err != nil {
			slog.Error("Entity extraction failed", "error", err, "id", memory.ID)
		}
	}()

	// 4. Insert into MemTree hierarchy (async)
	go func() {
		if _, err := m.treeManager.InsertMemory(context.Background(), db, memory); err != nil {
			slog.Error("MemTree insertion failed", "error", err, "id", memory.ID)
		}
	}()

	// 5. Check reflection trigger (only for observations)
	if memoryType == MemoryTypeObservation {
		go func() {
			m.checkReflectionTrigger(context.Background(), db, userID)
		}()
	}

	// 2b. Write to VFS (fs / hybrid mode, async)
	if (m.storageMode == StorageModeFS || m.storageMode == StorageModeHybrid) && m.fsStore != nil {
		go func() {
			m.writeMemoryToVFS(context.Background(), memory, content, userID, memoryType)
		}()
	}

	return memory, nil
}

// AsyncMemoryResult wraps the result of an async memory operation.
// O6 优化：参考 mem0 async mode，提供非阻塞记忆写入通道。
type AsyncMemoryResult struct {
	Memory *models.Memory
	Err    error
}

// AddMemoryAsync performs AddMemory in a background goroutine and returns a channel.
// Callers receive the result via the returned channel without blocking the request handler.
// The channel is buffered (size 1) and will be closed after the result is sent.
func (m *MemoryManager) AddMemoryAsync(
	ctx context.Context,
	db *gorm.DB,
	content, userID string,
	memoryType string,
	importanceScore *float64,
	metadata map[string]any,
) <-chan AsyncMemoryResult {
	ch := make(chan AsyncMemoryResult, 1)
	go func() {
		defer close(ch)
		mem, err := m.AddMemory(ctx, db, content, userID, memoryType, importanceScore, metadata)
		ch <- AsyncMemoryResult{Memory: mem, Err: err}
	}()
	return ch
}

// AddObservation is a convenience wrapper for AddMemory with type=observation.
func (m *MemoryManager) AddObservation(
	ctx context.Context,
	db *gorm.DB,
	content, userID string,
	metadata map[string]any,
) (*models.Memory, error) {
	return m.AddMemory(ctx, db, content, userID, MemoryTypeObservation, nil, metadata)
}

// SearchMemories performs hybrid semantic search over a user's memories.
func (m *MemoryManager) SearchMemories(
	ctx context.Context,
	db *gorm.DB,
	query, userID string,
	limit int,
	memoryTypes []string,
	minImportance float64,
	category string,
	eventFrom, eventTo *time.Time,
) ([]VectorSearchResult, error) {
	if query == "" {
		return nil, errors.New("query is required")
	}
	if limit <= 0 {
		limit = 5
	}

	// Read-through cache: return immediately on hit.
	var cKey string
	if m.cache != nil {
		cKey = searchCacheKeyFor(query, userID, limit, memoryTypes, minImportance, category, eventFrom, eventTo)
		if data, ok := m.cache.GetBytes(ctx, cKey); ok {
			var cached []VectorSearchResult
			if json.Unmarshal(data, &cached) == nil {
				slog.Debug("SearchMemories cache hit", "user_id", userID, "query", query)
				return cached, nil
			}
		}
	}

	// Fetch more results than limit if we need to filter by category
	fetchLimit := limit
	if category != "" {
		fetchLimit = limit * 3 // over-fetch to account for filtering
	}
	// Over-fetch more if event_time filtering is active
	if eventFrom != nil || eventTo != nil {
		fetchLimit = fetchLimit * 2
	}

	// 1. Vector search
	results, err := m.vectorStore.HybridSearch(ctx, query, userID, fetchLimit, memoryTypes, minImportance, category, nil, nil, eventFrom, eventTo)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}

	// 2. Filter by category if specified (enrich from DB)
	if category != "" && len(results) > 0 {
		ids := make([]uuid.UUID, len(results))
		for i, r := range results {
			ids[i] = r.MemoryID
		}

		var memories []models.Memory
		db.Where("id IN ? AND category = ?", ids, category).Find(&memories)
		allowed := make(map[uuid.UUID]string)
		for _, mem := range memories {
			allowed[mem.ID] = mem.Category
		}

		filtered := make([]VectorSearchResult, 0, limit)
		for _, r := range results {
			if cat, ok := allowed[r.MemoryID]; ok {
				r.Category = cat
				filtered = append(filtered, r)
				if len(filtered) >= limit {
					break
				}
			}
		}
		results = filtered
	}

	// 3. Update access count for returned memories
	if len(results) > 0 {
		ids := make([]uuid.UUID, len(results))
		for i, r := range results {
			ids[i] = r.MemoryID
		}
		go func() {
			db.Model(&models.Memory{}).Where("id IN ?", ids).
				UpdateColumn("access_count", gorm.Expr("access_count + 1"))
		}()
	}

	// OPT-5 on-read decay: compute effective importance and re-rank results.
	// This ensures search results reflect real-time memory freshness even between
	// batch decay runs, addressing the steepest decay curve for new memories (<24h).
	if len(results) > 0 {
		ids := make([]uuid.UUID, len(results))
		for i, r := range results {
			ids[i] = r.MemoryID
		}
		var memories []models.Memory
		if dbErr := db.Where("id IN ?", ids).
			Select("id, memory_type, user_id, importance_score, decay_factor, last_accessed_at, access_count").
			Find(&memories).Error; dbErr == nil && len(memories) > 0 {

			memMap := make(map[uuid.UUID]models.Memory, len(memories))
			for _, mem := range memories {
				memMap[mem.ID] = mem
			}

			now := time.Now().UTC()
			for i, r := range results {
				mem, ok := memMap[r.MemoryID]
				if !ok {
					continue
				}
				hl := GetAdaptiveHalfLife(db, mem.UserID, mem.MemoryType)
				effImportance := ComputeEffectiveImportance(
					mem.ImportanceScore, mem.DecayFactor,
					mem.LastAccessedAt, mem.AccessCount, now, hl,
				)
				// Hybrid score: 70% vector similarity + 30% effective importance
				results[i].Score = r.Score*0.7 + effImportance*0.3
			}

			// Re-sort by hybrid score descending
			for i := 0; i < len(results)-1; i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].Score > results[i].Score {
						results[i], results[j] = results[j], results[i]
					}
				}
			}
		}
	}

	// 4. VFS 融合搜索 (hybrid / fs 模式)
	if (m.storageMode == StorageModeHybrid || m.storageMode == StorageModeFS) && m.fsStore != nil {
		// Phase 5: prefer segment semantic search over keyword search
		if m.vfsSemIdx != nil {
			queryVec, embedErr := m.vectorStore.embedText(ctx, query)
			if embedErr == nil {
				fsHits, semErr := m.vfsSemIdx.SemanticSearchToFSHits(queryVec, limit)
				if semErr == nil && len(fsHits) > 0 {
					results = m.fuseSearchResults(results, fsHits, limit)
				}
			} else {
				slog.Warn("VFS 语义搜索 embedding 失败，回退关键词搜索", "error", embedErr)
				// Fallback to keyword search
				tenantID := middleware.TenantFromCtx(ctx)
				fsHits, fsErr := m.fsStore.SearchMemories(tenantID, userID, query, limit)
				if fsErr == nil && len(fsHits) > 0 {
					results = m.fuseSearchResults(results, fsHits, limit)
				}
			}
		} else {
			// Original keyword search path
			tenantID := middleware.TenantFromCtx(ctx)
			fsHits, fsErr := m.fsStore.SearchMemories(tenantID, userID, query, limit)
			if fsErr == nil && len(fsHits) > 0 {
				results = m.fuseSearchResults(results, fsHits, limit)
			}
		}

		// Phase 1 (渐进式加载): 批量填充 L0 摘要到搜索结果
		// 收集具有 VFS 可定位信息的结果，通过 BatchReadL0 批量获取摘要
		if len(results) > 0 {
			tenantID := middleware.TenantFromCtx(ctx)
			uris := make([]string, 0, len(results))
			uriIdx := make(map[string][]int) // URI -> result indices
			for i, r := range results {
				if r.MemoryType != "" && r.Category != "" {
					section, cat := memoryTypeToVFSPath(r.MemoryType)
					uri := fmt.Sprintf("%s/%s/%s", section, cat, r.MemoryID.String())
					uris = append(uris, uri)
					uriIdx[uri] = append(uriIdx[uri], i)
				}
			}
			if len(uris) > 0 {
				l0Entries, l0Err := m.fsStore.BatchReadL0(tenantID, userID, uris)
				if l0Err == nil {
					for _, entry := range l0Entries {
						if indices, ok := uriIdx[entry.URI]; ok {
							for _, idx := range indices {
								results[idx].L0Abstract = entry.L0Abstract
								// Phase 3: populate available levels based on memory type
								results[idx].AvailableLevels = AvailableLevelsForType(entry.MemoryType)
							}
						}
					}

					// Phase 2 (渐进式加载): TieredLoader LLM 筛选 + L1 按需加载
					if m.tieredLoader != nil && len(l0Entries) > 0 {
						results = m.applyTieredLoading(ctx, tenantID, userID, query, results, l0Entries, uriIdx)
					}
				} else {
					slog.Debug("L0 批量读取失败，搜索结果不含摘要", "error", l0Err)
				}
			}
		}
	}

	// Write-through cache: populate on miss (async to avoid blocking the response).
	if m.cache != nil && cKey != "" && len(results) > 0 {
		if data, err := json.Marshal(results); err == nil {
			go m.cache.SetBytes(context.Background(), cKey, data, 5*time.Minute)
		}
	}

	return results, nil
}

// SearchMemoriesForContext performs SearchMemories then compresses results for LLM consumption.
// Returns a single context string ready to be injected into an LLM prompt.
// maxTokens ≤ 0 uses the default budget (2000 tokens). Pass nil llm to skip LLM summarisation.
func (m *MemoryManager) SearchMemoriesForContext(
	ctx context.Context,
	db *gorm.DB,
	query, userID string,
	limit int,
	memoryTypes []string,
	minImportance float64,
	category string,
	eventFrom, eventTo *time.Time,
	maxTokens int,
) (string, error) {
	results, err := m.SearchMemories(ctx, db, query, userID, limit, memoryTypes, minImportance, category, eventFrom, eventTo)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}

	ctxMems := make([]ContextMemory, len(results))
	for i, r := range results {
		ctxMems[i] = ContextMemory{
			Content:    r.Content,
			Score:      r.Score,
			MemoryType: r.MemoryType,
		}
	}
	return CompressContext(ctx, ctxMems, query, m.llmClient, maxTokens), nil
}

// GetMemory retrieves a single memory by ID.
func (m *MemoryManager) GetMemory(db *gorm.DB, memoryID uuid.UUID) (*models.Memory, error) {
	var memory models.Memory
	result := db.Where("id = ?", memoryID).First(&memory)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get memory: %w", result.Error)
	}
	return &memory, nil
}

// DeleteMemory removes a memory from both DB and vector store.
func (m *MemoryManager) DeleteMemory(ctx context.Context, db *gorm.DB, memoryID uuid.UUID) error {
	// 1. Delete from DB
	result := db.Where("id = ?", memoryID).Delete(&models.Memory{})
	if result.Error != nil {
		return fmt.Errorf("delete memory: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("memory not found")
	}

	// Invalidate single-memory cache entry.
	if m.cache != nil {
		m.cache.InvalidateMemory(ctx, memoryID.String())
	}

	// 2. Delete from vector store (async)
	go func() {
		if err := m.vectorStore.DeleteMemory(ctx, memoryID); err != nil {
			slog.Error("Failed to delete memory from vector store", "error", err, "id", memoryID)
		}
	}()

	// 2b. Delete from VFS semantic index (async, Phase 5)
	if m.vfsSemIdx != nil {
		go func() {
			if err := m.vfsSemIdx.RemoveIndex(memoryID.String()); err != nil {
				slog.Error("VFS 语义索引删除失败", "error", err, "id", memoryID)
			}
		}()
	}

	// 3. Delete memory-entity links
	go func() {
		db.Where("memory_id = ?", memoryID).Delete(&models.MemoryEntityLink{})
	}()

	return nil
}

// TriggerReflection synthesizes recent high-importance memories into insights.
// N2 自编辑: 反思后 LLM 可主动编辑核心记忆 (persona/preferences/instructions)。
func (m *MemoryManager) TriggerReflection(ctx context.Context, db *gorm.DB, userID string) (*models.Memory, error) {
	// 1. Fetch recent high-importance memories
	var recentMemories []models.Memory
	err := db.Where("user_id = ? AND importance_score >= ?", userID, 0.7).
		Order("created_at DESC").
		Limit(10).
		Find(&recentMemories).Error
	if err != nil {
		return nil, fmt.Errorf("fetch recent memories: %w", err)
	}

	if len(recentMemories) < 3 {
		return nil, nil // Not enough memories for reflection
	}

	// 2. Extract memory content for LLM
	memoryTexts := make([]string, len(recentMemories))
	for i, mem := range recentMemories {
		memoryTexts[i] = mem.Content
	}

	// 3. Generate reflection via LLM
	if m.llmClient == nil {
		slog.Warn("LLMClient not initialized, skipping reflection")
		return nil, nil
	}

	// N2: 获取当前核心记忆上下文，注入 Prompt 以启用自编辑检测
	coreMemoryCtx := ""
	cm, cmErr := GetCoreMemory(db, userID)
	if cmErr == nil && cm != nil {
		coreMemoryCtx = fmt.Sprintf("Persona: %s\nPreferences: %s\nInstructions: %s",
			cm.Persona, cm.Preferences, cm.Instructions)
	}

	reflectionText, err := m.llmClient.GenerateReflection(ctx, memoryTexts, coreMemoryCtx)
	if err != nil {
		return nil, fmt.Errorf("generate reflection: %w", err)
	}
	if reflectionText == "" {
		slog.Warn("Reflection generation returned empty result", "user_id", userID)
		return nil, nil // No actionable result
	}

	// N2: 尝试解析结构化 JSON 响应 (reflection + core_memory_edits)
	var reflectionContent string
	var editsApplied int
	parsedJSON, parseErr := ParseLLMJSON(reflectionText)
	if parseErr == nil {
		// 成功解析 JSON — 提取 reflection 文本
		if r, ok := parsedJSON["reflection"].(string); ok && r != "" {
			reflectionContent = r
		} else {
			reflectionContent = reflectionText // fallback
		}

		// 提取并执行核心记忆编辑
		if editsRaw, ok := parsedJSON["core_memory_edits"]; ok {
			var edits []CoreMemoryEditAction
			editsJSON, _ := json.Marshal(editsRaw)
			if json.Unmarshal(editsJSON, &edits) == nil && len(edits) > 0 {
				applied, applyErr := ApplyCoreMemoryEdits(db, userID, "reflection", edits, false)
				if applyErr != nil {
					slog.Error("Core Memory 自编辑（反思）失败", "error", applyErr, "user_id", userID)
				}
				editsApplied = applied
			}
		}
	} else {
		// JSON 解析失败 — 纯文本反思（向后兼容）
		reflectionContent = reflectionText
	}

	slog.Info("Generated reflection",
		"user_id", userID,
		"memory_count", len(recentMemories),
		"reflection_len", len(reflectionContent),
		"core_memory_edits", editsApplied,
	)

	// 4. Store reflection as a new memory
	metadata := map[string]any{
		"source_memory_count":     len(recentMemories),
		"core_memory_edits_count": editsApplied,
	}
	reflectionMemory := &models.Memory{
		Content:         reflectionContent,
		UserID:          userID,
		MemoryType:      MemoryTypeReflection,
		Category:        CategoryInsight,
		ImportanceScore: 0.8, // Reflections are high importance by default
		Metadata:        &metadata,
	}

	if err := db.Create(reflectionMemory).Error; err != nil {
		return nil, fmt.Errorf("save reflection: %w", err)
	}

	// 5. Add reflection to vector store (async)
	go func() {
		if err := m.vectorStore.AddMemory(ctx, reflectionMemory.ID, reflectionContent, userID,
			MemoryTypeReflection, 0.8, nil); err != nil {
			slog.Error("Failed to add reflection to vector store", "error", err)
		}
	}()

	return reflectionMemory, nil
}

// extractAndStoreEntities uses LLM to extract entities from content.
// BUG-06 修复: 实体/关系写入在事务中执行，失败时自动 rollback。
// Bi-temporal: eventTime 参数会传播到新建的 Entity.EventTime。
func (m *MemoryManager) extractAndStoreEntities(
	ctx context.Context,
	db *gorm.DB,
	content, userID string,
	memoryID uuid.UUID,
	eventTime *time.Time,
) error {
	if m.llmClient == nil {
		slog.Debug("LLMClient not initialized, skipping entity extraction")
		return nil
	}

	// 1. Call LLM to extract entities and relations
	extractionResult, err := m.llmClient.ExtractEntities(ctx, content)
	if err != nil {
		slog.Error("Entity extraction failed", "error", err, "memory_id", memoryID)
		return nil // Non-fatal: LLM failure should not block memory creation
	}

	if len(extractionResult.Entities) == 0 {
		slog.Debug("No entities extracted", "memory_id", memoryID)
		return nil
	}

	slog.Info("Entities extracted",
		"memory_id", memoryID,
		"entity_count", len(extractionResult.Entities),
		"relation_count", len(extractionResult.Relations),
	)

	// 2. Persist entities + relations in a transaction (atomic rollback on failure)
	return db.Transaction(func(tx *gorm.DB) error {
		entityMap := make(map[string]uuid.UUID)
		for _, extracted := range extractionResult.Entities {
			entity := &models.Entity{
				Name:        extracted.Name,
				UserID:      userID,
				EntityType:  extracted.EntityType,
				Description: &extracted.Description,
				EventTime:   eventTime, // Bi-temporal: 从记忆传播
			}

			if err := m.graphStore.AddEntity(tx, entity); err != nil {
				return fmt.Errorf("add entity %q: %w", extracted.Name, err)
			}
			entityMap[extracted.Name] = entity.ID

			link := &models.MemoryEntityLink{
				MemoryID: memoryID,
				EntityID: entity.ID,
			}
			if err := tx.Create(link).Error; err != nil {
				return fmt.Errorf("create memory-entity link: %w", err)
			}
		}

		for _, extracted := range extractionResult.Relations {
			sourceID, sourceExists := entityMap[extracted.Source]
			targetID, targetExists := entityMap[extracted.Target]

			if !sourceExists || !targetExists {
				slog.Debug("Skipping relation - entity not found",
					"source", extracted.Source,
					"target", extracted.Target,
				)
				continue
			}

			relation := &models.Relation{
				SourceID:     sourceID,
				TargetID:     targetID,
				RelationType: extracted.RelationType,
				Weight:       1.0,
			}

			if err := m.graphStore.AddRelation(tx, relation); err != nil {
				return fmt.Errorf("add relation %q: %w", extracted.RelationType, err)
			}
		}

		return nil
	})
}

// classifyCategory determines the semantic category for a memory.
// Priority: 1) explicit metadata → 2) rule-based from type → 3) LLM classification.
func (m *MemoryManager) classifyCategory(
	ctx context.Context,
	content, memoryType string,
	metadata map[string]any,
) string {
	// 1. Explicit category from metadata takes precedence
	if metadata != nil {
		if cat, ok := metadata["category"].(string); ok && validCategories[cat] {
			return cat
		}
	}

	// 2. Derive from memory type for non-observation types
	switch memoryType {
	case MemoryTypeReflection:
		return CategoryInsight
	case MemoryTypePlan:
		return CategoryTask
	}

	// 3. Use LLM to classify observations and dialogues
	llm := GetLLMClient()
	if llm == nil {
		return CategoryFact // fallback if LLM unavailable
	}

	prompt := fmt.Sprintf(
		`Classify the following text into exactly ONE category.
Categories: preference, habit, profile, skill, relationship, event, opinion, fact, goal, task, reminder
Text: "%s"
Respond with ONLY the category name, nothing else.`, content)

	result, err := llm.Generate(ctx, prompt)
	if err != nil {
		slog.Warn("LLM category classification failed, using default", "error", err)
		return CategoryFact
	}

	// Validate LLM response
	category := strings.TrimSpace(strings.ToLower(result))
	if validCategories[category] {
		return category
	}

	slog.Warn("LLM returned invalid category, using default", "raw", result)
	return CategoryFact
}

// checkReflectionTrigger checks if automatic reflection should be triggered.
func (m *MemoryManager) checkReflectionTrigger(ctx context.Context, db *gorm.DB, userID string) {
	// Count recent observations
	var count int64
	db.Model(&models.Memory{}).
		Where("user_id = ? AND memory_type = ?", userID, MemoryTypeObservation).
		Count(&count)

	// Trigger reflection every 5 observations
	if count > 0 && count%5 == 0 {
		slog.Info("Reflection trigger threshold reached", "user_id", userID, "count", count)
		if _, err := m.TriggerReflection(ctx, db, userID); err != nil {
			slog.Error("Auto-reflection failed", "error", err, "user_id", userID)
		}
	}
}

// --- VFS integration helpers ---

// memoryTypeToVFSPath maps memoryType to VFS (section, category).
func memoryTypeToVFSPath(memoryType string) (section, category string) {
	switch memoryType {
	case MemoryTypeObservation, MemoryTypeEpisodic:
		return "episodic", "observations"
	case MemoryTypeDialogue:
		return "episodic", "dialogues"
	case MemoryTypeReflection, MemoryTypeSemantic:
		return "semantic", "reflections"
	case MemoryTypePlan, MemoryTypeProcedural:
		return "semantic", "knowledge"
	default:
		// Unknown types → semantic/knowledge as fallback
		return "semantic", "knowledge"
	}
}

// writeMemoryToVFS writes a memory to VFS based on its type.
// Permanent memories are handled by MemoryArchiver, not here.
func (m *MemoryManager) writeMemoryToVFS(
	ctx context.Context,
	memory *models.Memory,
	content, userID, memoryType string,
) {
	// Skip permanent/imagination — those are handled by their own services
	if memoryType == MemoryTypePermanent || memoryType == MemoryTypeImagination {
		return
	}

	section, category := memoryTypeToVFSPath(memoryType)
	tenantID := middleware.TenantFromCtx(ctx)

	// Generate L0: first 80 chars of content
	l0 := content
	if len(l0) > 80 {
		l0 = l0[:80] + "…"
	}

	if err := m.fsStore.WriteMemoryTo(
		ctx, tenantID, userID, memory.ID,
		section, category, content, l0, content,
	); err != nil {
		slog.Error("VFS 记忆写入失败",
			"error", err,
			"memory_id", memory.ID,
			"section", section,
			"category", category,
		)
		return
	}

	// Phase 5: 同步建立 VFS 语义索引 (content → segment upsert with URI)
	if m.vfsSemIdx != nil {
		if err := m.vfsSemIdx.IndexContent(
			ctx,
			memory.ID.String(), tenantID, userID,
			section, category, content,
		); err != nil {
			slog.Error("VFS 语义索引建立失败",
				"error", err,
				"memory_id", memory.ID,
				"section", section,
			)
		}
	}
}

// applyTieredLoading executes the Phase 2 tiered loading pipeline:
//  1. Classify each L0 entry by memory tier policy
//  2. AlwaysL1 entries → go directly to L1 read list
//  3. Standard entries → LLM filters Top-K
//  4. L0Only entries → skip (imagination memories stay at L0)
//  5. BatchReadL1 for all selected URIs → populate L1Overview in results
func (m *MemoryManager) applyTieredLoading(
	ctx context.Context,
	tenantID, userID, query string,
	results []VectorSearchResult,
	l0Entries []L0Entry,
	uriIdx map[string][]int,
) []VectorSearchResult {
	// Classify entries into three groups by tier policy.
	var alwaysL1URIs []string
	var standardEntries []L0Entry

	for _, entry := range l0Entries {
		policy := ClassifyMemoryTier(entry.MemoryType)
		switch policy {
		case TierAlwaysL1:
			alwaysL1URIs = append(alwaysL1URIs, entry.URI)
		case TierL0Only:
			// Skip — imagination memories stay at L0 level only
			continue
		default: // TierStandard
			standardEntries = append(standardEntries, entry)
		}
	}

	// LLM filter the standard entries to get Top-K.
	var filteredURIs []string
	if len(standardEntries) > 0 {
		var filterErr error
		filteredURIs, filterErr = m.tieredLoader.FilterByL0(ctx, query, standardEntries)
		if filterErr != nil {
			slog.Warn("TieredLoader L0 筛选出错，回退全标准条目",
				"error", filterErr)
			filteredURIs = allURIs(standardEntries)
		}
	}

	// Combine: alwaysL1 + filtered standard URIs.
	l1URIs := make([]string, 0, len(alwaysL1URIs)+len(filteredURIs))
	l1URIs = append(l1URIs, alwaysL1URIs...)
	l1URIs = append(l1URIs, filteredURIs...)

	if len(l1URIs) == 0 {
		return results
	}

	// BatchReadL1 for the combined URI list.
	l1Entries, l1Err := m.tieredLoader.fsStore.BatchReadL1(tenantID, userID, l1URIs)
	if l1Err != nil {
		slog.Warn("BatchReadL1 失败，跳过 L1 填充", "error", l1Err)
		return results
	}

	// Phase 3: apply token budget before populating L1 overviews.
	l1Entries = m.tieredLoader.ApplyTokenBudget(l1Entries)

	// Populate L1Overview into results.
	for _, l1Entry := range l1Entries {
		if indices, ok := uriIdx[l1Entry.URI]; ok {
			for _, idx := range indices {
				if idx < len(results) {
					results[idx].L1Overview = l1Entry.L1Overview
				}
			}
		}
	}

	slog.Info("Phase 2 分层加载完成",
		"alwaysL1", len(alwaysL1URIs),
		"standard_filtered", len(filteredURIs),
		"l0_only_skipped", len(l0Entries)-len(alwaysL1URIs)-len(standardEntries),
		"l1_loaded", len(l1Entries),
	)

	return results
}

// fuseSearchResults merges vector search results with VFS keyword search hits
// using Reciprocal Rank Fusion (RRF): score(d) = Σ 1/(k + rank(d)), k=60.
func (m *MemoryManager) fuseSearchResults(
	vectorResults []VectorSearchResult,
	fsHits []SearchHit,
	limit int,
) []VectorSearchResult {
	const k = 60.0

	type fusedItem struct {
		result   VectorSearchResult
		rrfScore float64
	}

	scoreMap := make(map[string]*fusedItem)

	// Score vector results by rank
	for i, r := range vectorResults {
		key := r.MemoryID.String()
		scoreMap[key] = &fusedItem{
			result:   r,
			rrfScore: 1.0 / (k + float64(i+1)),
		}
	}

	// Add VFS results by rank (contribute to RRF score)
	for i, hit := range fsHits {
		key := hit.MemoryID
		rrfContrib := 1.0 / (k + float64(i+1))
		if item, ok := scoreMap[key]; ok {
			// Memory found in both — add scores
			item.rrfScore += rrfContrib
		} else {
			// VFS-only result — create a placeholder VectorSearchResult
			uid, err := uuid.Parse(key)
			if err != nil {
				continue
			}
			scoreMap[key] = &fusedItem{
				result: VectorSearchResult{
					MemoryID:   uid,
					Content:    hit.L0Abstract,
					Score:      hit.Score,
					MemoryType: "", // Will be enriched from DB if needed
					Category:   hit.Category,
				},
				rrfScore: rrfContrib,
			}
		}
	}

	// Collect and sort by RRF score
	items := make([]fusedItem, 0, len(scoreMap))
	for _, item := range scoreMap {
		items = append(items, *item)
	}

	// Sort descending by rrfScore
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].rrfScore > items[i].rrfScore {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Truncate and collect
	if len(items) > limit {
		items = items[:limit]
	}
	results := make([]VectorSearchResult, len(items))
	for i, item := range items {
		results[i] = item.result
	}
	return results
}

// --- Singleton ---

// --- Phase 3: L2 Detail Reader ---

// MemoryDetailResult holds the response for a detail-level memory read.
type MemoryDetailResult struct {
	MemoryID string `json:"memory_id"`
	Level    int    `json:"level"`
	Content  string `json:"content"`
	Type     string `json:"memory_type"`
	Category string `json:"category"`
}

// GetMemoryDetail reads a specific tier level for a memory.
// level: 0 = L0 abstract, 1 = L1 overview, 2 = L2 full content.
// Returns ErrImagineL2Blocked if an imagination memory requests L2.
func (m *MemoryManager) GetMemoryDetail(
	tenantID, userID, memoryID, memoryType, category string,
	level int,
) (*MemoryDetailResult, error) {
	if m.fsStore == nil {
		return nil, errors.New("FSStoreService not available")
	}

	// Imagination memories cannot expand beyond L0.
	if memoryType == MemoryTypeImagination && level > 0 {
		return nil, ErrImagineL2Blocked
	}

	// Build VFS path from memory type
	section, cat := memoryTypeToVFSPath(memoryType)
	if category != "" {
		cat = category
	}
	path := fmt.Sprintf("%s/%s/%s", section, cat, memoryID)

	content, err := m.fsStore.ReadMemory(tenantID, userID, path, level)
	if err != nil {
		return nil, fmt.Errorf("read memory detail: %w", err)
	}

	return &MemoryDetailResult{
		MemoryID: memoryID,
		Level:    level,
		Content:  content,
		Type:     memoryType,
		Category: cat,
	}, nil
}

// ErrImagineL2Blocked is returned when an imagination memory is requested beyond L0.
var ErrImagineL2Blocked = errors.New("imagination memories cannot be expanded beyond L0")

// --- Singleton ---

var (
	memManagerOnce    sync.Once
	memManagerService *MemoryManager
)

// GetMemoryManager returns the singleton MemoryManager.
func GetMemoryManager() *MemoryManager {
	memManagerOnce.Do(func() {
		memManagerService = NewMemoryManager(GetVectorStore(), GetGraphStore(), GetLLMProvider())
	})
	return memManagerService
}
