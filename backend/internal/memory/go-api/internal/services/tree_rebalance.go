// Package services — MemTree 树再平衡（Paper §3.1 Rebalance）。
//
// RebalanceTask 定期检查叶子过多的节点，对其子节点进行聚类重组。
// 当 children_count > TreeMaxChildren 时触发。
package services

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// RebalanceTask scans for overloaded nodes and restructures them.
// Should be called periodically (e.g. every hour via cron/ticker).
func (tm *TreeManager) RebalanceTask(ctx context.Context, db *gorm.DB) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 1. Find nodes where children_count > TreeMaxChildren
	var overloaded []models.MemoryTreeNode
	if err := db.Where("children_count > ? AND node_type NOT IN ?",
		TreeMaxChildren, []string{NodeTypeRoot, NodeTypeLeaf}).
		Find(&overloaded).Error; err != nil {
		return fmt.Errorf("find overloaded nodes: %w", err)
	}

	if len(overloaded) == 0 {
		slog.Debug("RebalanceTask: no overloaded nodes")
		return nil
	}

	slog.Info("RebalanceTask: found overloaded nodes", "count", len(overloaded))

	for _, node := range overloaded {
		if err := tm.rebalanceNode(ctx, db, &node); err != nil {
			slog.Warn("RebalanceTask: rebalance failed",
				"node_id", node.ID, "error", err)
			// Continue with other nodes
		}
	}

	return nil
}

// rebalanceNode reorganizes children of an overloaded node into sub-groups.
// Strategy: group children by content similarity into ceil(N/maxGroup) clusters,
// create intermediate aggregate nodes for each cluster.
func (tm *TreeManager) rebalanceNode(ctx context.Context, db *gorm.DB, node *models.MemoryTreeNode) error {
	// Load all children
	var children []models.MemoryTreeNode
	if err := db.Where("parent_id = ?", node.ID).
		Order("created_at").Find(&children).Error; err != nil {
		return fmt.Errorf("load children: %w", err)
	}

	if len(children) <= TreeMaxChildren {
		return nil // No longer overloaded
	}

	slog.Info("RebalanceTask: rebalancing node",
		"node_id", node.ID,
		"children", len(children),
		"depth", node.Depth,
	)

	// Skip if at max depth (can't push deeper)
	if node.Depth >= TreeMaxDepth-1 {
		slog.Debug("RebalanceTask: at max depth, skipping", "node_id", node.ID)
		return nil
	}

	// Compute target group count: aim for ~TreeMaxChildren/2 children per group
	targetGroupSize := TreeMaxChildren / 2
	if targetGroupSize < 2 {
		targetGroupSize = 2
	}
	numGroups := (len(children) + targetGroupSize - 1) / targetGroupSize

	// Group by similarity using simple greedy clustering
	groups := tm.greedyCluster(ctx, children, numGroups)

	// Create intermediate aggregate nodes for groups > 1 child
	for _, group := range groups {
		if len(group) <= 1 {
			// Single child — leave it under the original parent
			continue
		}

		// Create aggregate node
		aggregate := models.MemoryTreeNode{
			UserID:        node.UserID,
			ParentID:      &node.ID,
			Content:       summarizeGroup(group),
			Category:      node.Category,
			Depth:         node.Depth + 1,
			IsLeaf:        false,
			NodeType:      NodeTypeAggregate,
			ChildrenCount: len(group),
		}
		if err := db.Create(&aggregate).Error; err != nil {
			return fmt.Errorf("create aggregate: %w", err)
		}

		// Re-parent children under the new aggregate
		for _, child := range group {
			child.ParentID = &aggregate.ID
			child.Depth = node.Depth + 2
			if err := db.Model(&child).Updates(map[string]any{
				"parent_id": aggregate.ID,
				"depth":     node.Depth + 2,
			}).Error; err != nil {
				slog.Warn("RebalanceTask: re-parent failed",
					"child_id", child.ID, "error", err)
			}
		}
	}

	// Update original node's children_count
	var newCount int64
	db.Model(&models.MemoryTreeNode{}).Where("parent_id = ?", node.ID).Count(&newCount)
	db.Model(node).UpdateColumn("children_count", newCount)

	slog.Info("RebalanceTask: rebalanced node",
		"node_id", node.ID,
		"old_children", len(children),
		"new_direct_children", newCount,
	)

	return nil
}

// greedyCluster groups nodes by similarity using a simple greedy approach.
// Each new node joins the most similar existing group, or starts a new one.
func (tm *TreeManager) greedyCluster(
	ctx context.Context,
	nodes []models.MemoryTreeNode,
	maxGroups int,
) [][]models.MemoryTreeNode {
	if len(nodes) == 0 {
		return nil
	}

	type group struct {
		nodes    []models.MemoryTreeNode
		centroid string // representative content for similarity comparison
	}

	groups := make([]group, 0, maxGroups)
	groups = append(groups, group{
		nodes:    []models.MemoryTreeNode{nodes[0]},
		centroid: nodes[0].Content,
	})

	for _, n := range nodes[1:] {
		// Find most similar group
		bestIdx := -1
		bestSim := 0.0

		for i, g := range groups {
			sim := tm.computeSimilarity(ctx, n.Content, g.centroid)
			if sim > bestSim {
				bestSim = sim
				bestIdx = i
			}
		}

		// Join existing group if similar enough, or create new group
		if bestIdx >= 0 && (bestSim >= 0.3 || len(groups) >= maxGroups) {
			groups[bestIdx].nodes = append(groups[bestIdx].nodes, n)
		} else {
			groups = append(groups, group{
				nodes:    []models.MemoryTreeNode{n},
				centroid: n.Content,
			})
		}
	}

	// Convert to result
	result := make([][]models.MemoryTreeNode, len(groups))
	for i, g := range groups {
		result[i] = g.nodes
	}
	return result
}

// computeSimilarity uses embedding cosine similarity when available, otherwise Jaccard.
func (tm *TreeManager) computeSimilarity(ctx context.Context, a, b string) float64 {
	embA := tm.embedText(ctx, a)
	embB := tm.embedText(ctx, b)
	if embA != nil && embB != nil {
		return cosineSimilarity32(embA, embB)
	}
	return textSimilarity(a, b)
}

// summarizeGroup builds a simple content summary for a group of nodes.
func summarizeGroup(nodes []models.MemoryTreeNode) string {
	if len(nodes) == 0 {
		return ""
	}

	// Sort by content length descending — take the most informative
	sorted := make([]models.MemoryTreeNode, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Content) > len(sorted[j].Content)
	})

	// Use the longest content as representative, truncated
	content := sorted[0].Content
	if len(content) > 200 {
		content = content[:200] + "..."
	}
	return fmt.Sprintf("[聚合 %d 条] %s", len(nodes), content)
}
