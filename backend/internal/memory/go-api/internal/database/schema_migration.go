// Package database — Schema 版本管理（P3-2）。
// 追踪每个租户 DB 的 schema 版本，防止不一致。
package database

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// CurrentSchemaVersion 当前应用要求的 schema 版本。
// 每次 models 变更（新增表/列）时递增此值。
const CurrentSchemaVersion = 3

// appVersion 从编译信息中提取应用版本。
func appVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "dev"
}

// EnsureSchemaVersion 检查并记录当前 DB 的 schema 版本。
// 若 schema_versions 表为空（首次迁移），插入初始版本记录。
// 若已有记录且版本低于 CurrentSchemaVersion，记录警告。
func EnsureSchemaVersion(db *gorm.DB) error {
	var latest models.SchemaVersion
	err := db.Order("version DESC").First(&latest).Error

	if err == gorm.ErrRecordNotFound {
		// 首次迁移：插入初始版本
		initial := models.SchemaVersion{
			Version:    CurrentSchemaVersion,
			AppVersion: appVersion(),
			Detail:     "初始 schema（AutoMigrate）",
		}
		if createErr := db.Create(&initial).Error; createErr != nil {
			return fmt.Errorf("insert initial schema version: %w", createErr)
		}
		slog.Info("Schema version initialized", "version", CurrentSchemaVersion)
		return nil
	}
	if err != nil {
		return fmt.Errorf("query schema version: %w", err)
	}

	// 版本一致性检查
	if latest.Version < CurrentSchemaVersion {
		slog.Warn("Schema version behind",
			"db_version", latest.Version,
			"app_version", CurrentSchemaVersion)
	}
	return nil
}

// GetSchemaStatus 返回指定 DB 的 schema 版本状态。
func GetSchemaStatus(db *gorm.DB) (*SchemaStatus, error) {
	var latest models.SchemaVersion
	err := db.Order("version DESC").First(&latest).Error

	if err == gorm.ErrRecordNotFound {
		return &SchemaStatus{
			CurrentVersion: 0,
			TargetVersion:  CurrentSchemaVersion,
			NeedsMigration: true,
			Detail:         "未找到版本记录",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query schema version: %w", err)
	}

	return &SchemaStatus{
		CurrentVersion: latest.Version,
		TargetVersion:  CurrentSchemaVersion,
		NeedsMigration: latest.Version < CurrentSchemaVersion,
		AppliedAt:      latest.AppliedAt.Format("2006-01-02T15:04:05Z"),
		AppVersion:     latest.AppVersion,
		Detail:         latest.Detail,
	}, nil
}

// SchemaStatus schema 版本状态响应结构。
type SchemaStatus struct {
	CurrentVersion int    `json:"current_version"`
	TargetVersion  int    `json:"target_version"`
	NeedsMigration bool   `json:"needs_migration"`
	AppliedAt      string `json:"applied_at,omitempty"`
	AppVersion     string `json:"app_version,omitempty"`
	Detail         string `json:"detail,omitempty"`
}
