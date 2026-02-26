// Package vfslocktest contains isolated tests for VFSPathLock.
// Separated from services package to avoid CGO/FFI linkage requirements.
//
// Phase B: Updated to reflect AGFS distributed lock architecture.
// Tests run in local-only mode (no AGFS client) to avoid external dependencies.
// The DistributedLock in local-only mode behaves identically to sync.Mutex.
package vfslocktest

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// DistributedLock is a test-local copy mirroring the production DistributedLock.
// In local-only mode (no AGFS client), it delegates to a plain sync.Mutex.
type DistributedLock struct {
	localMu *sync.Mutex
}

func (dl *DistributedLock) Lock()   { dl.localMu.Lock() }
func (dl *DistributedLock) Unlock() { dl.localMu.Unlock() }

// VFSPathLock is a test-local copy (local-only mode without AGFS).
type VFSPathLock struct {
	mu         sync.Mutex
	localLocks map[string]*sync.Mutex
}

func NewVFSPathLock() *VFSPathLock {
	return &VFSPathLock{localLocks: make(map[string]*sync.Mutex)}
}

func (l *VFSPathLock) ForUser(tenantID, userID string) *DistributedLock {
	key := tenantID + "/" + userID
	l.mu.Lock()
	defer l.mu.Unlock()
	localMu, ok := l.localLocks[key]
	if !ok {
		localMu = &sync.Mutex{}
		l.localLocks[key] = localMu
	}
	return &DistributedLock{localMu: localMu}
}

// TestVFSPathLock_ConcurrentSameUser — 100 goroutines writing to the same
// user are serialized: a non-atomic increment+decrement stays at 0.
func TestVFSPathLock_ConcurrentSameUser(t *testing.T) {
	lock := NewVFSPathLock()

	const goroutines = 100
	var counter int64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			dl := lock.ForUser("tenant-1", "user-1")
			dl.Lock()
			cur := atomic.LoadInt64(&counter)
			time.Sleep(time.Microsecond)
			atomic.StoreInt64(&counter, cur+1)
			atomic.AddInt64(&counter, -1)
			dl.Unlock()
		}()
	}

	wg.Wait()

	if v := atomic.LoadInt64(&counter); v != 0 {
		t.Errorf("expected counter=0 after serialized ops, got %d", v)
	}
}

// TestVFSPathLock_DifferentUsersNotBlocked — different users' locks are isolated.
func TestVFSPathLock_DifferentUsersNotBlocked(t *testing.T) {
	lock := NewVFSPathLock()

	dl1 := lock.ForUser("tenant-1", "user-A")
	dl2 := lock.ForUser("tenant-1", "user-B")

	dl1.Lock()

	done := make(chan struct{})
	go func() {
		dl2.Lock()
		dl2.Unlock()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("user-B lock blocked by user-A lock — isolation broken")
	}

	dl1.Unlock()
}

// TestVFSPathLock_SameUserReturnsSameLock — ForUser is idempotent.
func TestVFSPathLock_SameUserReturnsSameLock(t *testing.T) {
	lock := NewVFSPathLock()

	dl1 := lock.ForUser("t1", "u1")
	dl2 := lock.ForUser("t1", "u1")

	// Both should wrap the same underlying mutex.
	if dl1.localMu != dl2.localMu {
		t.Error("expected same underlying mutex for same tenant+user, got different pointers")
	}
}

// TestVFSPathLock_DifferentTenantsIsolated — same userID under different tenants
// gets separate locks.
func TestVFSPathLock_DifferentTenantsIsolated(t *testing.T) {
	lock := NewVFSPathLock()

	dl1 := lock.ForUser("tenant-A", "user-1")
	dl2 := lock.ForUser("tenant-B", "user-1")

	if dl1.localMu == dl2.localMu {
		t.Error("expected different mutexes for different tenants, got same pointer")
	}
}

// TestVFSPathLock_ConcurrentForUser — 200 goroutines calling ForUser for the
// same key all get the same underlying mutex (no map corruption).
func TestVFSPathLock_ConcurrentForUser(t *testing.T) {
	lock := NewVFSPathLock()

	const goroutines = 200
	mutexes := make(chan *sync.Mutex, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			dl := lock.ForUser("t1", "u1")
			mutexes <- dl.localMu
		}()
	}

	wg.Wait()
	close(mutexes)

	var first *sync.Mutex
	for m := range mutexes {
		if first == nil {
			first = m
			continue
		}
		if m != first {
			t.Fatal("concurrent ForUser returned different mutex pointers")
		}
	}
}
