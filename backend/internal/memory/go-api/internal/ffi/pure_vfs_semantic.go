//go:build !cgo

package ffi

// VFSSemanticInit initializes the VFS semantic index collection (pure Go fallback).
// Delegates to the generic CreateCollection method.
func (s *SegmentStore) VFSSemanticInit(dim int) error {
	return s.CreateCollection("vfs_semantic", dim)
}

// VFSSemanticIndex indexes a VFS memory entry for semantic search (pure Go fallback).
// Delegates to the generic Upsert method.
func (s *SegmentStore) VFSSemanticIndex(pointID string, denseVec []float32, payloadJSON []byte) error {
	return s.Upsert("vfs_semantic", pointID, denseVec, payloadJSON)
}

// VFSSemanticSearch queries the VFS semantic index (pure Go fallback).
// Delegates to the generic Search method.
func (s *SegmentStore) VFSSemanticSearch(queryVec []float32, limit int) ([]SegmentSearchHit, error) {
	return s.Search("vfs_semantic", queryVec, limit)
}

// VFSSemanticDelete removes a point from the VFS semantic index (pure Go fallback).
// Delegates to the generic Delete method.
func (s *SegmentStore) VFSSemanticDelete(pointID string) error {
	return s.Delete("vfs_semantic", pointID)
}
