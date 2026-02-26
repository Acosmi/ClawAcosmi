package cron

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
)

// ============================================================================
// 文件持久化 — 加载/保存 cron jobs.json
// 对应 TS: cron/store.ts (62L)
// ============================================================================

const (
	storeVersion = 1
)

// LoadCronStore 从 JSON 文件加载 cron store
// 文件不存在返回空 store
func LoadCronStore(filePath string) (*CronStoreFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CronStoreFile{Version: storeVersion, Jobs: []CronJob{}}, nil
		}
		return nil, fmt.Errorf("cron store: read failed: %w", err)
	}

	var store CronStoreFile
	if err := json.Unmarshal(data, &store); err != nil {
		// TS: 尝试 JSON5，Go: 标准 JSON + 宽容处理
		// 解析失败返回空 store（与 TS 行为一致）
		return &CronStoreFile{Version: storeVersion, Jobs: []CronJob{}}, nil
	}

	if store.Jobs == nil {
		store.Jobs = []CronJob{}
	}
	if store.Version == 0 {
		store.Version = storeVersion
	}

	return &store, nil
}

// SaveCronStore 原子保存 cron store
// 先写临时文件，再 rename（原子操作）
func SaveCronStore(filePath string, store *CronStoreFile) error {
	if store == nil {
		return fmt.Errorf("cron store: nil store")
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cron store: mkdir failed: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("cron store: marshal failed: %w", err)
	}
	data = append(data, '\n')

	// 写临时文件 → rename（原子操作）
	pid := os.Getpid()
	suffix := strconv.FormatInt(rand.Int63(), 16)
	tmpPath := filePath + "." + strconv.Itoa(pid) + "." + suffix + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("cron store: write tmp failed: %w", err)
	}

	// Backup 旧文件
	if _, err := os.Stat(filePath); err == nil {
		backupPath := filePath + ".bak"
		_ = os.Rename(filePath, backupPath) // best-effort
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath) // cleanup
		return fmt.Errorf("cron store: rename failed: %w", err)
	}

	return nil
}

// ResolveCronStorePath 解析 cron store 文件路径
func ResolveCronStorePath(storePath string) string {
	return filepath.Clean(storePath)
}
