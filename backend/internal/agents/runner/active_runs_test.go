package runner

import (
	"sync"
	"testing"
	"time"
)

func TestActiveRuns_RegisterDeregister(t *testing.T) {
	m := NewActiveRunsManager()
	h := &stubRunHandle{}
	m.RegisterRun("s1", h)
	if !m.IsRunning("s1") {
		t.Error("expected s1 running")
	}
	m.DeregisterRun("s1", h)
	if m.IsRunning("s1") {
		t.Error("expected s1 not running")
	}
}

func TestActiveRuns_HandleMismatch(t *testing.T) {
	m := NewActiveRunsManager()
	h1 := &stubRunHandle{}
	h2 := &stubRunHandle{}
	m.RegisterRun("s1", h1)
	m.DeregisterRun("s1", h2) // wrong handle
	if !m.IsRunning("s1") {
		t.Error("should still be running after mismatch deregister")
	}
	m.DeregisterRun("s1", h1) // correct handle
	if m.IsRunning("s1") {
		t.Error("should not be running")
	}
}

func TestActiveRuns_WaitForRunEnd(t *testing.T) {
	m := NewActiveRunsManager()
	h := &stubRunHandle{}
	m.RegisterRun("s1", h)

	done := make(chan bool, 1)
	go func() {
		done <- m.WaitForRunEnd("s1", 2*time.Second)
	}()

	time.Sleep(50 * time.Millisecond)
	m.DeregisterRun("s1", h)

	result := <-done
	if !result {
		t.Error("expected true (normal end)")
	}
}

func TestActiveRuns_WaitTimeout(t *testing.T) {
	m := NewActiveRunsManager()
	h := &stubRunHandle{}
	m.RegisterRun("s1", h)

	result := m.WaitForRunEnd("s1", 50*time.Millisecond)
	if result {
		t.Error("expected false (timeout)")
	}
	m.DeregisterRun("s1", h) // cleanup
}

func TestActiveRuns_WaitNoRun(t *testing.T) {
	m := NewActiveRunsManager()
	result := m.WaitForRunEnd("nonexistent", time.Second)
	if !result {
		t.Error("should return true when no run")
	}
}

func TestActiveRuns_Concurrent(t *testing.T) {
	m := NewActiveRunsManager()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			h := &stubRunHandle{}
			m.RegisterRun(id, h)
			m.IsRunning(id)
			m.QueueMessage(id, "test")
			m.AbortRun(id)
			m.DeregisterRun(id, h)
		}(string(rune('a' + i%26)))
	}
	wg.Wait()
}
