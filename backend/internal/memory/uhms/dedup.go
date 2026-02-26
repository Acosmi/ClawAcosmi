package uhms

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
)

// dedupAction represents the outcome of a deduplication check.
type dedupAction int

const (
	dedupAdd    dedupAction = iota // 新记忆, 直接添加
	dedupNoop                      // 完全重复, 跳过
	dedupUpdate                    // 语义重复但内容更新, 需要合并
)

// checkDuplicate checks if content already exists in the user's memory.
// Three-stage pipeline: hash exact match → FTS5 similarity → (optional) LLM judgment.
func (m *DefaultManager) checkDuplicate(ctx context.Context, userID, content string) (dedupAction, string) {
	// Stage 1: Hash exact match (fastest)
	hash := contentHash(content)
	existing, err := m.store.SearchByFTS5(userID, truncate(content, 100), 5)
	if err != nil {
		return dedupAdd, ""
	}

	for _, r := range existing {
		if contentHash(r.Memory.Content) == hash {
			return dedupNoop, r.Memory.ID
		}
	}

	// Stage 2: FTS5 similarity check
	// 如果搜索结果中有高度相似的内容
	for _, r := range existing {
		if r.Score > 0.8 && isSemanticallySimilar(content, r.Memory.Content) {
			return dedupUpdate, r.Memory.ID
		}
	}

	// Stage 3: (可选) 向量相似度 — 仅当 vectorIndex 启用时
	if m.vectorIndex != nil && m.embedder != nil {
		if action, id := m.vectorDedup(ctx, userID, content); action != dedupAdd {
			return action, id
		}
	}

	return dedupAdd, ""
}

// contentHash returns SHA-256 hash of normalized content.
func contentHash(content string) string {
	normalized := strings.TrimSpace(strings.ToLower(content))
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

// isSemanticallySimilar performs a simple string-based similarity check.
// Uses Jaccard similarity on word sets as a fast approximation.
func isSemanticallySimilar(a, b string) bool {
	wordsA := wordSet(a)
	wordsB := wordSet(b)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return false
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}
	union := len(wordsA) + len(wordsB) - intersection
	if union == 0 {
		return false
	}

	jaccard := float64(intersection) / float64(union)
	return jaccard > 0.7
}

func wordSet(text string) map[string]bool {
	words := strings.Fields(strings.ToLower(text))
	set := make(map[string]bool, len(words))
	for _, w := range words {
		if len(w) > 2 { // 忽略短词
			set[w] = true
		}
	}
	return set
}

// vectorDedup uses vector similarity for deduplication when vector backend is available.
func (m *DefaultManager) vectorDedup(ctx context.Context, userID, content string) (dedupAction, string) {
	vec, err := m.embedder.Embed(ctx, content)
	if err != nil {
		return dedupAdd, ""
	}

	// 搜索所有集合
	for _, mt := range AllMemoryTypes {
		collection := "mem_" + string(mt)
		hits, err := m.vectorIndex.Search(ctx, collection, vec, 3)
		if err != nil {
			continue
		}
		for _, hit := range hits {
			if hit.Score > 0.95 {
				// 几乎完全相同
				mem, err := m.store.GetMemory(hit.ID)
				if err != nil || mem.UserID != userID {
					continue
				}
				return dedupNoop, hit.ID
			}
			if hit.Score > 0.85 {
				// 语义相似但不完全相同
				mem, err := m.store.GetMemory(hit.ID)
				if err != nil || mem.UserID != userID {
					continue
				}
				slog.Debug("uhms/dedup: vector similarity match", "id", hit.ID, "score", hit.Score)
				return dedupUpdate, hit.ID
			}
		}
	}

	return dedupAdd, ""
}
