package pipeline

import (
	"testing"
)

// TestRustCalcChangeRatio_Identical verifies identical frames yield ratio ≈ 0.
func TestRustCalcChangeRatio_Identical(t *testing.T) {
	w, h := 640, 480
	frame := makeTestFrame(w, h, 128)

	ratio, err := RustCalcChangeRatio(frame, frame, w, h, w*4, 20)
	if err != nil {
		t.Fatalf("RustCalcChangeRatio failed: %v", err)
	}
	if ratio > 0.001 {
		t.Errorf("expected ratio ≈ 0 for identical frames, got %.4f", ratio)
	}
	t.Logf("Identical frames: change ratio = %.6f", ratio)
}

// TestRustCalcChangeRatio_TotallyDifferent verifies completely different frames yield ratio ≈ 1.
func TestRustCalcChangeRatio_TotallyDifferent(t *testing.T) {
	w, h := 640, 480
	frameA := makeTestFrame(w, h, 0)
	frameB := makeTestFrame(w, h, 255)

	ratio, err := RustCalcChangeRatio(frameA, frameB, w, h, w*4, 20)
	if err != nil {
		t.Fatalf("RustCalcChangeRatio failed: %v", err)
	}
	if ratio < 0.99 {
		t.Errorf("expected ratio ≈ 1.0 for totally different frames, got %.4f", ratio)
	}
	t.Logf("Totally different frames: change ratio = %.6f", ratio)
}

// TestRustCalcChangeRatio_PartialChange verifies partial changes yield intermediate ratio.
func TestRustCalcChangeRatio_PartialChange(t *testing.T) {
	w, h := 100, 100
	frameA := makeTestFrame(w, h, 100)
	frameB := make([]byte, w*h*4)
	copy(frameB, frameA)

	// Change exactly half the pixels (above threshold of 20)
	halfPixels := w * h / 2
	for i := 0; i < halfPixels; i++ {
		off := i * 4
		frameB[off] = 200   // B changed by 100
		frameB[off+1] = 200 // G changed by 100
		frameB[off+2] = 200 // R changed by 100
	}

	ratio, err := RustCalcChangeRatio(frameA, frameB, w, h, w*4, 20)
	if err != nil {
		t.Fatalf("RustCalcChangeRatio failed: %v", err)
	}
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("expected ratio ≈ 0.5 for half-changed frame, got %.4f", ratio)
	}
	t.Logf("Half-changed frame: change ratio = %.4f", ratio)
}

// TestRustFrameHash_Consistent verifies same frame produces same hash.
func TestRustFrameHash_Consistent(t *testing.T) {
	w, h := 640, 480
	frame := makeTestFrame(w, h, 128)

	hash1, err := RustFrameHash(frame, w, h, w*4)
	if err != nil {
		t.Fatalf("RustFrameHash (1st call) failed: %v", err)
	}

	hash2, err := RustFrameHash(frame, w, h, w*4)
	if err != nil {
		t.Fatalf("RustFrameHash (2nd call) failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("inconsistent hash: %016x vs %016x", hash1, hash2)
	}
	t.Logf("Consistent hash: 0x%016x", hash1)
}

// TestRustFrameHash_Different verifies different frames produce different hashes.
func TestRustFrameHash_Different(t *testing.T) {
	w, h := 640, 480

	// dHash compares adjacent pixels, so we need gradient patterns
	// (not solid-fill) to produce non-zero hashes.
	frameA := makeGradientFrame(w, h, false)
	frameB := makeGradientFrame(w, h, true) // reversed gradient

	hashA, err := RustFrameHash(frameA, w, h, w*4)
	if err != nil {
		t.Fatalf("RustFrameHash (A) failed: %v", err)
	}
	hashB, err := RustFrameHash(frameB, w, h, w*4)
	if err != nil {
		t.Fatalf("RustFrameHash (B) failed: %v", err)
	}

	if hashA == hashB {
		t.Errorf("expected different hashes for different gradient frames, both = 0x%016x", hashA)
	}
	t.Logf("Hash A=0x%016x, Hash B=0x%016x", hashA, hashB)
}

// TestRustCalcChangeRatio_InvalidParams verifies error handling.
func TestRustCalcChangeRatio_InvalidParams(t *testing.T) {
	_, err := RustCalcChangeRatio(nil, nil, 100, 100, 400, 20)
	if err == nil {
		t.Error("expected error for nil frames, got nil")
	}
}

// --- helpers ---

func makeTestFrame(w, h int, fill byte) []byte {
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = fill   // B
		pixels[i+1] = fill // G
		pixels[i+2] = fill // R
		pixels[i+3] = 255  // A
	}
	return pixels
}

// makeGradientFrame creates a frame with a horizontal gradient pattern.
// If reverse is true, the gradient runs right-to-left instead of left-to-right.
func makeGradientFrame(w, h int, reverse bool) []byte {
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			val := byte(x * 255 / w)
			if reverse {
				val = byte((w - 1 - x) * 255 / w)
			}
			pixels[off] = val   // B
			pixels[off+1] = val // G
			pixels[off+2] = val // R
			pixels[off+3] = 255 // A
		}
	}
	return pixels
}

// ===== Benchmarks =====

// BenchmarkRustChangeRatio_1080p benchmarks Rust SIMD keyframe diff on 1080p frames.
func BenchmarkRustChangeRatio_1080p(b *testing.B) {
	w, h := 1920, 1080
	frameA := makeTestFrame(w, h, 100)
	frameB := makeTestFrame(w, h, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RustCalcChangeRatio(frameA, frameB, w, h, w*4, 20)
	}
}

// BenchmarkGoChangeRatio_1080p benchmarks pure-Go keyframe diff on 1080p frames.
func BenchmarkGoChangeRatio_1080p(b *testing.B) {
	w, h := 1920, 1080
	frameA := makeTestFrame(w, h, 100)
	frameB := makeTestFrame(w, h, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		goCalcChangeRatio(frameA, frameB, w, h)
	}
}

// goCalcChangeRatio is the Go baseline (mirrors keyframe.go calcChangeRatio).
func goCalcChangeRatio(prev, curr []byte, w, h int) float64 {
	totalPixels := w * h
	changed := 0
	for i := 0; i < totalPixels; i++ {
		off := i * 4
		if off+2 >= len(curr) || off+2 >= len(prev) {
			break
		}
		dr := absDiffByte(curr[off], prev[off])
		dg := absDiffByte(curr[off+1], prev[off+1])
		db := absDiffByte(curr[off+2], prev[off+2])
		if dr > 20 || dg > 20 || db > 20 {
			changed++
		}
	}
	return float64(changed) / float64(totalPixels)
}

func absDiffByte(a, b byte) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}
