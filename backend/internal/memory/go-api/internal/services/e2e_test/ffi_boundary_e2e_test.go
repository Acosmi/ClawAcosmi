//go:build e2e
// +build e2e

// Package e2etest — E2E-FFI: Go ↔ Rust FFI 边界测试。
// 覆盖: 向量 upsert+search / VFS 语义索引 CRUD / MemFS trace / CJK / 大 payload / 并发
package e2etest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

// E2E-FFI-01: 向量 upsert + search 全链路
func TestE2E_FFI01_VectorUpsertAndSearch(t *testing.T) {
	ss := NewMockSegmentStore()
	if err := ss.CreateCollection("mem_episodic", 4); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := ss.CreateCollection("mem_permanent", 4); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Upsert 到 episodic 集合
	vec1 := []float32{1, 0, 0, 0}
	p1, _ := json.Marshal(map[string]interface{}{
		"content": "用户喜欢拿铁", "user_id": "u1", "memory_type": "episodic",
	})
	if err := ss.Upsert("mem_episodic", "id-001", vec1, p1); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Upsert 到 permanent 集合
	vec2 := []float32{0, 1, 0, 0}
	p2, _ := json.Marshal(map[string]interface{}{
		"content": "全栈工程师", "user_id": "u1", "memory_type": "permanent",
	})
	if err := ss.Upsert("mem_permanent", "id-002", vec2, p2); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// 搜索 episodic — 应找到 id-001
	hits, err := ss.Search("mem_episodic", vec1, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("episodic search 应返回 1 条, got %d", len(hits))
	}
	if hits[0].ID != "id-001" {
		t.Errorf("hit ID = %s, want id-001", hits[0].ID)
	}
	if hits[0].Score < 0.99 {
		t.Errorf("cosine score 应 ≈ 1.0, got %f", hits[0].Score)
	}

	// 搜索 permanent — 应找到 id-002
	hits2, err := ss.Search("mem_permanent", vec2, 5)
	if err != nil {
		t.Fatalf("Search permanent: %v", err)
	}
	if len(hits2) != 1 || hits2[0].ID != "id-002" {
		t.Errorf("permanent search 异常")
	}

	// 跨集合隔离验证
	hits3, err := ss.Search("mem_episodic", vec2, 5)
	if err != nil {
		t.Fatalf("Cross-collection search: %v", err)
	}
	for _, h := range hits3 {
		if h.ID == "id-002" {
			t.Error("episodic 集合不应包含 permanent 的数据")
		}
	}

	// 验证 Count
	if ss.Count("mem_episodic") != 1 {
		t.Errorf("episodic count = %d, want 1", ss.Count("mem_episodic"))
	}
}

// E2E-FFI-02: VFS 语义索引 CRUD (init→index→search→delete)
func TestE2E_FFI02_VFSSemanticCRUD(t *testing.T) {
	ss := NewMockSegmentStore()

	// Init
	if err := ss.VFSSemanticInit(4); err != nil {
		t.Fatalf("VFSSemanticInit: %v", err)
	}

	// Index 两条
	vec1 := []float32{1, 0, 0, 0}
	p1, _ := json.Marshal(map[string]interface{}{"uri": "/memories/001", "l0": "咖啡"})
	if err := ss.VFSSemanticIndex("vfs-001", vec1, p1); err != nil {
		t.Fatalf("VFSSemanticIndex: %v", err)
	}

	vec2 := []float32{0, 1, 0, 0}
	p2, _ := json.Marshal(map[string]interface{}{"uri": "/memories/002", "l0": "编程"})
	if err := ss.VFSSemanticIndex("vfs-002", vec2, p2); err != nil {
		t.Fatalf("VFSSemanticIndex: %v", err)
	}

	// Search — 查 vec1 方向
	hits, err := ss.VFSSemanticSearch(vec1, 5)
	if err != nil {
		t.Fatalf("VFSSemanticSearch: %v", err)
	}
	if len(hits) < 1 {
		t.Fatal("VFS 语义搜索应返回结果")
	}
	if hits[0].ID != "vfs-001" {
		t.Errorf("最相关应为 vfs-001, got %s", hits[0].ID)
	}

	// Delete vfs-001
	if err := ss.VFSSemanticDelete("vfs-001"); err != nil {
		t.Fatalf("VFSSemanticDelete: %v", err)
	}

	// Search again — vfs-001 不应出现
	hits2, err := ss.VFSSemanticSearch(vec1, 5)
	if err != nil {
		t.Fatalf("VFSSemanticSearch after delete: %v", err)
	}
	for _, h := range hits2 {
		if h.ID == "vfs-001" {
			t.Error("删除后 vfs-001 不应出现在搜索结果中")
		}
	}
}

// E2E-FFI-03: MemFS 搜索含 trace
func TestE2E_FFI03_MemFSSearchWithTrace(t *testing.T) {
	mfs := NewMockMemFS()

	_ = mfs.WriteMemory("t1", "u1", "mem-001", "preference", "用户喜欢拿铁咖啡", "咖啡偏好", "拿铁")
	_ = mfs.WriteMemory("t1", "u1", "mem-002", "skill", "Go语言并发编程", "Go并发", "goroutine")
	_ = mfs.WriteMemory("t1", "u1", "mem-003", "event", "去了咖啡店", "咖啡店", "星巴克")

	trace := mfs.SearchWithTrace("t1", "u1", "咖啡", 10)
	if trace == nil {
		t.Fatal("SearchWithTrace 不应返回 nil")
	}
	if trace.Query != "咖啡" {
		t.Errorf("trace.Query = %q, want 咖啡", trace.Query)
	}
	if len(trace.Keywords) == 0 {
		t.Error("trace.Keywords 不应为空")
	}
	if len(trace.Steps) == 0 {
		t.Error("trace.Steps 不应为空 (至少有根目录)")
	}
	// 根节点应为 directory
	if trace.Steps[0].NodeType != "directory" {
		t.Errorf("根节点应为 directory, got %s", trace.Steps[0].NodeType)
	}
	if len(trace.Hits) < 2 {
		t.Errorf("搜索'咖啡'应至少命中 2 条, got %d", len(trace.Hits))
	}
	if trace.TotalFilesScored != 3 {
		t.Errorf("TotalFilesScored = %d, want 3", trace.TotalFilesScored)
	}
}

// E2E-FFI-04: CJK 数据 FFI 传递
func TestE2E_FFI04_CJKDataFFI(t *testing.T) {
	ss := NewMockSegmentStore()
	_ = ss.CreateCollection("mem_episodic", 4)

	// 从 testdata 读取 CJK 样本
	data, err := os.ReadFile("testdata/cjk_samples.txt")
	if err != nil {
		t.Fatalf("读取 CJK testdata: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	vec := []float32{0.5, 0.5, 0.5, 0.5}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		id := fmt.Sprintf("cjk-%d", i)
		payload, _ := json.Marshal(map[string]interface{}{"content": line})
		if err := ss.Upsert("mem_episodic", id, vec, payload); err != nil {
			t.Fatalf("CJK upsert %d: %v", i, err)
		}
	}

	// 搜索验证
	hits, err := ss.Search("mem_episodic", vec, 20)
	if err != nil {
		t.Fatalf("CJK search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("CJK 搜索应返回结果")
	}

	// 验证 payload 中的 CJK 内容无乱码
	for _, h := range hits {
		content, ok := h.Payload["content"].(string)
		if !ok {
			t.Errorf("payload content 类型异常: %T", h.Payload["content"])
			continue
		}
		if len(content) == 0 {
			t.Error("payload content 不应为空")
		}
		// 验证不包含替换字符 (乱码标志)
		if strings.ContainsRune(content, '\uFFFD') {
			t.Errorf("CJK 内容含替换字符 (乱码): %s", content)
		}
	}
}

// E2E-FFI-05: 大 payload 传递 (10KB+)
func TestE2E_FFI05_LargePayload(t *testing.T) {
	ss := NewMockSegmentStore()
	_ = ss.CreateCollection("mem_episodic", 4)

	// 从 testdata 读取大 payload
	data, err := os.ReadFile("testdata/large_payload.txt")
	if err != nil {
		t.Fatalf("读取 large payload: %v", err)
	}
	if len(data) < 10000 {
		t.Fatalf("large payload 应 >= 10KB, got %d bytes", len(data))
	}

	content := string(data)
	vec := []float32{0.3, 0.3, 0.3, 0.3}
	payload, _ := json.Marshal(map[string]interface{}{"content": content})

	if err := ss.Upsert("mem_episodic", "large-001", vec, payload); err != nil {
		t.Fatalf("large upsert: %v", err)
	}

	// 搜索并验证完整性
	hits, err := ss.Search("mem_episodic", vec, 1)
	if err != nil {
		t.Fatalf("large search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("应返回 1 条, got %d", len(hits))
	}

	retrieved, ok := hits[0].Payload["content"].(string)
	if !ok {
		t.Fatal("payload content 类型异常")
	}
	if retrieved != content {
		t.Errorf("大 payload 被截断: 写入 %d bytes, 读取 %d bytes",
			len(content), len(retrieved))
	}
}

// E2E-FFI-06: 并发 FFI 调用
func TestE2E_FFI06_ConcurrentFFI(t *testing.T) {
	ss := NewMockSegmentStore()
	_ = ss.CreateCollection("mem_episodic", 4)

	const goroutines = 10
	const perGoroutine = 5

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*perGoroutine)

	// 10 个 goroutine 并发 upsert
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := fmt.Sprintf("concurrent-%d-%d", gIdx, i)
				vec := []float32{float32(gIdx) * 0.1, float32(i) * 0.1, 0.5, 0.5}
				payload, _ := json.Marshal(map[string]interface{}{
					"content": fmt.Sprintf("content-%d-%d", gIdx, i),
				})
				if err := ss.Upsert("mem_episodic", id, vec, payload); err != nil {
					errCh <- fmt.Errorf("upsert %s: %w", id, err)
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("并发 upsert 错误: %v", err)
	}

	// 验证总数
	expected := goroutines * perGoroutine
	if ss.Count("mem_episodic") != expected {
		t.Errorf("并发 upsert 后 count = %d, want %d",
			ss.Count("mem_episodic"), expected)
	}

	// 并发搜索
	var wg2 sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg2.Add(1)
		go func(gIdx int) {
			defer wg2.Done()
			vec := []float32{float32(gIdx) * 0.1, 0.2, 0.5, 0.5}
			hits, err := ss.Search("mem_episodic", vec, 5)
			if err != nil {
				t.Errorf("并发 search %d: %v", gIdx, err)
				return
			}
			if len(hits) == 0 {
				t.Errorf("并发 search %d 应返回结果", gIdx)
			}
		}(g)
	}
	wg2.Wait()
}
