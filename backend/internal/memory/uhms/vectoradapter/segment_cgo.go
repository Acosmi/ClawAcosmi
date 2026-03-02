//go:build cgo

package vectoradapter

// CGO 绑定：openviking-vector — 进程内向量存储 segment API。
// 通过 openviking-ffi Rust 库调用 Qdrant segment 引擎。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../../cli-rust/libs/openviking-rs/target/release -lopenviking_ffi -lm -ldl
#cgo darwin LDFLAGS: -framework Security -framework CoreFoundation
#cgo CFLAGS: -I${SRCDIR}/../../../../../cli-rust/libs/openviking-rs/openviking-ffi/include

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

// Scroll (enumerate points without vector similarity)
extern int ovk_segment_scroll(void* handle,
    const unsigned char* col, unsigned long col_len,
    unsigned long limit,
    unsigned char* out_json, unsigned long out_cap);

extern int ovk_segment_scroll_filtered(void* handle,
    const unsigned char* col, unsigned long col_len,
    const unsigned char* filter_json, unsigned long filter_json_len,
    unsigned long limit,
    unsigned char* out_json, unsigned long out_cap);

// Point count
extern int ovk_segment_point_count(void* handle,
    const unsigned char* col, unsigned long col_len);

// Optimize collection (build HNSW index)
extern int ovk_segment_optimize_collection(void* handle,
    const unsigned char* name, unsigned long name_len);

// V2: Collection creation with JSON config (HNSW, quantization, distance)
extern int ovk_segment_create_collection_v2(void* handle,
    const unsigned char* name, unsigned long name_len,
    const unsigned char* config_json, unsigned long config_json_len);

// V2: Search with configurable search params
extern int ovk_segment_search_v2(void* handle,
    const unsigned char* col, unsigned long col_len,
    const float* dense_vec, unsigned long dense_len,
    unsigned long limit,
    const unsigned char* search_params_json, unsigned long search_params_json_len,
    unsigned char* out_json, unsigned long out_cap);

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

// segmentSearchHit represents a single search result from the segment store.
type segmentSearchHit struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// NewSegmentStore creates a new in-process segment vector store.
// dataDir is the path where segment data will be stored on disk.
func NewSegmentStore(dataDir string) (*SegmentStore, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("vectoradapter: dataDir must not be empty")
	}
	dirBytes := []byte(dataDir)
	handle := C.ovk_segment_store_new(
		(*C.uchar)(unsafe.Pointer(&dirBytes[0])),
		C.ulong(len(dirBytes)),
	)
	if handle == nil {
		return nil, fmt.Errorf("vectoradapter: segment store init failed: %s", lastError())
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
// Idempotent: returns nil if the collection already exists.
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
		return fmt.Errorf("vectoradapter: create collection %q: %s", name, lastError())
	}
	return nil
}

// CreateCollectionV2 creates a collection with full JSON config (HNSW, quantization, distance).
// configJSON is a JSON object matching the Rust FfiCollectionConfig schema.
func (s *SegmentStore) CreateCollectionV2(name string, configJSON []byte) error {
	if len(configJSON) == 0 {
		return fmt.Errorf("vectoradapter: create collection v2 %q: config JSON must not be empty", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	nameBytes := []byte(name)
	rc := C.ovk_segment_create_collection_v2(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&nameBytes[0])),
		C.ulong(len(nameBytes)),
		(*C.uchar)(unsafe.Pointer(&configJSON[0])),
		C.ulong(len(configJSON)),
	)
	if rc != 0 {
		return fmt.Errorf("vectoradapter: create collection v2 %q: %s", name, lastError())
	}
	return nil
}

// Upsert inserts or updates a point in the specified collection.
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
		return fmt.Errorf("vectoradapter: upsert %q: %s", pointID, lastError())
	}
	return nil
}

// Search queries the collection for nearest neighbours. Returns up to limit results.
func (s *SegmentStore) Search(collection string, queryVec []float32, limit int) ([]segmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)

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
		return nil, fmt.Errorf("vectoradapter: search: %s", lastError())
	}
	if rc == 0 {
		return nil, nil
	}

	var hits []segmentSearchHit
	if err := json.Unmarshal(buf[:rc], &hits); err != nil {
		return nil, fmt.Errorf("vectoradapter: search result parse: %w", err)
	}
	return hits, nil
}

// SearchV2 queries the collection with configurable search params (HNSW ef tuning).
// searchParamsJSON is optional — pass nil for default params.
func (s *SegmentStore) SearchV2(collection string, queryVec []float32, limit int, searchParamsJSON []byte) ([]segmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)

	var paramsPtr *C.uchar
	var paramsLen C.ulong
	if len(searchParamsJSON) > 0 {
		paramsPtr = (*C.uchar)(unsafe.Pointer(&searchParamsJSON[0]))
		paramsLen = C.ulong(len(searchParamsJSON))
	}

	// Initial 512KB buffer for large result sets.
	bufSize := 512 * 1024
	buf := make([]byte, bufSize)

	rc := C.ovk_segment_search_v2(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
		(*C.float)(unsafe.Pointer(&queryVec[0])), C.ulong(len(queryVec)),
		C.ulong(limit),
		paramsPtr, paramsLen,
		(*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(bufSize),
	)

	// Negative = required buffer size; retry once.
	if rc < 0 {
		needed := int(-rc)
		buf = make([]byte, needed)
		rc = C.ovk_segment_search_v2(
			s.handle,
			(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
			(*C.float)(unsafe.Pointer(&queryVec[0])), C.ulong(len(queryVec)),
			C.ulong(limit),
			paramsPtr, paramsLen,
			(*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(needed),
		)
		if rc < 0 {
			return nil, fmt.Errorf("vectoradapter: search_v2 buffer retry failed: %s", lastError())
		}
	}
	if rc == 0 {
		return nil, nil
	}

	var hits []segmentSearchHit
	if err := json.Unmarshal(buf[:rc], &hits); err != nil {
		return nil, fmt.Errorf("vectoradapter: search_v2 result parse: %w", err)
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
		return fmt.Errorf("vectoradapter: delete %q: %s", pointID, lastError())
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
		return fmt.Errorf("vectoradapter: flush %q: %s", collection, lastError())
	}
	return nil
}

// scrollHit represents a scroll result (no score, just id + payload).
type scrollHit struct {
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

// Scroll enumerates up to limit points in a collection (no vector similarity).
func (s *SegmentStore) Scroll(collection string, limit int) ([]scrollHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)

	// Initial 512KB buffer — scroll may return large payloads.
	bufSize := 512 * 1024
	buf := make([]byte, bufSize)

	rc := C.ovk_segment_scroll(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
		C.ulong(limit),
		(*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(bufSize),
	)

	// Negative = required buffer size; retry once with the exact size.
	if rc < 0 {
		needed := int(-rc)
		buf = make([]byte, needed)
		rc = C.ovk_segment_scroll(
			s.handle,
			(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
			C.ulong(limit),
			(*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(needed),
		)
		if rc < 0 {
			return nil, fmt.Errorf("vectoradapter: scroll buffer retry failed: %s", lastError())
		}
	}
	if rc == 0 {
		return nil, nil
	}

	var hits []scrollHit
	if err := json.Unmarshal(buf[:rc], &hits); err != nil {
		return nil, fmt.Errorf("vectoradapter: scroll result parse: %w", err)
	}
	return hits, nil
}

// PointCount returns the number of available points in a collection.
func (s *SegmentStore) PointCount(collection string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	colBytes := []byte(collection)
	rc := C.ovk_segment_point_count(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&colBytes[0])), C.ulong(len(colBytes)),
	)
	if rc < 0 {
		return 0, fmt.Errorf("vectoradapter: point_count %q: %s", collection, lastError())
	}
	return int(rc), nil
}

// OptimizeCollection triggers HNSW index build for a collection.
// Returns (true, nil) if optimized, (false, nil) if skipped, (false, err) on error.
func (s *SegmentStore) OptimizeCollection(name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nameBytes := []byte(name)
	rc := C.ovk_segment_optimize_collection(
		s.handle,
		(*C.uchar)(unsafe.Pointer(&nameBytes[0])),
		C.ulong(len(nameBytes)),
	)
	if rc < 0 {
		return false, fmt.Errorf("vectoradapter: optimize %q: %s", name, lastError())
	}
	return rc == 1, nil
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
