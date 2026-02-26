package ipc

import (
	"testing"
	"time"
)

func TestRustShmWriter_CreateAndClose(t *testing.T) {
	shmName := "/argus_rtest_shm"
	semW := "/argus_rtest_semw"
	semR := "/argus_rtest_semr"

	CleanupRustShm(shmName, semW, semR)

	w, err := NewRustShmWriterWithNames(1920, 1080, 4, shmName, semW, semR)
	if err != nil {
		t.Fatalf("failed to create RustShmWriter: %v", err)
	}
	defer w.Close()

	if w.FrameNumber() != 0 {
		t.Errorf("expected initial frame number 0, got %d", w.FrameNumber())
	}
	t.Log("RustShmWriter created and initial state verified")
}

func TestRustShmWriter_WriteFrame(t *testing.T) {
	shmName := "/argus_rtest_shm2"
	semW := "/argus_rtest_semw2"
	semR := "/argus_rtest_semr2"

	CleanupRustShm(shmName, semW, semR)

	width, height, channels := 640, 480, 4
	w, err := NewRustShmWriterWithNames(width, height, channels, shmName, semW, semR)
	if err != nil {
		t.Fatalf("failed to create RustShmWriter: %v", err)
	}
	defer w.Close()

	pixels := make([]byte, width*height*channels)
	for i := range pixels {
		pixels[i] = byte(i % 256)
	}

	start := time.Now()
	err = w.WriteFrame(width, height, channels, pixels)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	if w.FrameNumber() != 1 {
		t.Errorf("expected frame number 1, got %d", w.FrameNumber())
	}

	t.Logf("Rust SHM write latency: %v", elapsed)
	if elapsed > 10*time.Millisecond {
		t.Errorf("write latency too high: %v (expected < 10ms)", elapsed)
	}
}

func TestRustShmWriter_MultipleFrames(t *testing.T) {
	shmName := "/argus_rtest_shm3"
	semW := "/argus_rtest_semw3"
	semR := "/argus_rtest_semr3"

	CleanupRustShm(shmName, semW, semR)

	width, height, channels := 320, 240, 4
	w, err := NewRustShmWriterWithNames(width, height, channels, shmName, semW, semR)
	if err != nil {
		t.Fatalf("failed to create RustShmWriter: %v", err)
	}
	defer w.Close()

	pixels := make([]byte, width*height*channels)
	numFrames := 100

	start := time.Now()
	for i := 0; i < numFrames; i++ {
		err := w.WriteFrame(width, height, channels, pixels)
		if err != nil {
			t.Fatalf("WriteFrame %d failed: %v", i, err)
		}
		w.SimulateReaderConsume()
	}
	elapsed := time.Since(start)

	t.Logf("Wrote %d frames in %v (avg: %v/frame)", numFrames, elapsed, elapsed/time.Duration(numFrames))

	if w.FrameNumber() != uint64(numFrames) {
		t.Errorf("expected frame number %d, got %d", numFrames, w.FrameNumber())
	}
}

// ===== Benchmarks =====

// BenchmarkRustShmWrite_720p benchmarks Rust SHM write throughput for 720p frames.
func BenchmarkRustShmWrite_720p(b *testing.B) {
	shmName := "/argus_bench_shm"
	semW := "/argus_bench_semw"
	semR := "/argus_bench_semr"

	CleanupRustShm(shmName, semW, semR)

	width, height, channels := 1280, 720, 4
	w, err := NewRustShmWriterWithNames(width, height, channels, shmName, semW, semR)
	if err != nil {
		b.Fatalf("failed to create RustShmWriter: %v", err)
	}
	defer w.Close()

	pixels := make([]byte, width*height*channels)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.WriteFrame(width, height, channels, pixels)
		w.SimulateReaderConsume()
	}
}
