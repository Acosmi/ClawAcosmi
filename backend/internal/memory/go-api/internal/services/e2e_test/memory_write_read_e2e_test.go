//go:build e2e
// +build e2e

// Package e2etest — E2E-WR: 记忆写入→检索全链路测试。
// 覆盖: 写入 → VFS L0/L1/L2 → 向量 upsert → 搜索 → 分层返回
package e2etest

import (
	"os"
	"strings"
	"testing"
)

// E2E-WR-01: 情景记忆写入+搜索
func TestE2E_WR01_EpisodicWriteAndSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	// 写入一条情景记忆
	mem := TestMemory{
		MemoryID: "wr01-001", MemoryType: MemoryTypeEpisodic, Category: CategoryPreference,
		Content:    "用户喜欢喝拿铁咖啡，每天早上都会去星巴克",
		L0Abstract: "咖啡偏好：拿铁", L1Overview: "用户偏好：每天早上喝星巴克拿铁",
		UserID: "user-wr01",
	}
	_ = fs.WriteMemory("t1", mem.UserID, mem.MemoryID, mem.MemoryType, mem.Category,
		mem.Content, mem.L0Abstract, mem.L1Overview)
	vs.Upsert(mem.MemoryID, mem.Content, mem.UserID, mem.MemoryType, mem.Category, 0.8)

	// 搜索"咖啡偏好"
	results := vs.Search("咖啡偏好拿铁", mem.UserID, 5, nil)
	if len(results) == 0 {
		t.Fatal("搜索\"咖啡偏好\"应返回至少 1 条结果")
	}

	// 验证搜索结果包含写入的记忆
	found := false
	for _, r := range results {
		if r.MemoryID == mem.MemoryID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("搜索结果中未找到 %s", mem.MemoryID)
	}

	// 验证 L0 摘要可读
	uri := mem.MemoryType + "/" + mem.Category + "/" + mem.MemoryID
	l0, err := fs.ReadMemory("t1", mem.UserID, uri, 0)
	if err != nil {
		t.Fatalf("读取 L0 失败: %v", err)
	}
	if l0 != mem.L0Abstract {
		t.Errorf("L0 = %q, want %q", l0, mem.L0Abstract)
	}
}

// E2E-WR-02: 观察记忆写入+搜索 (L0→L1 渐进加载)
func TestE2E_WR02_ObservationWriteAndSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	mem := TestMemory{
		MemoryID: "wr02-001", MemoryType: MemoryTypeEpisodic, Category: CategoryHabit,
		Content:    "饮品偏好：拿铁，不加糖，加燕麦奶",
		L0Abstract: "饮品偏好", L1Overview: "拿铁，不加糖，加燕麦奶",
		UserID: "user-wr02",
	}
	_ = fs.WriteMemory("t1", mem.UserID, mem.MemoryID, mem.MemoryType, mem.Category,
		mem.Content, mem.L0Abstract, mem.L1Overview)
	vs.Upsert(mem.MemoryID, mem.Content, mem.UserID, mem.MemoryType, mem.Category, 0.7)

	// 验证 L0→L1 渐进可读
	uri := mem.MemoryType + "/" + mem.Category + "/" + mem.MemoryID
	l0, _ := fs.ReadMemory("t1", mem.UserID, uri, 0)
	l1, _ := fs.ReadMemory("t1", mem.UserID, uri, 1)

	if l0 == "" {
		t.Error("L0 不应为空")
	}
	if l1 == "" {
		t.Error("L1 不应为空")
	}
	// L1 应比 L0 包含更多信息
	if len(l1) <= len(l0) {
		t.Logf("注意: L1 (%d chars) 不长于 L0 (%d chars)，可能需要检查数据质量", len(l1), len(l0))
	}

	// 验证分层策略为 Standard
	tier := ClassifyMemoryTier(mem.MemoryType)
	if tier != TierStandard {
		t.Errorf("episodic 记忆应为 TierStandard, got %d", tier)
	}
}

// E2E-WR-03: 永久记忆钉选+搜索 (跳过 L0 筛选)
func TestE2E_WR03_PermanentPinAndSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	mem := TestMemory{
		MemoryID: "wr03-001", MemoryType: MemoryTypePermanent, Category: CategoryFact,
		Content:    "用户是全栈工程师，精通 Go 和 Next.js",
		L0Abstract: "职业信息", L1Overview: "全栈工程师，精通 Go 和 Next.js",
		UserID: "user-wr03",
	}
	_ = fs.WriteMemory("t1", mem.UserID, mem.MemoryID, mem.MemoryType, mem.Category,
		mem.Content, mem.L0Abstract, mem.L1Overview)
	vs.Upsert(mem.MemoryID, mem.Content, mem.UserID, mem.MemoryType, mem.Category, 0.9)

	// 永久记忆应跳过 L0 筛选 (TierAlwaysL1)
	tier := ClassifyMemoryTier(MemoryTypePermanent)
	if tier != TierAlwaysL1 {
		t.Fatalf("permanent 记忆应为 TierAlwaysL1, got %d", tier)
	}

	// 搜索应返回结果
	results := vs.Search("工程师 Go", mem.UserID, 5, []string{MemoryTypePermanent})
	if len(results) == 0 {
		t.Fatal("永久记忆搜索应返回结果")
	}

	// L1 应直接可用
	uri := mem.MemoryType + "/" + mem.Category + "/" + mem.MemoryID
	l1, err := fs.ReadMemory("t1", mem.UserID, uri, 1)
	if err != nil {
		t.Fatalf("永久记忆 L1 读取失败: %v", err)
	}
	if l1 == "" {
		t.Error("永久记忆 L1 不应为空")
	}
}

// E2E-WR-04: 想象记忆触发+搜索 (仅返回 L0)
func TestE2E_WR04_ImaginationSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	mem := TestMemory{
		MemoryID: "wr04-001", MemoryType: MemoryTypeImagination, Category: CategoryInsight,
		Content:    "基于用户对 Rust 的关注，预测核心模块可能迁移到 Rust",
		L0Abstract: "预测：Rust 迁移", L1Overview: "核心模块可能迁移到 Rust",
		UserID: "user-wr04",
	}
	_ = fs.WriteMemory("t1", mem.UserID, mem.MemoryID, mem.MemoryType, mem.Category,
		mem.Content, mem.L0Abstract, mem.L1Overview)
	vs.Upsert(mem.MemoryID, mem.Content, mem.UserID, mem.MemoryType, mem.Category, 0.6)

	// 想象记忆策略: TierL0Only
	tier := ClassifyMemoryTier(MemoryTypeImagination)
	if tier != TierL0Only {
		t.Fatalf("imagination 记忆应为 TierL0Only, got %d", tier)
	}

	// 搜索应返回结果
	results := vs.Search("Rust 迁移", mem.UserID, 5, nil)
	if len(results) == 0 {
		t.Fatal("想象记忆搜索应返回结果")
	}

	// L0 可读，但 L2 应被保护 (在实际系统中返回 403)
	uri := mem.MemoryType + "/" + mem.Category + "/" + mem.MemoryID
	l0, err := fs.ReadMemory("t1", mem.UserID, uri, 0)
	if err != nil {
		t.Fatalf("想象记忆 L0 读取失败: %v", err)
	}
	if l0 == "" {
		t.Error("想象记忆 L0 不应为空")
	}
}

// E2E-WR-05: 反思记忆生成+搜索 (标准 L0→L1 流程)
func TestE2E_WR05_ReflectionSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	mem := TestMemory{
		MemoryID: "wr05-001", MemoryType: MemoryTypeReflection, Category: CategoryInsight,
		Content:    "用户长期关注 Rust 和 Go，倾向于系统级编程",
		L0Abstract: "编程偏好趋势", L1Overview: "系统级编程偏好：Rust + Go",
		UserID: "user-wr05",
	}
	_ = fs.WriteMemory("t1", mem.UserID, mem.MemoryID, mem.MemoryType, mem.Category,
		mem.Content, mem.L0Abstract, mem.L1Overview)
	vs.Upsert(mem.MemoryID, mem.Content, mem.UserID, mem.MemoryType, mem.Category, 0.7)

	// 反思记忆应使用标准流程
	tier := ClassifyMemoryTier(MemoryTypeReflection)
	if tier != TierStandard {
		t.Errorf("reflection 记忆应为 TierStandard, got %d", tier)
	}

	results := vs.Search("编程偏好", mem.UserID, 5, nil)
	if len(results) == 0 {
		t.Fatal("反思记忆搜索应返回结果")
	}
}

// E2E-WR-06: CJK 多字节写入+搜索 (无截断 panic)
func TestE2E_WR06_CJKWriteAndSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	// 从 testdata 读取 CJK 样本
	data, err := os.ReadFile("testdata/cjk_samples.txt")
	if err != nil {
		t.Fatalf("读取 CJK testdata: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	// 写入多条 CJK 记忆
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		memID := strings.Replace(strings.Replace("cjk-"+string(rune('a'+i)), " ", "", -1), "\t", "", -1)
		l0 := line
		if len([]rune(l0)) > 20 {
			l0 = string([]rune(line)[:20])
		}
		_ = fs.WriteMemory("t1", "user-cjk", memID, MemoryTypeEpisodic, CategoryFact,
			line, l0, line)
		vs.Upsert(memID, line, "user-cjk", MemoryTypeEpisodic, CategoryFact, 0.5)
	}

	// 搜索中文关键词
	results := vs.Search("拿铁咖啡", "user-cjk", 10, nil)
	if len(results) == 0 {
		t.Fatal("CJK 搜索应返回结果")
	}

	// 验证批量 L0 读取无 panic
	var uris []string
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		memID := strings.Replace(strings.Replace("cjk-"+string(rune('a'+i)), " ", "", -1), "\t", "", -1)
		uris = append(uris, MemoryTypeEpisodic+"/"+CategoryFact+"/"+memID)
	}
	entries, err := fs.BatchReadL0("t1", "user-cjk", uris)
	if err != nil {
		t.Fatalf("CJK BatchReadL0: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("CJK BatchReadL0 应返回条目")
	}
	// 验证 L0 摘要无截断（每条均非空）
	for _, e := range entries {
		if e.L0Abstract == "" {
			t.Errorf("CJK L0 摘要不应为空: URI=%s", e.URI)
		}
	}
}

// E2E-WR-07: 大批量写入 (50条) + 搜索
func TestE2E_WR07_BulkWriteAndSearch(t *testing.T) {
	fs := NewMockFSStore()
	vs := NewMockVectorStore()

	const count = 50
	var uris []string

	// 批量写入 50 条记忆
	for i := 0; i < count; i++ {
		memID := strings.Replace(strings.Replace("bulk-"+string(rune('A'+i%26))+string(rune('0'+i/26)), " ", "", -1), "\t", "", -1)
		content := strings.Repeat("Memory content block ", 3) + memID
		l0 := "L0-" + memID
		l1 := "L1 overview for " + memID

		memType := MemoryTypeEpisodic
		if i%5 == 0 {
			memType = MemoryTypePermanent
		}
		_ = fs.WriteMemory("t1", "user-bulk", memID, memType, CategoryFact,
			content, l0, l1)
		vs.Upsert(memID, content, "user-bulk", memType, CategoryFact, 0.5+float64(i)*0.01)
		uris = append(uris, memType+"/"+CategoryFact+"/"+memID)
	}

	// 验证 VectorStore 计数
	if vs.Count() != count {
		t.Fatalf("VectorStore count = %d, want %d", vs.Count(), count)
	}

	// 批量 L0 读取
	entries, err := fs.BatchReadL0("t1", "user-bulk", uris)
	if err != nil {
		t.Fatalf("BatchReadL0: %v", err)
	}
	if len(entries) != count {
		t.Fatalf("BatchReadL0 returned %d entries, want %d", len(entries), count)
	}

	// 搜索应返回结果
	results := vs.Search("Memory content block", "user-bulk", 10, nil)
	if len(results) == 0 {
		t.Fatal("批量写入后搜索应返回结果")
	}
	if len(results) > 10 {
		t.Errorf("搜索结果应被限制为 10 条, got %d", len(results))
	}
}
