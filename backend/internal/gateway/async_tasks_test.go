package gateway

import (
	"sync"
	"sync/atomic"
	"testing"
)

// ---------- TryAcquireAsync 并发守卫 ----------

func TestTryAcquireAsync_UnderLimit(t *testing.T) {
	s := NewChatRunState()
	for i := 0; i < MaxAsyncTasks; i++ {
		if !s.TryAcquireAsync() {
			t.Fatalf("should acquire slot %d (max=%d)", i, MaxAsyncTasks)
		}
	}
	if s.AsyncCount() != MaxAsyncTasks {
		t.Errorf("count = %d, want %d", s.AsyncCount(), MaxAsyncTasks)
	}
}

func TestTryAcquireAsync_AtLimit(t *testing.T) {
	s := NewChatRunState()
	for i := 0; i < MaxAsyncTasks; i++ {
		s.TryAcquireAsync()
	}
	// 第 6 个应该被拒绝
	if s.TryAcquireAsync() {
		t.Error("should reject when at max capacity")
	}
}

func TestReleaseAsync_AllowsReacquire(t *testing.T) {
	s := NewChatRunState()
	for i := 0; i < MaxAsyncTasks; i++ {
		s.TryAcquireAsync()
	}
	// 释放一个
	s.ReleaseAsync()
	if s.AsyncCount() != MaxAsyncTasks-1 {
		t.Errorf("count after release = %d, want %d", s.AsyncCount(), MaxAsyncTasks-1)
	}
	// 应该能重新获取
	if !s.TryAcquireAsync() {
		t.Error("should acquire after release")
	}
}

func TestAsyncCount_Accuracy(t *testing.T) {
	s := NewChatRunState()
	if s.AsyncCount() != 0 {
		t.Error("initial count should be 0")
	}
	s.TryAcquireAsync()
	s.TryAcquireAsync()
	if s.AsyncCount() != 2 {
		t.Errorf("count = %d, want 2", s.AsyncCount())
	}
	s.ReleaseAsync()
	if s.AsyncCount() != 1 {
		t.Errorf("count after release = %d, want 1", s.AsyncCount())
	}
}

func TestClearResetsAsyncCount(t *testing.T) {
	s := NewChatRunState()
	s.TryAcquireAsync()
	s.TryAcquireAsync()
	s.Clear()
	if s.AsyncCount() != 0 {
		t.Errorf("count after clear = %d, want 0", s.AsyncCount())
	}
	// 清空后应能重新获取
	if !s.TryAcquireAsync() {
		t.Error("should acquire after clear")
	}
}

// ---------- 并发安全性 ----------

func TestTryAcquireAsync_ConcurrentSafety(t *testing.T) {
	s := NewChatRunState()
	var acquired atomic.Int32
	var wg sync.WaitGroup

	// 启动 20 个 goroutine 竞争 5 个槽位
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.TryAcquireAsync() {
				acquired.Add(1)
			}
		}()
	}
	wg.Wait()

	if int(acquired.Load()) != MaxAsyncTasks {
		t.Errorf("acquired = %d, want %d (MaxAsyncTasks)", acquired.Load(), MaxAsyncTasks)
	}
	if s.AsyncCount() != MaxAsyncTasks {
		t.Errorf("count = %d, want %d", s.AsyncCount(), MaxAsyncTasks)
	}
}

func TestTryAcquireRelease_ConcurrentCycle(t *testing.T) {
	s := NewChatRunState()
	var wg sync.WaitGroup

	// 100 个 goroutine 各自 acquire → release，应不泄漏
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for attempt := 0; attempt < 10; attempt++ {
				if s.TryAcquireAsync() {
					s.ReleaseAsync()
				}
			}
		}()
	}
	wg.Wait()

	if s.AsyncCount() != 0 {
		t.Errorf("final count = %d, want 0 (no leak)", s.AsyncCount())
	}
}

// ---------- 下溢保护 ----------

func TestReleaseAsync_UnderflowProtection(t *testing.T) {
	s := NewChatRunState()
	// 未 acquire 直接 release 多次 — 不应下溢到负值
	s.ReleaseAsync()
	s.ReleaseAsync()
	s.ReleaseAsync()
	if s.AsyncCount() != 0 {
		t.Errorf("count after underflow releases = %d, want 0", s.AsyncCount())
	}
	// 下溢后应仍能正常 acquire
	if !s.TryAcquireAsync() {
		t.Error("should acquire after underflow releases")
	}
	if s.AsyncCount() != 1 {
		t.Errorf("count after acquire = %d, want 1", s.AsyncCount())
	}
}

// ---------- MaxAsyncTasks 常量 ----------

func TestMaxAsyncTasks_Value(t *testing.T) {
	if MaxAsyncTasks != 5 {
		t.Errorf("MaxAsyncTasks = %d, want 5", MaxAsyncTasks)
	}
}
