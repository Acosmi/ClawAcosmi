package ipc

import (
	"encoding/binary"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

/*
#include <stdlib.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <semaphore.h>
#include <string.h>
#include <errno.h>

// Create or open a POSIX shared memory segment
int shm_create(const char* name, int size) {
    int fd = shm_open(name, O_CREAT | O_RDWR, 0666);
    if (fd < 0) return -1;
    if (ftruncate(fd, size) < 0) {
        close(fd);
        return -2;
    }
    return fd;
}

// Open an existing shared memory segment (read-only)
int shm_open_ro(const char* name) {
    return shm_open(name, O_RDONLY, 0);
}

// Remove a shared memory segment
int shm_remove(const char* name) {
    return shm_unlink(name);
}

// Create a named semaphore
sem_t* sem_create(const char* name, unsigned int value) {
    return sem_open(name, O_CREAT, 0666, value);
}

// Wait (decrement) on a semaphore
int sem_wait_named(sem_t* sem) {
    return sem_wait(sem);
}

// Try wait (non-blocking)
int sem_trywait_named(sem_t* sem) {
    return sem_trywait(sem);
}

// Post (increment) on a semaphore
int sem_post_named(sem_t* sem) {
    return sem_post(sem);
}

// Close semaphore
int sem_close_named(sem_t* sem) {
    return sem_close(sem);
}

// Remove named semaphore
int sem_remove(const char* name) {
    return sem_unlink(name);
}
*/
import "C"

const (
	// Header layout: [width:4][height:4][channels:4][frameNo:8][timestamp:8][dataSize:4] = 32 bytes
	HeaderSize = 32

	DefaultShmName   = "/argus_frame"
	DefaultSemWriter = "/argus_sem_writer"
	DefaultSemReader = "/argus_sem_reader"
)

// FrameHeader contains metadata about a frame stored in shared memory.
type FrameHeader struct {
	Width     uint32
	Height    uint32
	Channels  uint32
	FrameNo   uint64
	Timestamp int64
	DataSize  uint32
}

// ShmWriter writes video frames to POSIX shared memory.
type ShmWriter struct {
	shmName   string
	semWriter string
	semReader string

	shmFD    int
	shmSize  int
	mmapData []byte

	semW *C.sem_t
	semR *C.sem_t

	frameNo uint64
	mu      sync.Mutex
	closed  bool
}

// NewShmWriter creates a shared memory writer for the given max frame dimensions.
func NewShmWriter(maxWidth, maxHeight, channels int) (*ShmWriter, error) {
	return NewShmWriterWithNames(maxWidth, maxHeight, channels,
		DefaultShmName, DefaultSemWriter, DefaultSemReader)
}

// NewShmWriterWithNames creates a shared memory writer with custom names.
func NewShmWriterWithNames(maxWidth, maxHeight, channels int, shmName, semWriter, semReader string) (*ShmWriter, error) {
	maxDataSize := maxWidth * maxHeight * channels
	shmSize := HeaderSize + maxDataSize

	// Create shared memory
	cName := C.CString(shmName)
	defer C.free(unsafe.Pointer(cName))

	fd := C.shm_create(cName, C.int(shmSize))
	if fd < 0 {
		return nil, fmt.Errorf("failed to create shared memory '%s': errno=%d", shmName, fd)
	}

	// mmap the shared memory
	mmapData, err := syscall.Mmap(int(fd), 0, shmSize,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(int(fd))
		return nil, fmt.Errorf("mmap failed: %w", err)
	}

	// Create semaphores
	cSemW := C.CString(semWriter)
	defer C.free(unsafe.Pointer(cSemW))
	semW := C.sem_create(cSemW, C.uint(1)) // writer starts unlocked
	if semW == C.SEM_FAILED {
		syscall.Munmap(mmapData)
		syscall.Close(int(fd))
		return nil, fmt.Errorf("failed to create writer semaphore")
	}

	cSemR := C.CString(semReader)
	defer C.free(unsafe.Pointer(cSemR))
	semR := C.sem_create(cSemR, C.uint(0)) // reader starts locked
	if semR == C.SEM_FAILED {
		C.sem_close_named(semW)
		syscall.Munmap(mmapData)
		syscall.Close(int(fd))
		return nil, fmt.Errorf("failed to create reader semaphore")
	}

	return &ShmWriter{
		shmName:   shmName,
		semWriter: semWriter,
		semReader: semReader,
		shmFD:     int(fd),
		shmSize:   shmSize,
		mmapData:  mmapData,
		semW:      semW,
		semR:      semR,
	}, nil
}

// WriteFrame writes raw pixel data to shared memory.
// The pixels slice should contain BGRA data of size width*height*channels.
func (w *ShmWriter) WriteFrame(width, height, channels int, pixels []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	dataSize := width * height * channels
	if len(pixels) < dataSize {
		return fmt.Errorf("pixel buffer too small: got %d, need %d", len(pixels), dataSize)
	}
	if HeaderSize+dataSize > w.shmSize {
		return fmt.Errorf("frame too large for shared memory: %d > %d", HeaderSize+dataSize, w.shmSize)
	}

	// Wait for writer semaphore (consumer has finished reading)
	C.sem_wait_named(w.semW)

	// Write header
	w.frameNo++
	now := time.Now().UnixNano()

	binary.LittleEndian.PutUint32(w.mmapData[0:4], uint32(width))
	binary.LittleEndian.PutUint32(w.mmapData[4:8], uint32(height))
	binary.LittleEndian.PutUint32(w.mmapData[8:12], uint32(channels))
	binary.LittleEndian.PutUint64(w.mmapData[12:20], w.frameNo)
	binary.LittleEndian.PutUint64(w.mmapData[20:28], uint64(now))
	binary.LittleEndian.PutUint32(w.mmapData[28:32], uint32(dataSize))

	// Write pixel data — direct memcpy, bypasses GC
	copy(w.mmapData[HeaderSize:HeaderSize+dataSize], pixels)

	// Signal reader
	C.sem_post_named(w.semR)

	return nil
}

// Close releases all resources.
func (w *ShmWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Cleanup semaphores
	if w.semW != nil {
		C.sem_close_named(w.semW)
		cName := C.CString(w.semWriter)
		C.sem_remove(cName)
		C.free(unsafe.Pointer(cName))
	}
	if w.semR != nil {
		C.sem_close_named(w.semR)
		cName := C.CString(w.semReader)
		C.sem_remove(cName)
		C.free(unsafe.Pointer(cName))
	}

	// Cleanup shared memory
	if w.mmapData != nil {
		syscall.Munmap(w.mmapData)
	}
	if w.shmFD > 0 {
		syscall.Close(w.shmFD)
	}

	cShmName := C.CString(w.shmName)
	C.shm_remove(cShmName)
	C.free(unsafe.Pointer(cShmName))

	return nil
}

// FrameNumber returns the current frame number.
func (w *ShmWriter) FrameNumber() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.frameNo
}

// ShmInfo returns info about the shared memory segment for debugging.
func (w *ShmWriter) ShmInfo() map[string]interface{} {
	return map[string]interface{}{
		"shm_name":   w.shmName,
		"shm_size":   w.shmSize,
		"shm_fd":     w.shmFD,
		"frame_no":   w.frameNo,
		"header_size": HeaderSize,
	}
}

// --- Utility functions for cleanup ---

// CleanupShm removes shared memory and semaphore resources.
// Useful for cleaning up after a crash.
func CleanupShm(shmName, semWriter, semReader string) {
	cShm := C.CString(shmName)
	C.shm_remove(cShm)
	C.free(unsafe.Pointer(cShm))

	cSemW := C.CString(semWriter)
	C.sem_remove(cSemW)
	C.free(unsafe.Pointer(cSemW))

	cSemR := C.CString(semReader)
	C.sem_remove(cSemR)
	C.free(unsafe.Pointer(cSemR))
}

// CleanupDefaults removes the default shared memory and semaphore resources.
func CleanupDefaults() {
	CleanupShm(DefaultShmName, DefaultSemWriter, DefaultSemReader)
}

// --- Helper to check if shm file exists ---

// ShmExists checks if the shared memory segment exists.
// Uses shm_open instead of /dev/shm path (which doesn't exist on macOS).
func ShmExists(shmName string) bool {
	cName := C.CString(shmName)
	defer C.free(unsafe.Pointer(cName))
	fd := C.shm_open_ro(cName)
	if fd < 0 {
		return false
	}
	C.close(C.int(fd))
	return true
}

// SimulateReaderConsume simulates a reader consuming a frame.
// Used in tests to complete the producer/consumer cycle without a real reader.
func (w *ShmWriter) SimulateReaderConsume() {
	C.sem_post_named(w.semW)
	C.sem_wait_named(w.semR)
}
