package capture

import (
	"fmt"
	"log"
	"time"
)

// CaptureBackend identifies the screen capture implementation.
type CaptureBackend string

const (
	// BackendSCK uses Apple ScreenCaptureKit (macOS 12.3+).
	// Hardware-accelerated, event-driven, supports window-level filtering.
	BackendSCK CaptureBackend = "sck"

	// BackendCG uses legacy CoreGraphics CGWindowListCreateImage.
	// Broader compatibility, polling-based.
	BackendCG CaptureBackend = "cg"
)

// Frame represents a captured screen frame.
type Frame struct {
	Width     int
	Height    int
	Stride    int    // bytes per row (may differ from Width*4 due to alignment)
	Channels  int    // typically 4 (BGRA)
	Pixels    []byte // raw pixel data (BGRA format)
	Timestamp time.Time
	FrameNo   uint64
}

// PixelCount returns the total number of pixels.
func (f *Frame) PixelCount() int {
	return f.Width * f.Height
}

// DataSize returns the size of the pixel data in bytes.
func (f *Frame) DataSize() int {
	return f.Width * f.Height * f.Channels
}

// String returns a summary of the frame.
func (f *Frame) String() string {
	return fmt.Sprintf("Frame#%d %dx%d@%dch (%d bytes)", f.FrameNo, f.Width, f.Height, f.Channels, f.DataSize())
}

// Capturer defines the interface for screen capture across platforms.
type Capturer interface {
	// Start begins screen capture at the given FPS rate.
	Start(fps int) error

	// Stop halts screen capture and releases resources.
	Stop() error

	// LatestFrame returns the most recently captured frame.
	// Returns nil if no frame has been captured yet.
	LatestFrame() *Frame

	// FrameChan returns a shared channel that emits new frames.
	// Deprecated: Use Subscribe() for multi-consumer scenarios.
	FrameChan() <-chan *Frame

	// Subscribe returns a dedicated channel for this consumer.
	// Each subscriber receives a copy of every frame.
	Subscribe() <-chan *Frame

	// Unsubscribe removes a subscriber channel and closes it.
	Unsubscribe(ch <-chan *Frame)

	// IsRunning returns true if the capturer is actively capturing.
	IsRunning() bool

	// DisplayInfo returns information about the captured display.
	DisplayInfo() DisplayInfo

	// ListWindows returns all on-screen windows.
	ListWindows() ([]WindowInfo, error)

	// SetExcludedWindows sets windows to exclude from capture by window ID.
	// Pass empty slice to clear exclusions. Hot-updates the stream filter.
	SetExcludedWindows(windowIDs []uint32) error

	// ExcludeApp excludes all windows belonging to a bundle ID.
	ExcludeApp(bundleID string) error

	// GetExcludedWindows returns currently excluded window IDs.
	GetExcludedWindows() []uint32
}

// WindowInfo represents a window visible on screen.
type WindowInfo struct {
	WindowID uint32 `json:"window_id"`
	Title    string `json:"title"`
	AppName  string `json:"app_name"`
	BundleID string `json:"bundle_id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	OnScreen bool   `json:"on_screen"`
	Layer    int    `json:"layer"`
}

// DisplayInfo contains metadata about the display being captured.
type DisplayInfo struct {
	ID            uint32  `json:"id"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	ScaleFactor   float64 `json:"scale_factor"` // Retina scaling (e.g., 2.0)
	RefreshRateHz int     `json:"refresh_rate_hz"`
}

// String returns a summary of the display.
func (d DisplayInfo) String() string {
	return fmt.Sprintf("Display#%d %dx%d @%.1fx scale, %dHz",
		d.ID, d.Width, d.Height, d.ScaleFactor, d.RefreshRateHz)
}

// CaptureConfig holds configuration for screen capture.
type CaptureConfig struct {
	Backend       CaptureBackend // "sck" (default) or "cg"
	FPS           int            // Target frames per second (1-30)
	DisplayID     uint32         // 0 = main display
	ScaleFactor   float64        // 0 = native, <1 = downscale for performance
	Region        *Region        // nil = full screen
	IncludeCursor bool           // Whether to include cursor in capture
}

// Region defines a rectangular area of the screen.
type Region struct {
	X, Y          int
	Width, Height int
}

// DefaultConfig returns a sensible default capture configuration.
func DefaultConfig() CaptureConfig {
	return CaptureConfig{
		Backend:       BackendSCK,
		FPS:           0, // 0 = auto-detect from display refresh rate
		DisplayID:     0,
		ScaleFactor:   0,
		Region:        nil,
		IncludeCursor: true,
	}
}

// NewCapturer creates a platform-specific capturer based on the configured backend.
// Falls back to CoreGraphics if ScreenCaptureKit initialisation fails.
func NewCapturer(config CaptureConfig) (Capturer, error) {
	switch config.Backend {
	case BackendCG:
		log.Println("[Capture] Backend: CoreGraphics (legacy)")
		return NewDarwinCapturer(config)
	default: // BackendSCK or unset
		cap, err := NewSCKCapturer(config)
		if err != nil {
			log.Printf("[Capture] ScreenCaptureKit init failed: %v — falling back to CoreGraphics", err)
			return NewDarwinCapturer(config)
		}
		log.Println("[Capture] Backend: ScreenCaptureKit")
		return cap, nil
	}
}
