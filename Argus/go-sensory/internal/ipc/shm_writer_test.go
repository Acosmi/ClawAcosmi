package ipc

import (
	"testing"
	"time"
)

func TestShmWriterCreateAndClose(t *testing.T) {
	// Use unique names to avoid conflicts
	shmName := "/argus_test_shm"
	semW := "/argus_test_semw"
	semR := "/argus_test_semr"

	// Cleanup from any previous failed test
	CleanupShm(shmName, semW, semR)

	w, err := NewShmWriterWithNames(1920, 1080, 4, shmName, semW, semR)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	info := w.ShmInfo()
	expectedSize := HeaderSize + 1920*1080*4
	if info["shm_size"].(int) != expectedSize {
		t.Errorf("expected shm_size=%d, got=%d", expectedSize, info["shm_size"])
	}
}

func TestShmWriteFrame(t *testing.T) {
	shmName := "/argus_test_shm2"
	semW := "/argus_test_semw2"
	semR := "/argus_test_semr2"

	CleanupShm(shmName, semW, semR)

	width, height, channels := 640, 480, 4
	w, err := NewShmWriterWithNames(width, height, channels, shmName, semW, semR)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	// Create test pixel data
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

	t.Logf("Frame write latency: %v", elapsed)

	if w.FrameNumber() != 1 {
		t.Errorf("expected frame number 1, got %d", w.FrameNumber())
	}

	// Verify the latency is acceptable (< 10ms for 640x480)
	if elapsed > 10*time.Millisecond {
		t.Errorf("frame write latency too high: %v (expected < 10ms)", elapsed)
	}
}

func TestShmWriteMultipleFrames(t *testing.T) {
	shmName := "/argus_test_shm3"
	semW := "/argus_test_semw3"
	semR := "/argus_test_semr3"

	CleanupShm(shmName, semW, semR)

	width, height, channels := 320, 240, 4
	w, err := NewShmWriterWithNames(width, height, channels, shmName, semW, semR)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	pixels := make([]byte, width*height*channels)
	numFrames := 100

	start := time.Now()
	for i := 0; i < numFrames; i++ {
		// Write frame, then immediately signal consumer done (simulate fast reader)
		err := w.WriteFrame(width, height, channels, pixels)
		if err != nil {
			t.Fatalf("WriteFrame %d failed: %v", i, err)
		}
		// Simulate reader consuming the frame
		w.SimulateReaderConsume()
	}
	elapsed := time.Since(start)

	t.Logf("Wrote %d frames in %v (avg: %v/frame)", numFrames, elapsed, elapsed/time.Duration(numFrames))

	if w.FrameNumber() != uint64(numFrames) {
		t.Errorf("expected frame number %d, got %d", numFrames, w.FrameNumber())
	}
}
