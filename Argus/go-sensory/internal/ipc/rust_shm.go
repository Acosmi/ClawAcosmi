package ipc

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include <stdlib.h>
#include "argus_core.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

const (
	DefaultRustShmName   = "/argus_frame"
	DefaultRustSemWriter = "/argus_sem_writer"
	DefaultRustSemReader = "/argus_sem_reader"
)

// RustShmWriter writes video frames to shared memory via Rust FFI.
type RustShmWriter struct {
	handle *C.ShmWriter
	closed bool
}

// NewRustShmWriter creates a shared memory writer using default names.
func NewRustShmWriter(maxWidth, maxHeight, channels int) (*RustShmWriter, error) {
	return NewRustShmWriterWithNames(maxWidth, maxHeight, channels,
		DefaultRustShmName, DefaultRustSemWriter, DefaultRustSemReader)
}

// NewRustShmWriterWithNames creates a shared memory writer with custom names.
func NewRustShmWriterWithNames(maxWidth, maxHeight, channels int, shmName, semWriter, semReader string) (*RustShmWriter, error) {
	if maxWidth <= 0 || maxHeight <= 0 || channels <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%dx%d", maxWidth, maxHeight, channels)
	}

	cShm := C.CString(shmName)
	defer C.free(unsafe.Pointer(cShm))
	cSemW := C.CString(semWriter)
	defer C.free(unsafe.Pointer(cSemW))
	cSemR := C.CString(semReader)
	defer C.free(unsafe.Pointer(cSemR))

	var handle *C.ShmWriter
	rc := C.argus_shm_create(
		cShm, cSemW, cSemR,
		C.int32_t(maxWidth), C.int32_t(maxHeight), C.int32_t(channels),
		&handle,
	)
	if rc != 0 {
		return nil, fmt.Errorf("argus_shm_create failed: error code %d", rc)
	}

	return &RustShmWriter{handle: handle}, nil
}

// WriteFrame writes raw pixel data to shared memory.
// Blocks until a reader has consumed the previous frame.
func (w *RustShmWriter) WriteFrame(width, height, channels int, pixels []byte) error {
	if w.closed || w.handle == nil {
		return fmt.Errorf("writer is closed")
	}
	dataSize := width * height * channels
	if len(pixels) < dataSize {
		return fmt.Errorf("pixel buffer too small: got %d, need %d", len(pixels), dataSize)
	}

	rc := C.argus_shm_write_frame(
		w.handle,
		C.int32_t(width), C.int32_t(height), C.int32_t(channels),
		(*C.uint8_t)(unsafe.Pointer(&pixels[0])),
		C.size_t(len(pixels)),
	)
	if rc != 0 {
		return fmt.Errorf("argus_shm_write_frame failed: error code %d", rc)
	}
	return nil
}

// Close releases all shared memory and semaphore resources.
func (w *RustShmWriter) Close() error {
	if w.closed || w.handle == nil {
		return nil
	}
	w.closed = true
	C.argus_shm_destroy(w.handle)
	w.handle = nil
	return nil
}

// FrameNumber returns the current frame number.
func (w *RustShmWriter) FrameNumber() uint64 {
	if w.handle == nil {
		return 0
	}
	var frameNo C.uint64_t
	C.argus_shm_frame_number(w.handle, &frameNo)
	return uint64(frameNo)
}

// SimulateReaderConsume simulates a reader consuming a frame (for testing).
func (w *RustShmWriter) SimulateReaderConsume() {
	if w.handle != nil {
		C.argus_shm_simulate_reader(w.handle)
	}
}

// CleanupRustShm removes residual SHM and semaphore resources.
func CleanupRustShm(shmName, semWriter, semReader string) {
	cShm := C.CString(shmName)
	defer C.free(unsafe.Pointer(cShm))
	cSemW := C.CString(semWriter)
	defer C.free(unsafe.Pointer(cSemW))
	cSemR := C.CString(semReader)
	defer C.free(unsafe.Pointer(cSemR))

	C.argus_shm_cleanup(cShm, cSemW, cSemR)
}

// CleanupRustDefaults removes the default shared memory and semaphore resources.
func CleanupRustDefaults() {
	CleanupRustShm(DefaultRustShmName, DefaultRustSemWriter, DefaultRustSemReader)
}
