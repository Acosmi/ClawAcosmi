//go:build e2e
// +build e2e

// Package e2etest — E2E-TL: 渐进式加载管线测试。
// 覆盖: L0 批量读 → LLM 筛选 → L1 加载 → L2 按需展开 → Token 预算
package e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode"
)

// ============================================================================
// TieredLoader 测试副本 — 镜像生产代码逻辑
// ============================================================================

// TieredLoaderConfig 渐进式加载配置
type TieredLoaderConfig struct {
	MaxL0Count       int
	TopK             int
	TokenBudget      int
	LLMFilterEnabled bool
}

// TestTieredLoader 测试用分层加载器
type TestTieredLoader struct {
	llm    *MockLLM
	fs     *MockFSStore
	config TieredLoaderConfig
}

// NewTestTieredLoader 创建测试用加载器
func NewTestTieredLoader(llm *MockLLM, fs *MockFSStore, cfg TieredLoaderConfig) *TestTieredLoader {
	return &TestTieredLoader{llm: llm, fs: fs, config: cfg}
}

// FilterByL0 使用 LLM 过滤 L0 条目
func (tl *TestTieredLoader) FilterByL0(query string, entries []L0Entry) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	if !tl.config.LLMFilterEnabled || tl.llm == nil {
		return allURIs(entries), nil
	}
	if len(entries) <= tl.config.TopK {
		return allURIs(entries), nil
	}

	response, err := tl.llm.Generate(context.Background(), query)
	if err != nil {
		return allURIs(entries), nil // fallback
	}

	resp := strings.TrimSpace(response)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var uris []string
	if jerr := json.Unmarshal([]byte(resp), &uris); jerr != nil {
		return allURIs(entries), nil
	}

	validURIs := make(map[string]bool)
	for _, e := range entries {
		validURIs[e.URI] = true
	}
	validated := make([]string, 0)
	for _, uri := range uris {
		if validURIs[uri] {
			validated = append(validated, uri)
		}
	}
	if len(validated) == 0 {
		return allURIs(entries), nil
	}
	return validated, nil
}

// ApplyTokenBudget 执行 Token 预算控制
func (tl *TestTieredLoader) ApplyTokenBudget(entries []L1Entry) []L1Entry {
	if tl.config.TokenBudget <= 0 || len(entries) == 0 {
		return entries
	}
	totalTokens := 0
	for i := range entries {
		entryTokens := estimateTokens(entries[i].L1Overview)
		if totalTokens+entryTokens > tl.config.TokenBudget {
			entries[i].L1Overview = ""
		} else {
			totalTokens += entryTokens
		}
	}
	return entries
}

func allURIs(entries []L0Entry) []string {
	uris := make([]string, len(entries))
	for i, e := range entries {
		uris[i] = e.URI
	}
	return uris
}

// estimateTokens CJK 安全的 Token 估算
func estimateTokens(s string) int {
	count := 0
	for _, r := range s {
		if isCJK(r) {
			count += 2
		} else if r == ' ' || r == '\n' || r == '\t' {
			continue
		} else {
			count++
		}
	}
	if count == 0 {
		count = len([]rune(s))
	}
	return count
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// ============================================================================
// E2E-TL 测试用例
// ============================================================================

// E2E-TL-01: 标准 L0→L1 流程
func TestE2E_TL01_StandardL0ToL1(t *testing.T) {
	fs := NewMockFSStore()
	seeds := SeedTestMemories()
	SeedToFSStore(fs, "t1", seeds)

	// 模拟 LLM 选择 2 条
	selectedURIs := []string{
		seeds[0].MemoryType + "/" + seeds[0].Category + "/" + seeds[0].MemoryID,
		seeds[2].MemoryType + "/" + seeds[2].Category + "/" + seeds[2].MemoryID,
	}
	llmResp, _ := json.Marshal(selectedURIs)
	llm := NewMockLLM(string(llmResp))

	tl := NewTestTieredLoader(llm, fs, TieredLoaderConfig{
		MaxL0Count: 50, TopK: 2, TokenBudget: 20000, LLMFilterEnabled: true,
	})

	// 构建 L0 条目
	var l0Entries []L0Entry
	for _, m := range seeds {
		// 跳过 permanent (TierAlwaysL1) 和 imagination (TierL0Only)
		tier := ClassifyMemoryTier(m.MemoryType)
		if tier != TierStandard {
			continue
		}
		l0Entries = append(l0Entries, L0Entry{
			URI:        m.MemoryType + "/" + m.Category + "/" + m.MemoryID,
			L0Abstract: m.L0Abstract, MemoryType: m.MemoryType,
		})
	}

	filtered, err := tl.FilterByL0("咖啡偏好和编程技能", l0Entries)
	if err != nil {
		t.Fatalf("FilterByL0: %v", err)
	}

	// 验证经 LLM 筛选后返回正确的 URI
	if len(filtered) != 2 {
		t.Fatalf("FilterByL0 返回 %d 条, want 2", len(filtered))
	}

	// L1 加载
	l1Entries, err := fs.BatchReadL1("t1", seeds[0].UserID, filtered)
	if err != nil {
		t.Fatalf("BatchReadL1: %v", err)
	}
	for _, e := range l1Entries {
		if e.L1Overview == "" {
			t.Errorf("L1 概述不应为空: URI=%s", e.URI)
		}
	}
}

// E2E-TL-02: Token 预算超限降级
func TestE2E_TL02_TokenBudgetExceeded(t *testing.T) {
	tl := NewTestTieredLoader(nil, nil, TieredLoaderConfig{
		TopK: 8, TokenBudget: 30,
	})

	entries := []L1Entry{
		{URI: "u1", L1Overview: "这是第一条概述内容"},
		{URI: "u2", L1Overview: "这是第二条比较长的概述内容，应该会超出预算"},
		{URI: "u3", L1Overview: "这是第三条同样很长的概述内容"},
	}

	result := tl.ApplyTokenBudget(entries)
	if len(result) != 3 {
		t.Fatalf("ApplyTokenBudget 应返回 3 条, got %d", len(result))
	}

	// 至少有一条被降级
	degraded := 0
	for _, e := range result {
		if e.L1Overview == "" {
			degraded++
		}
	}
	if degraded == 0 {
		t.Error("预算=30 应触发至少一条降级")
	}
}

// E2E-TL-03: L2 按需展开 API
func TestE2E_TL03_L2OnDemandExpand(t *testing.T) {
	fs := NewMockFSStore()
	_ = fs.WriteMemory("t1", "u1", "detail-001", MemoryTypePermanent, CategoryFact,
		"完整的 L2 原文内容：Go 语言的并发模型基于 CSP 理论",
		"Go 并发", "Go 使用 goroutine 和 channel 实现并发")

	uri := MemoryTypePermanent + "/" + CategoryFact + "/detail-001"

	// L2 读取应返回完整原文
	l2, err := fs.ReadMemory("t1", "u1", uri, 2)
	if err != nil {
		t.Fatalf("ReadMemory L2: %v", err)
	}
	if !strings.Contains(l2, "完整的 L2 原文内容") {
		t.Errorf("L2 应包含完整原文, got %q", l2)
	}
}

// E2E-TL-04: 想象记忆 L2 保护
func TestE2E_TL04_ImaginationL2Protection(t *testing.T) {
	// 验证想象记忆的分层策略为 L0Only
	tier := ClassifyMemoryTier(MemoryTypeImagination)
	if tier != TierL0Only {
		t.Fatalf("imagination 应为 TierL0Only, got %d", tier)
	}

	// 在实际系统中 GET /memories/:id/detail?level=2 应返回 403
	// 这里验证分层策略正确
	levels := availableLevelsForType(MemoryTypeImagination)
	if len(levels) != 1 || levels[0] != 0 {
		t.Errorf("imagination 可用层级应为 [0], got %v", levels)
	}
}

// E2E-TL-05: 永久记忆跳过 L0 筛选
func TestE2E_TL05_PermanentSkipL0Filter(t *testing.T) {
	tier := ClassifyMemoryTier(MemoryTypePermanent)
	if tier != TierAlwaysL1 {
		t.Fatalf("permanent 应为 TierAlwaysL1, got %d", tier)
	}

	// permanent 类型不应进入 LLM 筛选流程
	levels := availableLevelsForType(MemoryTypePermanent)
	if len(levels) != 3 {
		t.Errorf("permanent 可用层级应为 [0,1,2], got %v", levels)
	}
}

// E2E-TL-06: LLM 筛选失败回退
func TestE2E_TL06_LLMFilterFailureFallback(t *testing.T) {
	llm := NewMockLLMWithError(fmt.Errorf("API timeout"))
	tl := NewTestTieredLoader(llm, nil, TieredLoaderConfig{
		TopK: 2, LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "p/f/m1", L0Abstract: "Go"},
		{URI: "p/f/m2", L0Abstract: "Rust"},
		{URI: "p/f/m3", L0Abstract: "Python"},
	}

	uris, err := tl.FilterByL0("anything", entries)
	if err != nil {
		t.Fatalf("FilterByL0 fallback: %v", err)
	}
	// LLM 失败应回退到返回所有 URI
	if len(uris) != 3 {
		t.Errorf("LLM 失败应回退返回全部 URI, got %d", len(uris))
	}
}

// E2E-TL-07: 空搜索结果
func TestE2E_TL07_EmptySearchResults(t *testing.T) {
	tl := NewTestTieredLoader(nil, nil, TieredLoaderConfig{
		TopK: 8, LLMFilterEnabled: true,
	})

	// 空条目不应调用 BatchReadL0
	uris, err := tl.FilterByL0("query", nil)
	if err != nil {
		t.Fatalf("FilterByL0 empty: %v", err)
	}
	if uris != nil {
		t.Errorf("空搜索结果应返回 nil, got %v", uris)
	}
}

// E2E-TL-08: CJK Token 估算精度
func TestE2E_TL08_CJKTokenEstimation(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		minTok int
		maxTok int
	}{
		{"纯英文", "Hello world this is a test", 20, 30},
		{"纯中文", "这是一段中文测试内容", 16, 24},
		{"中英混合", "Go语言的goroutine并发模型", 18, 30},
		{"日文", "これはテストです", 12, 20},
		{"韩文", "이것은테스트입니다", 14, 22},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := estimateTokens(tt.input)
			if tokens < tt.minTok || tokens > tt.maxTok {
				t.Errorf("estimateTokens(%q) = %d, 期望在 [%d, %d] 范围内",
					tt.input, tokens, tt.minTok, tt.maxTok)
			}
		})
	}
}

// --- 辅助函数 ---

func availableLevelsForType(memoryType string) []int {
	tier := ClassifyMemoryTier(memoryType)
	if tier == TierL0Only {
		return []int{0}
	}
	return []int{0, 1, 2}
}
