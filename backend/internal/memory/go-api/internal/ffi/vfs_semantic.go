//go:build cgo

package ffi

// CGO 绑定：VFS 语义索引 — 专用 FFI API (Phase D)。
//
// 封装 Rust openviking-ffi/vfs_semantic_api.rs 中的 4 个函数。
// 复用 SegmentStore handle（与 vector_api 共享）。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../libs/openviking-rs/target/release -lopenviking_ffi -lm -ldl -framework Security
#cgo CFLAGS: -I${SRCDIR}/../../../../libs/openviking-rs/openviking-ffi/include

// VFS 语义索引 API
extern int ovk_vfs_semantic_init(void* handle, unsigned long dim);

extern int ovk_vfs_semantic_index(void* handle,
    const unsigned char* id, unsigned long id_len,
    const float* dense_vec, unsigned long dense_len,
    const unsigned char* payload_json, unsigned long payload_json_len);

extern int ovk_vfs_semantic_search(void* handle,
    const float* dense_vec, unsigned long dense_len,
    unsigned long limit,
    unsigned char* out_json, unsigned long out_cap);

extern int ovk_vfs_semantic_delete(void* handle,
    const unsigned char* id, unsigned long id_len);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// VFSSemanticInit initializes the VFS semantic index collection.
// dim is the embedding vector dimension.
func (s *SegmentStore) VFSSemanticInit(dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rc := C.ovk_vfs_semantic_init(s.handle, C.ulong(dim))
	if rc != 0 {
		return fmt.Errorf("vfs semantic init: %s", lastError())
	}
	return nil
}

// VFSSemanticIndex indexes a VFS memory entry for semantic search.
func (s *SegmentStore) VFSSemanticIndex(pointID string, denseVec []float32, payloadJSON []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idBytes := []byte(pointID)

	var payloadPtr *C.uchar
	var payloadLen C.ulong
	if len(payloadJSON) > 0 {
		payloadPtr = (*C.uchar)(unsafe.Pointer(&payloadJSON[0]))
		payloadLen = C.ulong(len(payloadJSON))
	}

	rc := C.ovk_vfs_semantic_index(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&idBytes[0])), C.ulong(len(idBytes)),
		(*C.float)(unsafe.Pointer(&denseVec[0])), C.ulong(len(denseVec)),
		payloadPtr, payloadLen,
	)
	if rc != 0 {
		return fmt.Errorf("vfs semantic index %q: %s", pointID, lastError())
	}
	return nil
}

// VFSSemanticSearch queries the VFS semantic index for nearest neighbours.
// Returns up to limit results.
func (s *SegmentStore) VFSSemanticSearch(queryVec []float32, limit int) ([]SegmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const bufSize = 65536
	buf := make([]byte, bufSize)

	rc := C.ovk_vfs_semantic_search(
		s.handle,
		(*C.float)(unsafe.Pointer(&queryVec[0])), C.ulong(len(queryVec)),
		C.ulong(limit),
		(*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(bufSize),
	)
	if rc < 0 {
		return nil, fmt.Errorf("vfs semantic search: %s", lastError())
	}
	if rc == 0 {
		return nil, nil
	}

	var hits []SegmentSearchHit
	if err := json.Unmarshal(buf[:rc], &hits); err != nil {
		return nil, fmt.Errorf("vfs semantic search parse: %w", err)
	}
	return hits, nil
}

// VFSSemanticDelete removes a point from the VFS semantic index.
func (s *SegmentStore) VFSSemanticDelete(pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idBytes := []byte(pointID)

	rc := C.ovk_vfs_semantic_delete(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&idBytes[0])), C.ulong(len(idBytes)),
	)
	if rc != 0 {
		return fmt.Errorf("vfs semantic delete %q: %s", pointID, lastError())
	}
	return nil
}
