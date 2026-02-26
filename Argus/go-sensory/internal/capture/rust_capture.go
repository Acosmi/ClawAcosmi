package capture

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include "argus_core.h"
*/
import "C"
import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// rustBackendMode selects the underlying Rust capture strategy.
type rustBackendMode int

const (
	rustModeCG  rustBackendMode = iota // Legacy CoreGraphics polling
	rustModeSCK                        // ScreenCaptureKit event-driven
)

// RustCapturer implements the Capturer interface using Rust FFI (libargus_core).
//
// Supports two capture strategies:
//   - SCK mode (ScreenCaptureKit): event-driven frame callbacks via
//     argus_sck_*, hardware-accelerated, lower CPU usage.
//   - CG mode (CoreGraphics): polling-based via argus_capture_frame,
//     broader compatibility.
//
// Constructor tries SCK first and falls back to CG automatically.
type RustCapturer struct {
	config CaptureConfig
	mode   rustBackendMode

	latestFrame atomic.Pointer[Frame]
	frameChan   chan *Frame // legacy shared channel

	subscribers   []chan *Frame
	subscribersMu sync.Mutex

	running  atomic.Bool
	stopChan chan struct{}
	mu       sync.Mutex
	frameNo  uint64
}

// NewRustCapturer creates a new Rust-backed screen capturer.
// Tries SCK first for event-driven capture; falls back to CG polling.
func NewRustCapturer(config CaptureConfig) (*RustCapturer, error) {
	// Try SCK first
	displayIdx := 0
	if config.DisplayID != 0 {
		displayIdx = int(config.DisplayID)
	}

	rc := C.argus_sck_discover(C.int32_t(displayIdx))
	if rc == 0 {
		log.Println("[RustCapture] Backend: ScreenCaptureKit (Rust)")
		return &RustCapturer{
			config:    config,
			mode:      rustModeSCK,
			frameChan: make(chan *Frame, 4),
			stopChan:  make(chan struct{}),
		}, nil
	}

	// Fallback to CG
	log.Printf("[RustCapture] SCK discover failed (code %d), falling back to CoreGraphics", rc)
	var dw, dh C.int32_t
	var did C.uint32_t
	rc2 := C.argus_capture_display_info(&dw, &dh, &did)
	if rc2 != 0 {
		return nil, fmt.Errorf("Rust capture init failed: both SCK (code %d) and CG (code %d)", rc, rc2)
	}
	log.Printf("[RustCapture] Backend: CoreGraphics (Rust), Display: %dx%d (id=%d)", dw, dh, did)

	return &RustCapturer{
		config:    config,
		mode:      rustModeCG,
		frameChan: make(chan *Frame, 4),
		stopChan:  make(chan struct{}),
	}, nil
}

// Start begins screen capture.
// SCK mode: starts the Rust stream + Go poller for frame retrieval.
// CG mode: starts a Go polling loop calling argus_capture_frame.
func (r *RustCapturer) Start(fps int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running.Load() {
		return fmt.Errorf("capturer is already running")
	}
	if fps < 1 || fps > 60 {
		return fmt.Errorf("FPS must be between 1 and 60, got %d", fps)
	}

	r.config.FPS = fps
	r.stopChan = make(chan struct{})

	if r.mode == rustModeSCK {
		showCursor := 1
		if !r.config.IncludeCursor {
			showCursor = 0
		}
		rc := C.argus_sck_start_stream(C.int32_t(fps), C.int32_t(showCursor))
		if rc != 0 {
			return fmt.Errorf("argus_sck_start_stream failed: code %d", rc)
		}
	}

	r.running.Store(true)
	go r.frameLoop(fps)
	return nil
}

// Stop halts screen capture.
func (r *RustCapturer) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running.Load() {
		return nil
	}
	r.running.Store(false)
	close(r.stopChan)

	if r.mode == rustModeSCK {
		C.argus_sck_stop_stream()
	}

	return nil
}

// LatestFrame returns the most recently captured frame.
func (r *RustCapturer) LatestFrame() *Frame {
	return r.latestFrame.Load()
}

// FrameChan returns the legacy shared frame channel.
// Deprecated: Use Subscribe() for multi-consumer scenarios.
func (r *RustCapturer) FrameChan() <-chan *Frame {
	return r.frameChan
}

// Subscribe returns a dedicated channel for this consumer.
func (r *RustCapturer) Subscribe() <-chan *Frame {
	r.subscribersMu.Lock()
	defer r.subscribersMu.Unlock()
	ch := make(chan *Frame, 4)
	r.subscribers = append(r.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (r *RustCapturer) Unsubscribe(ch <-chan *Frame) {
	r.subscribersMu.Lock()
	defer r.subscribersMu.Unlock()
	for i, sub := range r.subscribers {
		if sub == ch {
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// IsRunning returns true if the capturer is active.
func (r *RustCapturer) IsRunning() bool {
	return r.running.Load()
}

// DisplayInfo returns metadata about the captured display.
// SCK mode returns scale factor and refresh rate; CG mode uses defaults.
func (r *RustCapturer) DisplayInfo() DisplayInfo {
	if r.mode == rustModeSCK {
		var w, h C.int32_t
		var scale C.double
		var id C.uint32_t
		var hz C.int32_t
		rc := C.argus_sck_display_info(&w, &h, &scale, &id, &hz)
		if rc == 0 {
			return DisplayInfo{
				ID:            uint32(id),
				Width:         int(w),
				Height:        int(h),
				ScaleFactor:   float64(scale),
				RefreshRateHz: int(hz),
			}
		}
	}

	// CG fallback
	var dw, dh C.int32_t
	var did C.uint32_t
	C.argus_capture_display_info(&dw, &dh, &did)
	return DisplayInfo{
		ID:            uint32(did),
		Width:         int(dw),
		Height:        int(dh),
		ScaleFactor:   2.0, // CG doesn't provide scale
		RefreshRateHz: 60,
	}
}

// ListWindows is not yet supported by the Rust backend.
// Returns empty list — use SCKCapturer for window-level features.
func (r *RustCapturer) ListWindows() ([]WindowInfo, error) {
	return nil, fmt.Errorf("Rust backend does not support ListWindows yet")
}

// SetExcludedWindows is not yet supported by the Rust backend.
func (r *RustCapturer) SetExcludedWindows(_ []uint32) error {
	return fmt.Errorf("Rust backend does not support SetExcludedWindows yet")
}

// ExcludeApp is not yet supported by the Rust backend.
func (r *RustCapturer) ExcludeApp(_ string) error {
	return fmt.Errorf("Rust backend does not support ExcludeApp yet")
}

// GetExcludedWindows returns nil — not yet supported.
func (r *RustCapturer) GetExcludedWindows() []uint32 {
	return nil
}

// frameLoop polls for frames at the configured FPS rate.
// SCK mode: polls argus_sck_get_frame (frames buffered by Rust callback).
// CG mode: polls argus_capture_frame (each call does a CG capture).
func (r *RustCapturer) frameLoop(fps int) {
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			var frame *Frame
			if r.mode == rustModeSCK {
				frame = r.captureSCK()
			} else {
				frame = r.captureCG()
			}
			if frame == nil {
				continue
			}

			r.latestFrame.Store(frame)

			// Broadcast to legacy channel (non-blocking)
			select {
			case r.frameChan <- frame:
			default:
			}

			// Broadcast to subscribers (non-blocking)
			r.subscribersMu.Lock()
			for _, ch := range r.subscribers {
				select {
				case ch <- frame:
				default:
				}
			}
			r.subscribersMu.Unlock()
		}
	}
}

// captureSCK retrieves the latest frame from the SCK stream buffer.
func (r *RustCapturer) captureSCK() *Frame {
	var pixels *C.uint8_t
	var width, height, bytesPerRow C.int32_t
	var frameNo C.uint64_t

	rc := C.argus_sck_get_frame(&pixels, &width, &height, &bytesPerRow, &frameNo)
	if rc != 1 {
		return nil // no new frame
	}

	dataSize := int(bytesPerRow) * int(height)

	// Copy from Rust-owned memory to Go-managed memory
	goPixels := C.GoBytes(unsafe.Pointer(pixels), C.int(dataSize))

	// Free the Rust allocation
	C.argus_free_buffer(pixels, C.size_t(dataSize))

	return &Frame{
		Width:     int(width),
		Height:    int(height),
		Stride:    int(bytesPerRow),
		Channels:  4, // BGRA
		Pixels:    goPixels,
		Timestamp: time.Now(),
		FrameNo:   uint64(frameNo),
	}
}

// captureCG calls the CG Rust FFI to capture a single frame (legacy path).
func (r *RustCapturer) captureCG() *Frame {
	var pixels *C.uint8_t
	var width, height, stride C.int32_t

	rc := C.argus_capture_frame(&pixels, &width, &height, &stride)
	if rc != 0 {
		return nil
	}

	dataSize := int(stride) * int(height)

	// Copy from Rust-owned memory to Go-managed memory
	goPixels := C.GoBytes(unsafe.Pointer(pixels), C.int(dataSize))

	// Free the Rust allocation
	C.argus_free_buffer(pixels, C.size_t(dataSize))

	r.frameNo++
	return &Frame{
		Width:     int(width),
		Height:    int(height),
		Stride:    int(stride),
		Channels:  4, // BGRA
		Pixels:    goPixels,
		Timestamp: time.Now(),
		FrameNo:   r.frameNo,
	}
}
