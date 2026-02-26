//go:build cgo

package ffi

// CGO 绑定：openviking-vector — 进程内向量存储 segment API。
//
// 通过 openviking-ffi Rust 库调用 Qdrant segment 引擎，
// 替换远程 gRPC Qdrant 服务。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../libs/openviking-rs/target/release -lopenviking_ffi -lm -ldl -framework Security
#cgo CFLAGS: -I${SRCDIR}/../../../../libs/openviking-rs/openviking-ffi/include

// Segment store lifecycle
extern void* ovk_segment_store_new(const unsigned char* data_dir, unsigned long data_dir_len);
extern void  ovk_segment_store_free(void* handle);

// Collection management
extern int ovk_segment_create_collection(void* handle, const unsigned char* name, unsigned long name_len, unsigned long dim);

// Point operations
extern int ovk_segment_upsert(void* handle,
    const unsigned char* col, unsigned long col_len,
    const unsigned char* id, unsigned long id_len,
    const float* dense_vec, unsigned long dense_len,
    const unsigned char* payload_json, unsigned long payload_json_len);

extern int ovk_segment_search(void* handle,
    const unsigned char* col, unsigned long col_len,
    const float* dense_vec, unsigned long dense_len,
    unsigned long limit,
    unsigned char* out_json, unsigned long out_cap);

extern int ovk_segment_delete(void* handle,
    const unsigned char* col, unsigned long col_len,
    const unsigned char* id, unsigned long id_len);

extern int ovk_segment_flush(void* handle,
    const unsigned char* col, unsigned long col_len);

// Error retrieval
extern int ovk_last_error(unsigned char* buf, unsigned long buf_len);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"sync"
	"unsafe"
)

// SegmentStore wraps the Rust SegmentVectorStore via FFI.
type SegmentStore struct {
	handle unsafe.Pointer
	mu     sync.Mutex
}

// SegmentSearchHit represents a single search result from the segment store.
type SegmentSearchHit struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// NewSegmentStore creates a new in-process segment vector store.
// dataDir is the path where segment data will be stored on disk.
func NewSegmentStore(dataDir string) (*SegmentStore, error) {
	dirBytes := []byte(dataDir)
	handle := C.ovk_segment_store_new(
		(*C.uchar)(unsafe.Pointer(&dirBytes[0])),
		C.ulong(len(dirBytes)),
	)
	if handle == nil {
		return nil, fmt.Errorf("segment store init failed: %s", lastError())
	}
	return &SegmentStore{handle: handle}, nil
}

// Close releases the segment store resources.
func (s *SegmentStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle != nil {
		C.ovk_segment_store_free(s.handle)
		s.handle = nil
	}
}

// CreateCollection creates a new collection with the given name and vector dimension.
func (s *SegmentStore) CreateCollection(name string, dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nameBytes := []byte(name)
	rc := C.ovk_segment_create_collection(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&nameBytes[0])),
		C.ulong(len(nameBytes)),
		C.ulong(dim),
	)
	if rc != 0 {
		return fmt.Errorf("create collection %q: %s", name, lastError())
	}
	return nil
}

// Upsert inserts or updates a point in the specified collection.
// pointID must be a valid UUID string.
// denseVec is the embedding vector.
// payloadJSON is optional JSON payload (pass nil for no payload).
func (s *SegmentStore) Upsert(collection, pointID string, denseVec []float32, payloadJSON []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)
	idBytes := []byte(pointID)

	var payloadPtr *C.uchar
	var payloadLen C.ulong
	if len(payloadJSON) > 0 {
		payloadPtr = (*C.uchar)(unsafe.Pointer(&payloadJSON[0]))
		payloadLen = C.ulong(len(payloadJSON))
	}

	rc := C.ovk_segment_upsert(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
		(*C.uchar)(unsafe.Pointer(&idBytes[0])), C.ulong(len(idBytes)),
		(*C.float)(unsafe.Pointer(&denseVec[0])), C.ulong(len(denseVec)),
		payloadPtr, payloadLen,
	)
	if rc != 0 {
		return fmt.Errorf("upsert %q: %s", pointID, lastError())
	}
	return nil
}

// Search queries the collection for nearest neighbours.
// Returns up to limit results.
func (s *SegmentStore) Search(collection string, queryVec []float32, limit int) ([]SegmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)

	// Allocate output buffer (64KB should be sufficient for most queries)
	const bufSize = 65536
	buf := make([]byte, bufSize)

	rc := C.ovk_segment_search(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
		(*C.float)(unsafe.Pointer(&queryVec[0])), C.ulong(len(queryVec)),
		C.ulong(limit),
		(*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(bufSize),
	)
	if rc < 0 {
		return nil, fmt.Errorf("search: %s", lastError())
	}
	if rc == 0 {
		return nil, nil
	}

	var hits []SegmentSearchHit
	if err := json.Unmarshal(buf[:rc], &hits); err != nil {
		return nil, fmt.Errorf("search result parse: %w", err)
	}
	return hits, nil
}

// Delete removes a point from the collection.
func (s *SegmentStore) Delete(collection, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)
	idBytes := []byte(pointID)

	rc := C.ovk_segment_delete(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
		(*C.uchar)(unsafe.Pointer(&idBytes[0])), C.ulong(len(idBytes)),
	)
	if rc != 0 {
		return fmt.Errorf("delete %q: %s", pointID, lastError())
	}
	return nil
}

// Flush persists all pending changes of a collection to disk.
func (s *SegmentStore) Flush(collection string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)

	rc := C.ovk_segment_flush(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
	)
	if rc != 0 {
		return fmt.Errorf("flush %q: %s", collection, lastError())
	}
	return nil
}

// lastError retrieves the last error message from the Rust FFI layer.
func lastError() string {
	buf := make([]byte, 1024)
	n := C.ovk_last_error(
		(*C.uchar)(unsafe.Pointer(&buf[0])),
		C.ulong(len(buf)),
	)
	if n <= 0 {
		return "unknown error"
	}
	return string(buf[:n])
}
