//go:build cgo

// Package services — N2 LLM 自编辑记忆单元测试。
// 覆盖 Reflection 自编辑、Imagination 自编辑、审计日志和安全护栏。
package services

import (
	"context"
	"encoding/json"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// --- 测试 mock ---

// mockSelfEditLLM 支持返回结构化 JSON 的 mock LLM。
type mockSelfEditLLM struct {
	generateResult string
	generateError  error
}

func (m *mockSelfEditLLM) Generate(_ context.Context, _ string) (string, error) {
	return m.generateResult, m.generateError
}
func (m *mockSelfEditLLM) ExtractEntities(_ context.Context, _ string) (*ExtractionResult, error) {
	return &ExtractionResult{}, nil
}
func (m *mockSelfEditLLM) ScoreImportance(_ context.Context, _ string) (*ImportanceScore, error) {
	return &ImportanceScore{Score: 0.5}, nil
}
func (m *mockSelfEditLLM) GenerateReflection(_ context.Context, _ []string, _ string) (string, error) {
	return m.generateResult, m.generateError
}

// setupSelfEditDB creates an in-memory SQLite database with all necessary tables.
func setupSelfEditDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Memory{},
		&models.CoreMemory{},
		&models.CoreMemoryAuditLog{},
	); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	return db
}

// seedTestMemories inserts N test memories for a user.
func seedTestMemories(db *gorm.DB, userID string, count int) {
	for i := 0; i < count; i++ {
		db.Create(&models.Memory{
			Content:         "用户喜欢 Go 语言编程",
			UserID:          userID,
			MemoryType:      MemoryTypeObservation,
			Category:        CategoryFact,
			ImportanceScore: 0.8,
		})
	}
}

// --- Phase 1: Reflection 自编辑测试 ---

func TestTriggerReflection_WithCoreMemoryEdits(t *testing.T) {
	db := setupSelfEditDB(t)
	ctx := context.Background()
	userID := "self-edit-user"

	// 准备 LLM 返回带核心记忆编辑的 JSON
	reflectionJSON := map[string]any{
		"reflection": "用户近期频繁提到 Go 语言学习，显示出对后端开发的浓厚兴趣。",
		"core_memory_edits": []map[string]any{
			{"section": "preferences", "mode": "append", "content": "喜欢 Go 语言"},
			{"section": "persona", "mode": "replace", "content": "后端开发者"},
		},
	}
	jsonBytes, _ := json.Marshal(reflectionJSON)

	mockLLM := &mockSelfEditLLM{generateResult: string(jsonBytes)}
	memManager := &MemoryManager{
		vectorStore: &VectorStoreService{},
		graphStore:  &GraphStoreService{},
		treeManager: &TreeManager{},
		llmClient:   mockLLM,
	}

	// 插入足够的记忆触发反思
	seedTestMemories(db, userID, 5)

	// 触发反思
	reflection, err := memManager.TriggerReflection(ctx, db, userID)
	if err != nil {
		t.Fatalf("TriggerReflection failed: %v", err)
	}
	if reflection == nil {
		t.Fatal("Expected reflection, got nil")
	}

	// 验证反思内容是从 JSON 中提取的
	if reflection.Content != "用户近期频繁提到 Go 语言学习，显示出对后端开发的浓厚兴趣。" {
		t.Errorf("反思内容不匹配: %s", reflection.Content)
	}

	// 验证 metadata 中有 core_memory_edits_count
	if reflection.Metadata == nil {
		t.Fatal("反思 metadata 不应为 nil")
	}
	meta := *reflection.Metadata
	if count, ok := meta["core_memory_edits_count"].(int); !ok || count != 2 {
		t.Errorf("core_memory_edits_count 应为 2, 实际为 %v", meta["core_memory_edits_count"])
	}

	// 验证核心记忆被正确更新
	cm, err := GetCoreMemory(db, userID)
	if err != nil {
		t.Fatalf("GetCoreMemory failed: %v", err)
	}
	if cm.Persona != "后端开发者" {
		t.Errorf("persona 应为 '后端开发者', 实际为 '%s'", cm.Persona)
	}
	if cm.Preferences != "喜欢 Go 语言" {
		t.Errorf("preferences 应包含 '喜欢 Go 语言', 实际为 '%s'", cm.Preferences)
	}

	// 验证审计日志已记录
	var auditLogs []models.CoreMemoryAuditLog
	db.Where("user_id = ?", userID).Find(&auditLogs)
	if len(auditLogs) != 2 {
		t.Fatalf("审计日志应有 2 条, 实际有 %d 条", len(auditLogs))
	}
}

func TestTriggerReflection_NoEdits(t *testing.T) {
	db := setupSelfEditDB(t)
	ctx := context.Background()
	userID := "no-edit-user"

	// LLM 返回无编辑的 JSON
	reflectionJSON := map[string]any{
		"reflection":        "用户的日常活动无显著变化。",
		"core_memory_edits": []any{},
	}
	jsonBytes, _ := json.Marshal(reflectionJSON)

	mockLLM := &mockSelfEditLLM{generateResult: string(jsonBytes)}
	memManager := &MemoryManager{
		vectorStore: &VectorStoreService{},
		graphStore:  &GraphStoreService{},
		treeManager: &TreeManager{},
		llmClient:   mockLLM,
	}

	seedTestMemories(db, userID, 5)
	reflection, err := memManager.TriggerReflection(ctx, db, userID)
	if err != nil {
		t.Fatalf("TriggerReflection failed: %v", err)
	}
	if reflection == nil {
		t.Fatal("Expected reflection, got nil")
	}

	// 验证无编辑
	meta := *reflection.Metadata
	if count, ok := meta["core_memory_edits_count"].(int); !ok || count != 0 {
		t.Errorf("core_memory_edits_count 应为 0, 实际为 %v", meta["core_memory_edits_count"])
	}

	// 验证无审计日志
	var auditLogs []models.CoreMemoryAuditLog
	db.Where("user_id = ?", userID).Find(&auditLogs)
	if len(auditLogs) != 0 {
		t.Errorf("审计日志应有 0 条, 实际有 %d 条", len(auditLogs))
	}
}

func TestTriggerReflection_PlainTextFallback(t *testing.T) {
	db := setupSelfEditDB(t)
	ctx := context.Background()
	userID := "plain-text-user"

	// LLM 返回纯文本（非 JSON，向后兼容）
	mockLLM := &mockSelfEditLLM{generateResult: "用户的记忆模式显示对编程的持续兴趣。"}
	memManager := &MemoryManager{
		vectorStore: &VectorStoreService{},
		graphStore:  &GraphStoreService{},
		treeManager: &TreeManager{},
		llmClient:   mockLLM,
	}

	seedTestMemories(db, userID, 5)
	reflection, err := memManager.TriggerReflection(ctx, db, userID)
	if err != nil {
		t.Fatalf("TriggerReflection failed: %v", err)
	}
	if reflection == nil {
		t.Fatal("Expected reflection, got nil")
	}

	// 纯文本应作为反思内容直接使用
	if reflection.Content != "用户的记忆模式显示对编程的持续兴趣。" {
		t.Errorf("反思内容不匹配: %s", reflection.Content)
	}
}

// --- Phase 2: ApplyCoreMemoryEdits 安全护栏测试 ---

func TestApplyCoreMemoryEdits_ForceAppendOnly(t *testing.T) {
	db := setupSelfEditDB(t)
	userID := "append-only-user"

	// 先设置初始核心记忆
	if err := UpdateCoreMemory(db, userID, "preferences", "原始偏好", "test"); err != nil {
		t.Fatalf("设置初始核心记忆失败: %v", err)
	}

	// 尝试 replace 操作，但 forceAppendOnly=true
	edits := []CoreMemoryEditAction{
		{Section: "preferences", Mode: "replace", Content: "覆盖的偏好"},
	}

	applied, err := ApplyCoreMemoryEdits(db, userID, "imagination", edits, true)
	if err != nil {
		t.Fatalf("ApplyCoreMemoryEdits failed: %v", err)
	}
	if applied != 1 {
		t.Fatalf("应有 1 条被执行（降级为 append）, 实际 %d", applied)
	}

	// 验证是 append 而非 replace
	cm, err := GetCoreMemory(db, userID)
	if err != nil {
		t.Fatalf("GetCoreMemory failed: %v", err)
	}
	// append 模式下应该是 "原始偏好\n覆盖的偏好"
	if cm.Preferences != "原始偏好\n覆盖的偏好" {
		t.Errorf("forceAppendOnly 应降级为 append, 实际结果: '%s'", cm.Preferences)
	}
}

func TestApplyCoreMemoryEdits_InvalidSection(t *testing.T) {
	db := setupSelfEditDB(t)
	userID := "invalid-section-user"

	edits := []CoreMemoryEditAction{
		{Section: "invalid_section", Mode: "append", Content: "test"},
		{Section: "preferences", Mode: "append", Content: "有效内容"},
	}

	applied, err := ApplyCoreMemoryEdits(db, userID, "reflection", edits, false)
	if err != nil {
		t.Fatalf("ApplyCoreMemoryEdits failed: %v", err)
	}
	// 无效 section 应被跳过, 只有 1 条被执行
	if applied != 1 {
		t.Errorf("应有 1 条被执行, 实际 %d", applied)
	}
}

func TestApplyCoreMemoryEdits_EmptyEdits(t *testing.T) {
	db := setupSelfEditDB(t)

	applied, err := ApplyCoreMemoryEdits(db, "any-user", "reflection", nil, false)
	if err != nil {
		t.Fatalf("空编辑不应返回错误: %v", err)
	}
	if applied != 0 {
		t.Errorf("空编辑应返回 0, 实际 %d", applied)
	}
}

// --- Audit Log 测试 ---

func TestLogCoreMemoryEdit_Creates(t *testing.T) {
	db := setupSelfEditDB(t)
	userID := "audit-user"

	err := LogCoreMemoryEdit(db, userID, "persona", "replace", "reflection", "旧值", "新值")
	if err != nil {
		t.Fatalf("LogCoreMemoryEdit failed: %v", err)
	}

	var logs []models.CoreMemoryAuditLog
	db.Where("user_id = ?", userID).Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("应有 1 条审计日志, 实际 %d", len(logs))
	}

	log := logs[0]
	if log.Section != "persona" {
		t.Errorf("section 应为 persona, 实际 %s", log.Section)
	}
	if log.Mode != "replace" {
		t.Errorf("mode 应为 replace, 实际 %s", log.Mode)
	}
	if log.Source != "reflection" {
		t.Errorf("source 应为 reflection, 实际 %s", log.Source)
	}
	if log.OldValue != "旧值" {
		t.Errorf("old_value 不匹配: %s", log.OldValue)
	}
	if log.NewValue != "新值" {
		t.Errorf("new_value 不匹配: %s", log.NewValue)
	}
}

// --- getCoreMemorySection 辅助函数测试 ---

func TestGetCoreMemorySection(t *testing.T) {
	cm := &CoreMemoryMap{
		Persona:      "test persona",
		Preferences:  "test prefs",
		Instructions: "test instrs",
	}

	tests := []struct {
		section  string
		expected string
	}{
		{"persona", "test persona"},
		{"preferences", "test prefs"},
		{"instructions", "test instrs"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := getCoreMemorySection(cm, tt.section)
		if got != tt.expected {
			t.Errorf("getCoreMemorySection(%s) = %s, want %s", tt.section, got, tt.expected)
		}
	}

	// nil 测试
	if getCoreMemorySection(nil, "persona") != "" {
		t.Error("nil CoreMemoryMap 应返回空字符串")
	}
}
