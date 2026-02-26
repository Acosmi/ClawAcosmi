package imaging

import (
	"image"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// TestRustResizeBGRA_Downscale verifies downscaling a 1920×1080 frame to 640×360.
func TestRustResizeBGRA_Downscale(t *testing.T) {
	w, h := 1920, 1080
	dstW, dstH := 640, 360
	pixels := makeBGRAFrame(w, h)

	result, err := RustResizeBGRA(pixels, w, h, 0, dstW, dstH, AlgoLanczos3)
	if err != nil {
		t.Fatalf("RustResizeBGRA downscale failed: %v", err)
	}

	expected := dstW * dstH * 4
	if len(result) != expected {
		t.Errorf("expected %d bytes, got %d", expected, len(result))
	}
	t.Logf("Downscale 1920x1080 → 640x360: %d bytes", len(result))
}

// TestRustResizeBGRA_Upscale verifies upscaling a 320×240 frame to 640×480.
func TestRustResizeBGRA_Upscale(t *testing.T) {
	w, h := 320, 240
	dstW, dstH := 640, 480
	pixels := makeBGRAFrame(w, h)

	result, err := RustResizeBGRA(pixels, w, h, 0, dstW, dstH, AlgoBilinear)
	if err != nil {
		t.Fatalf("RustResizeBGRA upscale failed: %v", err)
	}

	expected := dstW * dstH * 4
	if len(result) != expected {
		t.Errorf("expected %d bytes, got %d", expected, len(result))
	}
	t.Logf("Upscale 320x240 → 640x480: %d bytes", len(result))
}

// TestRustResizeBGRA_AllAlgorithms verifies all three resize algorithms.
func TestRustResizeBGRA_AllAlgorithms(t *testing.T) {
	w, h := 800, 600
	dstW, dstH := 400, 300
	pixels := makeBGRAFrame(w, h)

	algos := []struct {
		name string
		algo int
	}{
		{"Lanczos3", AlgoLanczos3},
		{"Bilinear", AlgoBilinear},
		{"Nearest", AlgoNearest},
	}

	for _, a := range algos {
		t.Run(a.name, func(t *testing.T) {
			result, err := RustResizeBGRA(pixels, w, h, 0, dstW, dstH, a.algo)
			if err != nil {
				t.Fatalf("RustResizeBGRA %s failed: %v", a.name, err)
			}
			expected := dstW * dstH * 4
			if len(result) != expected {
				t.Errorf("expected %d bytes, got %d", expected, len(result))
			}
		})
	}
}

// TestRustCalcFitSize verifies aspect-ratio-preserving resize calculation.
func TestRustCalcFitSize(t *testing.T) {
	tests := []struct {
		w, h, maxDim int
		expectW      int
		expectH      int
	}{
		{1920, 1080, 640, 640, 360},
		{1080, 1920, 640, 360, 640},
		{500, 500, 320, 320, 320},
		{200, 100, 640, 200, 100}, // already smaller
	}

	for _, tc := range tests {
		outW, outH, err := RustCalcFitSize(tc.w, tc.h, tc.maxDim)
		if err != nil {
			t.Fatalf("RustCalcFitSize(%d,%d,%d) error: %v", tc.w, tc.h, tc.maxDim, err)
		}
		// Allow ±1 pixel rounding tolerance
		if abs(outW-tc.expectW) > 1 || abs(outH-tc.expectH) > 1 {
			t.Errorf("RustCalcFitSize(%d,%d,%d) = (%d,%d), want (%d,%d)",
				tc.w, tc.h, tc.maxDim, outW, outH, tc.expectW, tc.expectH)
		}
		t.Logf("FitSize(%d×%d, max=%d) → %d×%d", tc.w, tc.h, tc.maxDim, outW, outH)
	}
}

// TestRustResizeBGRA_InvalidParams verifies error handling for bad inputs.
func TestRustResizeBGRA_InvalidParams(t *testing.T) {
	_, err := RustResizeBGRA(nil, 100, 100, 0, 50, 50, AlgoLanczos3)
	if err == nil {
		t.Error("expected error for nil pixels, got nil")
	}

	_, err = RustResizeBGRA([]byte{1, 2, 3}, 0, 0, 0, 50, 50, AlgoLanczos3)
	if err == nil {
		t.Error("expected error for zero dimensions, got nil")
	}
}

// --- helpers ---

func makeBGRAFrame(w, h int) []byte {
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = byte(i % 256)         // B
		pixels[i+1] = byte((i / 4) % 256) // G
		pixels[i+2] = byte((i / 8) % 256) // R
		pixels[i+3] = 255                 // A
	}
	return pixels
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ===== Benchmarks =====

// BenchmarkRustResize_1080p benchmarks Rust SIMD resize: 1920×1080 → 640×360.
func BenchmarkRustResize_1080p(b *testing.B) {
	pixels := makeBGRAFrame(1920, 1080)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RustResizeBGRA(pixels, 1920, 1080, 0, 640, 360, AlgoLanczos3)
	}
}

// BenchmarkGoResize_1080p benchmarks Go x/image CatmullRom resize: 1920×1080 → 640×360.
func BenchmarkGoResize_1080p(b *testing.B) {
	w, h := 1920, 1080
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Fill with test data
	for i := range src.Pix {
		src.Pix[i] = byte(i % 256)
	}
	dstW, dstH := 640, 360
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	}
}
