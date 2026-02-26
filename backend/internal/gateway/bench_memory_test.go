package gateway

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

// ============================================================================
// 内存快照基准测试
// 使用 runtime.MemStats 采集 Go 后端空闲/负载下的内存占用。
// ============================================================================

// memSnapshot 采集 GC 后的堆内存快照。
func memSnapshot() runtime.MemStats {
	runtime.GC()
	runtime.GC() // 双重 GC 确保回收完整
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms
}

// formatBytes 格式化字节数为人类可读的字符串。
func formatBytes(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// ---------- TestMemorySnapshot_IdleGateway ----------

func TestMemorySnapshot_IdleGateway(t *testing.T) {
	before := memSnapshot()

	// 启动完整网关
	rt, err := StartGatewayServer(0, GatewayServerOptions{})
	if err != nil {
		t.Fatalf("StartGatewayServer failed: %v", err)
	}

	after := memSnapshot()

	rt.Close("mem-test")
	final := memSnapshot()

	delta := after.HeapAlloc - before.HeapAlloc
	t.Logf("=== 空闲网关内存快照 ===")
	t.Logf("启动前 HeapAlloc:   %s", formatBytes(before.HeapAlloc))
	t.Logf("启动后 HeapAlloc:   %s", formatBytes(after.HeapAlloc))
	t.Logf("增量 HeapAlloc:     %s", formatBytes(delta))
	t.Logf("启动后 HeapInuse:   %s", formatBytes(after.HeapInuse))
	t.Logf("启动后 Sys(OS申请): %s", formatBytes(after.Sys))
	t.Logf("关闭后 HeapAlloc:   %s", formatBytes(final.HeapAlloc))
	t.Logf("GC 次数:            %d", after.NumGC-before.NumGC)
}

// ---------- TestMemorySnapshot_100Sessions ----------

func TestMemorySnapshot_100Sessions(t *testing.T) {
	rt, err := StartGatewayServer(0, GatewayServerOptions{})
	if err != nil {
		t.Fatalf("StartGatewayServer failed: %v", err)
	}
	defer rt.Close("mem-test")

	before := memSnapshot()

	// 创建 100 个 session entries
	store := NewSessionStore("")
	for i := 0; i < 100; i++ {
		store.Save(&SessionEntry{
			SessionId:  fmt.Sprintf("sess-%03d", i),
			SessionKey: fmt.Sprintf("agent-%d:main", i),
			Label:      fmt.Sprintf("Test Session %d", i),
		})
	}

	after := memSnapshot()
	delta := after.HeapAlloc - before.HeapAlloc

	t.Logf("=== 100 Session 内存增量 ===")
	t.Logf("增量 HeapAlloc: %s", formatBytes(delta))
	t.Logf("增量 HeapInuse: %s", formatBytes(after.HeapInuse-before.HeapInuse))
	t.Logf("每 Session ~%s", formatBytes(delta/100))
}

// ---------- TestMemorySnapshot_ConcurrentChatRuns ----------

func TestMemorySnapshot_ConcurrentChatRuns(t *testing.T) {
	const concurrency = 50

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())
	chatState := NewChatRunState()

	// 使用 os.MkdirTemp 替代 t.TempDir()，避免异步 goroutine 写入与
	// 自动 TempDir 清理的竞争。handleChatSend 内部 go func 会异步写入
	// StorePath，ACK 回调后 goroutine 可能仍在运行。
	dir, err := os.MkdirTemp("", "bench-memory-*")
	if err != nil {
		t.Fatalf("os.MkdirTemp: %v", err)
	}
	defer func() {
		// 等待异步 goroutine 完成后再清理（含重试）
		for i := 0; i < 5; i++ {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		// 最终尝试
		os.RemoveAll(dir)
	}()

	mctx := &GatewayMethodContext{
		ChatState:    chatState,
		SessionStore: NewSessionStore(""),
		StorePath:    dir,
	}

	before := memSnapshot()

	// 并发启动 50 个 chat.send（使用 stub dispatcher）
	done := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
				"text":       fmt.Sprintf("concurrent msg %d", idx),
				"sessionKey": fmt.Sprintf("chat-%d:main", idx),
			}}
			HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
		}(i)
	}
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 等待异步 goroutine（transcript 写入等）完成
	time.Sleep(200 * time.Millisecond)

	peak := memSnapshot()
	delta := peak.HeapAlloc - before.HeapAlloc

	t.Logf("=== %d 并发 ChatRun 内存压力 ===", concurrency)
	t.Logf("峰值 HeapAlloc:  %s", formatBytes(peak.HeapAlloc))
	t.Logf("增量 HeapAlloc:  %s", formatBytes(delta))
	t.Logf("峰值 HeapInuse:  %s", formatBytes(peak.HeapInuse))
	t.Logf("峰值 StackInuse: %s", formatBytes(peak.StackInuse))
}
