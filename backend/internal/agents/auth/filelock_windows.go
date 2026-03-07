//go:build windows

package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// FileLock 基于 LockFileEx 的跨进程文件排他锁。
type FileLock struct {
	path       string
	file       *os.File
	overlapped windows.Overlapped
}

func NewFileLock(lockPath string) *FileLock {
	return &FileLock{path: lockPath}
}

func (fl *FileLock) Lock() error {
	return fl.lock(false)
}

func (fl *FileLock) TryLock() error {
	return fl.lock(true)
}

func (fl *FileLock) lock(nonBlocking bool) error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o700); err != nil {
		return fmt.Errorf("filelock: create dir: %w", err)
	}

	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("filelock: open %s: %w", fl.path, err)
	}

	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK)
	if nonBlocking {
		flags |= windows.LOCKFILE_FAIL_IMMEDIATELY
	}

	if err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &fl.overlapped); err != nil {
		_ = f.Close()
		return fmt.Errorf("filelock: lock %s: %w", fl.path, err)
	}

	fl.file = f
	return nil
}

func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}

	err := windows.UnlockFileEx(windows.Handle(fl.file.Fd()), 0, 1, 0, &fl.overlapped)
	closeErr := fl.file.Close()
	fl.file = nil

	if err != nil {
		return fmt.Errorf("filelock: unlock %s: %w", fl.path, err)
	}
	if closeErr != nil {
		return fmt.Errorf("filelock: close %s: %w", fl.path, closeErr)
	}
	return nil
}
