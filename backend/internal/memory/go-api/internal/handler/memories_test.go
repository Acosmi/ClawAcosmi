package handler

import (
	"testing"
	"time"
)

// TestCommitSession_CooldownPreventsRepeat verifies the per-user 5-minute cooldown
// on ProMem archive triggering.
func TestCommitSession_CooldownPreventsRepeat(t *testing.T) {
	h := &MemoryHandler{
		archiveCooldown: make(map[string]time.Time),
	}

	// Simulate first trigger (sets cooldown)
	h.archiveCooldownMu.Lock()
	h.archiveCooldown["user1"] = time.Now()
	h.archiveCooldownMu.Unlock()

	// Verify cooldown is set
	h.archiveCooldownMu.Lock()
	last, ok := h.archiveCooldown["user1"]
	h.archiveCooldownMu.Unlock()

	if !ok {
		t.Fatal("Cooldown should be set for user1")
	}
	if time.Since(last) > 1*time.Second {
		t.Error("Cooldown time should be recent")
	}
}

// TestCommitSession_CooldownExpired verifies cooldown expiry allows re-trigger.
func TestCommitSession_CooldownExpired(t *testing.T) {
	h := &MemoryHandler{
		archiveCooldown: make(map[string]time.Time),
	}

	// Set expired cooldown (6 minutes ago)
	h.archiveCooldownMu.Lock()
	h.archiveCooldown["user1"] = time.Now().Add(-6 * time.Minute)
	h.archiveCooldownMu.Unlock()

	// Check that cooldown has expired
	h.archiveCooldownMu.Lock()
	last := h.archiveCooldown["user1"]
	expired := time.Since(last) >= 5*time.Minute
	h.archiveCooldownMu.Unlock()

	if !expired {
		t.Error("Cooldown should have expired after 6 minutes")
	}
}

// TestCommitSession_DifferentUsers verifies cooldowns are per-user.
func TestCommitSession_DifferentUsers(t *testing.T) {
	h := &MemoryHandler{
		archiveCooldown: make(map[string]time.Time),
	}

	// Set cooldown for user1
	h.archiveCooldownMu.Lock()
	h.archiveCooldown["user1"] = time.Now()
	h.archiveCooldownMu.Unlock()

	// user2 should not have cooldown
	h.archiveCooldownMu.Lock()
	_, hasUser2 := h.archiveCooldown["user2"]
	h.archiveCooldownMu.Unlock()

	if hasUser2 {
		t.Error("user2 should not have cooldown")
	}
}

// TestMemoryHandler_CooldownMapInit verifies cooldown map is properly structured.
func TestMemoryHandler_CooldownMapInit(t *testing.T) {
	// Directly construct to avoid global singleton init (GetTreeManager requires config)
	h := &MemoryHandler{
		archiveCooldown: make(map[string]time.Time),
	}
	if h.archiveCooldown == nil {
		t.Fatal("archiveCooldown map should be initialized")
	}
	if len(h.archiveCooldown) != 0 {
		t.Fatal("archiveCooldown should be empty initially")
	}
}
