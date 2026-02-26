//go:build e2e
// +build e2e

// Package e2etest — E2E-AGFS: AGFS 分布式基础设施测试。
// 覆盖: 队列入队出队 / 队列持久化 / 分布式锁 / 共享文件 / KV 一致性 / 多租户隔离
package e2etest

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// E2E-AGFS-01: 嵌入队列入队→出队
func TestE2E_AGFS01_QueueEnqueueDequeue(t *testing.T) {
	q := NewMockAGFSQueue()

	type EmbedItem struct {
		MemoryID string `json:"memory_id"`
		Content  string `json:"content"`
	}

	// 入队 10 条
	for i := 0; i < 10; i++ {
		item := EmbedItem{
			MemoryID: fmt.Sprintf("mem-%03d", i),
			Content:  fmt.Sprintf("content-%d", i),
		}
		if err := q.EnqueueJSON(item); err != nil {
			t.Fatalf("EnqueueJSON %d: %v", i, err)
		}
	}

	if q.Len() != 10 {
		t.Fatalf("队列长度 = %d, want 10", q.Len())
	}

	// 出队验证 FIFO 顺序
	for i := 0; i < 10; i++ {
		data, ok := q.Dequeue()
		if !ok {
			t.Fatalf("Dequeue %d 失败", i)
		}
		var item EmbedItem
		if err := json.Unmarshal(data, &item); err != nil {
			t.Fatalf("Unmarshal %d: %v", i, err)
		}
		expected := fmt.Sprintf("mem-%03d", i)
		if item.MemoryID != expected {
			t.Errorf("Dequeue 顺序错误: got %s, want %s", item.MemoryID, expected)
		}
	}

	// 空队列出队
	_, ok := q.Dequeue()
	if ok {
		t.Error("空队列 Dequeue 应返回 false")
	}

	if q.Len() != 0 {
		t.Errorf("出队后长度 = %d, want 0", q.Len())
	}
}

// E2E-AGFS-02: 队列 AGFS 重启恢复 (持久化验证)
func TestE2E_AGFS02_QueuePersistence(t *testing.T) {
	q := NewMockAGFSQueue()

	// 入队
	q.Enqueue([]byte("task-A"))
	q.Enqueue([]byte("task-B"))
	q.Enqueue([]byte("task-C"))

	// 快照 (模拟持久化到 DB)
	snap := q.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("快照长度 = %d, want 3", len(snap))
	}

	// 模拟出队一条
	q.Dequeue()

	// 模拟 AGFS 重启：新建队列 + 从快照恢复
	q2 := NewMockAGFSQueue()
	q2.Restore(snap)

	if q2.Len() != 3 {
		t.Fatalf("恢复后长度 = %d, want 3", q2.Len())
	}

	// 验证恢复的数据顺序
	d1, _ := q2.Dequeue()
	if string(d1) != "task-A" {
		t.Errorf("恢复后首条 = %q, want task-A", string(d1))
	}
	d2, _ := q2.Dequeue()
	if string(d2) != "task-B" {
		t.Errorf("恢复后第二条 = %q, want task-B", string(d2))
	}
}

// E2E-AGFS-03: VFS 分布式锁互斥
func TestE2E_AGFS03_DistributedLockMutex(t *testing.T) {
	lockMgr := NewMockAGFSLock()
	agfs := NewMockAGFS()

	const iterations = 100
	var counter int64

	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu := lockMgr.ForUser("tenant-1", "user-1")
			for i := 0; i < iterations; i++ {
				mu.Lock()
				// 临界区：读→修改→写
				path := "/data/counter.txt"
				data, _ := agfs.ReadFile(path)
				var val int64
				if len(data) > 0 {
					fmt.Sscanf(string(data), "%d", &val)
				}
				val++
				_ = agfs.WriteFile(path, []byte(fmt.Sprintf("%d", val)))
				atomic.AddInt64(&counter, 1)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 验证最终值
	data, _ := agfs.ReadFile("/data/counter.txt")
	var finalVal int64
	fmt.Sscanf(string(data), "%d", &finalVal)

	expected := int64(2 * iterations)
	if finalVal != expected {
		t.Errorf("锁保护的计数器 = %d, want %d (数据竞争!)", finalVal, expected)
	}
	if atomic.LoadInt64(&counter) != expected {
		t.Errorf("原子计数器 = %d, want %d", counter, expected)
	}
}

// E2E-AGFS-04: VFS 共享文件读写
func TestE2E_AGFS04_SharedFileReadWrite(t *testing.T) {
	agfs := NewMockAGFS()

	// 写入
	content := "这是一段中文内容，测试 VFS 共享存储"
	if err := agfs.WriteFile("/vfs/t1/u1/memories/doc.txt", []byte(content)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// 读取
	data, err := agfs.ReadFile("/vfs/t1/u1/memories/doc.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("读取内容不一致: got %q, want %q", string(data), content)
	}

	// 覆写
	newContent := "更新后的内容"
	if err := agfs.WriteFile("/vfs/t1/u1/memories/doc.txt", []byte(newContent)); err != nil {
		t.Fatalf("Overwrite: %v", err)
	}
	data2, _ := agfs.ReadFile("/vfs/t1/u1/memories/doc.txt")
	if string(data2) != newContent {
		t.Errorf("覆写后内容不一致")
	}

	// 目录列表
	entries, err := agfs.ListDir("/vfs/t1/u1/memories")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("目录应有 1 个文件, got %d", len(entries))
	}

	// 删除
	if err := agfs.RemoveAll("/vfs/t1/u1"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if agfs.FileExists("/vfs/t1/u1/memories/doc.txt") {
		t.Error("删除后文件应不存在")
	}
}

// E2E-AGFS-05: KV 缓存一致性
func TestE2E_AGFS05_KVConsistency(t *testing.T) {
	kv := NewMockAGFSKV()

	// 原始字节 KV
	kv.Set("config:token_budget", []byte("20000"))
	val, ok := kv.Get("config:token_budget")
	if !ok {
		t.Fatal("KV Get 应成功")
	}
	if string(val) != "20000" {
		t.Errorf("KV value = %q, want 20000", string(val))
	}

	// JSON KV
	type DecayConfig struct {
		HalfLifeDays float64 `json:"half_life_days"`
		MemoryType   string  `json:"memory_type"`
	}
	cfg := DecayConfig{HalfLifeDays: 30.0, MemoryType: "episodic"}
	if err := kv.SetJSON("decay:episodic", cfg); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}

	var retrieved DecayConfig
	if err := kv.GetJSON("decay:episodic", &retrieved); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if retrieved.HalfLifeDays != 30.0 {
		t.Errorf("HalfLifeDays = %f, want 30.0", retrieved.HalfLifeDays)
	}
	if retrieved.MemoryType != "episodic" {
		t.Errorf("MemoryType = %s, want episodic", retrieved.MemoryType)
	}

	// Delete 后读取
	kv.Delete("config:token_budget")
	_, ok = kv.Get("config:token_budget")
	if ok {
		t.Error("删除后 KV Get 应返回 false")
	}

	// 不存在的 key
	if err := kv.GetJSON("nonexistent", &retrieved); err == nil {
		t.Error("不存在的 key 应报错")
	}
}

// E2E-AGFS-06: 多租户 VFS 隔离
func TestE2E_AGFS06_MultiTenantIsolation(t *testing.T) {
	fs := NewMockFSStore()

	// tenant_a 写入
	_ = fs.WriteMemory("tenant_a", "user-1", "mem-001", "episodic", "preference",
		"租户A的秘密数据", "A-secret", "A详细")
	// tenant_b 写入
	_ = fs.WriteMemory("tenant_b", "user-1", "mem-001", "episodic", "preference",
		"租户B的公开数据", "B-public", "B详细")

	// tenant_a 读取自己的数据
	uri := "episodic/preference/mem-001"
	l0a, err := fs.ReadMemory("tenant_a", "user-1", uri, 0)
	if err != nil {
		t.Fatalf("tenant_a 读取: %v", err)
	}
	if l0a != "A-secret" {
		t.Errorf("tenant_a L0 = %q, want A-secret", l0a)
	}

	// tenant_b 读取自己的数据
	l0b, err := fs.ReadMemory("tenant_b", "user-1", uri, 0)
	if err != nil {
		t.Fatalf("tenant_b 读取: %v", err)
	}
	if l0b != "B-public" {
		t.Errorf("tenant_b L0 = %q, want B-public", l0b)
	}

	// 关键验证：tenant_a 的数据不会泄漏到 tenant_b
	if l0a == l0b {
		t.Error("不同租户的数据不应相同 (隔离失败)")
	}

	// tenant_c (从未写入) 不应读到任何数据
	_, err = fs.ReadMemory("tenant_c", "user-1", uri, 0)
	if err == nil {
		t.Error("未写入的租户不应读到数据")
	}

	// BatchReadL0 也应隔离
	uris := []string{uri}
	entriesA, _ := fs.BatchReadL0("tenant_a", "user-1", uris)
	entriesB, _ := fs.BatchReadL0("tenant_b", "user-1", uris)

	if len(entriesA) != 1 || len(entriesB) != 1 {
		t.Fatalf("BatchReadL0 各应返回 1 条: A=%d, B=%d", len(entriesA), len(entriesB))
	}
	if entriesA[0].L0Abstract == entriesB[0].L0Abstract {
		t.Error("BatchReadL0 隔离失败: 不同租户返回相同 L0")
	}
}
