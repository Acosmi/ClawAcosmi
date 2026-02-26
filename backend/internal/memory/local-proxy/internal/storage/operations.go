// Package storage — CRUD operations for local SQLite.
package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Memory CRUD
// ============================================================================

// SaveMemory creates or updates a memory in local SQLite.
func (s *Store) SaveMemory(m *Memory) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return s.db.Save(m).Error
}

// GetMemory retrieves a single memory by ID.
func (s *Store) GetMemory(id string) (*Memory, error) {
	var m Memory
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// SearchMemoriesByText performs a simple text-based search (LIKE query).
// Used as offline fallback when cloud embedding is unavailable.
func (s *Store) SearchMemoriesByText(userID, query string, limit int, category string) ([]Memory, error) {
	tx := s.db.Where("user_id = ?", userID)
	if query != "" {
		tx = tx.Where("content LIKE ?", "%"+query+"%")
	}
	if category != "" {
		tx = tx.Where("category = ?", category)
	}

	var memories []Memory
	err := tx.Order("importance_score DESC, created_at DESC").Limit(limit).Find(&memories).Error
	return memories, err
}

// SearchMemoriesByVector performs a local cosine similarity search.
// candidateLimit controls how many memories to scan from DB.
func (s *Store) SearchMemoriesByVector(userID string, queryEmbedding []float32, limit int, category string) ([]MemoryWithScore, error) {
	tx := s.db.Where("user_id = ? AND embedding IS NOT NULL", userID)
	if category != "" {
		tx = tx.Where("category = ?", category)
	}

	var memories []Memory
	if err := tx.Find(&memories).Error; err != nil {
		return nil, err
	}

	// Compute cosine similarity locally
	type scored struct {
		memory Memory
		score  float64
	}
	var results []scored
	for _, m := range memories {
		emb := bytesToFloat32(m.Embedding)
		if len(emb) == 0 {
			continue
		}
		sim := cosineSimilarity(queryEmbedding, emb)
		results = append(results, scored{memory: m, score: sim})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > len(results) {
		limit = len(results)
	}

	out := make([]MemoryWithScore, limit)
	for i := 0; i < limit; i++ {
		out[i] = MemoryWithScore{
			Memory: results[i].memory,
			Score:  results[i].score,
		}
	}
	return out, nil
}

// MemoryWithScore pairs a memory with its search relevance score.
type MemoryWithScore struct {
	Memory Memory  `json:"memory"`
	Score  float64 `json:"score"`
}

// GetRecentMemories returns the most recent memories for a user.
func (s *Store) GetRecentMemories(userID string, limit int) ([]Memory, error) {
	var memories []Memory
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&memories).Error
	return memories, err
}

// CountMemories returns the total memory count for a user.
func (s *Store) CountMemories(userID string) int64 {
	var count int64
	s.db.Model(&Memory{}).Where("user_id = ?", userID).Count(&count)
	return count
}

// ============================================================================
// Core Memory CRUD
// ============================================================================

// GetCoreMemory retrieves or creates the core memory for a user.
func (s *Store) GetCoreMemory(userID string) (*CoreMemory, error) {
	var cm CoreMemory
	result := s.db.FirstOrCreate(&cm, CoreMemory{UserID: userID})
	if result.Error != nil {
		return nil, result.Error
	}
	if cm.ID == "" {
		cm.ID = uuid.New().String()
		s.db.Save(&cm)
	}
	return &cm, nil
}

// UpdateCoreMemory updates a section of core memory.
func (s *Store) UpdateCoreMemory(userID, section, content, mode string) error {
	cm, err := s.GetCoreMemory(userID)
	if err != nil {
		return err
	}

	switch section {
	case "persona":
		if mode == "append" {
			cm.Persona += "\n" + content
		} else {
			cm.Persona = content
		}
	case "preferences":
		if mode == "append" {
			cm.Preferences += "\n" + content
		} else {
			cm.Preferences = content
		}
	case "instructions":
		if mode == "append" {
			cm.Instructions += "\n" + content
		} else {
			cm.Instructions = content
		}
	default:
		return fmt.Errorf("unknown section: %s", section)
	}

	return s.db.Save(cm).Error
}

// ============================================================================
// Knowledge Graph CRUD
// ============================================================================

// SaveEntity creates or updates an entity.
func (s *Store) SaveEntity(e *Entity) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return s.db.Save(e).Error
}

// SaveRelation creates or updates a relation.
func (s *Store) SaveRelation(r *Relation) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return s.db.Save(r).Error
}

// GetGraph returns all entities and relations for a user.
func (s *Store) GetGraph(userID string) ([]Entity, []Relation, error) {
	var entities []Entity
	if err := s.db.Where("user_id = ?", userID).Find(&entities).Error; err != nil {
		return nil, nil, err
	}
	var relations []Relation
	if err := s.db.Where("user_id = ?", userID).Find(&relations).Error; err != nil {
		return nil, nil, err
	}
	return entities, relations, nil
}

// ============================================================================
// Tree Node CRUD
// ============================================================================

// SaveTreeNode creates or updates a tree node.
func (s *Store) SaveTreeNode(n *TreeNode) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return s.db.Save(n).Error
}

// GetSubtree returns children of a parent node (or root nodes if parentID is empty).
func (s *Store) GetSubtree(userID, parentID, category string) ([]TreeNode, error) {
	tx := s.db.Where("user_id = ?", userID)
	if parentID == "" {
		tx = tx.Where("parent_id = '' OR parent_id IS NULL")
	} else {
		tx = tx.Where("parent_id = ?", parentID)
	}
	if category != "" {
		tx = tx.Where("category = ?", category)
	}

	var nodes []TreeNode
	err := tx.Order("created_at ASC").Find(&nodes).Error
	return nodes, err
}

// SearchTreeNodes performs a text search on tree nodes.
func (s *Store) SearchTreeNodes(userID, query string, limit int, category string) ([]TreeNode, error) {
	tx := s.db.Where("user_id = ?", userID)
	if query != "" {
		tx = tx.Where("content LIKE ?", "%"+query+"%")
	}
	if category != "" {
		tx = tx.Where("category = ?", category)
	}

	var nodes []TreeNode
	err := tx.Order("depth ASC").Limit(limit).Find(&nodes).Error
	return nodes, err
}

// GetTreeStats returns basic statistics about the memory tree.
func (s *Store) GetTreeStats(userID string) map[string]any {
	var totalNodes int64
	s.db.Model(&TreeNode{}).Where("user_id = ?", userID).Count(&totalNodes)

	var leafCount int64
	s.db.Model(&TreeNode{}).Where("user_id = ? AND is_leaf = ?", userID, true).Count(&leafCount)

	var maxDepth int
	s.db.Model(&TreeNode{}).Where("user_id = ?", userID).
		Select("COALESCE(MAX(depth), 0)").Scan(&maxDepth)

	return map[string]any{
		"total_nodes": totalNodes,
		"leaf_count":  leafCount,
		"max_depth":   maxDepth,
	}
}

// ============================================================================
// Metrics
// ============================================================================

// GetMetrics returns basic system metrics.
func (s *Store) GetMetrics(userID string) map[string]any {
	memCount := s.CountMemories(userID)
	treeStats := s.GetTreeStats(userID)

	var entityCount, relationCount int64
	s.db.Model(&Entity{}).Where("user_id = ?", userID).Count(&entityCount)
	s.db.Model(&Relation{}).Where("user_id = ?", userID).Count(&relationCount)

	return map[string]any{
		"memories":     memCount,
		"entities":     entityCount,
		"relations":    relationCount,
		"tree":         treeStats,
		"storage":      "local-sqlite",
		"last_checked": time.Now().Format(time.RFC3339),
	}
}

// ============================================================================
// Vector helpers
// ============================================================================

// float32ToBytes serializes a float32 slice to bytes (little-endian).
func Float32ToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToFloat32 deserializes bytes to float32 slice.
func bytesToFloat32(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// cosineSimilarity computes cos(θ) between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// MetadataToJSON converts a map to JSON string.
func MetadataToJSON(m map[string]any) string {
	if m == nil {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// MetadataFromJSON parses JSON string to map.
func MetadataFromJSON(s string) map[string]any {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

// CoreMemoryToString formats core memory as readable text.
func CoreMemoryToString(cm *CoreMemory) string {
	var parts []string
	if cm.Persona != "" {
		parts = append(parts, "## Persona\n"+cm.Persona)
	}
	if cm.Preferences != "" {
		parts = append(parts, "## Preferences\n"+cm.Preferences)
	}
	if cm.Instructions != "" {
		parts = append(parts, "## Instructions\n"+cm.Instructions)
	}
	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, "\n\n")
}
