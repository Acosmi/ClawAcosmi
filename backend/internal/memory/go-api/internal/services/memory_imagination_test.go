// Package services — L4 想象记忆单元测试。
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uhms/go-api/internal/models"
)

// --- MemoryPlugin 接口编译期契约验证 ---

func TestImaginationEngine_ImplementsMemoryPlugin(t *testing.T) {
	// 编译期已通过 var _ MemoryPlugin = (*ImaginationEngine)(nil) 验证
	// 此处做运行时 nil 安全检查
	var plugin MemoryPlugin = &ImaginationEngine{}
	if plugin.Name() != "imagination" {
		t.Fatalf("插件名称应为 imagination，实际为 %s", plugin.Name())
	}
}

// --- ImaginationEngine 参数验证 ---

func TestImaginationEngine_Run_EmptyUserID(t *testing.T) {
	engine := NewImaginationEngine(&GraphStoreService{}, nil, &mockLLMProvider{}, nil)
	_, err := engine.Run(t.Context(), nil, "")
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
	if err.Error() != "user_id is required" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

func TestImaginationEngine_Run_NilLLM(t *testing.T) {
	engine := NewImaginationEngine(&GraphStoreService{}, nil, nil, nil)
	_, err := engine.Run(t.Context(), nil, "user1")
	if err == nil {
		t.Fatal("nil LLM 应返回错误")
	}
	if err.Error() != "LLM provider is required for imagination" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

func TestImaginationEngine_Run_NilGraphStore(t *testing.T) {
	engine := NewImaginationEngine(nil, nil, &mockLLMProvider{}, nil)
	_, err := engine.Run(t.Context(), nil, "user1")
	if err == nil {
		t.Fatal("nil graph store 应返回错误")
	}
	if err.Error() != "graph store is required for imagination" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

// --- NewImaginationEngine 构造函数 ---

func TestNewImaginationEngine_NonNil(t *testing.T) {
	engine := NewImaginationEngine(&GraphStoreService{}, nil, &mockLLMProvider{}, nil)
	if engine == nil {
		t.Fatal("NewImaginationEngine 不应返回 nil")
	}
	if engine.graphStore == nil {
		t.Fatal("graphStore 不应为 nil")
	}
}

// --- GetTrendingEntities 参数验证 ---

func TestGetTrendingEntities_EmptyUserID(t *testing.T) {
	gs := &GraphStoreService{}
	_, err := gs.GetTrendingEntities(nil, "", 3, 3)
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
	if err.Error() != "user_id is required" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

func TestGetTrendingEntities_DefaultParams(t *testing.T) {
	gs := &GraphStoreService{}
	// minDaysSpan < 0 → 默认 3, topN <= 0 → 默认 3
	// nil db 会导致 panic（说明参数验证通过了）
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil db 应导致 panic（说明参数验证通过、默认值已应用）")
		}
	}()
	_, _ = gs.GetTrendingEntities(nil, "user1", -1, 0)
}

func TestGetTrendingEntities_OverTopN(t *testing.T) {
	gs := &GraphStoreService{}
	// topN > 50 → 默认 3
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil db 应导致 panic（说明参数验证通过、topN 被修正）")
		}
	}()
	_, _ = gs.GetTrendingEntities(nil, "user1", 3, 100)
}

// --- TrendingEntity 零值 ---

func TestTrendingEntity_Defaults(t *testing.T) {
	te := TrendingEntity{}
	if te.RelationCount != 0 {
		t.Fatal("默认 RelationCount 应为 0")
	}
	if te.HeatScore != 0 {
		t.Fatal("默认 HeatScore 应为 0")
	}
	if te.DaysSpan != 0 {
		t.Fatal("默认 DaysSpan 应为 0")
	}
}

// --- ListImaginations 参数验证 ---

func TestListImaginations_EmptyUserID(t *testing.T) {
	_, err := ListImaginations(nil, "", 1, 20)
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
}

func TestListImaginations_DefaultPageSize(t *testing.T) {
	// page < 1 → 1, pageSize < 1 → 20
	// nil db 会因为实际查询而 panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil db 应导致 panic（说明参数验证通过）")
		}
	}()
	_, _ = ListImaginations(nil, "user1", 0, 0)
}

// --- ParseImaginationMeta ---

func TestParseImaginationMeta_NilMetadata(t *testing.T) {
	_, err := ParseImaginationMeta(nil)
	if err == nil {
		t.Fatal("nil metadata 应返回错误")
	}
}

func TestParseImaginationMeta_ValidData(t *testing.T) {
	meta := map[string]any{
		"simulation_depth":  1,
		"source_type":       "llm_inference",
		"source_urls":       []string{},
		"imagination_probe": "测试问题",
		"trending_entity":   "Go语言",
		"entity_type":       "technology",
		"heat_score":        12.5,
		"days_span":         30,
		"generated_at":      time.Now().UTC().Format(time.RFC3339),
	}
	result, err := ParseImaginationMeta(&meta)
	if err != nil {
		t.Fatalf("解析不应失败: %v", err)
	}
	if result.SimulationDepth != 1 {
		t.Fatalf("SimulationDepth 应为 1，实际为 %d", result.SimulationDepth)
	}
	if result.SourceType != "llm_inference" {
		t.Fatalf("SourceType 应为 llm_inference，实际为 %s", result.SourceType)
	}
	if result.TrendingEntity != "Go语言" {
		t.Fatalf("TrendingEntity 应为 Go语言，实际为 %s", result.TrendingEntity)
	}
	if result.HeatScore != 12.5 {
		t.Fatalf("HeatScore 应为 12.5，实际为 %f", result.HeatScore)
	}
}

// --- ImaginationResult 零值 ---

func TestImaginationResult_Defaults(t *testing.T) {
	r := &ImaginationResult{}
	if r.Memories != nil {
		t.Fatal("默认 Memories 应为 nil")
	}
	if r.Total != 0 {
		t.Fatal("默认 Total 应为 0")
	}
}

// --- 辅助：验证想象记忆元数据完整性 ---

func TestImaginationMetadata_AntiPollution(t *testing.T) {
	// 模拟生成元数据并验证防污染字段存在性
	metadata := map[string]any{
		"simulation_depth":  1,
		"source_type":       "llm_inference",
		"source_urls":       []string{},
		"imagination_probe": "AI 助手记忆系统未来会如何演进？",
		"trending_entity":   "UHMS",
		"entity_type":       "project",
		"heat_score":        15.3,
		"days_span":         45,
		"generated_at":      "2026-02-21T00:00:00Z",
	}

	memory := &models.Memory{
		ID:              uuid.New(),
		Content:         "测试想象记忆内容",
		UserID:          "test-user",
		MemoryType:      MemoryTypeImagination,
		Category:        CategoryInsight,
		ImportanceScore: 0.6,
		DecayFactor:     MaxDecayFactor,
		Metadata:        &metadata,
	}

	// 验证记忆类型标记
	if memory.MemoryType != "imagination" {
		t.Fatalf("记忆类型应为 imagination, 实际为 %s", memory.MemoryType)
	}

	// 验证防污染断言
	result, err := ParseImaginationMeta(memory.Metadata)
	if err != nil {
		t.Fatalf("解析防污染元数据失败: %v", err)
	}
	if result.SimulationDepth < 1 {
		t.Fatal("simulation_depth 必须 >= 1")
	}
	if result.SourceType == "" {
		t.Fatal("source_type 不能为空")
	}
	if result.Probe == "" {
		t.Fatal("imagination_probe 不能为空")
	}
	if result.TrendingEntity == "" {
		t.Fatal("trending_entity 不能为空")
	}
}

// --- 事件驱动触发器集成测试 ---

// TestImaginationEngine_RegisterTrigger verifies trigger registration.
func TestImaginationEngine_RegisterTrigger(t *testing.T) {
	engine := NewImaginationEngine(&GraphStoreService{}, nil, &mockLLMProvider{}, nil)
	if len(engine.triggers) != 0 {
		t.Fatalf("New engine should have 0 triggers, got %d", len(engine.triggers))
	}

	engine.RegisterTrigger(NewActivityBurstTrigger())
	engine.RegisterTrigger(NewEntityClusterTrigger())
	engine.RegisterTrigger(NewTopicDriftTrigger())

	if len(engine.triggers) != 3 {
		t.Fatalf("Expected 3 triggers, got %d", len(engine.triggers))
	}
}

// TestImaginationEngine_CheckTriggers_NoMatch verifies empty result when no trigger fires.
func TestImaginationEngine_CheckTriggers_NoMatch(t *testing.T) {
	engine := NewImaginationEngine(&GraphStoreService{}, nil, &mockLLMProvider{}, nil)
	engine.RegisterTrigger(NewActivityBurstTrigger())

	// nil db → no trigger should fire
	result := engine.CheckTriggers(nil, "user1")
	if result != "" {
		t.Errorf("Expected empty trigger name, got '%s'", result)
	}
}
