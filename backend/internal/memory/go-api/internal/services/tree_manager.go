// Package services — TreeManager implements MemTree (arXiv:2410.14052)
// dynamic tree-structured memory for hierarchical organization and retrieval.
//
// 文件拆分:
//   - tree_manager.go  — 结构体、常量、类型、公开 API、单例
//   - tree_algorithm.go — 内部树操作、LLM 聚合、工具函数
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// ============================================================================
// Constants — Paper parameters (Appendix A.1.3)
// ============================================================================

const (
	TreeMaxDepth     = 5    // Maximum tree depth
	TreeMaxChildren  = 10   // Max children per node
	BaseThreshold    = 0.4  // θ₀ — base similarity threshold
	ThresholdRate    = 0.5  // λ — depth-adaptive rate
	RetrieveMinScore = 0.25 // Minimum cosine similarity for retrieval
	DefaultTreeTopK  = 5    // Default top-K results

	NodeTypeRoot         = "root"
	NodeTypeCategoryRoot = "category_root"
	NodeTypeAggregate    = "aggregate"
	NodeTypeLeaf         = "leaf"
)

// ============================================================================
// TreeManager
// ============================================================================

// TreeManager implements the MemTree algorithm for hierarchical memory.
type TreeManager struct {
	mu       sync.RWMutex
	embedSvc EmbeddingService // optional — nil 时退化为 Jaccard 文本相似度
}

// NewTreeManager creates a new TreeManager.
func NewTreeManager() *TreeManager {
	tm := &TreeManager{}
	// 尝试加载 EmbeddingService（可能因配置缺失而返回 nil）
	tm.embedSvc = GetEmbeddingService()
	if tm.embedSvc != nil {
		slog.Info("TreeManager: embedding service loaded", "dim", tm.embedSvc.Dimension())
	} else {
		slog.Warn("TreeManager: embedding service unavailable, using Jaccard fallback")
	}
	return tm
}

// ReloadEmbedding refreshes the embedding service (e.g. after config change).
func (tm *TreeManager) ReloadEmbedding() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.embedSvc = ReloadEmbeddingService()
}

// embedText computes an embedding for text content.
// Returns nil if the embedding service is unavailable.
func (tm *TreeManager) embedText(ctx context.Context, text string) []float32 {
	if tm.embedSvc == nil || text == "" {
		return nil
	}
	emb, err := tm.embedSvc.EmbedQuery(ctx, text)
	if err != nil {
		slog.Debug("TreeManager: embed failed", "error", err)
		return nil
	}
	return emb
}

// TreeSearchResult represents a single result from collapsed tree retrieval.
type TreeSearchResult struct {
	NodeID   uuid.UUID  `json:"node_id"`
	MemoryID *uuid.UUID `json:"memory_id,omitempty"`
	Content  string     `json:"content"`
	Score    float64    `json:"score"`
	Depth    int        `json:"depth"`
	IsLeaf   bool       `json:"is_leaf"`
	Category string     `json:"category"`
	NodeType string     `json:"node_type"`
}

// ============================================================================
// Core Algorithm: Insert
// ============================================================================

// InsertMemory inserts a memory into the tree structure.
// Flow: classify → locate subtree → cosine traverse → attach leaf → aggregate parents.
func (tm *TreeManager) InsertMemory(
	ctx context.Context, db *gorm.DB,
	memory *models.Memory,
) (*models.MemoryTreeNode, error) {
	if memory == nil {
		return nil, errors.New("memory is nil")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 1. Get or create user root node (depth=0)
	root, err := tm.getOrCreateUserRoot(db, memory.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user root: %w", err)
	}

	// 2. Get or create category subtree root (depth=1)
	catRoot, err := tm.getOrCreateCategoryRoot(db, root.ID, memory.UserID, memory.Category)
	if err != nil {
		return nil, fmt.Errorf("get category root: %w", err)
	}

	// 3. Traverse tree to find insertion point
	targetParent := tm.traverseForInsert(ctx, db, catRoot, memory.Content)

	// 4. Attach leaf node
	leaf, err := tm.attachLeaf(db, targetParent, memory)
	if err != nil {
		return nil, fmt.Errorf("attach leaf: %w", err)
	}

	// 5. Async backpropagate aggregation through parent nodes
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := tm.backpropagateAggregation(bgCtx, db, leaf); err != nil {
			slog.Warn("backpropagation failed", "node_id", leaf.ID, "error", err)
		}
	}()

	slog.Info("MemTree: inserted memory",
		"memory_id", memory.ID,
		"node_id", leaf.ID,
		"parent_id", targetParent.ID,
		"category", memory.Category,
		"depth", leaf.Depth,
	)

	return leaf, nil
}

// ============================================================================
// Core Algorithm: Retrieval (Collapsed Tree — Paper §3.2)
// ============================================================================

// CollapsedTreeRetrieve performs collapsed tree retrieval.
// Flattens all tree nodes, computes cosine similarity with query, returns top-K.
func (tm *TreeManager) CollapsedTreeRetrieve(
	ctx context.Context, db *gorm.DB,
	query, userID string,
	topK int,
	category string,
) ([]TreeSearchResult, error) {
	if query == "" {
		return nil, errors.New("query is required")
	}
	if topK <= 0 {
		topK = DefaultTreeTopK
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 1. Load all content-bearing nodes for the user
	var nodes []models.MemoryTreeNode
	q := db.Where("user_id = ? AND node_type NOT IN ?", userID, []string{NodeTypeRoot})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if err := q.Find(&nodes).Error; err != nil {
		return nil, fmt.Errorf("load tree nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil, nil
	}

	// 2. Compute similarity — prefer embedding cosine, fallback to Jaccard
	queryEmb := tm.embedText(ctx, query)

	type scoredNode struct {
		Node  models.MemoryTreeNode
		Score float64
	}
	scored := make([]scoredNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Content == "" {
			continue
		}
		var sim float64
		if queryEmb != nil {
			nodeEmb := tm.embedText(ctx, n.Content)
			if nodeEmb != nil {
				sim = cosineSimilarity32(queryEmb, nodeEmb)
			} else {
				sim = textSimilarity(query, n.Content)
			}
		} else {
			sim = textSimilarity(query, n.Content)
		}
		if sim >= RetrieveMinScore {
			scored = append(scored, scoredNode{Node: n, Score: sim})
		}
	}

	// 3. Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 4. Take top-K
	if len(scored) > topK {
		scored = scored[:topK]
	}

	// 5. Convert to results
	results := make([]TreeSearchResult, len(scored))
	for i, s := range scored {
		results[i] = TreeSearchResult{
			NodeID:   s.Node.ID,
			MemoryID: s.Node.MemoryID,
			Content:  s.Node.Content,
			Score:    s.Score,
			Depth:    s.Node.Depth,
			IsLeaf:   s.Node.IsLeaf,
			Category: s.Node.Category,
			NodeType: s.Node.NodeType,
		}
	}

	return results, nil
}

// GetSubtree retrieves a node and its immediate children for tree browsing.
func (tm *TreeManager) GetSubtree(
	db *gorm.DB, userID string, nodeID *uuid.UUID, category string,
) ([]models.MemoryTreeNode, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if nodeID != nil {
		// Get children of specific node
		var children []models.MemoryTreeNode
		if err := db.Where("parent_id = ? AND user_id = ?", *nodeID, userID).
			Order("created_at").Find(&children).Error; err != nil {
			return nil, err
		}
		return children, nil
	}

	// Get category roots for user
	var roots []models.MemoryTreeNode
	q := db.Where("user_id = ? AND node_type = ?", userID, NodeTypeCategoryRoot)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if err := q.Order("category").Find(&roots).Error; err != nil {
		return nil, err
	}
	return roots, nil
}

// GetTreeStats returns statistics about a user's memory tree.
func (tm *TreeManager) GetTreeStats(db *gorm.DB, userID string) map[string]any {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var totalNodes, leafNodes, aggregateNodes int64
	db.Model(&models.MemoryTreeNode{}).Where("user_id = ?", userID).Count(&totalNodes)
	db.Model(&models.MemoryTreeNode{}).Where("user_id = ? AND is_leaf = TRUE", userID).Count(&leafNodes)
	db.Model(&models.MemoryTreeNode{}).Where("user_id = ? AND node_type = ?", userID, NodeTypeAggregate).Count(&aggregateNodes)

	// Category breakdown
	type catCount struct {
		Category string
		Count    int64
	}
	var categories []catCount
	db.Model(&models.MemoryTreeNode{}).
		Select("category, count(*) as count").
		Where("user_id = ? AND node_type NOT IN ?", userID, []string{NodeTypeRoot}).
		Group("category").
		Scan(&categories)

	catMap := make(map[string]int64)
	for _, c := range categories {
		catMap[c.Category] = c.Count
	}

	var maxDepth int
	db.Model(&models.MemoryTreeNode{}).
		Select("COALESCE(MAX(depth), 0)").
		Where("user_id = ?", userID).
		Scan(&maxDepth)

	return map[string]any{
		"total_nodes":     totalNodes,
		"leaf_nodes":      leafNodes,
		"aggregate_nodes": aggregateNodes,
		"max_depth":       maxDepth,
		"categories":      catMap,
	}
}

// ============================================================================
// Singleton
// ============================================================================

var (
	treeManagerOnce    sync.Once
	treeManagerService *TreeManager
)

// GetTreeManager returns the singleton TreeManager.
func GetTreeManager() *TreeManager {
	treeManagerOnce.Do(func() {
		treeManagerService = NewTreeManager()
	})
	return treeManagerService
}
