package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---------- 会话文件修复 ----------

// TS 参考: src/agents/session-file-repair.ts (110 行)

// RepairReport 修复报告。
type RepairReport struct {
	Repaired     bool   `json:"repaired"`
	DroppedLines int    `json:"droppedLines"`
	BackupPath   string `json:"backupPath,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// isSessionHeader 检查是否为有效会话头。
func isSessionHeader(entry map[string]interface{}) bool {
	if entry == nil {
		return false
	}
	t, ok := entry["type"].(string)
	if !ok || t != "session" {
		return false
	}
	id, ok := entry["id"].(string)
	return ok && id != ""
}

// RepairSessionFileIfNeeded 修复损坏的会话文件。
// 删除无效 JSON 行，保留备份。
func RepairSessionFileIfNeeded(sessionFile string, warn func(string)) RepairReport {
	sessionFile = strings.TrimSpace(sessionFile)
	if sessionFile == "" {
		return RepairReport{Repaired: false, DroppedLines: 0, Reason: "missing session file"}
	}

	content, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return RepairReport{Repaired: false, DroppedLines: 0, Reason: "missing session file"}
		}
		reason := fmt.Sprintf("failed to read session file: %v", err)
		if warn != nil {
			warn(fmt.Sprintf("session file repair skipped: %s (%s)", reason, filepath.Base(sessionFile)))
		}
		return RepairReport{Repaired: false, DroppedLines: 0, Reason: reason}
	}

	lines := strings.Split(string(content), "\n")
	var entries []map[string]interface{}
	droppedLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
			droppedLines++
			continue
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return RepairReport{Repaired: false, DroppedLines: droppedLines, Reason: "empty session file"}
	}

	if !isSessionHeader(entries[0]) {
		if warn != nil {
			warn(fmt.Sprintf("session file repair skipped: invalid session header (%s)", filepath.Base(sessionFile)))
		}
		return RepairReport{Repaired: false, DroppedLines: droppedLines, Reason: "invalid session header"}
	}

	if droppedLines == 0 {
		return RepairReport{Repaired: false, DroppedLines: 0}
	}

	// 构建清理后的内容
	var cleanedLines []string
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		cleanedLines = append(cleanedLines, string(data))
	}
	cleaned := strings.Join(cleanedLines, "\n") + "\n"

	// 备份 + 写入
	backupPath := fmt.Sprintf("%s.bak-%d", sessionFile, os.Getpid())
	tmpPath := fmt.Sprintf("%s.repair-%d.tmp", sessionFile, os.Getpid())

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return RepairReport{Repaired: false, DroppedLines: droppedLines,
			Reason: fmt.Sprintf("repair failed: %v", err)}
	}

	if err := os.WriteFile(tmpPath, []byte(cleaned), 0644); err != nil {
		os.Remove(tmpPath)
		return RepairReport{Repaired: false, DroppedLines: droppedLines,
			Reason: fmt.Sprintf("repair failed: %v", err)}
	}

	if err := os.Rename(tmpPath, sessionFile); err != nil {
		os.Remove(tmpPath)
		return RepairReport{Repaired: false, DroppedLines: droppedLines,
			Reason: fmt.Sprintf("repair failed: %v", err)}
	}

	if warn != nil {
		warn(fmt.Sprintf("session file repaired: dropped %d malformed line(s) (%s)",
			droppedLines, filepath.Base(sessionFile)))
	}

	return RepairReport{Repaired: true, DroppedLines: droppedLines, BackupPath: backupPath}
}
