//go:build !windows

// auth/filelock.go — 跨进程文件排他锁。
// TS 参考: proper-lockfile npm 包 (隐式依赖于 store.ts 的磁盘写入路径)
//
// 提供基于 flock(2) 的跨进程排他锁，防止多实例并发写入认证存储文件。
// macOS/Linux 使用 syscall.Flock, 同一进程内的并发由 AuthStore.mu 保护。
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// FileLock 基于 flock(2) 的跨进程文件排他锁。
// 对齐 TS proper-lockfile 的核心语义：排他 + 自动清理。
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock 创建文件锁实例。
// lockPath 通常为 storePath + ".lock"。
func NewFileLock(lockPath string) *FileLock {
	return &FileLock{path: lockPath}
}

// Lock 获取排他锁（阻塞）。
// 如果锁文件不存在则自动创建。
func (fl *FileLock) Lock() error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o700); err != nil {
		return fmt.Errorf("filelock: create dir: %w", err)
	}

	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("filelock: open %s: %w", fl.path, err)
	}

	// LOCK_EX: 排他锁，阻塞等待
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return fmt.Errorf("filelock: flock LOCK_EX %s: %w", fl.path, err)
	}

	fl.file = f
	return nil
}

// TryLock 尝试获取排他锁（非阻塞）。
// 如果锁已被占用，返回错误。
func (fl *FileLock) TryLock() error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o700); err != nil {
		return fmt.Errorf("filelock: create dir: %w", err)
	}

	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("filelock: open %s: %w", fl.path, err)
	}

	// LOCK_EX | LOCK_NB: 排他锁 + 非阻塞
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return fmt.Errorf("filelock: lock busy %s: %w", fl.path, err)
	}

	fl.file = f
	return nil
}

// Unlock 释放锁并关闭文件。
func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}

	// 先解锁再关闭
	err := syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
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
