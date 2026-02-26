// Package services — VFSSemanticIndex: segment-backed semantic index for VFS content.
//
// Phase D migration: changed from direct SegmentStore API calls to dedicated
// VFS semantic FFI functions (ovk_vfs_semantic_*).
//
// Data flow:
//
//	WriteMemory → VFS write + IndexContent (embed → FFI ovk_vfs_semantic_index)
//	SearchMemory → embed → FFI ovk_vfs_semantic_search → URI list → VFS ReadMemory
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/uhms/go-api/internal/ffi"
)

// VFSSemanticCollectionName is the dedicated segment collection for VFS semantic search.
const VFSSemanticCollectionName = "vfs_semantic"

// VFSSemanticHit represents a single result from VFS semantic search.
// Contains the segment search score plus VFS metadata for content retrieval.
type VFSSemanticHit struct {
	MemoryID string  `json:"memory_id"`
	Score    float32 `json:"score"`
	// VFS coordinates for ReadMemory
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	Section  string `json:"section"`
	Category string `json:"category"`
	VFSURI   string `json:"vfs_uri"` // "{section}/{category}/{memoryID}"
	// Content retrieved from segment payload (L0 abstract)
	Content string `json:"content"`
}

// VFSSemanticIndex manages the segment-based semantic index for VFS content.
// Phase D: uses dedicated FFI functions instead of generic SegmentStore API.
type VFSSemanticIndex struct {
	store        *ffi.SegmentStore
	embeddingSvc EmbeddingService
	initialized  bool
}

// NewVFSSemanticIndex creates a new semantic index using existing segment store and embedding service.
func NewVFSSemanticIndex(store *ffi.SegmentStore, embeddingSvc EmbeddingService) *VFSSemanticIndex {
	return &VFSSemanticIndex{
		store:        store,
		embeddingSvc: embeddingSvc,
	}
}

// EnsureCollection creates the vfs_semantic collection if it doesn't exist.
// dim should match the embedding service dimension.
// Phase D: delegates to ovk_vfs_semantic_init FFI.
func (idx *VFSSemanticIndex) EnsureCollection(dim int) error {
	if err := idx.store.VFSSemanticInit(dim); err != nil {
		return fmt.Errorf("vfs semantic init: %w", err)
	}
	idx.initialized = true
	slog.Info("VFS 语义索引集合已就绪 (FFI)", "collection", VFSSemanticCollectionName, "dim", dim)
	return nil
}

// IndexContent embeds the content and stores it in the vfs_semantic collection
// with VFS URI as part of the payload for later retrieval.
//
// This should be called after a successful VFS write. The memoryID serves as
// the point ID (same UUID used in VFS), enabling deletion by ID.
// Phase D: embedding stays in Go, upsert delegates to ovk_vfs_semantic_index FFI.
func (idx *VFSSemanticIndex) IndexContent(
	ctx context.Context,
	memoryID, tenantID, userID, section, category, content string,
) error {
	if !idx.initialized {
		return fmt.Errorf("VFSSemanticIndex not initialized — call EnsureCollection first")
	}

	// 1. Generate dense embedding (stays in Go — LLM HTTP call)
	embedding, err := idx.embeddingSvc.EmbedQuery(ctx, content)
	if err != nil {
		return fmt.Errorf("vfs semantic embed: %w", err)
	}

	// 2. Build payload with VFS URI
	vfsURI := fmt.Sprintf("%s/%s/%s", section, category, memoryID)
	payload := map[string]any{
		"content":   content,
		"tenant_id": tenantID,
		"user_id":   userID,
		"section":   section,
		"category":  category,
		"vfs_uri":   vfsURI,
		"memory_id": memoryID,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("vfs semantic payload marshal: %w", err)
	}

	// 3. Upsert via dedicated FFI function
	if err := idx.store.VFSSemanticIndex(memoryID, embedding, payloadJSON); err != nil {
		return fmt.Errorf("vfs semantic upsert: %w", err)
	}

	slog.Debug("VFS 语义索引已建立 (FFI)",
		"memory_id", memoryID,
		"vfs_uri", vfsURI,
	)
	return nil
}

// RemoveIndex deletes a point from the vfs_semantic collection.
// Phase D: delegates to ovk_vfs_semantic_delete FFI.
func (idx *VFSSemanticIndex) RemoveIndex(memoryID string) error {
	if !idx.initialized {
		return nil // no-op if not initialized
	}
	return idx.store.VFSSemanticDelete(memoryID)
}

// SemanticSearch queries the vfs_semantic collection and returns hits with VFS URIs.
// The caller is responsible for embedding the query text first.
// Phase D: delegates to ovk_vfs_semantic_search FFI.
func (idx *VFSSemanticIndex) SemanticSearch(queryVec []float32, limit int) ([]VFSSemanticHit, error) {
	if !idx.initialized {
		return nil, nil
	}

	hits, err := idx.store.VFSSemanticSearch(queryVec, limit)
	if err != nil {
		return nil, fmt.Errorf("vfs semantic search: %w", err)
	}

	results := make([]VFSSemanticHit, 0, len(hits))
	for _, hit := range hits {
		result := VFSSemanticHit{
			MemoryID: hit.ID,
			Score:    hit.Score,
			Content:  payloadStr(hit.Payload, "content"),
			TenantID: payloadStr(hit.Payload, "tenant_id"),
			UserID:   payloadStr(hit.Payload, "user_id"),
			Section:  payloadStr(hit.Payload, "section"),
			Category: payloadStr(hit.Payload, "category"),
			VFSURI:   payloadStr(hit.Payload, "vfs_uri"),
		}
		results = append(results, result)
	}
	return results, nil
}

// SemanticSearchToFSHits performs semantic search and converts results to
// SearchHit format for compatibility with the existing fuseSearchResults.
func (idx *VFSSemanticIndex) SemanticSearchToFSHits(queryVec []float32, limit int) ([]SearchHit, error) {
	semHits, err := idx.SemanticSearch(queryVec, limit)
	if err != nil {
		return nil, err
	}

	fsHits := make([]SearchHit, 0, len(semHits))
	for _, sh := range semHits {
		fsHits = append(fsHits, SearchHit{
			MemoryID:   sh.MemoryID,
			Score:      float64(sh.Score),
			Path:       sh.VFSURI,
			L0Abstract: sh.Content,
			Category:   sh.Category,
		})
	}
	return fsHits, nil
}
