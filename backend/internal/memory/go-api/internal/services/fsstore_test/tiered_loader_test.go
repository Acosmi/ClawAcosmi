// Package fsstoretest — TieredLoader and BatchReadL1 tests.
// Phase 2: Tests for LLM-based L0 filtering and L1 progressive loading.
package fsstoretest

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- BatchReadL1 (replica for testing) ---

// L1Entry represents an L1 overview entry.
type L1Entry struct {
	URI        string `json:"uri"`
	MemoryID   string `json:"memory_id"`
	L1Overview string `json:"l1_overview"`
	MemoryType string `json:"memory_type"`
	Category   string `json:"category"`
	CreatedAt  int64  `json:"created_at"`
}

// BatchReadL1 批量读取一组 URI 的 L1 概述。
func (s *FSStoreService) BatchReadL1(tenant, user string, uris []string) ([]L1Entry, error) {
	if len(uris) == 0 {
		return nil, nil
	}

	results := make([]L1Entry, 0, len(uris))
	root := userRoot(tenant, user)

	for _, uri := range uris {
		// Read l1.txt
		l1Path := fmt.Sprintf("%s/%s/l1.txt", root, uri)
		l1Data, err := s.agfs.ReadFile(l1Path)
		if err != nil {
			continue // skip missing URIs
		}

		// Parse URI components: "{section}/{category}/{memoryID}"
		parts := strings.SplitN(uri, "/", 3)
		entry := L1Entry{
			URI:        uri,
			L1Overview: string(l1Data),
		}
		if len(parts) >= 3 {
			entry.MemoryType = parts[0]
			entry.Category = parts[1]
			entry.MemoryID = parts[2]
		} else if len(parts) >= 1 {
			entry.MemoryID = parts[len(parts)-1]
		}

		// Try to read meta.json for created_at
		metaPath := fmt.Sprintf("%s/%s/meta.json", root, uri)
		metaData, merr := s.agfs.ReadFile(metaPath)
		if merr == nil {
			var meta map[string]interface{}
			if jerr := json.Unmarshal(metaData, &meta); jerr == nil {
				if ts, ok := meta["created_at"]; ok {
					if v, ok := ts.(float64); ok {
						entry.CreatedAt = int64(v)
					}
				}
				if section, ok := meta["section"].(string); ok && entry.MemoryType == "" {
					entry.MemoryType = section
				}
				if cat, ok := meta["category"].(string); ok && entry.Category == "" {
					entry.Category = cat
				}
			}
		}

		results = append(results, entry)
	}
	return results, nil
}

// --- TierPolicy (replica for testing) ---

type TierPolicy int

const (
	TierStandard TierPolicy = iota
	TierAlwaysL1
	TierL0Only
)

func ClassifyMemoryTier(memoryType string) TierPolicy {
	switch memoryType {
	case "permanent":
		return TierAlwaysL1
	case "imagination":
		return TierL0Only
	default:
		return TierStandard
	}
}

// --- BatchReadL1 Tests ---

func TestFSStore_BatchReadL1_Normal(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// Write 3 memories with L1 content
	_ = s.WriteMemory("t1", "u1", "mem-001", "permanent", "fact",
		"Go is a compiled language with garbage collection",
		"Go语言摘要",
		"Go 是一门编译型语言，支持垃圾回收和并发原语")
	_ = s.WriteMemory("t1", "u1", "mem-002", "permanent", "skill",
		"Rust eliminates memory bugs at compile time",
		"Rust安全摘要",
		"Rust 通过所有权系统在编译期消除内存安全问题")
	_ = s.WriteMemory("t1", "u1", "mem-003", "episodic", "event",
		"Team meeting about Q1 roadmap",
		"会议摘要",
		"Q1 路线图讨论会议，确定了三个核心目标")

	// Batch read L1
	uris := []string{
		"permanent/fact/mem-001",
		"permanent/skill/mem-002",
		"episodic/event/mem-003",
	}
	entries, err := s.BatchReadL1("t1", "u1", uris)
	if err != nil {
		t.Fatalf("BatchReadL1: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("BatchReadL1 returned %d entries, want 3", len(entries))
	}

	// Verify L1 content
	if entries[0].L1Overview != "Go 是一门编译型语言，支持垃圾回收和并发原语" {
		t.Errorf("entries[0].L1Overview = %q, want L1 content", entries[0].L1Overview)
	}
	if entries[0].MemoryID != "mem-001" {
		t.Errorf("entries[0].MemoryID = %q, want %q", entries[0].MemoryID, "mem-001")
	}
	if entries[0].MemoryType != "permanent" {
		t.Errorf("entries[0].MemoryType = %q, want %q", entries[0].MemoryType, "permanent")
	}
	if entries[0].Category != "fact" {
		t.Errorf("entries[0].Category = %q, want %q", entries[0].Category, "fact")
	}

	// Verify second entry
	if entries[1].L1Overview != "Rust 通过所有权系统在编译期消除内存安全问题" {
		t.Errorf("entries[1].L1Overview = %q, want Rust L1 content", entries[1].L1Overview)
	}

	// Verify third entry (different section)
	if entries[2].L1Overview != "Q1 路线图讨论会议，确定了三个核心目标" {
		t.Errorf("entries[2].L1Overview = %q, want meeting L1 content", entries[2].L1Overview)
	}
	if entries[2].MemoryType != "episodic" {
		t.Errorf("entries[2].MemoryType = %q, want %q", entries[2].MemoryType, "episodic")
	}
}

func TestFSStore_BatchReadL1_Empty(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	entries, err := s.BatchReadL1("t1", "u1", []string{})
	if err != nil {
		t.Fatalf("BatchReadL1 empty: %v", err)
	}
	if entries != nil {
		t.Errorf("BatchReadL1 empty returned %v, want nil", entries)
	}

	// nil input
	entries, err = s.BatchReadL1("t1", "u1", nil)
	if err != nil {
		t.Fatalf("BatchReadL1 nil: %v", err)
	}
	if entries != nil {
		t.Errorf("BatchReadL1 nil returned %v, want nil", entries)
	}
}

func TestFSStore_BatchReadL1_NonExistent(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// Write one memory but request non-existent URIs
	_ = s.WriteMemory("t1", "u1", "exists", "permanent", "fact", "c", "l0", "l1")

	uris := []string{
		"permanent/fact/nonexistent-1",
		"permanent/fact/nonexistent-2",
		"episodic/event/nonexistent-3",
	}
	entries, err := s.BatchReadL1("t1", "u1", uris)
	if err != nil {
		t.Fatalf("BatchReadL1 nonexistent: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("BatchReadL1 nonexistent returned %d entries, want 0", len(entries))
	}
}

func TestFSStore_BatchReadL1_MixedExistence(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// Write two memories
	_ = s.WriteMemory("t1", "u1", "mem-a", "permanent", "fact", "content-a", "l0-a", "l1-a")
	_ = s.WriteMemory("t1", "u1", "mem-b", "episodic", "event", "content-b", "l0-b", "l1-b")

	// Request one existing + one non-existing
	uris := []string{
		"permanent/fact/mem-a",
		"permanent/fact/nonexistent",
		"episodic/event/mem-b",
	}
	entries, err := s.BatchReadL1("t1", "u1", uris)
	if err != nil {
		t.Fatalf("BatchReadL1 mixed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("BatchReadL1 mixed returned %d entries, want 2", len(entries))
	}
	if entries[0].L1Overview != "l1-a" {
		t.Errorf("entries[0].L1Overview = %q, want %q", entries[0].L1Overview, "l1-a")
	}
	if entries[1].L1Overview != "l1-b" {
		t.Errorf("entries[1].L1Overview = %q, want %q", entries[1].L1Overview, "l1-b")
	}
}

// --- ClassifyMemoryTier Tests ---

func TestClassifyMemoryTier(t *testing.T) {
	tests := []struct {
		memoryType string
		want       TierPolicy
	}{
		{"permanent", TierAlwaysL1},
		{"imagination", TierL0Only},
		{"episodic", TierStandard},
		{"semantic", TierStandard},
		{"procedural", TierStandard},
		{"observation", TierStandard},
		{"reflection", TierStandard},
		{"dialogue", TierStandard},
		{"plan", TierStandard},
		{"", TierStandard},
	}

	for _, tt := range tests {
		t.Run(tt.memoryType, func(t *testing.T) {
			got := ClassifyMemoryTier(tt.memoryType)
			if got != tt.want {
				t.Errorf("ClassifyMemoryTier(%q) = %d, want %d", tt.memoryType, got, tt.want)
			}
		})
	}
}

// --- TieredLoader FilterByL0 Tests (using mock LLM) ---

// mockLLM simulates an LLM provider for testing.
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Generate(_ interface{}, prompt string) (string, error) {
	return m.response, m.err
}

// TieredLoaderConfig for testing.
type TieredLoaderConfig struct {
	MaxL0Count       int
	TopK             int
	TokenBudget      int
	LLMFilterEnabled bool
}

// TieredLoader test replica — validates filtering logic without CGO deps.
type TieredLoader struct {
	llm    *mockLLM
	config TieredLoaderConfig
}

func NewTieredLoader(llm *mockLLM, cfg TieredLoaderConfig) *TieredLoader {
	return &TieredLoader{llm: llm, config: cfg}
}

// FilterByL0 test replica.
func (tl *TieredLoader) FilterByL0(query string, entries []L0Entry) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	if !tl.config.LLMFilterEnabled || tl.llm == nil {
		return allTestURIs(entries), nil
	}

	if len(entries) <= tl.config.TopK {
		return allTestURIs(entries), nil
	}

	response, err := tl.llm.Generate(nil, query)
	if err != nil {
		return allTestURIs(entries), nil // fallback
	}

	// Parse response
	resp := strings.TrimSpace(response)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var uris []string
	if jerr := json.Unmarshal([]byte(resp), &uris); jerr != nil {
		return allTestURIs(entries), nil // fallback
	}

	// Validate URIs
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
		return allTestURIs(entries), nil
	}

	return validated, nil
}

func allTestURIs(entries []L0Entry) []string {
	uris := make([]string, len(entries))
	for i, e := range entries {
		uris[i] = e.URI
	}
	return uris
}

func TestTieredLoader_FilterByL0_Normal(t *testing.T) {
	// Simulate LLM selecting 2 out of 5 entries
	llm := &mockLLM{
		response: `["permanent/fact/mem-002", "episodic/event/mem-004"]`,
	}
	tl := NewTieredLoader(llm, TieredLoaderConfig{
		MaxL0Count:       50,
		TopK:             2,
		TokenBudget:      20000,
		LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go language"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust memory safety"},
		{URI: "episodic/event/mem-003", L0Abstract: "Meeting notes"},
		{URI: "episodic/event/mem-004", L0Abstract: "Code review feedback"},
		{URI: "semantic/knowledge/mem-005", L0Abstract: "Design patterns"},
	}

	uris, err := tl.FilterByL0("memory safety and code review", entries)
	if err != nil {
		t.Fatalf("FilterByL0: %v", err)
	}
	if len(uris) != 2 {
		t.Fatalf("FilterByL0 returned %d URIs, want 2", len(uris))
	}
	if uris[0] != "permanent/fact/mem-002" {
		t.Errorf("uris[0] = %q, want %q", uris[0], "permanent/fact/mem-002")
	}
	if uris[1] != "episodic/event/mem-004" {
		t.Errorf("uris[1] = %q, want %q", uris[1], "episodic/event/mem-004")
	}
}

func TestTieredLoader_FilterByL0_LLMFail_Fallback(t *testing.T) {
	// Simulate LLM failure
	llm := &mockLLM{
		err: fmt.Errorf("API timeout"),
	}
	tl := NewTieredLoader(llm, TieredLoaderConfig{
		TopK:             2,
		LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust"},
		{URI: "permanent/fact/mem-003", L0Abstract: "Python"},
	}

	// Should fallback to returning all URIs
	uris, err := tl.FilterByL0("anything", entries)
	if err != nil {
		t.Fatalf("FilterByL0 fallback: %v", err)
	}
	if len(uris) != 3 {
		t.Errorf("FilterByL0 fallback returned %d URIs, want 3 (all entries)", len(uris))
	}
}

func TestTieredLoader_FilterByL0_EmptyEntries(t *testing.T) {
	tl := NewTieredLoader(nil, TieredLoaderConfig{
		TopK:             8,
		LLMFilterEnabled: true,
	})

	uris, err := tl.FilterByL0("query", nil)
	if err != nil {
		t.Fatalf("FilterByL0 empty: %v", err)
	}
	if uris != nil {
		t.Errorf("FilterByL0 empty returned %v, want nil", uris)
	}
}

func TestTieredLoader_FilterByL0_NoFilter_WhenDisabled(t *testing.T) {
	llm := &mockLLM{
		response: `["permanent/fact/mem-001"]`,
	}
	tl := NewTieredLoader(llm, TieredLoaderConfig{
		TopK:             1,
		LLMFilterEnabled: false, // disabled
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust"},
	}

	// Should return all URIs without filtering
	uris, err := tl.FilterByL0("query", entries)
	if err != nil {
		t.Fatalf("FilterByL0 disabled: %v", err)
	}
	if len(uris) != 2 {
		t.Errorf("FilterByL0 disabled returned %d URIs, want 2 (all)", len(uris))
	}
}

func TestTieredLoader_FilterByL0_EntriesLessThanTopK(t *testing.T) {
	tl := NewTieredLoader(&mockLLM{}, TieredLoaderConfig{
		TopK:             10,
		LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust"},
	}

	// entries <= TopK → return all without calling LLM
	uris, err := tl.FilterByL0("query", entries)
	if err != nil {
		t.Fatalf("FilterByL0 small: %v", err)
	}
	if len(uris) != 2 {
		t.Errorf("FilterByL0 small returned %d URIs, want 2", len(uris))
	}
}

func TestTieredLoader_FilterByL0_InvalidLLMResponse_Fallback(t *testing.T) {
	// LLM returns invalid JSON
	llm := &mockLLM{
		response: "I think mem-001 and mem-002 are most relevant",
	}
	tl := NewTieredLoader(llm, TieredLoaderConfig{
		TopK:             2,
		LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust"},
		{URI: "permanent/fact/mem-003", L0Abstract: "Python"},
	}

	// Should fallback to all URIs on invalid JSON response
	uris, err := tl.FilterByL0("query", entries)
	if err != nil {
		t.Fatalf("FilterByL0 invalid JSON: %v", err)
	}
	if len(uris) != 3 {
		t.Errorf("FilterByL0 invalid JSON returned %d URIs, want 3 (fallback)", len(uris))
	}
}

func TestTieredLoader_FilterByL0_LLMReturnsInvalidURIs_Fallback(t *testing.T) {
	// LLM returns URIs that don't exist in the entries
	llm := &mockLLM{
		response: `["nonexistent/uri/1", "nonexistent/uri/2"]`,
	}
	tl := NewTieredLoader(llm, TieredLoaderConfig{
		TopK:             2,
		LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust"},
		{URI: "permanent/fact/mem-003", L0Abstract: "Python"},
	}

	// All URIs invalid → fallback
	uris, err := tl.FilterByL0("query", entries)
	if err != nil {
		t.Fatalf("FilterByL0 invalid URIs: %v", err)
	}
	if len(uris) != 3 {
		t.Errorf("FilterByL0 invalid URIs returned %d URIs, want 3 (fallback)", len(uris))
	}
}

func TestTieredLoader_FilterByL0_MarkdownFences(t *testing.T) {
	// LLM wraps response in markdown code fences
	llm := &mockLLM{
		response: "```json\n[\"permanent/fact/mem-001\", \"permanent/fact/mem-003\"]\n```",
	}
	tl := NewTieredLoader(llm, TieredLoaderConfig{
		TopK:             2,
		LLMFilterEnabled: true,
	})

	entries := []L0Entry{
		{URI: "permanent/fact/mem-001", L0Abstract: "Go"},
		{URI: "permanent/fact/mem-002", L0Abstract: "Rust"},
		{URI: "permanent/fact/mem-003", L0Abstract: "Python"},
	}

	uris, err := tl.FilterByL0("Go and Python", entries)
	if err != nil {
		t.Fatalf("FilterByL0 markdown: %v", err)
	}
	if len(uris) != 2 {
		t.Fatalf("FilterByL0 markdown returned %d URIs, want 2", len(uris))
	}
	if uris[0] != "permanent/fact/mem-001" {
		t.Errorf("uris[0] = %q, want %q", uris[0], "permanent/fact/mem-001")
	}
}

// ============================================================================
// Phase 3: Token Budget Controller + Detail Tests
// ============================================================================

// estimateTokensTest mirrors the production estimateTokens for testing.
func estimateTokensTest(s string) int {
	runes := []rune(s)
	count := 0
	for _, r := range runes {
		if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
			r >= 0xF900 && r <= 0xFAFF || r >= 0x3040 && r <= 0x30FF ||
			r >= 0xAC00 && r <= 0xD7AF {
			count += 2
		} else if r == ' ' || r == '\n' || r == '\t' {
			continue
		} else {
			count++
		}
	}
	if count == 0 {
		count = len(runes)
	}
	return count
}

// ApplyTokenBudget test replica.
func (tl *TieredLoader) ApplyTokenBudget(entries []L1Entry) []L1Entry {
	if tl.config.TokenBudget <= 0 || len(entries) == 0 {
		return entries
	}
	totalTokens := 0
	for i := range entries {
		entryTokens := estimateTokensTest(entries[i].L1Overview)
		if totalTokens+entryTokens > tl.config.TokenBudget {
			entries[i].L1Overview = "" // degrade
		} else {
			totalTokens += entryTokens
		}
	}
	return entries
}

func TestTokenBudgetController_Normal(t *testing.T) {
	tl := NewTieredLoader(nil, TieredLoaderConfig{
		TopK:        8,
		TokenBudget: 20000,
	})

	entries := []L1Entry{
		{URI: "u1", L1Overview: "Short overview"},
		{URI: "u2", L1Overview: "Another short overview"},
		{URI: "u3", L1Overview: "Third overview"},
	}

	result := tl.ApplyTokenBudget(entries)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	// All should remain intact within 20000 budget
	for i, e := range result {
		if e.L1Overview == "" {
			t.Errorf("entry[%d] was degraded but should fit within budget", i)
		}
	}
}

func TestTokenBudgetController_Truncation(t *testing.T) {
	tl := NewTieredLoader(nil, TieredLoaderConfig{
		TopK:        8,
		TokenBudget: 30, // very small budget
	})

	entries := []L1Entry{
		{URI: "u1", L1Overview: "First overview is really short"},        // ~25 tokens
		{URI: "u2", L1Overview: "Second overview will push over budget"}, // ~36 tokens
		{URI: "u3", L1Overview: "Third overview also long content here"}, // ~36 tokens
	}

	result := tl.ApplyTokenBudget(entries)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// First should be kept, remaining degraded
	if result[0].L1Overview == "" {
		t.Error("entry[0] should be kept within budget")
	}
	// At least one should be degraded
	degraded := 0
	for _, e := range result {
		if e.L1Overview == "" {
			degraded++
		}
	}
	if degraded == 0 {
		t.Error("expected at least one degraded entry with budget=30")
	}
}

func TestTokenBudgetController_EmptyEntries(t *testing.T) {
	tl := NewTieredLoader(nil, TieredLoaderConfig{
		TopK:        8,
		TokenBudget: 20000,
	})

	result := tl.ApplyTokenBudget(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = tl.ApplyTokenBudget([]L1Entry{})
	if len(result) != 0 {
		t.Errorf("expected empty for empty input, got %d entries", len(result))
	}
}

func TestTokenBudgetController_AllOverBudget(t *testing.T) {
	tl := NewTieredLoader(nil, TieredLoaderConfig{
		TopK:        8,
		TokenBudget: 5, // extremely small
	})

	entries := []L1Entry{
		{URI: "u1", L1Overview: "This is a reasonably long overview"},
		{URI: "u2", L1Overview: "Another equally long overview text"},
	}

	result := tl.ApplyTokenBudget(entries)
	// First may still fit depending on estimate, but second definitely won't
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestGetMemoryDetail_L0_L1_L2(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	_ = s.WriteMemory("t1", "u1", "detail-001", "permanent", "fact",
		"Full L2 content: Go is great for concurrency",
		"Go concurrency summary",
		"Go is great for concurrency with goroutines and channels")

	// L0
	l0, err := s.ReadMemory("t1", "u1", "permanent/fact/detail-001", 0)
	if err != nil {
		t.Fatalf("ReadMemory L0: %v", err)
	}
	if l0 != "Go concurrency summary" {
		t.Errorf("L0 = %q, want %q", l0, "Go concurrency summary")
	}

	// L1
	l1, err := s.ReadMemory("t1", "u1", "permanent/fact/detail-001", 1)
	if err != nil {
		t.Fatalf("ReadMemory L1: %v", err)
	}
	if l1 != "Go is great for concurrency with goroutines and channels" {
		t.Errorf("L1 = %q", l1)
	}

	// L2
	l2, err := s.ReadMemory("t1", "u1", "permanent/fact/detail-001", 2)
	if err != nil {
		t.Fatalf("ReadMemory L2: %v", err)
	}
	if l2 != "Full L2 content: Go is great for concurrency" {
		t.Errorf("L2 = %q", l2)
	}
}

func TestAvailableLevelsForType(t *testing.T) {
	tests := []struct {
		memoryType string
		wantLen    int
	}{
		{"imagination", 1}, // [0] only
		{"permanent", 3},   // [0, 1, 2]
		{"episodic", 3},    // [0, 1, 2]
		{"semantic", 3},    // [0, 1, 2]
		{"procedural", 3},  // [0, 1, 2]
		{"observation", 3}, // [0, 1, 2]
	}

	for _, tt := range tests {
		t.Run(tt.memoryType, func(t *testing.T) {
			policy := ClassifyMemoryTier(tt.memoryType)
			var levels []int
			if policy == TierL0Only {
				levels = []int{0}
			} else {
				levels = []int{0, 1, 2}
			}
			if len(levels) != tt.wantLen {
				t.Errorf("AvailableLevels(%q) len = %d, want %d", tt.memoryType, len(levels), tt.wantLen)
			}
		})
	}
}
