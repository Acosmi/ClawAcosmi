// Package services — Agent 自编辑 Core Memory 服务。
// 参考 Letta/MemGPT Core Memory 模型：persona + preferences + instructions。
package services

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// CoreMemoryMap 汇总用户所有 Core Memory 分区。
type CoreMemoryMap struct {
	Persona      string `json:"persona"`
	Preferences  string `json:"preferences"`
	Instructions string `json:"instructions"`
}

// validCoreSections 允许的 Core Memory 分区。
var validCoreSections = map[string]bool{
	models.CoreMemSectionPersona:      true,
	models.CoreMemSectionPreferences:  true,
	models.CoreMemSectionInstructions: true,
}

// GetCoreMemory 获取用户所有 Core Memory 分区。
func GetCoreMemory(db *gorm.DB, userID string) (*CoreMemoryMap, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}

	var records []models.CoreMemory
	if err := db.Where("user_id = ?", userID).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("get core memory: %w", err)
	}

	result := &CoreMemoryMap{}
	for _, r := range records {
		switch r.Section {
		case models.CoreMemSectionPersona:
			result.Persona = r.Content
		case models.CoreMemSectionPreferences:
			result.Preferences = r.Content
		case models.CoreMemSectionInstructions:
			result.Instructions = r.Content
		}
	}
	return result, nil
}

// UpdateCoreMemory 更新指定 Core Memory 分区（Upsert）。
func UpdateCoreMemory(db *gorm.DB, userID, section, content, updatedBy string) error {
	if userID == "" {
		return errors.New("user_id is required")
	}
	if !validCoreSections[section] {
		return fmt.Errorf("invalid section %q, allowed: persona, preferences, instructions", section)
	}
	if updatedBy == "" {
		updatedBy = "agent"
	}

	var existing models.CoreMemory
	err := db.Where("user_id = ? AND section = ?", userID, section).First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 新建
		record := &models.CoreMemory{
			UserID:    userID,
			Section:   section,
			Content:   content,
			UpdatedBy: updatedBy,
		}
		if err := db.Create(record).Error; err != nil {
			return fmt.Errorf("create core memory: %w", err)
		}
		slog.Info("Core Memory 创建", "user_id", userID, "section", section, "by", updatedBy)
		return nil
	}
	if err != nil {
		return fmt.Errorf("query core memory: %w", err)
	}

	// 更新
	if err := db.Model(&existing).Updates(map[string]any{
		"content":    content,
		"updated_by": updatedBy,
	}).Error; err != nil {
		return fmt.Errorf("update core memory: %w", err)
	}

	slog.Info("Core Memory 更新", "user_id", userID, "section", section, "by", updatedBy)
	return nil
}

// AppendCoreMemory 向指定分区追加内容（不覆盖）。
func AppendCoreMemory(db *gorm.DB, userID, section, addition, updatedBy string) error {
	if addition == "" {
		return nil
	}

	cm, err := GetCoreMemory(db, userID)
	if err != nil {
		return err
	}

	var existing string
	switch section {
	case models.CoreMemSectionPersona:
		existing = cm.Persona
	case models.CoreMemSectionPreferences:
		existing = cm.Preferences
	case models.CoreMemSectionInstructions:
		existing = cm.Instructions
	default:
		return fmt.Errorf("invalid section: %s", section)
	}

	newContent := existing
	if newContent != "" {
		newContent += "\n"
	}
	newContent += addition

	return UpdateCoreMemory(db, userID, section, newContent, updatedBy)
}
