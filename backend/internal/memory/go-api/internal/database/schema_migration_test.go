// Package database — Schema 版本管理单元测试。
package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// schemaVersionSQLite 是 SchemaVersion 的 SQLite 兼容版本。
// 与 models.SchemaVersion 列名一致，但去掉 gen_random_uuid() 默认值。
type schemaVersionSQLite struct {
	ID         uuid.UUID `gorm:"type:text;primaryKey"`
	Version    int       `gorm:"not null;default:1"`
	AppliedAt  time.Time `gorm:"autoCreateTime"`
	AppVersion string    `gorm:"type:varchar(50);not null"`
	Detail     string    `gorm:"type:text;not null;default:''"`
}

func (schemaVersionSQLite) TableName() string { return "schema_versions" }

// newTestDB 创建内存 SQLite 测试数据库。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	// Use SQLite-compatible model to create table.
	if err := db.AutoMigrate(&schemaVersionSQLite{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	return db
}

func TestEnsureSchemaVersion_FirstRun(t *testing.T) {
	db := newTestDB(t)

	err := EnsureSchemaVersion(db)
	if err != nil {
		t.Fatalf("EnsureSchemaVersion first run: %v", err)
	}

	// Should have inserted version record.
	var v schemaVersionSQLite
	if err := db.First(&v).Error; err != nil {
		t.Fatalf("Expected version record, got: %v", err)
	}
	if v.Version != CurrentSchemaVersion {
		t.Errorf("Version = %d, want %d", v.Version, CurrentSchemaVersion)
	}
}

func TestEnsureSchemaVersion_Idempotent(t *testing.T) {
	db := newTestDB(t)

	// Run twice — should not error or create duplicate.
	_ = EnsureSchemaVersion(db)
	err := EnsureSchemaVersion(db)
	if err != nil {
		t.Fatalf("EnsureSchemaVersion second run: %v", err)
	}

	var count int64
	db.Model(&schemaVersionSQLite{}).Count(&count)
	if count != 1 {
		t.Errorf("SchemaVersion count = %d, want 1", count)
	}
}

func TestGetSchemaStatus_NoRecords(t *testing.T) {
	db := newTestDB(t)

	status, err := GetSchemaStatus(db)
	if err != nil {
		t.Fatalf("GetSchemaStatus: %v", err)
	}
	if !status.NeedsMigration {
		t.Error("Expected NeedsMigration = true when no records")
	}
	if status.CurrentVersion != 0 {
		t.Errorf("CurrentVersion = %d, want 0", status.CurrentVersion)
	}
}

func TestGetSchemaStatus_AfterMigration(t *testing.T) {
	db := newTestDB(t)
	_ = EnsureSchemaVersion(db)

	status, err := GetSchemaStatus(db)
	if err != nil {
		t.Fatalf("GetSchemaStatus: %v", err)
	}
	if status.NeedsMigration {
		t.Error("Expected NeedsMigration = false after EnsureSchemaVersion")
	}
	if status.CurrentVersion != CurrentSchemaVersion {
		t.Errorf("CurrentVersion = %d, want %d", status.CurrentVersion, CurrentSchemaVersion)
	}
	if status.AppVersion == "" {
		t.Error("AppVersion should not be empty")
	}
}
