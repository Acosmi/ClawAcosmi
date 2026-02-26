// Package services — Core Memory 审计日志服务。
// 记录所有自动核心记忆编辑行为，提供完整的 audit trail。
package services

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// CoreMemoryEditAction 表示 LLM 返回的单次核心记忆编辑指令。
type CoreMemoryEditAction struct {
	Section string `json:"section"` // persona / preferences / instructions
	Mode    string `json:"mode"`    // replace / append
	Content string `json:"content"` // 新内容
}

// ReflectionWithEdits 表示 LLM 返回的反思结果 + 核心记忆编辑指令。
type ReflectionWithEdits struct {
	Reflection      string                 `json:"reflection"`
	CoreMemoryEdits []CoreMemoryEditAction `json:"core_memory_edits"`
}

// ImaginationWithAppends 表示 LLM 返回的想象结果 + 核心记忆追加指令。
type ImaginationWithAppends struct {
	Imagination       string                 `json:"imagination"`
	CoreMemoryAppends []CoreMemoryEditAction `json:"core_memory_appends"`
}

// LogCoreMemoryEdit 记录一次核心记忆自动编辑到审计日志。
func LogCoreMemoryEdit(db *gorm.DB, userID, section, mode, source, oldValue, newValue string) error {
	entry := &models.CoreMemoryAuditLog{
		UserID:   userID,
		Section:  section,
		Mode:     mode,
		Source:   source,
		OldValue: oldValue,
		NewValue: newValue,
		EditedBy: "llm",
	}

	if err := db.Create(entry).Error; err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}

	slog.Info("Core Memory 自动编辑审计",
		"user_id", userID,
		"section", section,
		"mode", mode,
		"source", source,
		"old_len", len(oldValue),
		"new_len", len(newValue),
	)

	return nil
}

// ApplyCoreMemoryEdits 批量执行核心记忆编辑指令，写入 DB 并记录审计日志。
// source 标识触发来源 ("reflection" / "imagination")。
// forceAppendOnly=true 时拒绝 replace 操作（用于 Imagination 安全限制）。
func ApplyCoreMemoryEdits(
	db *gorm.DB,
	userID, source string,
	edits []CoreMemoryEditAction,
	forceAppendOnly bool,
) (int, error) {
	if len(edits) == 0 {
		return 0, nil
	}

	// 获取当前核心记忆快照（用于 audit old_value）
	currentCM, err := GetCoreMemory(db, userID)
	if err != nil {
		return 0, fmt.Errorf("get current core memory for audit: %w", err)
	}

	applied := 0
	for _, edit := range edits {
		// 验证 section 合法性
		if !validCoreSections[edit.Section] {
			slog.Warn("Core Memory 自编辑: 无效 section, 跳过",
				"section", edit.Section, "user_id", userID)
			continue
		}

		// 验证内容非空
		if edit.Content == "" {
			continue
		}

		// 安全护栏: 想象模式下强制 append
		mode := edit.Mode
		if mode == "" {
			mode = "append"
		}
		if forceAppendOnly && mode != "append" {
			slog.Warn("Core Memory 自编辑: 想象模式下拒绝 replace 操作, 降级为 append",
				"section", edit.Section, "user_id", userID)
			mode = "append"
		}

		// 获取旧值
		oldValue := getCoreMemorySection(currentCM, edit.Section)

		// 执行编辑
		switch mode {
		case "append":
			err = AppendCoreMemory(db, userID, edit.Section, edit.Content, "llm_"+source)
		default: // replace
			err = UpdateCoreMemory(db, userID, edit.Section, edit.Content, "llm_"+source)
		}
		if err != nil {
			slog.Error("Core Memory 自编辑失败",
				"section", edit.Section, "mode", mode, "error", err)
			continue
		}

		// 记录审计日志
		if logErr := LogCoreMemoryEdit(db, userID, edit.Section, mode, source, oldValue, edit.Content); logErr != nil {
			slog.Error("审计日志写入失败", "error", logErr)
		}

		// Prometheus metric
		coreMemoryAutoEditsTotal.WithLabelValues(edit.Section, mode, source).Inc()

		applied++
	}

	if applied > 0 {
		slog.Info("Core Memory 自编辑批量完成",
			"user_id", userID,
			"source", source,
			"applied", applied,
			"total", len(edits),
		)
	}

	return applied, nil
}

// getCoreMemorySection 从 CoreMemoryMap 中获取指定 section 的值。
func getCoreMemorySection(cm *CoreMemoryMap, section string) string {
	if cm == nil {
		return ""
	}
	switch section {
	case "persona":
		return cm.Persona
	case "preferences":
		return cm.Preferences
	case "instructions":
		return cm.Instructions
	default:
		return ""
	}
}
