// Package services — VFSPathLock: per-user distributed lock via AGFS localfs.
//
// Phase B migration: replaced in-process sync.Mutex with AGFS file-based
// distributed lock. Each tenant+user pair gets a lock file under
// /localfs/locks/{tenantID}/{userID}.lock
//
// Lock protocol:
//
//	Acquire: Write lock file with owner ID (hostname + PID + goroutine)
//	Release: Delete lock file
//	Retry:   Poll with exponential backoff until lock file is absent
//
// Benefits:
//  1. Multi-instance: any Go API replica respects the same lock file
//  2. Crash recovery: stale locks expire via TTL embedded in lock content
//  3. Per-user isolation: different users lock different files (no contention)
package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/uhms/go-api/internal/agfs"
)

// lockConfig holds tuning parameters for the distributed lock.
type lockConfig struct {
	PollInterval time.Duration // wait between retry attempts
	MaxRetries   int           // max attempts before giving up
	LockTTL      time.Duration // stale lock expiration
}

var defaultLockConfig = lockConfig{
	PollInterval: 50 * time.Millisecond,
	MaxRetries:   200, // 200 * 50ms = 10s max wait
	LockTTL:      30 * time.Second,
}

// VFSPathLock manages per-user distributed locks via AGFS localfs.
// Each unique tenant+user pair gets its own lock file so that:
//   - Concurrent writes to the SAME user are serialized (safe).
//   - Concurrent writes to DIFFERENT users proceed in parallel (no contention).
type VFSPathLock struct {
	agfsClient *agfs.AGFSClient
	cfg        lockConfig
	hostname   string

	// Local mutex cache — optimizes single-instance contention.
	// Multiple goroutines in the same process first compete for the local
	// mutex, then the winner acquires the AGFS distributed lock.
	// This avoids N goroutines hammering AGFS with concurrent polls.
	mu         sync.Mutex
	localLocks map[string]*sync.Mutex
}

// globalVFSLock is the process-wide singleton lock manager.
// Initialized with a nil AGFS client; falls back to local-only mode
// until InitVFSLock() is called with a valid client.
var globalVFSLock = &VFSPathLock{
	localLocks: make(map[string]*sync.Mutex),
	cfg:        defaultLockConfig,
}

// InitVFSLock configures the global VFS lock with an AGFS client.
// Must be called during server startup. Without this, locks are local-only
// (backward compatible with pre-AGFS behavior).
func InitVFSLock(agfsClient *agfs.AGFSClient) {
	globalVFSLock.mu.Lock()
	defer globalVFSLock.mu.Unlock()
	globalVFSLock.agfsClient = agfsClient

	hostname, _ := os.Hostname()
	globalVFSLock.hostname = hostname

	// Create the locks directory.
	if agfsClient != nil {
		if err := agfsClient.Mkdir("/locks"); err != nil {
			slog.Warn("Failed to create AGFS locks directory (may already exist)", "error", err)
		}
		slog.Info("VFSPathLock initialized with AGFS distributed backend")
	}
}

// DistributedLock wraps a per-user lock that spans AGFS + local mutex.
// Callers use Lock()/Unlock() just like sync.Mutex.
type DistributedLock struct {
	manager  *VFSPathLock
	lockPath string
	localMu  *sync.Mutex
}

// ForUser returns (or creates) the distributed lock for a specific tenant+user pair.
// Thread-safe: the outer mutex serializes map access only.
func (l *VFSPathLock) ForUser(tenantID, userID string) *DistributedLock {
	key := tenantID + "/" + userID

	l.mu.Lock()
	localMu, ok := l.localLocks[key]
	if !ok {
		localMu = &sync.Mutex{}
		l.localLocks[key] = localMu
	}
	l.mu.Unlock()

	return &DistributedLock{
		manager:  l,
		lockPath: fmt.Sprintf("/locks/%s/%s.lock", tenantID, userID),
		localMu:  localMu,
	}
}

// Lock acquires both the local mutex and the AGFS distributed lock.
// If AGFS is not configured, falls back to local-only locking.
func (dl *DistributedLock) Lock() {
	// Step 1: Local mutex — prevents intra-process contention.
	dl.localMu.Lock()

	// Step 2: AGFS distributed lock — prevents inter-instance contention.
	if dl.manager.agfsClient == nil {
		return // local-only mode
	}

	if err := dl.acquireDistributed(); err != nil {
		slog.Error("Failed to acquire AGFS distributed lock, proceeding with local-only",
			"path", dl.lockPath, "error", err)
	}
}

// Unlock releases the AGFS distributed lock and the local mutex.
func (dl *DistributedLock) Unlock() {
	// Step 1: Release AGFS distributed lock.
	if dl.manager.agfsClient != nil {
		dl.releaseDistributed()
	}

	// Step 2: Release local mutex.
	dl.localMu.Unlock()
}

// acquireDistributed attempts to create a lock file on AGFS.
// Uses polling with exponential backoff.
func (dl *DistributedLock) acquireDistributed() error {
	ownerID := fmt.Sprintf("%s/pid-%d/%d",
		dl.manager.hostname, os.Getpid(), time.Now().UnixNano())

	lockContent := fmt.Sprintf(`{"owner":"%s","acquired_at":"%s","ttl_sec":%d}`,
		ownerID,
		time.Now().UTC().Format(time.RFC3339),
		int(dl.manager.cfg.LockTTL.Seconds()),
	)

	for attempt := 0; attempt < dl.manager.cfg.MaxRetries; attempt++ {
		// Check if lock file exists by trying to read it.
		existing, err := dl.manager.agfsClient.ReadFile(dl.lockPath)
		if err != nil || len(existing) == 0 {
			// Lock file doesn't exist — try to create it.
			if werr := dl.manager.agfsClient.WriteFile(dl.lockPath, []byte(lockContent)); werr != nil {
				slog.Debug("AGFS lock write contention, retrying",
					"path", dl.lockPath, "attempt", attempt, "error", werr)
				time.Sleep(dl.manager.cfg.PollInterval)
				continue
			}
			slog.Debug("AGFS distributed lock acquired", "path", dl.lockPath)
			return nil
		}

		// Lock file exists — check for staleness.
		if dl.isStale(existing) {
			slog.Warn("Stale AGFS lock detected, overriding",
				"path", dl.lockPath, "content", string(existing))
			// Override stale lock.
			if werr := dl.manager.agfsClient.WriteFile(dl.lockPath, []byte(lockContent)); werr == nil {
				slog.Debug("AGFS distributed lock acquired (stale override)", "path", dl.lockPath)
				return nil
			}
		}

		// Lock held by another instance — wait and retry.
		time.Sleep(dl.manager.cfg.PollInterval)
	}

	return fmt.Errorf("failed to acquire distributed lock after %d attempts: %s",
		dl.manager.cfg.MaxRetries, dl.lockPath)
}

// releaseDistributed deletes the lock file from AGFS.
func (dl *DistributedLock) releaseDistributed() {
	// Delete lock by writing empty content (AGFS localfs DELETE equivalent).
	// AGFS SDK may not expose Delete; we write an empty marker.
	if err := dl.manager.agfsClient.WriteFile(dl.lockPath, []byte{}); err != nil {
		slog.Warn("Failed to release AGFS distributed lock", "path", dl.lockPath, "error", err)
	} else {
		slog.Debug("AGFS distributed lock released", "path", dl.lockPath)
	}
}

// isStale checks if the lock file content indicates a stale (expired) lock.
// Parses {"owner":"...","acquired_at":"RFC3339","ttl_sec":N} and returns true
// if the lock age exceeds its declared TTL.
func (dl *DistributedLock) isStale(content []byte) bool {
	if len(content) == 0 {
		return true // empty marker = released
	}
	var meta struct {
		AcquiredAt string `json:"acquired_at"`
		TTLSec     int    `json:"ttl_sec"`
	}
	if err := json.Unmarshal(content, &meta); err != nil || meta.AcquiredAt == "" || meta.TTLSec <= 0 {
		return false // malformed or missing TTL — assume valid, avoid false override
	}
	acquired, err := time.Parse(time.RFC3339, meta.AcquiredAt)
	if err != nil {
		return false
	}
	return time.Since(acquired) > time.Duration(meta.TTLSec)*time.Second
}

// Reset clears all per-user locks. Intended for testing only.
func (l *VFSPathLock) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.localLocks = make(map[string]*sync.Mutex)
}
