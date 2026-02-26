//go:build cgo

// Package services — Agent 注册/发现服务单元测试。
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/uhms/go-api/internal/models"
)

// agentSQLite 是 Agent 的 SQLite 兼容版本。
// 列名与 models.Agent 一致，UUID 存为 text。
type agentSQLite struct {
	ID        uuid.UUID       `gorm:"type:text;primaryKey"`
	TenantID  string          `gorm:"type:varchar(100);not null;index"`
	Name      string          `gorm:"type:varchar(200);not null"`
	Status    string          `gorm:"type:varchar(20);not null;default:'offline'"`
	Endpoint  string          `gorm:"type:varchar(500);not null"`
	LastSeen  *time.Time      `gorm:"index"`
	Metadata  *map[string]any `gorm:"type:text"`
	CreatedAt time.Time       `gorm:"autoCreateTime"`
	UpdatedAt *time.Time      `gorm:"autoUpdateTime"`
}

func (agentSQLite) TableName() string { return "agents" }

// newTestDB 创建内存 SQLite 测试数据库并迁移 agents 表。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	// Use SQLite-compatible model to create table schema.
	if err := db.AutoMigrate(&agentSQLite{}); err != nil {
		t.Fatalf("AutoMigrate agents: %v", err)
	}
	return db
}

func TestAgentRegistry_RegisterAndGet(t *testing.T) {
	db := newTestDB(t)
	registry := NewAgentRegistry(db)

	agent, err := registry.RegisterAgent("tenant-1", "agent-a", "ws://localhost:9100", nil)
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if agent.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", agent.TenantID, "tenant-1")
	}
	if agent.Status != models.AgentStatusOnline {
		t.Errorf("Status = %q, want %q", agent.Status, models.AgentStatusOnline)
	}

	// Should be retrievable.
	got, ok := registry.GetAgent("tenant-1")
	if !ok {
		t.Fatal("GetAgent returned false, want true")
	}
	if got.Name != "agent-a" {
		t.Errorf("Name = %q, want %q", got.Name, "agent-a")
	}
}

func TestAgentRegistry_RegisterUpsert(t *testing.T) {
	db := newTestDB(t)
	registry := NewAgentRegistry(db)

	// Register first time.
	_, _ = registry.RegisterAgent("tenant-1", "agent-v1", "ws://old", nil)

	// Register again — should update, not create duplicate.
	agent, err := registry.RegisterAgent("tenant-1", "agent-v2", "ws://new", nil)
	if err != nil {
		t.Fatalf("RegisterAgent upsert: %v", err)
	}
	if agent.Name != "agent-v2" {
		t.Errorf("Name after upsert = %q, want %q", agent.Name, "agent-v2")
	}

	// DB should have only 1 record for this tenant.
	var count int64
	db.Model(&agentSQLite{}).Where("tenant_id = ?", "tenant-1").Count(&count)
	if count != 1 {
		t.Errorf("Agent count = %d, want 1", count)
	}
}

func TestAgentRegistry_Deregister(t *testing.T) {
	db := newTestDB(t)
	registry := NewAgentRegistry(db)

	_, _ = registry.RegisterAgent("tenant-1", "agent-a", "ws://t", nil)
	registry.DeregisterAgent("tenant-1")

	// Should not be in cache.
	_, ok := registry.GetAgent("tenant-1")
	if ok {
		t.Error("GetAgent returned true after deregister, want false")
	}

	// DB should show offline.
	var agent agentSQLite
	db.Where("tenant_id = ?", "tenant-1").First(&agent)
	if agent.Status != models.AgentStatusOffline {
		t.Errorf("Status = %q, want %q", agent.Status, models.AgentStatusOffline)
	}
}

func TestAgentRegistry_ListAgents(t *testing.T) {
	db := newTestDB(t)
	registry := NewAgentRegistry(db)

	_, _ = registry.RegisterAgent("tenant-a", "agent-1", "ws://a", nil)
	_, _ = registry.RegisterAgent("tenant-b", "agent-2", "ws://b", nil)

	agents, err := registry.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("ListAgents count = %d, want 2", len(agents))
	}
}

func TestAgentRegistry_UpdateHeartbeat(t *testing.T) {
	db := newTestDB(t)
	registry := NewAgentRegistry(db)

	_, _ = registry.RegisterAgent("tenant-1", "agent-a", "ws://t", nil)
	initialAgent, _ := registry.GetAgent("tenant-1")
	initialSeen := initialAgent.LastSeen

	// Small sleep to ensure time difference.
	time.Sleep(10 * time.Millisecond)

	// Update heartbeat — should change last_seen.
	registry.UpdateHeartbeat("tenant-1")
	updatedAgent, _ := registry.GetAgent("tenant-1")
	if updatedAgent.LastSeen == nil {
		t.Fatal("LastSeen is nil after heartbeat")
	}
	if initialSeen != nil && !updatedAgent.LastSeen.After(*initialSeen) {
		t.Error("LastSeen did not advance after heartbeat")
	}
}

func TestAgentRegistry_GetNonExistent(t *testing.T) {
	db := newTestDB(t)
	registry := NewAgentRegistry(db)

	_, ok := registry.GetAgent("non-existent")
	if ok {
		t.Error("GetAgent for non-existent tenant returned true")
	}
}
