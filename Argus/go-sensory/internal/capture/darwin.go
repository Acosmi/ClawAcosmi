package capture

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework ImageIO

#include <stdlib.h>
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>

// Get the main display ID
CGDirectDisplayID get_main_display() {
    return CGMainDisplayID();
}

// Get display dimensions
int get_display_width(CGDirectDisplayID displayID) {
    return (int)CGDisplayPixelsWide(displayID);
}

int get_display_height(CGDirectDisplayID displayID) {
    return (int)CGDisplayPixelsHigh(displayID);
}

// Capture the full desktop using CGWindowListCreateImage (available on macOS 15+).
// This replaces CGDisplayCreateImage which was removed in macOS 15.
unsigned char* capture_screen(CGDirectDisplayID displayID,
                               int* outWidth, int* outHeight,
                               int* outBytesPerRow) {
    CGRect displayBounds = CGDisplayBounds(displayID);

    CGImageRef image = CGWindowListCreateImage(
        displayBounds,
        kCGWindowListOptionOnScreenOnly,
        kCGNullWindowID,
        kCGWindowImageDefault
    );
    if (image == NULL) {
        return NULL;
    }

    size_t width = CGImageGetWidth(image);
    size_t height = CGImageGetHeight(image);
    size_t bytesPerRow = width * 4;
    size_t bufSize = bytesPerRow * height;

    unsigned char* buf = (unsigned char*)malloc(bufSize);
    if (buf == NULL) {
        CGImageRelease(image);
        return NULL;
    }

    CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
    CGContextRef ctx = CGBitmapContextCreate(
        buf, width, height, 8, bytesPerRow, colorSpace,
        kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little
    );

    if (ctx == NULL) {
        free(buf);
        CGColorSpaceRelease(colorSpace);
        CGImageRelease(image);
        return NULL;
    }

    CGRect rect = CGRectMake(0, 0, width, height);
    CGContextDrawImage(ctx, rect, image);

    CGContextRelease(ctx);
    CGColorSpaceRelease(colorSpace);
    CGImageRelease(image);

    *outWidth = (int)width;
    *outHeight = (int)height;
    *outBytesPerRow = (int)bytesPerRow;

    return buf;
}

// Capture a region of the screen
unsigned char* capture_screen_region(CGDirectDisplayID displayID,
                                      int x, int y, int w, int h,
                                      int* outWidth, int* outHeight,
                                      int* outBytesPerRow) {
    CGRect captureRect = CGRectMake(x, y, w, h);

    CGImageRef image = CGWindowListCreateImage(
        captureRect,
        kCGWindowListOptionOnScreenOnly,
        kCGNullWindowID,
        kCGWindowImageDefault
    );
    if (image == NULL) {
        return NULL;
    }

    size_t width = CGImageGetWidth(image);
    size_t height = CGImageGetHeight(image);
    size_t bytesPerRow = width * 4;
    size_t bufSize = bytesPerRow * height;

    unsigned char* buf = (unsigned char*)malloc(bufSize);
    if (buf == NULL) {
        CGImageRelease(image);
        return NULL;
    }

    CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
    CGContextRef ctx = CGBitmapContextCreate(
        buf, width, height, 8, bytesPerRow, colorSpace,
        kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little
    );

    if (ctx == NULL) {
        free(buf);
        CGColorSpaceRelease(colorSpace);
        CGImageRelease(image);
        return NULL;
    }

    CGRect rect = CGRectMake(0, 0, width, height);
    CGContextDrawImage(ctx, rect, image);

    CGContextRelease(ctx);
    CGColorSpaceRelease(colorSpace);
    CGImageRelease(image);

    *outWidth = (int)width;
    *outHeight = (int)height;
    *outBytesPerRow = (int)bytesPerRow;

    return buf;
}

// Get display refresh rate via CGDisplayModeGetRefreshRate.
// Returns 0 if mode is unavailable (caller should fall back to safe default).
double get_display_refresh_rate(CGDirectDisplayID displayID) {
    CGDisplayModeRef mode = CGDisplayCopyDisplayMode(displayID);
    if (!mode) return 0.0;
    double hz = CGDisplayModeGetRefreshRate(mode);
    CGDisplayModeRelease(mode);
    return hz;
}

// Free a buffer allocated by capture_screen
void free_buffer(unsigned char* buf) {
    if (buf != NULL) {
        free(buf);
    }
}
*/
import "C"

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// DarwinCapturer implements the Capturer interface for macOS using CoreGraphics.
type DarwinCapturer struct {
	config    CaptureConfig
	displayID C.CGDirectDisplayID

	latestFrame atomic.Pointer[Frame]
	frameChan   chan *Frame // legacy shared channel
	frameNo     uint64

	subscribers   []chan *Frame
	subscribersMu sync.Mutex

	running  atomic.Bool
	stopChan chan struct{}
	mu       sync.Mutex
}

// NewDarwinCapturer creates a new screen capturer for macOS.
func NewDarwinCapturer(config CaptureConfig) (*DarwinCapturer, error) {
	var displayID C.CGDirectDisplayID
	if config.DisplayID == 0 {
		displayID = C.get_main_display()
	} else {
		displayID = C.CGDirectDisplayID(config.DisplayID)
	}

	return &DarwinCapturer{
		config:    config,
		displayID: displayID,
		frameChan: make(chan *Frame, 4),
		stopChan:  make(chan struct{}),
	}, nil
}

// Start begins screen capture at the configured FPS.
func (d *DarwinCapturer) Start(fps int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running.Load() {
		return fmt.Errorf("capturer is already running")
	}

	if fps < 1 || fps > 30 {
		return fmt.Errorf("FPS must be between 1 and 30, got %d", fps)
	}

	d.config.FPS = fps
	d.running.Store(true)

	go d.captureLoop()
	return nil
}

// Stop halts screen capture.
func (d *DarwinCapturer) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running.Load() {
		return nil
	}

	d.running.Store(false)
	close(d.stopChan)
	return nil
}

// LatestFrame returns the most recently captured frame.
func (d *DarwinCapturer) LatestFrame() *Frame {
	return d.latestFrame.Load()
}

// FrameChan returns the legacy shared channel.
// Deprecated: Use Subscribe() for multi-consumer scenarios.
func (d *DarwinCapturer) FrameChan() <-chan *Frame {
	return d.frameChan
}

// Subscribe returns a new dedicated channel for this consumer.
func (d *DarwinCapturer) Subscribe() <-chan *Frame {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()

	ch := make(chan *Frame, 4)
	d.subscribers = append(d.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (d *DarwinCapturer) Unsubscribe(ch <-chan *Frame) {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()

	for i, sub := range d.subscribers {
		if sub == ch {
			d.subscribers = append(d.subscribers[:i], d.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// IsRunning returns true if the capturer is active.
func (d *DarwinCapturer) IsRunning() bool {
	return d.running.Load()
}

// DisplayInfo returns information about the captured display.
func (d *DarwinCapturer) DisplayInfo() DisplayInfo {
	w := int(C.get_display_width(d.displayID))
	h := int(C.get_display_height(d.displayID))

	hz := int(C.get_display_refresh_rate(d.displayID))
	if hz <= 0 {
		hz = 60 // 安全默认值
	}

	return DisplayInfo{
		ID:            uint32(d.displayID),
		Width:         w,
		Height:        h,
		ScaleFactor:   2.0, // assume Retina
		RefreshRateHz: hz,
	}
}

// CaptureOnce captures a single frame immediately.
func (d *DarwinCapturer) CaptureOnce() (*Frame, error) {
	return d.captureFrame()
}

func (d *DarwinCapturer) captureLoop() {
	ticker := time.NewTicker(time.Second / time.Duration(d.config.FPS))
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			frame, err := d.captureFrame()
			if err != nil {
				continue
			}

			d.latestFrame.Store(frame)

			// Legacy shared channel (non-blocking)
			select {
			case d.frameChan <- frame:
			default:
			}

			// Fan-out to all subscribers (non-blocking)
			d.subscribersMu.Lock()
			for _, sub := range d.subscribers {
				select {
				case sub <- frame:
				default:
					// Slow consumer, drop frame
				}
			}
			d.subscribersMu.Unlock()
		}
	}
}

func (d *DarwinCapturer) captureFrame() (*Frame, error) {
	var cWidth, cHeight, cBytesPerRow C.int

	var buf *C.uchar
	if d.config.Region != nil {
		r := d.config.Region
		buf = C.capture_screen_region(d.displayID,
			C.int(r.X), C.int(r.Y), C.int(r.Width), C.int(r.Height),
			&cWidth, &cHeight, &cBytesPerRow)
	} else {
		buf = C.capture_screen(d.displayID, &cWidth, &cHeight, &cBytesPerRow)
	}

	if buf == nil {
		return nil, fmt.Errorf("screen capture failed")
	}
	defer C.free_buffer(buf)

	width := int(cWidth)
	height := int(cHeight)
	stride := int(cBytesPerRow) // actual bytes per row from CG (may include padding)
	channels := 4
	bufSize := stride * height

	newFrameNo := atomic.AddUint64(&d.frameNo, 1)
	pixels := make([]byte, bufSize)
	copy(pixels, unsafe.Slice((*byte)(unsafe.Pointer(buf)), bufSize))

	return &Frame{
		Width:     width,
		Height:    height,
		Stride:    stride,
		Channels:  channels,
		Pixels:    pixels,
		Timestamp: time.Now(),
		FrameNo:   newFrameNo,
	}, nil
}

// Note: The unified NewCapturer() factory is in capture.go.
// It routes to NewDarwinCapturer (CG) or NewSCKCapturer (SCK) based on config.Backend.

// ── Window management stubs (CG backend does not support window exclusion) ──

// ListWindows is not supported by the CoreGraphics backend.
func (d *DarwinCapturer) ListWindows() ([]WindowInfo, error) {
	return nil, fmt.Errorf("ListWindows not supported on CoreGraphics backend, use SCK")
}

// SetExcludedWindows is not supported by the CoreGraphics backend.
func (d *DarwinCapturer) SetExcludedWindows(windowIDs []uint32) error {
	return fmt.Errorf("SetExcludedWindows not supported on CoreGraphics backend, use SCK")
}

// ExcludeApp is not supported by the CoreGraphics backend.
func (d *DarwinCapturer) ExcludeApp(bundleID string) error {
	return fmt.Errorf("ExcludeApp not supported on CoreGraphics backend, use SCK")
}

// GetExcludedWindows always returns nil for the CoreGraphics backend.
func (d *DarwinCapturer) GetExcludedWindows() []uint32 {
	return nil
}
