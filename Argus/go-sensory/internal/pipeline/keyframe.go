// Package pipeline implements frame processing: keyframe extraction,
// PII filtering, and async pipeline orchestration.
package pipeline

import (
	"bytes"
	"image"
	"image/jpeg"
	"log"
	"sync"
	"time"
)

// Keyframe holds a captured keyframe with metadata.
type Keyframe struct {
	FrameNo       int64          `json:"frame_no"`
	Timestamp     float64        `json:"timestamp"`
	ChangeRatio   float64        `json:"change_ratio"`
	TriggerReason string         `json:"trigger_reason"`
	ThumbnailJPEG []byte         `json:"-"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// KeyframeExtractorConfig holds tunable parameters.
type KeyframeExtractorConfig struct {
	ChangeThreshold float64 // min pixel-change ratio to trigger (default 0.05)
	MinIntervalMs   int     // cooldown between keyframes (default 500)
	MaxIntervalMs   int     // max gap before forced capture (default 30000)
	MaxBufferSize   int     // ring buffer capacity (default 1000)
	ThumbnailMax    int     // thumbnail longest edge (default 320)
	ThumbnailQ      int     // JPEG quality 1-100 (default 60)
}

// DefaultKeyframeConfig returns sensible defaults.
func DefaultKeyframeConfig() KeyframeExtractorConfig {
	return KeyframeExtractorConfig{
		ChangeThreshold: 0.05,
		MinIntervalMs:   500,
		MaxIntervalMs:   30000,
		MaxBufferSize:   1000,
		ThumbnailMax:    320,
		ThumbnailQ:      60,
	}
}

// KeyframeExtractor detects significant frames by pixel-change ratio.
type KeyframeExtractor struct {
	cfg KeyframeExtractorConfig

	mu            sync.Mutex
	prevPixels    []byte // raw RGBA/RGB previous frame
	prevWidth     int
	prevHeight    int
	prevTime      float64
	keyframes     []Keyframe
	frameCount    int64
	keyframeCount int64
}

// NewKeyframeExtractor creates an extractor with the given config.
func NewKeyframeExtractor(cfg KeyframeExtractorConfig) *KeyframeExtractor {
	return &KeyframeExtractor{cfg: cfg}
}

// ProcessFrame evaluates a raw RGBA frame and returns a Keyframe if significant.
// pixels must be width*height*4 bytes (RGBA).
func (ke *KeyframeExtractor) ProcessFrame(pixels []byte, width, height int, frameNo int64, ts float64) *Keyframe {
	ke.mu.Lock()
	defer ke.mu.Unlock()

	if ts == 0 {
		ts = float64(time.Now().UnixMilli()) / 1000.0
	}
	ke.frameCount++

	// First frame is always a keyframe.
	if ke.prevPixels == nil {
		ke.prevPixels = copyBytes(pixels)
		ke.prevWidth = width
		ke.prevHeight = height
		ke.prevTime = ts
		return ke.createKeyframe(pixels, width, height, frameNo, ts, 1.0, "initial")
	}

	// Cooldown check.
	elapsedMs := (ts - ke.prevTime) * 1000
	if elapsedMs < float64(ke.cfg.MinIntervalMs) {
		return nil
	}

	// Calculate pixel change ratio.
	changeRatio := ke.calcChangeRatio(pixels, width, height)

	shouldCapture := false
	reason := ""

	if changeRatio >= ke.cfg.ChangeThreshold {
		shouldCapture = true
		reason = "threshold"
	} else if elapsedMs >= float64(ke.cfg.MaxIntervalMs) {
		shouldCapture = true
		reason = "max_interval"
	}

	if shouldCapture {
		ke.prevPixels = copyBytes(pixels)
		ke.prevWidth = width
		ke.prevHeight = height
		ke.prevTime = ts
		return ke.createKeyframe(pixels, width, height, frameNo, ts, changeRatio, reason)
	}

	return nil
}

// ForceCapture captures a keyframe regardless of change ratio.
func (ke *KeyframeExtractor) ForceCapture(pixels []byte, width, height int, frameNo int64, reason string) *Keyframe {
	ke.mu.Lock()
	defer ke.mu.Unlock()

	ts := float64(time.Now().UnixMilli()) / 1000.0
	changeRatio := ke.calcChangeRatio(pixels, width, height)
	ke.prevPixels = copyBytes(pixels)
	ke.prevWidth = width
	ke.prevHeight = height
	ke.prevTime = ts
	return ke.createKeyframe(pixels, width, height, frameNo, ts, changeRatio, reason)
}

// RecentKeyframes returns the last n keyframes.
func (ke *KeyframeExtractor) RecentKeyframes(n int) []Keyframe {
	ke.mu.Lock()
	defer ke.mu.Unlock()

	if n > len(ke.keyframes) {
		n = len(ke.keyframes)
	}
	out := make([]Keyframe, n)
	copy(out, ke.keyframes[len(ke.keyframes)-n:])
	return out
}

// Stats returns pipeline statistics.
func (ke *KeyframeExtractor) Stats() map[string]any {
	ke.mu.Lock()
	defer ke.mu.Unlock()

	ratio := 0.0
	if ke.frameCount > 0 {
		ratio = float64(ke.keyframeCount) / float64(ke.frameCount)
	}
	return map[string]any{
		"total_frames":    ke.frameCount,
		"total_keyframes": ke.keyframeCount,
		"capture_ratio":   ratio,
		"buffer_size":     len(ke.keyframes),
		"threshold":       ke.cfg.ChangeThreshold,
	}
}

// --- internal helpers ---

func (ke *KeyframeExtractor) calcChangeRatio(current []byte, width, height int) float64 {
	if ke.prevPixels == nil {
		return 1.0
	}
	if ke.prevWidth != width || ke.prevHeight != height {
		return 1.0 // resolution changed → 100%
	}
	if len(current) != len(ke.prevPixels) {
		return 1.0
	}

	totalPixels := width * height
	changedPixels := 0
	stride := 4 // RGBA

	for i := 0; i < totalPixels; i++ {
		off := i * stride
		if off+2 >= len(current) {
			break
		}
		// A pixel is "changed" if any RGB channel differs by > 20.
		dr := absDiff(current[off], ke.prevPixels[off])
		dg := absDiff(current[off+1], ke.prevPixels[off+1])
		db := absDiff(current[off+2], ke.prevPixels[off+2])
		if dr > 20 || dg > 20 || db > 20 {
			changedPixels++
		}
	}

	return float64(changedPixels) / float64(totalPixels)
}

func (ke *KeyframeExtractor) createKeyframe(pixels []byte, width, height int, frameNo int64, ts, changeRatio float64, reason string) *Keyframe {
	ke.keyframeCount++

	thumb := ke.createThumbnail(pixels, width, height)

	kf := Keyframe{
		FrameNo:       frameNo,
		Timestamp:     ts,
		ChangeRatio:   changeRatio,
		TriggerReason: reason,
		ThumbnailJPEG: thumb,
		Metadata: map[string]any{
			"keyframe_id":       ke.keyframeCount,
			"total_frames_seen": ke.frameCount,
			"width":             width,
			"height":            height,
		},
	}

	ke.keyframes = append(ke.keyframes, kf)

	// Trim ring buffer.
	if len(ke.keyframes) > ke.cfg.MaxBufferSize {
		ke.keyframes = ke.keyframes[len(ke.keyframes)-ke.cfg.MaxBufferSize:]
	}

	log.Printf("[Keyframe] #%d: frame=%d change=%.3f reason=%s",
		ke.keyframeCount, frameNo, changeRatio, reason)

	return &kf
}

func (ke *KeyframeExtractor) createThumbnail(pixels []byte, width, height int) []byte {
	// Build RGBA image from raw bytes.
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, pixels)

	// Scale down if needed.
	maxDim := ke.cfg.ThumbnailMax
	if width <= maxDim && height <= maxDim {
		// Small enough, encode directly.
		return jpegEncode(img, ke.cfg.ThumbnailQ)
	}

	// Simple nearest-neighbor downscale.
	ratio := float64(maxDim) / float64(max(width, height))
	newW := int(float64(width) * ratio)
	newH := int(float64(height) * ratio)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	thumb := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * height / newH
		for x := 0; x < newW; x++ {
			srcX := x * width / newW
			thumb.Set(x, y, img.At(srcX, srcY))
		}
	}

	return jpegEncode(thumb, ke.cfg.ThumbnailQ)
}

// --- utility functions ---

func absDiff(a, b byte) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

func copyBytes(src []byte) []byte {
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func jpegEncode(img image.Image, quality int) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		log.Printf("[Keyframe] thumbnail encode error: %v", err)
		return nil
	}
	return buf.Bytes()
}
