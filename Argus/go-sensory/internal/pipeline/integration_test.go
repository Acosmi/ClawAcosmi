package pipeline

import (
	"testing"

	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/ipc"
)

// TestFullPipeline_RustPath verifies the full pipeline:
// Simulate frame → Rust Resize → Rust Keyframe Diff → Rust Hash → Rust SHM Write.
func TestFullPipeline_RustPath(t *testing.T) {
	w, h := 1920, 1080
	dstW, dstH := 640, 360

	// 1. Create synthetic BGRA frame
	frame := makeTestFrame(w, h, 100)
	t.Logf("Step 1: Created %dx%d BGRA frame (%d bytes)", w, h, len(frame))

	// 2. Resize via Rust SIMD
	resized, err := imaging.RustResizeBGRA(frame, w, h, 0, dstW, dstH, imaging.AlgoLanczos3)
	if err != nil {
		t.Fatalf("Step 2: RustResizeBGRA failed: %v", err)
	}
	expectedLen := dstW * dstH * 4
	if len(resized) != expectedLen {
		t.Fatalf("Step 2: expected %d bytes, got %d", expectedLen, len(resized))
	}
	t.Logf("Step 2: Resized to %dx%d (%d bytes)", dstW, dstH, len(resized))

	// 3. Keyframe diff (against itself → ratio ≈ 0)
	ratio, err := RustCalcChangeRatio(resized, resized, dstW, dstH, dstW*4, 20)
	if err != nil {
		t.Fatalf("Step 3: RustCalcChangeRatio failed: %v", err)
	}
	if ratio > 0.001 {
		t.Errorf("Step 3: expected ratio ≈ 0 for identical frames, got %.4f", ratio)
	}
	t.Logf("Step 3: Change ratio = %.6f (identical frames)", ratio)

	// 4. Perceptual hash
	hash, err := RustFrameHash(resized, dstW, dstH, dstW*4)
	if err != nil {
		t.Fatalf("Step 4: RustFrameHash failed: %v", err)
	}
	t.Logf("Step 4: Frame hash = 0x%016x", hash)

	// 5. Write to SHM via Rust
	shmName := "/argus_integ_shm"
	semW := "/argus_integ_semw"
	semR := "/argus_integ_semr"

	ipc.CleanupRustShm(shmName, semW, semR)
	writer, err := ipc.NewRustShmWriterWithNames(dstW, dstH, 4, shmName, semW, semR)
	if err != nil {
		t.Fatalf("Step 5: NewRustShmWriterWithNames failed: %v", err)
	}
	defer writer.Close()

	err = writer.WriteFrame(dstW, dstH, 4, resized)
	if err != nil {
		t.Fatalf("Step 5: WriteFrame failed: %v", err)
	}
	if writer.FrameNumber() != 1 {
		t.Errorf("Step 5: expected frame number 1, got %d", writer.FrameNumber())
	}
	t.Logf("Step 5: Written to SHM, frame #%d", writer.FrameNumber())

	t.Log("✅ Full pipeline Rust path: Resize → Diff → Hash → SHM — PASS")
}

// TestKeyframeExtractor_WithRustDiff verifies KeyframeExtractor detects
// changed frames using synthetic data compatible with Rust diff.
func TestKeyframeExtractor_WithRustDiff(t *testing.T) {
	cfg := DefaultKeyframeConfig()
	cfg.ChangeThreshold = 0.05
	cfg.MinIntervalMs = 0 // disable cooldown for test
	ext := NewKeyframeExtractor(cfg)

	w, h := 320, 240

	// Frame 1: initial (always a keyframe)
	frame1 := makeTestFrame(w, h, 50)
	kf1 := ext.ProcessFrame(frame1, w, h, 1, 1.0)
	if kf1 == nil {
		t.Fatal("expected initial frame to be a keyframe")
	}
	t.Logf("Frame 1: keyframe (reason=%s)", kf1.TriggerReason)

	// Frame 2: identical → should NOT be a keyframe
	kf2 := ext.ProcessFrame(frame1, w, h, 2, 2.0)
	if kf2 != nil {
		t.Errorf("expected identical frame 2 to NOT be a keyframe, got reason=%s", kf2.TriggerReason)
	} else {
		t.Log("Frame 2: not a keyframe (identical) — correct")
	}

	// Frame 3: very different → should be a keyframe
	frame3 := makeTestFrame(w, h, 200)
	kf3 := ext.ProcessFrame(frame3, w, h, 3, 3.0)
	if kf3 == nil {
		t.Error("expected different frame 3 to be a keyframe")
	} else {
		t.Logf("Frame 3: keyframe (reason=%s, change=%.3f)", kf3.TriggerReason, kf3.ChangeRatio)
	}

	stats := ext.Stats()
	t.Logf("Extractor stats: %v", stats)
}
