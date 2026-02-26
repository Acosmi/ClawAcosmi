// Package services — MemTree 内部树操作与工具函数。
//
// 职责:
//   - 树节点 CRUD (getOrCreateUserRoot, getOrCreateCategoryRoot, attachLeaf)
//   - 插入遍历算法 (traverseForInsert)
//   - LLM 父节点聚合 (backpropagateAggregation)
//   - 文本相似度工具 (textSimilarity, tokenize, cosineSimilarity, adaptiveThreshold)
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// ============================================================================
// Internal: Tree Structure Operations
// ============================================================================

// getOrCreateUserRoot finds or creates the root node for a user.
func (tm *TreeManager) getOrCreateUserRoot(db *gorm.DB, userID string) (*models.MemoryTreeNode, error) {
	var root models.MemoryTreeNode
	err := db.Where("user_id = ? AND node_type = ?", userID, NodeTypeRoot).First(&root).Error
	if err == nil {
		return &root, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	root = models.MemoryTreeNode{
		UserID:   userID,
		Content:  "",
		NodeType: NodeTypeRoot,
		Depth:    0,
		IsLeaf:   false,
	}
	if err := db.Create(&root).Error; err != nil {
		return nil, err
	}
	slog.Debug("MemTree: created user root", "user_id", userID, "node_id", root.ID)
	return &root, nil
}

// getOrCreateCategoryRoot finds or creates a category subtree root.
func (tm *TreeManager) getOrCreateCategoryRoot(
	db *gorm.DB, parentID uuid.UUID, userID, category string,
) (*models.MemoryTreeNode, error) {
	var catRoot models.MemoryTreeNode
	err := db.Where("user_id = ? AND parent_id = ? AND node_type = ? AND category = ?",
		userID, parentID, NodeTypeCategoryRoot, category).First(&catRoot).Error
	if err == nil {
		return &catRoot, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	catRoot = models.MemoryTreeNode{
		UserID:   userID,
		ParentID: &parentID,
		Content:  fmt.Sprintf("[%s] category root", category),
		Category: category,
		NodeType: NodeTypeCategoryRoot,
		Depth:    1,
		IsLeaf:   false,
	}
	if err := db.Create(&catRoot).Error; err != nil {
		return nil, err
	}

	// Update parent children count
	db.Model(&models.MemoryTreeNode{}).Where("id = ?", parentID).
		UpdateColumn("children_count", gorm.Expr("children_count + 1"))

	slog.Debug("MemTree: created category root", "category", category, "node_id", catRoot.ID)
	return &catRoot, nil
}

// traverseForInsert walks the tree to find the best insertion point.
// Implements paper Algorithm 1 with depth-adaptive threshold.
// Uses embedding-based cosine similarity when EmbeddingService is available.
func (tm *TreeManager) traverseForInsert(
	ctx context.Context, db *gorm.DB,
	node *models.MemoryTreeNode,
	newContent string,
) *models.MemoryTreeNode {
	// Base case: leaf node or max depth reached
	if node.IsLeaf || node.Depth >= TreeMaxDepth {
		return node
	}

	// Load children
	var children []models.MemoryTreeNode
	db.Where("parent_id = ?", node.ID).Find(&children)

	if len(children) == 0 {
		return node
	}

	// Compute similarity with each child
	threshold := adaptiveThreshold(node.Depth)
	bestSim := 0.0
	var bestChild *models.MemoryTreeNode

	// Try embedding-based similarity first
	newEmb := tm.embedText(ctx, newContent)

	for i := range children {
		var sim float64
		if newEmb != nil {
			childEmb := tm.embedText(ctx, children[i].Content)
			if childEmb != nil {
				sim = cosineSimilarity32(newEmb, childEmb)
			} else {
				sim = textSimilarity(newContent, children[i].Content)
			}
		} else {
			sim = textSimilarity(newContent, children[i].Content)
		}
		if sim > bestSim {
			bestSim = sim
			bestChild = &children[i]
		}
	}

	// Decision: traverse deeper if similarity exceeds threshold
	if bestSim >= threshold && bestChild != nil {
		return tm.traverseForInsert(ctx, db, bestChild, newContent)
	}

	// Otherwise, attach as new child of current node
	return node
}

// attachLeaf creates a new leaf node under the target parent.
// If target is a leaf, it performs "leaf expansion" (paper §3.1 Boundary case).
func (tm *TreeManager) attachLeaf(
	db *gorm.DB,
	parent *models.MemoryTreeNode,
	memory *models.Memory,
) (*models.MemoryTreeNode, error) {
	// Leaf expansion: if parent is currently a leaf, promote it to aggregate
	if parent.IsLeaf && parent.NodeType == NodeTypeLeaf {
		// Create a copy of the original leaf as a child
		origChild := models.MemoryTreeNode{
			UserID:   parent.UserID,
			ParentID: &parent.ID,
			MemoryID: parent.MemoryID,
			Content:  parent.Content,
			Category: parent.Category,
			Depth:    parent.Depth + 1,
			IsLeaf:   true,
			NodeType: NodeTypeLeaf,
		}
		if err := db.Create(&origChild).Error; err != nil {
			return nil, fmt.Errorf("create original child: %w", err)
		}

		// Promote parent to aggregate
		parent.IsLeaf = false
		parent.NodeType = NodeTypeAggregate
		parent.MemoryID = nil
		parent.ChildrenCount = 1
		if err := db.Save(parent).Error; err != nil {
			return nil, fmt.Errorf("promote parent: %w", err)
		}
	}

	// Create the new leaf
	leaf := models.MemoryTreeNode{
		UserID:   memory.UserID,
		ParentID: &parent.ID,
		MemoryID: &memory.ID,
		Content:  memory.Content,
		Category: memory.Category,
		Depth:    parent.Depth + 1,
		IsLeaf:   true,
		NodeType: NodeTypeLeaf,
	}
	if err := db.Create(&leaf).Error; err != nil {
		return nil, fmt.Errorf("create leaf: %w", err)
	}

	// Update parent children count
	db.Model(&models.MemoryTreeNode{}).Where("id = ?", parent.ID).
		UpdateColumn("children_count", gorm.Expr("children_count + 1"))

	return &leaf, nil
}

// ============================================================================
// Internal: Parent Aggregation (Paper §3.1 — Aggregate Operation)
// ============================================================================

// backpropagateAggregation updates parent nodes along the insertion path
// using LLM-based content aggregation (Paper §3.1, Appendix A.1.2).
func (tm *TreeManager) backpropagateAggregation(
	ctx context.Context, db *gorm.DB,
	leaf *models.MemoryTreeNode,
) error {
	if leaf.ParentID == nil {
		return nil
	}

	llm := GetLLMClient()
	if llm == nil {
		slog.Debug("MemTree: LLM unavailable, skipping aggregation")
		return nil
	}

	current := leaf
	for current.ParentID != nil {
		var parent models.MemoryTreeNode
		if err := db.First(&parent, "id = ?", *current.ParentID).Error; err != nil {
			return fmt.Errorf("load parent %s: %w", current.ParentID, err)
		}

		// Skip root and category_root — they don't aggregate
		if parent.NodeType == NodeTypeRoot || parent.NodeType == NodeTypeCategoryRoot {
			break
		}

		// Build aggregation prompt (Paper Appendix A.1.2)
		prompt := fmt.Sprintf(
			`You will receive two pieces of information: New Information is detailed, and Existing Information is a summary from %d previous entries.
Merge these into a single, cohesive summary highlighting the most important insights.
If the number of previous entries is accumulating (more than 2), summarize more concisely, only capturing the overarching theme.
Output the summary directly, in the SAME LANGUAGE as the input.

[New Information] %s
[Existing Information (from %d previous entries)] %s
[Output Summary]`,
			parent.ChildrenCount, current.Content, parent.ChildrenCount, parent.Content)

		result, err := llm.Generate(ctx, prompt)
		if err != nil {
			slog.Warn("MemTree: aggregation LLM call failed", "node_id", parent.ID, "error", err)
			break
		}

		// Update parent content
		parent.Content = strings.TrimSpace(result)
		parent.UpdatedAt = time.Now()
		if err := db.Save(&parent).Error; err != nil {
			slog.Warn("MemTree: save aggregated parent failed", "node_id", parent.ID, "error", err)
			break
		}

		slog.Debug("MemTree: aggregated parent",
			"node_id", parent.ID,
			"depth", parent.Depth,
			"children", parent.ChildrenCount,
		)

		current = &parent
	}
	return nil
}

// ============================================================================
// Utility Functions
// ============================================================================

// adaptiveThreshold computes the depth-adaptive similarity threshold.
// θ(d) = θ₀ + λ * (d / max_depth) — Paper Appendix A.1.3
func adaptiveThreshold(depth int) float64 {
	return BaseThreshold + ThresholdRate*float64(depth)/float64(TreeMaxDepth)
}

// textSimilarity computes a simple content-based similarity score.
// This is a stopgap for when Qdrant vector search is not yet available.
// Will be replaced by cosine similarity on embeddings in Phase 6.
func textSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	// Jaccard similarity on word sets
	wordsA := tokenize(a)
	wordsB := tokenize(b)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	setA := make(map[string]bool, len(wordsA))
	for _, w := range wordsA {
		setA[w] = true
	}

	intersection := 0
	setB := make(map[string]bool, len(wordsB))
	for _, w := range wordsB {
		setB[w] = true
		if setA[w] {
			intersection++
		}
	}

	union := len(setA)
	for w := range setB {
		if !setA[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// tokenize splits text into lowercase word tokens.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r >= 0x4E00 && r <= 0x9FFF) // Include CJK characters
	})
	return words
}

// cosineSimilarity computes cosine similarity between two float64 vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// cosineSimilarity32 computes cosine similarity between two float32 vectors.
// Used with EmbeddingService which returns float32 slices.
func cosineSimilarity32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
