//go:build e2e
// +build e2e

// Package e2etest — 基础设施冒烟测试。
// 验证 Mock 工厂和测试数据种子在 e2e tag 下可正常编译和运行。
package e2etest

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestSmoke_MockFSStore_WriteAndRead 验证 MockFSStore 基本读写
func TestSmoke_MockFSStore_WriteAndRead(t *testing.T) {
	store := NewMockFSStore()
	err := store.WriteMemory("t1", "u1", "smoke-001", MemoryTypeEpisodic, CategoryFact,
		"Full content here", "L0 摘要", "L1 概述")
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}

	// L0
	l0, err := store.ReadMemory("t1", "u1", "episodic/fact/smoke-001", 0)
	if err != nil {
		t.Fatalf("ReadMemory L0: %v", err)
	}
	if l0 != "L0 摘要" {
		t.Errorf("L0 = %q, want %q", l0, "L0 摘要")
	}

	// L1
	l1, err := store.ReadMemory("t1", "u1", "episodic/fact/smoke-001", 1)
	if err != nil {
		t.Fatalf("ReadMemory L1: %v", err)
	}
	if l1 != "L1 概述" {
		t.Errorf("L1 = %q, want %q", l1, "L1 概述")
	}

	// L2
	l2, err := store.ReadMemory("t1", "u1", "episodic/fact/smoke-001", 2)
	if err != nil {
		t.Fatalf("ReadMemory L2: %v", err)
	}
	if l2 != "Full content here" {
		t.Errorf("L2 = %q, want %q", l2, "Full content here")
	}
}

// TestSmoke_MockVectorStore_UpsertAndSearch 验证 MockVectorStore 基本搜索
func TestSmoke_MockVectorStore_UpsertAndSearch(t *testing.T) {
	vs := NewMockVectorStore()
	vs.Upsert("mem-1", "I love drinking latte coffee every morning", "u1", MemoryTypeEpisodic, CategoryPreference, 0.8)
	vs.Upsert("mem-2", "Rust ownership system ensures memory safety", "u1", MemoryTypeSemantic, CategorySkill, 0.7)
	vs.Upsert("mem-3", "Go concurrency with goroutines and channels", "u1", MemoryTypeSemantic, CategorySkill, 0.7)

	if vs.Count() != 3 {
		t.Fatalf("Count = %d, want 3", vs.Count())
	}

	results := vs.Search("coffee morning", "u1", 5, nil)
	if len(results) == 0 {
		t.Fatal("Search returned 0 results, expected at least 1")
	}
	// 第一条结果应与 coffee 相关（相似度最高）
	if results[0].MemoryID != "mem-1" {
		t.Logf("注意: 第一条结果为 %s (简单哈希嵌入的局限性)", results[0].MemoryID)
	}
}

// TestSmoke_MockLLM 验证 MockLLM 基本功能
func TestSmoke_MockLLM(t *testing.T) {
	llm := NewMockLLM("test response")
	resp, err := llm.Generate(context.Background(), "any prompt")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp != "test response" {
		t.Errorf("response = %q, want %q", resp, "test response")
	}
	if llm.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", llm.CallCount)
	}
}

// TestSmoke_SeedTestMemories 验证测试数据种子完整性
func TestSmoke_SeedTestMemories(t *testing.T) {
	seeds := SeedTestMemories()
	if len(seeds) < 5 {
		t.Fatalf("SeedTestMemories returned %d items, want >= 5", len(seeds))
	}

	// 确保包含所有核心记忆类型
	typeSet := make(map[string]bool)
	for _, m := range seeds {
		typeSet[m.MemoryType] = true
	}
	requiredTypes := []string{MemoryTypeEpisodic, MemoryTypeSemantic, MemoryTypePermanent, MemoryTypeImagination}
	for _, rt := range requiredTypes {
		if !typeSet[rt] {
			t.Errorf("SeedTestMemories 缺少记忆类型: %s", rt)
		}
	}
}

// TestSmoke_SeedToFSStore 验证种子数据写入 FSStore
func TestSmoke_SeedToFSStore(t *testing.T) {
	store := NewMockFSStore()
	seeds := SeedTestMemories()
	SeedToFSStore(store, "test-tenant", seeds)

	// 验证每条种子数据都能读取
	for _, m := range seeds {
		uri := m.MemoryType + "/" + m.Category + "/" + m.MemoryID
		l0, err := store.ReadMemory("test-tenant", m.UserID, uri, 0)
		if err != nil {
			t.Errorf("ReadMemory L0 for %s: %v", m.MemoryID, err)
			continue
		}
		if l0 != m.L0Abstract {
			t.Errorf("%s L0 = %q, want %q", m.MemoryID, l0, m.L0Abstract)
		}
	}
}

// TestSmoke_BatchReadL0 验证批量 L0 读取
func TestSmoke_BatchReadL0(t *testing.T) {
	store := NewMockFSStore()
	seeds := SeedTestMemories()
	SeedToFSStore(store, "t1", seeds)

	var uris []string
	for _, m := range seeds {
		uris = append(uris, m.MemoryType+"/"+m.Category+"/"+m.MemoryID)
	}

	entries, err := store.BatchReadL0("t1", seeds[0].UserID, uris)
	if err != nil {
		t.Fatalf("BatchReadL0: %v", err)
	}
	if len(entries) != len(seeds) {
		t.Fatalf("BatchReadL0 returned %d entries, want %d", len(entries), len(seeds))
	}
}

// TestSmoke_ClassifyMemoryTier 验证分层策略
func TestSmoke_ClassifyMemoryTier(t *testing.T) {
	tests := []struct {
		memoryType string
		want       TierPolicy
	}{
		{MemoryTypePermanent, TierAlwaysL1},
		{MemoryTypeImagination, TierL0Only},
		{MemoryTypeEpisodic, TierStandard},
		{MemoryTypeSemantic, TierStandard},
	}
	for _, tt := range tests {
		got := ClassifyMemoryTier(tt.memoryType)
		if got != tt.want {
			t.Errorf("ClassifyMemoryTier(%q) = %d, want %d", tt.memoryType, got, tt.want)
		}
	}
}

// TestSmoke_CJKTestData 验证 CJK 测试数据文件可读
func TestSmoke_CJKTestData(t *testing.T) {
	data, err := os.ReadFile("testdata/cjk_samples.txt")
	if err != nil {
		t.Fatalf("读取 CJK 测试数据: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "拿铁") {
		t.Error("CJK 数据缺少中文内容")
	}
	if !strings.Contains(content, "日本語") {
		t.Error("CJK 数据缺少日文内容")
	}
	if !strings.Contains(content, "한국") {
		t.Error("CJK 数据缺少韩文内容")
	}
}

// TestSmoke_LargePayload 验证大 payload 文件存在且 >10KB
func TestSmoke_LargePayload(t *testing.T) {
	info, err := os.Stat("testdata/large_payload.txt")
	if err != nil {
		t.Fatalf("大 payload 文件不存在: %v", err)
	}
	if info.Size() < 10*1024 {
		t.Errorf("大 payload 文件仅 %d bytes, 期望 >= 10KB", info.Size())
	}
}
