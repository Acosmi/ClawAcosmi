// Package services — segment-backed vector storage (FFI).
// Uses in-process Qdrant segment engine via Rust FFI.
// RUST_CANDIDATE: bm25_tokenizer — BM25 分词后续迁移 Rust (nexus-tokenizer)
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/agfs"
	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/ffi"
)

// VectorStoreService manages memory vectors via in-process segment engine (FFI).
// Uses dense vectors with cosine similarity.
type VectorStoreService struct {
	store          *ffi.SegmentStore
	embeddingSvc   EmbeddingService
	collectionName string
	dimension      int
	initialized    bool
	embeddingQueue *EmbeddingQueue  // async batch embedding pipeline
	agfsClient     *agfs.AGFSClient // AGFS distributed backend
}

// VectorSearchResult represents a single hybrid search result.
type VectorSearchResult struct {
	MemoryID        uuid.UUID      `json:"memory_id"`
	Content         string         `json:"content"`
	Score           float64        `json:"score"`
	UserID          string         `json:"user_id"`
	MemoryType      string         `json:"memory_type"`
	Category        string         `json:"category"`
	ImportanceScore float64        `json:"importance_score"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	// L0Abstract holds the L0 tier summary for progressive loading (Phase 1).
	// Populated by SearchMemories when VFS is available (fs/hybrid mode).
	L0Abstract string `json:"l0_abstract,omitempty"`
	// L1Overview holds the L1 overview content for progressive loading (Phase 2).
	// Populated by SearchMemories when TieredLoader is available.
	L1Overview string `json:"l1_overview,omitempty"`
	// AvailableLevels indicates which tier levels can be requested via detail API (Phase 3).
	// Typical values: [0,1,2] for standard, [0] for imagination.
	AvailableLevels []int `json:"available_levels,omitempty"`
	// EventTime holds the bi-temporal event time for this memory (N1).
	// Populated from payload metadata during search.
	EventTime *time.Time `json:"event_time,omitempty"`
}

// --- Multi-Collection Routing (mirrors Rust CollectionSchemas) ---

// collectionMap maps cognitive memory types to segment collection names.
// Each collection has identical schema (dense + sparse vectors) but isolated data.
var collectionMap = map[string]string{
	MemoryTypeEpisodic:    "mem_episodic",
	MemoryTypeSemantic:    "mem_semantic",
	MemoryTypeProcedural:  "mem_procedural",
	MemoryTypePermanent:   "mem_permanent",
	MemoryTypeImagination: "mem_permanent", // shares collection with permanent
}

// collectionForType returns the segment collection for a given memory type.
// Legacy types (observation, dialogue, etc.) are normalized first.
func collectionForType(memoryType string) string {
	normalized := NormalizeMemoryType(memoryType)
	if c, ok := collectionMap[normalized]; ok {
		return c
	}
	return "mem_episodic" // default fallback
}

// allCollectionNames returns deduplicated collection names.
func allCollectionNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, col := range collectionMap {
		if !seen[col] {
			seen[col] = true
			names = append(names, col)
		}
	}
	return names
}

// collectionsForSearch determines which collections to search.
// If memoryTypes is specified, returns their mapped collections (deduplicated).
// If empty, returns all collections.
func collectionsForSearch(memoryTypes []string) []string {
	if len(memoryTypes) == 0 {
		return allCollectionNames()
	}
	seen := map[string]bool{}
	var cols []string
	for _, mt := range memoryTypes {
		c := collectionForType(mt)
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	return cols
}

// Initialize sets up the in-process segment vector store.
func (v *VectorStoreService) Initialize(cfg *config.Config) error {
	v.dimension = cfg.EmbeddingDimension

	// Create segment store (in-process vector engine).
	store, err := ffi.NewSegmentStore(cfg.MemFSRootPath + "/segment-vectors")
	if err != nil {
		return fmt.Errorf("segment store init: %w", err)
	}
	v.store = store
	slog.Info("Segment vector store initialized", "data_dir", cfg.MemFSRootPath+"/segment-vectors")

	// Initialize embedding service
	v.embeddingSvc = GetEmbeddingService()
	if v.embeddingSvc != nil {
		v.dimension = v.embeddingSvc.Dimension()
	}

	// Ensure all per-type collections exist.
	if err := v.ensureAllCollections(); err != nil {
		return fmt.Errorf("ensure collections: %w", err)
	}

	v.initialized = true

	// Initialize AGFS client for distributed queue.
	if cfg.AGFSServerURL != "" {
		v.agfsClient = agfs.NewAGFSClient(cfg.AGFSServerURL)
		slog.Info("AGFS client initialized for embedding queue", "url", cfg.AGFSServerURL)

		// Initialize distributed VFS path lock with AGFS backend.
		InitVFSLock(v.agfsClient)
	}

	// Start async embedding queue (requires both embedding service + AGFS).
	if v.embeddingSvc != nil && v.agfsClient != nil {
		v.embeddingQueue = NewEmbeddingQueue(
			DefaultEmbeddingQueueConfig(),
			v.embeddingSvc,
			v.store,
			v.agfsClient,
		)
	} else if v.embeddingSvc != nil {
		slog.Warn("AGFS not configured, embedding queue disabled — sync fallback only")
	}

	slog.Info("VectorStoreService initialized",
		"collections", allCollectionNames(),
		"dimension", v.dimension,
	)
	return nil
}

// ensureAllCollections creates all per-type collections if they don't exist.
func (v *VectorStoreService) ensureAllCollections() error {
	for _, colName := range allCollectionNames() {
		if err := v.store.CreateCollection(colName, v.dimension); err != nil {
			return fmt.Errorf("create collection %s: %w", colName, err)
		}
		slog.Debug("Ensured collection", "collection", colName)
	}
	return nil
}

// embedText generates a dense embedding for text using the configured embedding service.
func (v *VectorStoreService) embedText(ctx context.Context, text string) ([]float32, error) {
	if v.embeddingSvc == nil {
		return nil, fmt.Errorf("embedding service not initialized")
	}
	return v.embeddingSvc.EmbedQuery(ctx, text)
}

// ComputeCosineSimilarity computes the cosine similarity between two text strings.
// ALG-OPT-03: Used by FindSimilarEntities for semantic entity deduplication.
// Returns a value in [-1, 1] where 1 = identical direction, 0 = orthogonal.
func (v *VectorStoreService) ComputeCosineSimilarity(ctx context.Context, text1, text2 string) (float64, error) {
	vec1, err := v.embedText(ctx, text1)
	if err != nil {
		return 0, fmt.Errorf("embed text1: %w", err)
	}
	vec2, err := v.embedText(ctx, text2)
	if err != nil {
		return 0, fmt.Errorf("embed text2: %w", err)
	}
	return cosineSimF32(vec1, vec2), nil
}

// cosineSimF32 computes cos(θ) between two float32 vectors.
func cosineSimF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// AddMemory enqueues a memory for async embedding + upsert.
// Returns immediately (<1ms) — actual embedding happens in background.
// Falls back to synchronous mode if queue is not available.
func (v *VectorStoreService) AddMemory(
	ctx context.Context,
	memoryID uuid.UUID,
	content, userID, memoryType string,
	importanceScore float64,
	metadata map[string]any,
) error {
	if !v.initialized || v.store == nil {
		slog.Warn("VectorStore not initialized, skipping AddMemory", "memory_id", memoryID)
		return nil
	}

	// Async path: enqueue and return immediately.
	if v.embeddingQueue != nil {
		return v.embeddingQueue.Enqueue(EmbedItem{
			MemoryID:        memoryID,
			Content:         content,
			UserID:          userID,
			MemoryType:      memoryType,
			ImportanceScore: importanceScore,
			Metadata:        metadata,
		})
	}

	// Sync fallback: embed + upsert inline (original behavior).
	return v.addMemorySync(ctx, memoryID, content, userID, memoryType, importanceScore, metadata)
}

// addMemorySync is the synchronous embed + upsert path.
func (v *VectorStoreService) addMemorySync(
	ctx context.Context,
	memoryID uuid.UUID,
	content, userID, memoryType string,
	importanceScore float64,
	metadata map[string]any,
) error {
	// Generate dense embedding
	denseVector, err := v.embedText(ctx, content)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	// Build payload as JSON
	payload := map[string]any{
		"content":          content,
		"user_id":          userID,
		"memory_type":      memoryType,
		"importance_score": importanceScore,
	}
	for k, val := range metadata {
		payload[k] = val
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Upsert via segment store FFI
	col := collectionForType(memoryType)
	if err := v.store.Upsert(col, memoryID.String(), denseVector, payloadJSON); err != nil {
		return fmt.Errorf("segment upsert: %w", err)
	}

	slog.Debug("Added memory to vector store (sync)", "memory_id", memoryID, "collection", col)
	return nil
}

// HybridSearch performs dense vector search with optional reranking.
// Searches across per-type collections and merges results.
// Note: segment engine uses dense-only search; payload filtering done in Go.
func (v *VectorStoreService) HybridSearch(
	ctx context.Context,
	query, userID string,
	limit int,
	memoryTypes []string,
	minImportance float64,
	category string,
	createdAfter, createdBefore *time.Time,
	eventAfter, eventBefore *time.Time,
) ([]VectorSearchResult, error) {
	if !v.initialized || v.store == nil {
		slog.Warn("VectorStore not initialized, returning empty results")
		return nil, nil
	}

	if limit <= 0 {
		limit = 5
	}

	// Generate query embedding
	denseVector, err := v.embedText(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	// Determine which collections to search.
	cols := collectionsForSearch(memoryTypes)

	// Search each collection via segment store and aggregate results.
	// Fetch extra results (3x) to allow Go-side filtering.
	fetchLimit := limit * 3
	var allResults []VectorSearchResult
	for _, col := range cols {
		hits, serr := v.store.Search(col, denseVector, fetchLimit)
		if serr != nil {
			slog.Warn("Segment search failed", "collection", col, "error", serr)
			continue
		}
		for _, hit := range hits {
			r := segmentHitToResult(hit)
			// Go-side payload filtering (segment doesn't support filters).
			if r.UserID != userID {
				continue
			}
			if minImportance > 0 && r.ImportanceScore < minImportance {
				continue
			}
			if category != "" && r.Category != category {
				continue
			}
			// Bi-temporal: event_time 范围过滤
			if (eventAfter != nil || eventBefore != nil) && r.EventTime != nil {
				if eventAfter != nil && r.EventTime.Before(*eventAfter) {
					continue
				}
				if eventBefore != nil && r.EventTime.After(*eventBefore) {
					continue
				}
			} else if (eventAfter != nil || eventBefore != nil) && r.EventTime == nil {
				// 搜索要求 event_time 范围但该记忆无 event_time → 排除
				continue
			}
			allResults = append(allResults, r)
		}
	}

	// Sort by score descending and trim to limit.
	sortAndTruncate(&allResults, limit)

	// Rerank: if RerankService available and results >= 2, rerank.
	if reranker := GetRerankService(); reranker != nil && len(allResults) >= 2 {
		docs := make([]string, len(allResults))
		for i, r := range allResults {
			docs[i] = r.Content
		}
		rerankResults, rerr := reranker.Rerank(ctx, query, docs, limit)
		if rerr != nil {
			slog.Warn("Rerank failed, using original order", "error", rerr)
		} else if len(rerankResults) > 0 {
			reranked := make([]VectorSearchResult, 0, len(rerankResults))
			for _, rr := range rerankResults {
				if rr.Index < len(allResults) {
					r := allResults[rr.Index]
					r.Score = rr.Score
					reranked = append(reranked, r)
				}
			}
			allResults = reranked
		}
	}

	return allResults, nil
}

// segmentHitToResult converts a SegmentSearchHit to VectorSearchResult.
func segmentHitToResult(hit ffi.SegmentSearchHit) VectorSearchResult {
	memoryID, _ := uuid.Parse(hit.ID)
	r := VectorSearchResult{
		MemoryID:        memoryID,
		Content:         payloadStr(hit.Payload, "content"),
		Score:           float64(hit.Score),
		UserID:          payloadStr(hit.Payload, "user_id"),
		MemoryType:      payloadStr(hit.Payload, "memory_type"),
		Category:        payloadStr(hit.Payload, "category"),
		ImportanceScore: payloadFlt(hit.Payload, "importance_score"),
	}
	// Bi-temporal: 从 payload 中解析 event_time
	if etStr := payloadStr(hit.Payload, "event_time"); etStr != "" {
		if et, err := time.Parse(time.RFC3339, etStr); err == nil {
			r.EventTime = &et
		}
	}
	return r
}

// sortAndTruncate sorts results by score descending and trims to limit.
func sortAndTruncate(results *[]VectorSearchResult, limit int) {
	rs := *results
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Score > rs[j-1].Score; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
	if len(rs) > limit {
		*results = rs[:limit]
	}
}

// DeleteMemory removes a memory from all collections.
func (v *VectorStoreService) DeleteMemory(ctx context.Context, memoryID uuid.UUID) error {
	if !v.initialized || v.store == nil {
		slog.Warn("VectorStore not initialized, skipping DeleteMemory", "memory_id", memoryID)
		return nil
	}

	pointID := memoryID.String()
	var lastErr error
	for _, col := range allCollectionNames() {
		if err := v.store.Delete(col, pointID); err != nil {
			lastErr = err
			slog.Warn("Delete from collection failed", "collection", col, "memory_id", pointID, "error", err)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("segment delete: %w", lastErr)
	}

	slog.Debug("Deleted memory from vector store", "memory_id", pointID)
	return nil
}

// Close drains the embedding queue then closes the segment store.
func (v *VectorStoreService) Close() error {
	if v.embeddingQueue != nil {
		v.embeddingQueue.Close()
		v.embeddingQueue = nil
	}

	if v.store != nil {
		v.store.Close()
		v.store = nil
		v.initialized = false
		slog.Info("VectorStoreService closed")
	}
	return nil
}

// --- Payload Helpers (for map[string]interface{} payloads) ---

func payloadStr(payload map[string]interface{}, key string) string {
	if v, ok := payload[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func payloadFlt(payload map[string]interface{}, key string) float64 {
	if v, ok := payload[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		}
	}
	return 0
}

// --- Accessors ---

// GetSegmentStore returns the underlying SegmentStore handle.
// Used by VFSSemanticIndex to create dedicated FFI bindings.
func (v *VectorStoreService) GetSegmentStore() *ffi.SegmentStore {
	return v.store
}

// GetEmbeddingSvc returns the embedding service.
func (v *VectorStoreService) GetEmbeddingSvc() EmbeddingService {
	return v.embeddingSvc
}

// Dimension returns the configured embedding dimension.
func (v *VectorStoreService) Dimension() int {
	return v.dimension
}

// --- Singleton ---

var (
	vectorStoreOnce    sync.Once
	vectorStoreService *VectorStoreService
)

// GetVectorStore returns the singleton VectorStoreService.
func GetVectorStore() *VectorStoreService {
	vectorStoreOnce.Do(func() {
		vectorStoreService = &VectorStoreService{}
	})
	return vectorStoreService
}

// InitVectorStore initializes the singleton with config.
func InitVectorStore(cfg *config.Config) error {
	svc := GetVectorStore()
	return svc.Initialize(cfg)
}

// --- Helper: Sparse BM25 Embedding ---

// SparseBM25Embed generates a sparse BM25-style embedding for text.
// Uses word-level tokenization with normalized term frequency.
// Returns indices and values for sparse vector.
func SparseBM25Embed(text string) (indices []uint32, values []float32) {
	// Word-level tokenization (split on whitespace + basic CJK character handling)
	words := tokenizeBM25(text)
	if len(words) == 0 {
		return nil, nil
	}

	// Count term frequencies
	termFreq := make(map[uint32]float32)
	for _, word := range words {
		hash := hashWord(word) % 100000 // Match Python's vocabulary size
		termFreq[hash]++
	}

	// Normalize by total word count
	total := float32(len(words))
	for idx, freq := range termFreq {
		indices = append(indices, idx)
		values = append(values, freq/total)
	}
	return indices, values
}

// tokenizeBM25 splits text into tokens for BM25.
// Handles both space-separated tokens and CJK individual characters.
func tokenizeBM25(text string) []string {
	text = strings.ToLower(text)
	var tokens []string

	for _, word := range strings.Fields(text) {
		// For CJK characters, split into individual characters
		runes := []rune(word)
		hasCJK := false
		for _, r := range runes {
			if isCJK(r) {
				hasCJK = true
				tokens = append(tokens, string(r))
			}
		}
		if !hasCJK {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// isCJK checks if a rune is a CJK character.
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0xF900 && r <= 0xFAFF) // CJK Compatibility Ideographs
}

// hashWord computes a simple hash for a word token.
func hashWord(word string) uint32 {
	h := uint32(0)
	for _, r := range word {
		h = h*31 + uint32(r)
	}
	return h
}
