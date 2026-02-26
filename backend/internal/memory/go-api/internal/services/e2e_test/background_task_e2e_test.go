//go:build e2e
// +build e2e

// Package e2etest — E2E-BG: 后台任务与前台读写并发安全测试。
// 覆盖: 衰减批处理 / 压缩合并 / 永久记忆保护 / 归档后排除 / CommitSession 异步
package e2etest

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// 衰减算法副本 — 镜像 services.ComputeEffectiveImportance
// ============================================================================

const (
	halfLifeDays           = 30.0
	minDecayFactor         = 0.01
	consolidationThreshold = 0.15
	archiveThreshold       = 0.05
	cLn2                   = 0.693147180559945
)

// computeEffectiveImportance 镜像生产代码
func computeEffectiveImportance(
	baseImportance, decayFactor float64,
	lastAccessedAt *time.Time,
	accessCount int,
	now time.Time,
) float64 {
	daysSinceAccess := 0.0
	if lastAccessedAt != nil {
		daysSinceAccess = now.Sub(*lastAccessedAt).Seconds() / 86400.0
	}
	if daysSinceAccess < 0 {
		daysSinceAccess = 0
	}
	timeDecay := math.Exp(-cLn2 * daysSinceAccess / halfLifeDays)
	recencyBoost := 1.0 + math.Log1p(float64(accessCount))*0.1
	effective := baseImportance * decayFactor * timeDecay * recencyBoost
	return math.Max(minDecayFactor, math.Min(1.0, effective))
}

// ============================================================================
// Mock 记忆模型 (简化版 models.Memory)
// ============================================================================

type mockMemory struct {
	ID             string
	Content        string
	UserID         string
	MemoryType     string
	Category       string
	Importance     float64
	DecayFactor    float64
	AccessCount    int
	LastAccessedAt *time.Time
	CreatedAt      time.Time
}

// mockMemoryDB 线程安全的内存数据库
type mockMemoryDB struct {
	mu       sync.RWMutex
	memories map[string]*mockMemory
}

func newMockMemoryDB() *mockMemoryDB {
	return &mockMemoryDB{memories: make(map[string]*mockMemory)}
}

func (db *mockMemoryDB) Insert(m *mockMemory) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.memories[m.ID] = m
}

func (db *mockMemoryDB) Get(id string) (*mockMemory, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	m, ok := db.memories[id]
	return m, ok
}

func (db *mockMemoryDB) FindByUser(userID string) []*mockMemory {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var result []*mockMemory
	for _, m := range db.memories {
		if m.UserID == userID {
			result = append(result, m)
		}
	}
	return result
}

func (db *mockMemoryDB) Search(userID, query string, excludeTypes []string) []*mockMemory {
	db.mu.RLock()
	defer db.mu.RUnlock()
	exclude := make(map[string]bool)
	for _, t := range excludeTypes {
		exclude[t] = true
	}
	var result []*mockMemory
	for _, m := range db.memories {
		if m.UserID == userID && !exclude[m.MemoryType] {
			result = append(result, m)
		}
	}
	return result
}

// applyDecayBatch 镜像 services.ApplyDecayBatch (不依赖 DB)
func applyDecayBatch(db *mockMemoryDB, userID string) int {
	db.mu.Lock()
	defer db.mu.Unlock()
	now := time.Now().UTC()
	updated := 0

	protected := map[string]bool{
		MemoryTypePermanent:   true,
		MemoryTypeImagination: true,
	}

	for _, m := range db.memories {
		if m.UserID != userID || protected[m.MemoryType] || m.DecayFactor <= minDecayFactor {
			continue
		}
		var days float64
		if m.LastAccessedAt != nil {
			days = now.Sub(*m.LastAccessedAt).Seconds() / 86400.0
		} else {
			days = now.Sub(m.CreatedAt).Seconds() / 86400.0
		}
		if days < 0 {
			days = 0
		}
		newDecay := m.DecayFactor * math.Exp(-cLn2*days/halfLifeDays)
		newDecay = math.Max(minDecayFactor, newDecay)
		if math.Abs(newDecay-m.DecayFactor) > 0.001 {
			m.DecayFactor = newDecay
			updated++
		}
	}
	return updated
}

// ============================================================================
// E2E-BG 测试用例
// ============================================================================

// E2E-BG-01: 衰减批处理 + 并发搜索不阻塞
func TestE2E_BG01_DecayBatchConcurrentSearch(t *testing.T) {
	db := newMockMemoryDB()
	vs := NewMockVectorStore()

	// 写入 20 条记忆
	past := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 20; i++ {
		m := &mockMemory{
			ID: fmt.Sprintf("bg01-%03d", i), UserID: "user-bg01",
			MemoryType: MemoryTypeEpisodic, Category: CategoryFact,
			Content:    fmt.Sprintf("memory content %d", i),
			Importance: 0.7, DecayFactor: 0.8,
			LastAccessedAt: &past, CreatedAt: past,
		}
		db.Insert(m)
		vs.Upsert(m.ID, m.Content, m.UserID, m.MemoryType, m.Category, m.Importance)
	}

	var wg sync.WaitGroup
	searchErrors := make(chan error, 100)

	// 后台：衰减批处理
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			applyDecayBatch(db, "user-bg01")
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// 前台：并发搜索
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				results := vs.Search("memory content", "user-bg01", 5, nil)
				if len(results) == 0 {
					searchErrors <- fmt.Errorf("goroutine %d iter %d: 搜索无结果", idx, i)
				}
			}
		}(g)
	}

	wg.Wait()
	close(searchErrors)

	for err := range searchErrors {
		t.Errorf("并发搜索错误: %v", err)
	}
}

// E2E-BG-02: 压缩合并 + 并发写入不丢数据
func TestE2E_BG02_MergeConcurrentWrite(t *testing.T) {
	db := newMockMemoryDB()

	// 预写入 10 条低衰减候选
	for i := 0; i < 10; i++ {
		past := time.Now().Add(-48 * time.Hour)
		db.Insert(&mockMemory{
			ID: fmt.Sprintf("old-%03d", i), UserID: "user-bg02",
			MemoryType: MemoryTypeEpisodic, Category: CategoryFact,
			Content:    fmt.Sprintf("old fact %d", i),
			Importance: 0.3, DecayFactor: 0.1,
			LastAccessedAt: &past, CreatedAt: past,
		})
	}

	var wg sync.WaitGroup
	newWriteCount := 20

	// 后台：模拟压缩合并 (将 old-* 标记为 archived)
	wg.Add(1)
	go func() {
		defer wg.Done()
		db.mu.Lock()
		for _, m := range db.memories {
			if m.UserID == "user-bg02" && m.DecayFactor < consolidationThreshold {
				m.MemoryType = "archived"
			}
		}
		db.mu.Unlock()
	}()

	// 前台：并发写入新记忆
	for i := 0; i < newWriteCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			now := time.Now()
			db.Insert(&mockMemory{
				ID: fmt.Sprintf("new-%03d", idx), UserID: "user-bg02",
				MemoryType: MemoryTypeEpisodic, Category: CategoryEvent,
				Content:    fmt.Sprintf("new event %d", idx),
				Importance: 0.8, DecayFactor: 1.0,
				LastAccessedAt: &now, CreatedAt: now,
			})
		}(i)
	}

	wg.Wait()

	// 验证新写入的不丢
	newFound := 0
	for i := 0; i < newWriteCount; i++ {
		id := fmt.Sprintf("new-%03d", i)
		if _, ok := db.Get(id); ok {
			newFound++
		}
	}
	if newFound != newWriteCount {
		t.Errorf("新写入丢失: found %d, want %d", newFound, newWriteCount)
	}
}

// E2E-BG-03: 永久记忆保护 (衰减跳过 permanent + imagination)
func TestE2E_BG03_PermanentMemoryProtection(t *testing.T) {
	db := newMockMemoryDB()

	past := time.Now().Add(-60 * 24 * time.Hour) // 60 天前

	// permanent 记忆
	db.Insert(&mockMemory{
		ID: "perm-001", UserID: "user-bg03",
		MemoryType: MemoryTypePermanent, Category: CategoryFact,
		Content: "核心事实", Importance: 0.9, DecayFactor: 0.8,
		LastAccessedAt: &past, CreatedAt: past,
	})
	// imagination 记忆
	db.Insert(&mockMemory{
		ID: "imag-001", UserID: "user-bg03",
		MemoryType: MemoryTypeImagination, Category: CategoryInsight,
		Content: "AI 预测", Importance: 0.6, DecayFactor: 0.7,
		LastAccessedAt: &past, CreatedAt: past,
	})
	// 普通 episodic (应该被衰减)
	db.Insert(&mockMemory{
		ID: "ep-001", UserID: "user-bg03",
		MemoryType: MemoryTypeEpisodic, Category: CategoryEvent,
		Content: "普通事件", Importance: 0.5, DecayFactor: 0.8,
		LastAccessedAt: &past, CreatedAt: past,
	})

	// 执行衰减
	updated := applyDecayBatch(db, "user-bg03")

	// permanent 不应被衰减
	perm, _ := db.Get("perm-001")
	if perm.DecayFactor != 0.8 {
		t.Errorf("permanent DecayFactor 被修改: %f, want 0.8", perm.DecayFactor)
	}

	// imagination 不应被衰减
	imag, _ := db.Get("imag-001")
	if imag.DecayFactor != 0.7 {
		t.Errorf("imagination DecayFactor 被修改: %f, want 0.7", imag.DecayFactor)
	}

	// episodic 应该被衰减
	ep, _ := db.Get("ep-001")
	if ep.DecayFactor >= 0.8 {
		t.Errorf("episodic 未被衰减: %f", ep.DecayFactor)
	}

	if updated != 1 {
		t.Errorf("应只更新 1 条 (episodic), got %d", updated)
	}
}

// E2E-BG-04: 归档后搜索排除
func TestE2E_BG04_ArchivedExcludedFromSearch(t *testing.T) {
	db := newMockMemoryDB()

	// 活跃记忆
	db.Insert(&mockMemory{
		ID: "active-001", UserID: "user-bg04",
		MemoryType: MemoryTypeEpisodic, Content: "活跃记忆",
	})
	// 归档记忆
	db.Insert(&mockMemory{
		ID: "archived-001", UserID: "user-bg04",
		MemoryType: "archived", Content: "已归档",
	})
	// permanent 记忆
	db.Insert(&mockMemory{
		ID: "perm-001", UserID: "user-bg04",
		MemoryType: MemoryTypePermanent, Content: "永久记忆",
	})

	// 搜索排除 archived
	results := db.Search("user-bg04", "", []string{"archived"})
	for _, r := range results {
		if r.MemoryType == "archived" {
			t.Errorf("搜索结果中不应包含 archived 记忆: %s", r.ID)
		}
	}
	if len(results) != 2 {
		t.Errorf("搜索应返回 2 条 (active + permanent), got %d", len(results))
	}
}

// E2E-BG-05: CommitSession 异步安全
func TestE2E_BG05_CommitSessionAsync(t *testing.T) {
	db := newMockMemoryDB()
	fs := NewMockFSStore()
	llm := NewMockLLM("摘要：用户讨论了咖啡偏好")

	const sessions = 5
	var wg sync.WaitGroup

	// 模拟 5 个并发会话提交
	for s := 0; s < sessions; s++ {
		wg.Add(1)
		go func(sessionIdx int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%03d", sessionIdx)
			userID := "user-bg05"

			// 模拟 LLM 提取
			_, _ = llm.Generate(nil, "extract memories")

			// 写入归档摘要到 VFS
			_ = fs.WriteMemory("t1", userID, sessionID,
				"session_archives", "summary",
				fmt.Sprintf("session %d content", sessionIdx),
				fmt.Sprintf("摘要-%d", sessionIdx),
				fmt.Sprintf("详细摘要-%d", sessionIdx),
			)

			// 写入提取的记忆到 DB
			now := time.Now()
			db.Insert(&mockMemory{
				ID: fmt.Sprintf("commit-%d", sessionIdx), UserID: userID,
				MemoryType: MemoryTypeEpisodic, Category: CategoryFact,
				Content:    fmt.Sprintf("extracted from session %d", sessionIdx),
				Importance: 0.6, DecayFactor: 1.0,
				LastAccessedAt: &now, CreatedAt: now,
			})
		}(s)
	}
	wg.Wait()

	// 验证所有提交完成
	memories := db.FindByUser("user-bg05")
	if len(memories) != sessions {
		t.Errorf("CommitSession 后记忆数 = %d, want %d", len(memories), sessions)
	}

	// 验证 LLM 调用次数
	if llm.CallCount != sessions {
		t.Errorf("LLM 调用次数 = %d, want %d", llm.CallCount, sessions)
	}

	// 验证 VFS 归档写入
	for s := 0; s < sessions; s++ {
		sessionID := fmt.Sprintf("session-%03d", s)
		uri := "session_archives/summary/" + sessionID
		content, err := fs.ReadMemory("t1", "user-bg05", uri, 0)
		if err != nil {
			t.Errorf("VFS 归档读取失败 %s: %v", sessionID, err)
			continue
		}
		if content == "" {
			t.Errorf("VFS 归档内容为空: %s", sessionID)
		}
	}
}
