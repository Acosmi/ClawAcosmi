package argus

import (
	"testing"
	"time"
)

func TestObservationBuffer_PushAndLast(t *testing.T) {
	buf := NewObservationBuffer(5)

	for i := uint64(1); i <= 3; i++ {
		buf.Push(&VisionObservation{ID: i, CapturedAt: int64(i * 1000)})
	}

	if buf.Count() != 3 {
		t.Fatalf("count: got %d, want 3", buf.Count())
	}

	got := buf.Last(2)
	if len(got) != 2 {
		t.Fatalf("Last(2): got %d items, want 2", len(got))
	}
	if got[0].ID != 3 || got[1].ID != 2 {
		t.Errorf("Last(2): got IDs [%d,%d], want [3,2]", got[0].ID, got[1].ID)
	}
}

func TestObservationBuffer_RingOverwrite(t *testing.T) {
	buf := NewObservationBuffer(3)
	for i := uint64(1); i <= 5; i++ {
		buf.Push(&VisionObservation{ID: i})
	}

	if buf.Count() != 3 {
		t.Fatalf("count: got %d, want 3", buf.Count())
	}
	got := buf.Last(3)
	if got[0].ID != 5 || got[1].ID != 4 || got[2].ID != 3 {
		t.Errorf("got IDs [%d,%d,%d], want [5,4,3]", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestObservationBuffer_Since(t *testing.T) {
	buf := NewObservationBuffer(10)
	for i := int64(1); i <= 5; i++ {
		buf.Push(&VisionObservation{ID: uint64(i), CapturedAt: i * 1000})
	}

	got := buf.Since(3000)
	if len(got) != 2 {
		t.Fatalf("Since(3000): got %d items, want 2", len(got))
	}
	if got[0].CapturedAt != 5000 || got[1].CapturedAt != 4000 {
		t.Errorf("got ts [%d,%d], want [5000,4000]", got[0].CapturedAt, got[1].CapturedAt)
	}
}

func TestObservationBuffer_LatestKeyframe(t *testing.T) {
	buf := NewObservationBuffer(10)

	// 无关键帧
	if buf.LatestKeyframe() != nil {
		t.Error("expected nil for empty buffer")
	}

	buf.Push(&VisionObservation{ID: 1, IsKeyframe: false})
	if buf.LatestKeyframe() != nil {
		t.Error("expected nil when no keyframe pushed")
	}

	buf.Push(&VisionObservation{ID: 2, IsKeyframe: true})
	kf := buf.LatestKeyframe()
	if kf == nil || kf.ID != 2 {
		t.Errorf("expected keyframe ID=2, got %v", kf)
	}

	buf.Push(&VisionObservation{ID: 3, IsKeyframe: true})
	kf = buf.LatestKeyframe()
	if kf == nil || kf.ID != 3 {
		t.Errorf("expected latest keyframe ID=3, got %v", kf)
	}
}

func TestObservationBuffer_Subscribe(t *testing.T) {
	buf := NewObservationBuffer(10)
	id, ch := buf.Subscribe(4)

	buf.Push(&VisionObservation{ID: 1})
	buf.Push(&VisionObservation{ID: 2})

	select {
	case obs := <-ch:
		if obs.ID != 1 {
			t.Errorf("expected ID=1, got %d", obs.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber timed out")
	}

	buf.Unsubscribe(id)
}

func TestObservationBuffer_LastExceedsCount(t *testing.T) {
	buf := NewObservationBuffer(10)
	buf.Push(&VisionObservation{ID: 1})

	got := buf.Last(100)
	if len(got) != 1 {
		t.Errorf("Last(100) with 1 item: got %d, want 1", len(got))
	}
}

func TestNewObservationBuffer_DefaultCapacity(t *testing.T) {
	buf := NewObservationBuffer(0)
	if buf.capacity != 500 {
		t.Errorf("expected default capacity 500, got %d", buf.capacity)
	}
}
