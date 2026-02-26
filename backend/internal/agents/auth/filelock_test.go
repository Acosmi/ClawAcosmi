package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileLock_ExclusiveAccess(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	fl1 := NewFileLock(lockPath)
	fl2 := NewFileLock(lockPath)

	// fl1 获取锁
	if err := fl1.Lock(); err != nil {
		t.Fatalf("fl1.Lock() failed: %v", err)
	}

	// fl2 应该获取不到（非阻塞模式）
	if err := fl2.TryLock(); err == nil {
		t.Error("fl2.TryLock() should have failed while fl1 holds the lock")
		fl2.Unlock()
	}

	// 释放 fl1
	if err := fl1.Unlock(); err != nil {
		t.Fatalf("fl1.Unlock() failed: %v", err)
	}

	// 现在 fl2 应该能获取
	if err := fl2.TryLock(); err != nil {
		t.Fatalf("fl2.TryLock() should succeed after fl1 unlocked: %v", err)
	}
	fl2.Unlock()
}

func TestFileLock_UnlockWithoutLock(t *testing.T) {
	fl := NewFileLock(filepath.Join(t.TempDir(), "no.lock"))
	// Unlock without Lock should be safe (no-op)
	if err := fl.Unlock(); err != nil {
		t.Errorf("Unlock without Lock should succeed: %v", err)
	}
}

func TestFileLock_LockCreatesFile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "subdir", "test.lock")

	fl := NewFileLock(lockPath)
	if err := fl.Lock(); err != nil {
		t.Fatalf("Lock() failed: %v", err)
	}
	defer fl.Unlock()

	// 锁文件应存在
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should be created")
	}
}
