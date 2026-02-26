package cron

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================================
// 运行日志 — JSONL 格式的 Job 运行日志
// 对应 TS: cron/run-log.ts (122L)
// ============================================================================

// CronRunLogEntry 运行日志条目
type CronRunLogEntry struct {
	Ts          int64  `json:"ts"`
	JobID       string `json:"jobId"`
	Action      string `json:"action"`
	Status      string `json:"status,omitempty"`
	Error       string `json:"error,omitempty"`
	Summary     string `json:"summary,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	SessionKey  string `json:"sessionKey,omitempty"`
	RunAtMs     *int64 `json:"runAtMs,omitempty"`
	DurationMs  *int64 `json:"durationMs,omitempty"`
	NextRunAtMs *int64 `json:"nextRunAtMs,omitempty"`
}

// ResolveCronRunLogPath 解析 Job 运行日志文件路径
func ResolveCronRunLogPath(storePath string, jobID string) string {
	dir := filepath.Dir(filepath.Clean(storePath))
	return filepath.Join(dir, "runs", fmt.Sprintf("%s.jsonl", jobID))
}

// AppendCronRunLog 追加运行日志条目
func AppendCronRunLog(filePath string, entry CronRunLogEntry, maxBytes int64, keepLines int) error {
	if maxBytes <= 0 {
		maxBytes = 2_000_000
	}
	if keepLines <= 0 {
		keepLines = 2_000
	}

	resolved := filepath.Clean(filePath)

	// 确保目录存在
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cron run log: mkdir failed: %w", err)
	}

	// 追加 JSONL 行
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("cron run log: marshal failed: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(resolved, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cron run log: open failed: %w", err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("cron run log: write failed: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cron run log: close failed: %w", err)
	}

	// 按需裁剪
	return pruneRunLogIfNeeded(resolved, maxBytes, keepLines)
}

// pruneRunLogIfNeeded 按文件大小裁剪日志
func pruneRunLogIfNeeded(filePath string, maxBytes int64, keepLines int) error {
	info, err := os.Stat(filePath)
	if err != nil || info.Size() <= maxBytes {
		return nil
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil // best-effort
	}

	lines := strings.Split(string(raw), "\n")
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}

	// 保留最后 keepLines 行
	start := len(nonEmpty) - keepLines
	if start < 0 {
		start = 0
	}
	kept := nonEmpty[start:]

	// 原子写入
	pid := os.Getpid()
	suffix := strconv.FormatInt(rand.Int63(), 16)
	tmpPath := fmt.Sprintf("%s.%d.%s.tmp", filePath, pid, suffix)

	content := strings.Join(kept, "\n") + "\n"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return nil // best-effort
	}

	return os.Rename(tmpPath, filePath)
}

// ReadCronRunLogEntries 读取运行日志条目（从末尾向前）
func ReadCronRunLogEntries(filePath string, limit int, jobIDFilter string) []CronRunLogEntry {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}

	jobIDFilter = strings.TrimSpace(jobIDFilter)

	raw, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}

	lines := strings.Split(string(raw), "\n")

	var parsed []CronRunLogEntry
	// 从末尾向前读取
	for i := len(lines) - 1; i >= 0 && len(parsed) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		// 验证必填字段
		action, _ := obj["action"].(string)
		if action != "finished" {
			continue
		}

		jobID, _ := obj["jobId"].(string)
		if strings.TrimSpace(jobID) == "" {
			continue
		}

		ts, _ := obj["ts"].(float64)
		if ts == 0 {
			continue
		}

		// JobID 过滤
		if jobIDFilter != "" && jobID != jobIDFilter {
			continue
		}

		entry := CronRunLogEntry{
			Ts:     int64(ts),
			JobID:  jobID,
			Action: "finished",
		}

		if s, ok := obj["status"].(string); ok {
			entry.Status = s
		}
		if s, ok := obj["error"].(string); ok {
			entry.Error = s
		}
		if s, ok := obj["summary"].(string); ok {
			entry.Summary = s
		}
		if s, ok := obj["sessionId"].(string); ok && strings.TrimSpace(s) != "" {
			entry.SessionID = s
		}
		if s, ok := obj["sessionKey"].(string); ok && strings.TrimSpace(s) != "" {
			entry.SessionKey = s
		}
		if n, ok := obj["runAtMs"].(float64); ok {
			v := int64(n)
			entry.RunAtMs = &v
		}
		if n, ok := obj["durationMs"].(float64); ok {
			v := int64(n)
			entry.DurationMs = &v
		}
		if n, ok := obj["nextRunAtMs"].(float64); ok {
			v := int64(n)
			entry.NextRunAtMs = &v
		}

		parsed = append(parsed, entry)
	}

	// 反转为正序
	for i, j := 0, len(parsed)-1; i < j; i, j = i+1, j-1 {
		parsed[i], parsed[j] = parsed[j], parsed[i]
	}

	return parsed
}
