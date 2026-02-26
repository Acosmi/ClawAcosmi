// Package services — Knowledge graph service.
// Mirrors Python services/graph_store.py — entity/relation CRUD with community detection.
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// RUST_CANDIDATE: graph_algorithms — 图算法后续迁移 Rust

// GraphEngine defines the abstract interface for graph storage backends.
// Current implementation: PostgreSQL adjacency table (GraphStoreService).
// Future candidates: Neo4j, ArangoDB, PostgreSQL+Apache AGE.
// Phase 15-B: 接口抽象，允许后续无缝切换图存储引擎。
type GraphEngine interface {
	AddEntity(db *gorm.DB, entity *models.Entity) error
	AddRelation(db *gorm.DB, relation *models.Relation) error
	GetEntityByName(db *gorm.DB, name, userID string) (*models.Entity, error)
	GetGraph(db *gorm.DB, userID string) ([]models.Entity, []models.Relation, error)
	QueryGraphAsOf(db *gorm.DB, userID string, asOf time.Time) (*TemporalQueryResult, error)
	QueryGraphByTimeRange(db *gorm.DB, userID string, from, to time.Time) (*TemporalQueryResult, error)
	QueryRecentEntities(db *gorm.DB, userID, entityType string, limit int) ([]models.Entity, error)
	DeleteEntity(db *gorm.DB, entityID uuid.UUID) error
	RunCommunityDetection(db *gorm.DB, userID string) (map[uuid.UUID]int, error)
	MergeEntities(db *gorm.DB, targetID, sourceID uuid.UUID) error
	FindSimilarEntities(db *gorm.DB, userID string) ([][2]models.Entity, error)
	GetTrendingEntities(db *gorm.DB, userID string, minDaysSpan int, topN int) ([]TrendingEntity, error)
}

// Compile-time check: GraphStoreService implements GraphEngine.
var _ GraphEngine = (*GraphStoreService)(nil)

// graphCacheEntry holds cached graph data with TTL.
type graphCacheEntry struct {
	entities  []models.Entity
	relations []models.Relation
	cachedAt  time.Time
}

const (
	graphCacheMaxUsers = 100
	graphCacheTTL      = 5 * time.Minute
)

// GraphStoreService manages the knowledge graph in PostgreSQL.
// Per-user entity and relation CRUD with LRU cache and community detection.
type GraphStoreService struct {
	mu    sync.RWMutex
	cache map[string]*graphCacheEntry // userID → cached graph
	vs    *VectorStoreService         // optional: for ALG-OPT-03 semantic entity dedup
	llm   LLMProvider                 // optional: for ALG-OPT-04 semantic conflict detection
}

// WithVectorStore attaches a VectorStoreService for semantic entity deduplication.
func (g *GraphStoreService) WithVectorStore(vs *VectorStoreService) *GraphStoreService {
	g.vs = vs
	return g
}

// WithLLM attaches an LLMProvider for semantic conflict detection (ALG-OPT-04).
func (g *GraphStoreService) WithLLM(llm LLMProvider) *GraphStoreService {
	g.llm = llm
	return g
}

// invalidateCache removes a user's graph cache entry.
func (g *GraphStoreService) invalidateCache(userID string) {
	g.mu.Lock()
	delete(g.cache, userID)
	g.mu.Unlock()
}

// AddEntity creates a new entity (graph node) in the database.
func (g *GraphStoreService) AddEntity(db *gorm.DB, entity *models.Entity) error {
	if entity.Name == "" {
		return errors.New("entity name is required")
	}
	if entity.UserID == "" {
		return errors.New("user_id is required")
	}

	// Check for duplicate by name + user
	var existing models.Entity
	result := db.Where("name = ? AND user_id = ?", entity.Name, entity.UserID).First(&existing)
	if result.Error == nil {
		// Entity exists — update description and metadata
		updates := map[string]any{}
		if entity.Description != nil {
			updates["description"] = *entity.Description
		}
		if entity.Metadata != nil {
			updates["metadata"] = entity.Metadata
		}
		if len(updates) > 0 {
			if err := db.Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update entity (dedup): %w", err)
			}
		}
		entity.ID = existing.ID
		slog.Debug("Entity updated (dedup)", "name", entity.Name, "id", existing.ID)
		return nil
	}

	if err := db.Create(entity).Error; err != nil {
		return fmt.Errorf("create entity: %w", err)
	}
	slog.Debug("Entity created", "name", entity.Name, "id", entity.ID)
	g.invalidateCache(entity.UserID)
	return nil
}

// AddRelation creates a new relation (graph edge) between two entities.
// Includes conflict detection: if a contradictory relation exists between the
// same source and target, the older one is soft-deleted (time-priority strategy).
func (g *GraphStoreService) AddRelation(db *gorm.DB, relation *models.Relation) error {
	if relation.SourceID == uuid.Nil || relation.TargetID == uuid.Nil {
		return errors.New("source_id and target_id are required")
	}

	// Check for duplicate relation
	var existing models.Relation
	result := db.Where(
		"source_id = ? AND target_id = ? AND relation_type = ?",
		relation.SourceID, relation.TargetID, relation.RelationType,
	).First(&existing)

	if result.Error == nil {
		// Relation exists — update weight
		if err := db.Model(&existing).Update("weight", relation.Weight).Error; err != nil {
			return fmt.Errorf("update relation weight (dedup): %w", err)
		}
		relation.ID = existing.ID
		slog.Debug("Relation updated (dedup)", "type", relation.RelationType, "id", existing.ID)
		return nil
	}

	// ALG-OPT-04: LLM semantic conflict detection — 完全替代硬编码矛盾对
	// 对同实体对已有关系逐一进行 NLI 语义矛盾判断
	// 无 LLM 时跳过冲突检测（安全降级）
	if g.llm != nil {
		var existing []models.Relation
		err := db.Where(
			"source_id = ? AND target_id = ? AND valid_until IS NULL",
			relation.SourceID, relation.TargetID,
		).Find(&existing).Error
		if err == nil {
			for _, ex := range existing {
				if ex.RelationType == relation.RelationType {
					continue // same type, not a conflict
				}
				isConflict := g.detectConflictLLM(relation.RelationType, ex.RelationType)
				if isConflict {
					now := time.Now()
					db.Model(&ex).Update("valid_until", now)
					slog.Info("Relation conflict resolved (LLM NLI)",
						"old_type", ex.RelationType, "new_type", relation.RelationType,
						"old_id", ex.ID,
					)
					break // resolve at most one conflict per AddRelation
				}
			}
		}
	}

	if err := db.Create(relation).Error; err != nil {
		return fmt.Errorf("create relation: %w", err)
	}
	slog.Debug("Relation created", "type", relation.RelationType, "id", relation.ID)
	return nil
}

// detectConflictLLM uses LLM NLI to determine if two relation types are semantically contradictory.
// ALG-OPT-04: Returns true if LLM judges them as contradictory.
// On LLM failure/timeout, returns false (safe fallback: no conflict resolved).
func (g *GraphStoreService) detectConflictLLM(newType, existingType string) bool {
	if g.llm == nil {
		return false
	}

	prompt := fmt.Sprintf(
		`Judge whether these two knowledge graph relation types are semantically contradictory:
  Relation A: %s
  Relation B: %s

Two relations are contradictory if they express opposite or mutually exclusive meanings between the same entity pair.
Examples: LIKES vs DISLIKES (contradictory), SUPPORTS vs OPPOSES (contradictory), KNOWS vs LIKES (not contradictory).

Answer with exactly one word: YES or NO.`,
		newType, existingType)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		slog.Debug("OPT-04 LLM conflict detection failed, fallback to safe pass", "error", err)
		return false
	}

	answer := strings.TrimSpace(strings.ToUpper(result))
	return answer == "YES"
}

// GetEntityByName finds an entity by name for a specific user.
func (g *GraphStoreService) GetEntityByName(db *gorm.DB, name, userID string) (*models.Entity, error) {
	var entity models.Entity
	result := db.Where("name = ? AND user_id = ?", name, userID).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get entity: %w", result.Error)
	}
	return &entity, nil
}

// GetGraph returns all entities and relations for a user (with LRU cache).
func (g *GraphStoreService) GetGraph(db *gorm.DB, userID string) ([]models.Entity, []models.Relation, error) {
	g.mu.RLock()
	if g.cache != nil {
		if entry, ok := g.cache[userID]; ok && time.Since(entry.cachedAt) < graphCacheTTL {
			g.mu.RUnlock()
			return entry.entities, entry.relations, nil
		}
	}
	g.mu.RUnlock()

	var entities []models.Entity
	if err := db.Where("user_id = ?", userID).Find(&entities).Error; err != nil {
		return nil, nil, fmt.Errorf("fetch entities: %w", err)
	}

	entityIDs := make([]uuid.UUID, len(entities))
	for i, e := range entities {
		entityIDs[i] = e.ID
	}

	var relations []models.Relation
	if len(entityIDs) > 0 {
		if err := db.Where("(source_id IN ? OR target_id IN ?) AND (valid_until IS NULL OR valid_until > ?)",
			entityIDs, entityIDs, time.Now()).
			Find(&relations).Error; err != nil {
			return nil, nil, fmt.Errorf("fetch relations: %w", err)
		}
	}

	// 缓存结果（LRU 驱逐）
	g.mu.Lock()
	if g.cache == nil {
		g.cache = make(map[string]*graphCacheEntry)
	}
	if len(g.cache) >= graphCacheMaxUsers {
		// 简单驱逐：清除最旧的条目
		var oldestKey string
		var oldestTime time.Time
		for k, v := range g.cache {
			if oldestKey == "" || v.cachedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.cachedAt
			}
		}
		delete(g.cache, oldestKey)
	}
	g.cache[userID] = &graphCacheEntry{entities: entities, relations: relations, cachedAt: time.Now()}
	g.mu.Unlock()

	return entities, relations, nil
}

// DeleteEntity removes an entity and its associated relations.
func (g *GraphStoreService) DeleteEntity(db *gorm.DB, entityID uuid.UUID) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Delete related relations first
		if err := tx.Where("source_id = ? OR target_id = ?", entityID, entityID).
			Delete(&models.Relation{}).Error; err != nil {
			return fmt.Errorf("delete relations: %w", err)
		}

		// Delete entity
		if err := tx.Where("id = ?", entityID).Delete(&models.Entity{}).Error; err != nil {
			return fmt.Errorf("delete entity: %w", err)
		}

		return nil
	})
}

// RunCommunityDetection runs Label Propagation community detection on a user's graph.
// Each node starts with its own label; iteratively adopts the most frequent
// neighbor label (weighted by relation weight). Converges in ≤10 iterations.
// Returns a mapping of entity_id → community_id.
func (g *GraphStoreService) RunCommunityDetection(db *gorm.DB, userID string) (map[uuid.UUID]int, error) {
	entities, relations, err := g.GetGraph(db, userID)
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return map[uuid.UUID]int{}, nil
	}

	// Initialize: each entity gets a unique label
	label := make(map[uuid.UUID]int, len(entities))
	for i, e := range entities {
		label[e.ID] = i
	}

	// Build weighted adjacency list
	type neighbor struct {
		id     uuid.UUID
		weight float64
	}
	adj := make(map[uuid.UUID][]neighbor)
	for _, r := range relations {
		w := r.Weight
		if w <= 0 {
			w = 1.0
		}
		adj[r.SourceID] = append(adj[r.SourceID], neighbor{r.TargetID, w})
		adj[r.TargetID] = append(adj[r.TargetID], neighbor{r.SourceID, w})
	}

	// Label Propagation: max 10 iterations
	// ALG-OPT-02: 异步模式 — 每轮随机打乱遍历顺序以加速收敛
	// 对标 NetworkX asyn_lpa_communities() (NI-LPA arXiv 2024, ELP NIH PMC 2025)
	const maxIter = 10
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		rng.Shuffle(len(entities), func(i, j int) {
			entities[i], entities[j] = entities[j], entities[i]
		})
		for _, e := range entities {
			neighbors := adj[e.ID]
			if len(neighbors) == 0 {
				continue
			}
			// Weighted vote for each label
			votes := make(map[int]float64)
			for _, n := range neighbors {
				votes[label[n.id]] += n.weight
			}
			// Find label with max weight
			bestLabel := label[e.ID]
			bestWeight := 0.0
			for l, w := range votes {
				if w > bestWeight {
					bestWeight = w
					bestLabel = l
				}
			}
			if bestLabel != label[e.ID] {
				label[e.ID] = bestLabel
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Compact labels to 0..N-1
	labelMap := make(map[int]int)
	nextID := 0
	communities := make(map[uuid.UUID]int, len(entities))
	for _, e := range entities {
		raw := label[e.ID]
		if _, ok := labelMap[raw]; !ok {
			labelMap[raw] = nextID
			nextID++
		}
		communities[e.ID] = labelMap[raw]
	}

	// Persist community IDs
	for eid, cid := range communities {
		if err := db.Model(&models.Entity{}).Where("id = ?", eid).Update("community_id", cid).Error; err != nil {
			slog.Warn("Failed to persist community_id", "entity_id", eid, "error", err)
		}
	}

	slog.Info("Label Propagation community detection completed",
		"user_id", userID,
		"entities", len(entities),
		"communities", nextID,
	)

	return communities, nil
}

// CommunityInfo 表示一个社区的信息。
type CommunityInfo struct {
	CommunityID int             `json:"community_id"`
	Entities    []models.Entity `json:"entities"`
	Summary     string          `json:"summary,omitempty"`
}

// GetCommunities returns entities grouped by community_id for a user.
func (g *GraphStoreService) GetCommunities(db *gorm.DB, userID string) ([]CommunityInfo, error) {
	var entities []models.Entity
	if err := db.Where("user_id = ? AND community_id IS NOT NULL", userID).
		Order("community_id ASC").Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("fetch community entities: %w", err)
	}

	groups := make(map[int][]models.Entity)
	for _, e := range entities {
		if e.CommunityID != nil {
			groups[*e.CommunityID] = append(groups[*e.CommunityID], e)
		}
	}

	result := make([]CommunityInfo, 0, len(groups))
	for cid, ents := range groups {
		result = append(result, CommunityInfo{CommunityID: cid, Entities: ents})
	}
	return result, nil
}

// GenerateCommunitySummary uses LLM to create a semantic summary of a community.
func (g *GraphStoreService) GenerateCommunitySummary(
	ctx context.Context, db *gorm.DB,
	userID string, communityID int, llm LLMProvider,
) (string, error) {
	var entities []models.Entity
	if err := db.Where("user_id = ? AND community_id = ?", userID, communityID).
		Find(&entities).Error; err != nil {
		return "", fmt.Errorf("fetch community: %w", err)
	}
	if len(entities) == 0 {
		return "", errors.New("empty community")
	}

	// Build description text for LLM
	desc := ""
	for _, e := range entities {
		d := ""
		if e.Description != nil {
			d = *e.Description
		}
		desc += fmt.Sprintf("- %s (%s): %s\n", e.Name, e.EntityType, d)
	}

	prompt := fmt.Sprintf(
		"Below is a cluster of related entities from a knowledge graph.\n\n%s\nSummarize this cluster in 1-2 sentences. What theme or topic connects these entities?",
		desc)

	summary, err := llm.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM summary: %w", err)
	}
	return strings.TrimSpace(summary), nil
}

// --- 知识图谱升级：关系衰减 + 实体合并 ---

// DecayRelationWeights applies time-based exponential decay to all relation weights.
// 衰减公式: new_weight = weight * decay_factor^(days_since_creation)
// Relations below minWeight are deleted to prune stale connections.
func (g *GraphStoreService) DecayRelationWeights(db *gorm.DB, userID string, decayFactor, minWeight float64) (updated, pruned int, err error) {
	if decayFactor <= 0 || decayFactor >= 1 {
		decayFactor = 0.995 // 默认每天衰减 0.5%
	}
	if minWeight <= 0 {
		minWeight = 0.1
	}

	entities, relations, err := g.GetGraph(db, userID)
	if err != nil {
		return 0, 0, err
	}
	if len(entities) == 0 {
		return 0, 0, nil
	}

	now := time.Now()
	for _, r := range relations {
		daysSinceCreation := now.Sub(r.CreatedAt).Hours() / 24
		if daysSinceCreation < 1 {
			continue // 不衰减当天创建的关系
		}

		newWeight := r.Weight * math.Pow(decayFactor, daysSinceCreation)
		if newWeight < minWeight {
			// 权重过低，删除关系
			db.Where("id = ?", r.ID).Delete(&models.Relation{})
			pruned++
		} else if newWeight != r.Weight {
			db.Model(&models.Relation{}).Where("id = ?", r.ID).Update("weight", newWeight)
			updated++
		}
	}

	slog.Info("Relation weight decay completed",
		"user_id", userID,
		"updated", updated,
		"pruned", pruned,
		"total", len(relations),
	)
	return updated, pruned, nil
}

// MergeEntities merges sourceEntity into targetEntity.
// All relations of sourceEntity are migrated to targetEntity, duplicates are consolidated.
// sourceEntity is deleted after merge.
func (g *GraphStoreService) MergeEntities(db *gorm.DB, targetID, sourceID uuid.UUID) error {
	if targetID == sourceID {
		return errors.New("cannot merge entity with itself")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// 验证两个实体都存在
		var target, source models.Entity
		if err := tx.First(&target, "id = ?", targetID).Error; err != nil {
			return fmt.Errorf("target entity not found: %w", err)
		}
		if err := tx.First(&source, "id = ?", sourceID).Error; err != nil {
			return fmt.Errorf("source entity not found: %w", err)
		}

		// 迁移 source 的出向关系 → target
		tx.Model(&models.Relation{}).
			Where("source_id = ? AND target_id != ?", sourceID, targetID).
			Update("source_id", targetID)

		// 迁移 source 的入向关系 → target
		tx.Model(&models.Relation{}).
			Where("target_id = ? AND source_id != ?", sourceID, targetID).
			Update("target_id", targetID)

		// 删除 source↔target 之间的直接关系（合并后无意义）
		tx.Where("(source_id = ? AND target_id = ?) OR (source_id = ? AND target_id = ?)",
			sourceID, targetID, targetID, sourceID).Delete(&models.Relation{})

		// 合并重复关系（同一 source→target→type 只保留最高权重）
		g.deduplicateRelations(tx, targetID)

		// 迁移 memory-entity 关联
		tx.Model(&models.MemoryEntityLink{}).
			Where("entity_id = ?", sourceID).
			Update("entity_id", targetID)

		// 合并描述信息
		if source.Description != nil && *source.Description != "" {
			if target.Description == nil || *target.Description == "" {
				tx.Model(&target).Update("description", *source.Description)
			} else {
				merged := *target.Description + "\n" + *source.Description
				tx.Model(&target).Update("description", merged)
			}
		}

		// 删除 source 实体
		if err := tx.Delete(&source).Error; err != nil {
			return fmt.Errorf("delete source entity: %w", err)
		}

		slog.Info("Entities merged",
			"target", target.Name,
			"source", source.Name,
			"target_id", targetID,
		)
		return nil
	})
}

// deduplicateRelations consolidates duplicate relations for an entity.
// When multiple relations have the same source+target+type, keeps the one with highest weight.
func (g *GraphStoreService) deduplicateRelations(tx *gorm.DB, entityID uuid.UUID) {
	var relations []models.Relation
	tx.Where("source_id = ? OR target_id = ?", entityID, entityID).Find(&relations)

	// Group by (source, target, type)
	type relKey struct {
		Source uuid.UUID
		Target uuid.UUID
		Type   string
	}
	groups := make(map[relKey][]models.Relation)
	for _, r := range relations {
		key := relKey{r.SourceID, r.TargetID, r.RelationType}
		groups[key] = append(groups[key], r)
	}

	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}
		// 保留权重最高的，删除其余
		bestIdx := 0
		for i, r := range group {
			if r.Weight > group[bestIdx].Weight {
				bestIdx = i
			}
		}
		for i, r := range group {
			if i != bestIdx {
				tx.Where("id = ?", r.ID).Delete(&models.Relation{})
			}
		}
	}
}

// ALG-OPT-03 统一阈值常量
const (
	SimilarityThreshold = 0.85 // cosine ≥ 0.85 → 确认重复
	SuspectThreshold    = 0.70 // cosine ∈ [0.70, 0.85) → 疑似重复
)

// FindSimilarEntities finds candidate entity pairs that may be duplicates.
// Phase A: case-insensitive substring matching (fast filter).
// Phase B (ALG-OPT-03): if VectorStoreService is available, verify with cosine similarity.
//   - cosine ≥ SimilarityThreshold → confirmed duplicate
//   - cosine ∈ [SuspectThreshold, SimilarityThreshold) → suspect (仍返回但日志标记)
//   - cosine < SuspectThreshold → rejected (false positive from substring match)
func (g *GraphStoreService) FindSimilarEntities(db *gorm.DB, userID string) ([][2]models.Entity, error) {
	var entities []models.Entity
	// Limit(500) 保护：防止 O(n²) 比较在大量实体时性能爆炸
	if err := db.Where("user_id = ?", userID).Order("name ASC").Limit(500).Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("fetch entities: %w", err)
	}

	// Phase A: 子串初筛
	type candidate struct {
		i, j int
	}
	var candidates []candidate
	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			nameI := strings.ToLower(entities[i].Name)
			nameJ := strings.ToLower(entities[j].Name)

			if nameI == nameJ ||
				strings.Contains(nameI, nameJ) ||
				strings.Contains(nameJ, nameI) {
				candidates = append(candidates, candidate{i, j})
			}
		}
	}

	// Phase B: 向量验证 (ALG-OPT-03)
	var pairs [][2]models.Entity
	if g.vs != nil && g.vs.GetEmbeddingSvc() != nil && len(candidates) > 0 {
		ctx := context.Background()
		for _, c := range candidates {
			score, err := g.vs.ComputeCosineSimilarity(ctx, entities[c.i].Name, entities[c.j].Name)
			if err != nil {
				// Embedding 失败 → fallback 保留该候选对
				slog.Debug("OPT-03 cosine fallback", "error", err,
					"entity_a", entities[c.i].Name, "entity_b", entities[c.j].Name)
				pairs = append(pairs, [2]models.Entity{entities[c.i], entities[c.j]})
				continue
			}
			if score >= SuspectThreshold {
				if score < SimilarityThreshold {
					slog.Info("OPT-03 suspect duplicate",
						"entity_a", entities[c.i].Name,
						"entity_b", entities[c.j].Name,
						"cosine", fmt.Sprintf("%.3f", score))
				}
				pairs = append(pairs, [2]models.Entity{entities[c.i], entities[c.j]})
			}
			// cosine < SuspectThreshold → rejected
		}
	} else {
		// 无向量服务 → 退化为纯子串匹配
		for _, c := range candidates {
			pairs = append(pairs, [2]models.Entity{entities[c.i], entities[c.j]})
		}
	}

	return pairs, nil
}

// --- 时序查询 (Phase 15-A) ---

// TemporalQueryResult wraps temporal graph query output with metadata.
type TemporalQueryResult struct {
	Entities       []models.Entity   `json:"entities"`
	Relations      []models.Relation `json:"relations"`
	AsOf           *time.Time        `json:"as_of,omitempty"`
	TimeFrom       *time.Time        `json:"time_from,omitempty"`
	TimeTo         *time.Time        `json:"time_to,omitempty"`
	IngestedAfter  *time.Time        `json:"ingested_after,omitempty"`  // Bitemporal: ingestion lower bound
	IngestedBefore *time.Time        `json:"ingested_before,omitempty"` // Bitemporal: ingestion upper bound
}

// QueryGraphAsOf returns a time-slice of the knowledge graph —
// only entities and relations that were valid at the given point in time.
// 参考 Zep/Graphiti Episode 时序模型。
//
// Entity validity: (valid_from IS NULL OR valid_from <= asOf)
//
//	AND (valid_until IS NULL OR valid_until > asOf)
//
// Relation validity: same logic on valid_from/valid_until.
func (g *GraphStoreService) QueryGraphAsOf(db *gorm.DB, userID string, asOf time.Time) (*TemporalQueryResult, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if asOf.IsZero() {
		return nil, errors.New("asOf time is required")
	}

	var entities []models.Entity
	err := db.Where(
		"user_id = ? AND (valid_from IS NULL OR valid_from <= ?) AND (valid_until IS NULL OR valid_until > ?)",
		userID, asOf, asOf,
	).Order("created_at DESC").Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("query entities as_of: %w", err)
	}

	if len(entities) == 0 {
		return &TemporalQueryResult{
			Entities:  []models.Entity{},
			Relations: []models.Relation{},
			AsOf:      &asOf,
		}, nil
	}

	entityIDs := make([]uuid.UUID, len(entities))
	for i, e := range entities {
		entityIDs[i] = e.ID
	}

	var relations []models.Relation
	err = db.Where(
		"(source_id IN ? OR target_id IN ?) AND (valid_from IS NULL OR valid_from <= ?) AND (valid_until IS NULL OR valid_until > ?)",
		entityIDs, entityIDs, asOf, asOf,
	).Find(&relations).Error
	if err != nil {
		return nil, fmt.Errorf("query relations as_of: %w", err)
	}

	// 过滤：只保留两端实体都在 entityIDs 中的关系
	validIDSet := make(map[uuid.UUID]bool, len(entityIDs))
	for _, id := range entityIDs {
		validIDSet[id] = true
	}
	filtered := relations[:0]
	for _, r := range relations {
		if validIDSet[r.SourceID] && validIDSet[r.TargetID] {
			filtered = append(filtered, r)
		}
	}

	slog.Info("QueryGraphAsOf completed",
		"user_id", userID,
		"as_of", asOf.Format(time.RFC3339),
		"entities", len(entities),
		"relations", len(filtered),
	)

	return &TemporalQueryResult{
		Entities:  entities,
		Relations: filtered,
		AsOf:      &asOf,
	}, nil
}

// QueryGraphByTimeRange returns entities whose event_time falls within [from, to].
// Useful for queries like "上周的规划", "去年提到的事件" etc.
func (g *GraphStoreService) QueryGraphByTimeRange(db *gorm.DB, userID string, from, to time.Time) (*TemporalQueryResult, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if from.IsZero() || to.IsZero() {
		return nil, errors.New("from and to times are required")
	}
	if to.Before(from) {
		return nil, errors.New("to must be after from")
	}

	var entities []models.Entity
	err := db.Where(
		"user_id = ? AND event_time IS NOT NULL AND event_time >= ? AND event_time <= ?",
		userID, from, to,
	).Order("event_time DESC").Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("query entities by time range: %w", err)
	}

	if len(entities) == 0 {
		return &TemporalQueryResult{
			Entities:  []models.Entity{},
			Relations: []models.Relation{},
			TimeFrom:  &from,
			TimeTo:    &to,
		}, nil
	}

	entityIDs := make([]uuid.UUID, len(entities))
	idSet := make(map[uuid.UUID]bool, len(entities))
	for i, e := range entities {
		entityIDs[i] = e.ID
		idSet[e.ID] = true
	}

	var relations []models.Relation
	err = db.Where("source_id IN ? OR target_id IN ?", entityIDs, entityIDs).
		Find(&relations).Error
	if err != nil {
		return nil, fmt.Errorf("query relations by time range: %w", err)
	}

	// 只保留两端实体都在结果集中的关系
	filtered := relations[:0]
	for _, r := range relations {
		if idSet[r.SourceID] && idSet[r.TargetID] {
			filtered = append(filtered, r)
		}
	}

	slog.Info("QueryGraphByTimeRange completed",
		"user_id", userID,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"entities", len(entities),
		"relations", len(filtered),
	)

	return &TemporalQueryResult{
		Entities:  entities,
		Relations: filtered,
		TimeFrom:  &from,
		TimeTo:    &to,
	}, nil
}

// QueryRecentEntities returns the most recently created entities for a user.
// Supports optional entity_type filter (e.g., "person" for "最近提到的人").
func (g *GraphStoreService) QueryRecentEntities(db *gorm.DB, userID, entityType string, limit int) ([]models.Entity, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	query := db.Where("user_id = ?", userID)
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}

	var entities []models.Entity
	err := query.Order("created_at DESC").Limit(limit).Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("query recent entities: %w", err)
	}

	return entities, nil
}

// --- L4 想象记忆：高频实体趋势监控 ---

// TrendingEntity 代表一个高热度知识图谱实体及其趋势指标。
// 用于 L4 想象记忆的 Value Gating 触发。
type TrendingEntity struct {
	Entity        models.Entity `json:"entity"`
	RelationCount int           `json:"relation_count"`  // 出入关系总数
	FirstSeen     time.Time     `json:"first_seen"`      // 最早关联时间
	LastSeen      time.Time     `json:"last_seen"`       // 最近关联时间
	DaysSpan      int           `json:"days_span"`       // 跨度天数
	DaysSinceLast int           `json:"days_since_last"` // 距今天数 (ALG-OPT-01)
	HeatScore     float64       `json:"heat_score"`      // 综合热度 = relation_count * log(days_span+1) * exp(-λ*daysSinceLast)
}

// GetTrendingEntities 返回指定用户知识图谱中热度最高的实体。
// 热度评分公式 (ALG-OPT-01): heat_score = relation_count * log(days_span + 1) * exp(-λ * daysSinceLast)
// λ = 0.02, 防止历史高频但近期冷门的实体持续占据 Top N。
// 仅返回跨度 >= minDaysSpan 天且关系数 >= 3 的实体。
// 用于 L4 想象记忆的 Value Gating 决策。
func (g *GraphStoreService) GetTrendingEntities(db *gorm.DB, userID string, minDaysSpan int, topN int) ([]TrendingEntity, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if minDaysSpan < 0 {
		minDaysSpan = 3
	}
	if topN <= 0 || topN > 50 {
		topN = 3
	}

	// 1. 获取用户所有实体
	var entities []models.Entity
	if err := db.Where("user_id = ?", userID).Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("fetch entities for trending: %w", err)
	}
	if len(entities) == 0 {
		return []TrendingEntity{}, nil
	}

	// 2. 构建实体 ID 集合和映射
	entityMap := make(map[uuid.UUID]models.Entity, len(entities))
	entityIDs := make([]uuid.UUID, len(entities))
	for i, e := range entities {
		entityMap[e.ID] = e
		entityIDs[i] = e.ID
	}

	// 3. 查询所有相关关系
	var relations []models.Relation
	if err := db.Where("source_id IN ? OR target_id IN ?", entityIDs, entityIDs).
		Find(&relations).Error; err != nil {
		return nil, fmt.Errorf("fetch relations for trending: %w", err)
	}

	// 4. 统计每个实体的关系数和时间跨度
	type entityStats struct {
		relCount  int
		firstSeen time.Time
		lastSeen  time.Time
	}
	stats := make(map[uuid.UUID]*entityStats)

	for _, r := range relations {
		// 为 source 和 target 两端实体分别累加
		for _, eid := range []uuid.UUID{r.SourceID, r.TargetID} {
			if _, ok := entityMap[eid]; !ok {
				continue // 跳过不属于该用户的实体
			}
			s, ok := stats[eid]
			if !ok {
				s = &entityStats{firstSeen: r.CreatedAt, lastSeen: r.CreatedAt}
				stats[eid] = s
			}
			s.relCount++
			if r.CreatedAt.Before(s.firstSeen) {
				s.firstSeen = r.CreatedAt
			}
			if r.CreatedAt.After(s.lastSeen) {
				s.lastSeen = r.CreatedAt
			}
		}
	}

	// 5. 计算热度评分并过滤
	candidates := make([]TrendingEntity, 0)
	for eid, s := range stats {
		daysSpan := int(s.lastSeen.Sub(s.firstSeen).Hours() / 24)
		if daysSpan < minDaysSpan || s.relCount < 3 {
			continue
		}
		// ALG-OPT-01: 加入时间衰减 exp(-λ × daysSinceLast)
		daysSinceLast := int(time.Since(s.lastSeen).Hours() / 24)
		const lambda = 0.02
		timeDecay := math.Exp(-lambda * float64(daysSinceLast))
		heatScore := float64(s.relCount) * math.Log(float64(daysSpan+1)) * timeDecay
		candidates = append(candidates, TrendingEntity{
			Entity:        entityMap[eid],
			RelationCount: s.relCount,
			FirstSeen:     s.firstSeen,
			LastSeen:      s.lastSeen,
			DaysSpan:      daysSpan,
			DaysSinceLast: daysSinceLast,
			HeatScore:     heatScore,
		})
	}

	// 6. 按热度降序排序
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].HeatScore > candidates[i].HeatScore {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// 7. 取 Top N
	if len(candidates) > topN {
		candidates = candidates[:topN]
	}

	slog.Info("GetTrendingEntities completed",
		"user_id", userID,
		"total_entities", len(entities),
		"candidates", len(candidates),
	)

	return candidates, nil
}

// --- Singleton ---

var (
	graphStoreOnce    sync.Once
	graphStoreService *GraphStoreService
)

// GetGraphStore returns the singleton GraphStoreService.
func GetGraphStore() *GraphStoreService {
	graphStoreOnce.Do(func() {
		graphStoreService = &GraphStoreService{}
	})
	return graphStoreService
}
