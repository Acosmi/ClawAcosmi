package gateway

import (
	"sync"
	"testing"
	"time"
)

func TestIdempotencyCache_NewKey(t *testing.T) {
	c := NewIdempotencyCache(time.Minute)
	defer c.Close()

	result := c.CheckOrRegister("key-1")
	if result.IsDuplicate {
		t.Fatal("expected new key to not be duplicate")
	}
}

func TestIdempotencyCache_DuplicateInFlight(t *testing.T) {
	c := NewIdempotencyCache(time.Minute)
	defer c.Close()

	c.CheckOrRegister("key-dup")
	result := c.CheckOrRegister("key-dup")
	if !result.IsDuplicate {
		t.Fatal("expected duplicate")
	}
	if result.State != IdempotencyInFlight {
		t.Fatalf("expected InFlight, got %d", result.State)
	}
}

func TestIdempotencyCache_DuplicateCompleted(t *testing.T) {
	c := NewIdempotencyCache(time.Minute)
	defer c.Close()

	c.CheckOrRegister("key-done")
	c.Complete("key-done", map[string]string{"ok": "true"})

	result := c.CheckOrRegister("key-done")
	if !result.IsDuplicate {
		t.Fatal("expected duplicate")
	}
	if result.State != IdempotencyCompleted {
		t.Fatalf("expected Completed, got %d", result.State)
	}
	m, ok := result.CachedResult.(map[string]string)
	if !ok || m["ok"] != "true" {
		t.Fatalf("unexpected cached result: %v", result.CachedResult)
	}
}

func TestIdempotencyCache_EmptyKey(t *testing.T) {
	c := NewIdempotencyCache(time.Minute)
	defer c.Close()

	result := c.CheckOrRegister("")
	if result.IsDuplicate {
		t.Fatal("empty key should never be duplicate")
	}
}

func TestIdempotencyCache_TTLExpiry(t *testing.T) {
	// 极短 TTL
	c := NewIdempotencyCache(50 * time.Millisecond)
	defer c.Close()

	c.CheckOrRegister("key-ttl")

	// 等待过期
	time.Sleep(80 * time.Millisecond)

	result := c.CheckOrRegister("key-ttl")
	if result.IsDuplicate {
		t.Fatal("expected expired key to not be duplicate")
	}
}

func TestIdempotencyCache_Remove(t *testing.T) {
	c := NewIdempotencyCache(time.Minute)
	defer c.Close()

	c.CheckOrRegister("key-rm")
	c.Remove("key-rm")

	result := c.CheckOrRegister("key-rm")
	if result.IsDuplicate {
		t.Fatal("expected removed key to not be duplicate")
	}
}

func TestIdempotencyCache_Concurrent(t *testing.T) {
	c := NewIdempotencyCache(time.Minute)
	defer c.Close()

	const n = 100
	var wg sync.WaitGroup
	results := make([]CheckResult, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = c.CheckOrRegister("same-key")
		}(i)
	}
	wg.Wait()

	newCount := 0
	dupCount := 0
	for _, r := range results {
		if r.IsDuplicate {
			dupCount++
		} else {
			newCount++
		}
	}
	if newCount != 1 {
		t.Fatalf("expected exactly 1 new registration, got %d new + %d dup", newCount, dupCount)
	}
}
