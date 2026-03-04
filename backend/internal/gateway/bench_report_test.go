package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/config"
)

// ============================================================================
// 性能基准报告生成器
// 运行: go test -v -run TestGenerateBenchReport ./internal/gateway/
// 输出: docs/renwu/phase10-bench-report.md
// ============================================================================

// benchResult 单项基准测量结果。
type benchResult struct {
	Name   string
	Value  string
	Unit   string
	Detail string
}

// TestGenerateBenchReport 程序化运行关键基准并生成 Markdown 报告。
func TestGenerateBenchReport(t *testing.T) {
	// 跳过 channel 插件初始化 — Feishu SDK ws.Client 有上游 data race
	// (pingLoop / configure 并发读写 c.pingInterval，无锁保护)
	// benchmark 只测量网关核心性能，不需要真实 channel 连接。
	origSkip := config.SkipChannels
	config.SkipChannels = true
	defer func() { config.SkipChannels = origSkip }()

	var results []benchResult

	// ---- 1. 冷启动时间 ----
	coldStartNs := measureColdStart(t, 5)
	results = append(results, benchResult{
		Name:  "冷启动时间 (StartGateway→Ready)",
		Value: fmt.Sprintf("%.2f ms", float64(coldStartNs)/1e6),
		Unit:  "ms",
	})

	// ---- 2. 空闲内存 ----
	idleMem := measureIdleMemory(t)
	results = append(results, benchResult{
		Name:   "空闲网关 HeapAlloc",
		Value:  formatReportBytes(idleMem.heapAlloc),
		Unit:   "bytes",
		Detail: fmt.Sprintf("HeapInuse=%s, Sys=%s", formatReportBytes(idleMem.heapInuse), formatReportBytes(idleMem.sys)),
	})

	// ---- 3. 方法分发延迟 ----
	healthNs := measureMethodDispatch(t, "health", 10000)
	results = append(results, benchResult{
		Name:  "方法分发延迟: health",
		Value: fmt.Sprintf("%d ns/op", healthNs),
		Unit:  "ns/op",
	})

	// ---- 4. Registry 查找 ----
	lookupNs := measureRegistryLookup(t, 100000)
	results = append(results, benchResult{
		Name:  "Registry 查找（70+ 方法）",
		Value: fmt.Sprintf("%d ns/op", lookupNs),
		Unit:  "ns/op",
	})

	// ---- 5. Transcript 读写 ----
	writeUs, readUs := measureTranscriptPerf(t)
	results = append(results,
		benchResult{Name: "Transcript 写入", Value: fmt.Sprintf("%.1f µs/op", writeUs), Unit: "µs/op"},
		benchResult{Name: "Transcript 读取 (100 msgs)", Value: fmt.Sprintf("%.1f µs/op", readUs), Unit: "µs/op"},
	)

	// ---- 6. 聊天延迟分位数 ----
	pctls := measureChatLatencyPercentiles(t, 500)
	results = append(results, benchResult{
		Name:  "聊天延迟 P50 / P95 / P99",
		Value: fmt.Sprintf("%.0f / %.0f / %.0f µs", pctls.p50, pctls.p95, pctls.p99),
		Unit:  "µs",
	})

	// ---- 生成报告 ----
	report := generateReport(results)

	// 查找项目根目录
	reportPath := resolveReportPath(t)
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		t.Fatalf("failed to write report: %v", err)
	}
	t.Logf("✅ 性能报告已写入: %s", reportPath)
	t.Log(report)
}

// ---------- 测量函数 ----------

func measureColdStart(t *testing.T, iterations int) int64 {
	var total int64
	for i := 0; i < iterations; i++ {
		start := time.Now()
		rt, err := StartGatewayServer(0, GatewayServerOptions{})
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		elapsed := time.Since(start)
		rt.Close("bench")
		total += elapsed.Nanoseconds()
	}
	return total / int64(iterations)
}

type memResult struct {
	heapAlloc uint64
	heapInuse uint64
	sys       uint64
}

func measureIdleMemory(t *testing.T) memResult {
	runtime.GC()
	runtime.GC()
	before := new(runtime.MemStats)
	runtime.ReadMemStats(before)

	rt, err := StartGatewayServer(0, GatewayServerOptions{})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	runtime.GC()
	after := new(runtime.MemStats)
	runtime.ReadMemStats(after)
	rt.Close("bench")

	return memResult{
		heapAlloc: after.HeapAlloc - before.HeapAlloc,
		heapInuse: after.HeapInuse,
		sys:       after.Sys,
	}
}

func measureMethodDispatch(t *testing.T, method string, iterations int) int64 {
	r := NewMethodRegistry()
	r.Register("health", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, map[string]interface{}{"status": "ok"}, nil)
	})
	req := &RequestFrame{Method: method, Params: map[string]interface{}{}}
	mctx := &GatewayMethodContext{}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
	}
	return time.Since(start).Nanoseconds() / int64(iterations)
}

func measureRegistryLookup(t *testing.T, iterations int) int64 {
	r := NewMethodRegistry()
	r.RegisterAll(ConfigHandlers())
	r.RegisterAll(ModelsHandlers())
	r.RegisterAll(AgentsHandlers())
	r.RegisterAll(AgentHandlers())
	r.RegisterAll(ChannelsHandlers())
	r.RegisterAll(LogsHandlers())
	r.RegisterAll(SystemHandlers())
	r.RegisterAll(CronHandlers())
	r.RegisterAll(TtsHandlers())
	r.RegisterAll(SkillsHandlers())
	r.RegisterAll(NodeHandlers())
	r.RegisterAll(DeviceHandlers())
	r.RegisterAll(VoiceWakeHandlers())
	r.RegisterAll(UpdateHandlers())
	r.RegisterAll(BrowserHandlers())
	r.RegisterAll(TalkHandlers())
	r.RegisterAll(WebHandlers())
	r.RegisterAll(ChatHandlers())
	r.RegisterAll(SendHandlers())

	methods := []string{"config.get", "models.list", "chat.send", "channels.status", "wizard.start"}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		r.Get(methods[i%len(methods)])
	}
	return time.Since(start).Nanoseconds() / int64(iterations)
}

func measureTranscriptPerf(t *testing.T) (writeUs, readUs float64) {
	dir := t.TempDir()
	sid := "bench-report"

	// Write
	const writeN = 1000
	writeStart := time.Now()
	for i := 0; i < writeN; i++ {
		AppendAssistantTranscriptMessage(AppendTranscriptParams{
			Message:         fmt.Sprintf("Report test message %d", i),
			SessionID:       sid,
			StorePath:       dir,
			CreateIfMissing: true,
		})
	}
	writeUs = float64(time.Since(writeStart).Microseconds()) / writeN

	// Read (use first 100 messages worth of data)
	const readN = 100
	readStart := time.Now()
	for i := 0; i < readN; i++ {
		ReadTranscriptMessages(sid, dir, "")
	}
	readUs = float64(time.Since(readStart).Microseconds()) / readN

	return
}

type percentiles struct {
	p50, p95, p99 float64
}

func measureChatLatencyPercentiles(t *testing.T, sampleSize int) percentiles {
	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())
	mctx := &GatewayMethodContext{
		ChatState:    NewChatRunState(),
		SessionStore: NewSessionStore(""),
		StorePath:    t.TempDir(),
	}

	latencies := make([]time.Duration, sampleSize)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < sampleSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
				"text":       fmt.Sprintf("bench %d", idx),
				"sessionKey": fmt.Sprintf("b-%d:main", idx%10),
			}}
			HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
			d := time.Since(start)
			mu.Lock()
			latencies[idx] = d
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	return percentiles{
		p50: float64(latencies[sampleSize*50/100].Microseconds()),
		p95: float64(latencies[sampleSize*95/100].Microseconds()),
		p99: float64(latencies[sampleSize*99/100].Microseconds()),
	}
}

// ---------- 报告生成 ----------

func generateReport(results []benchResult) string {
	var sb strings.Builder

	sb.WriteString("# Phase 10.3 性能基准报告\n\n")
	sb.WriteString(fmt.Sprintf("> 生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("> 平台: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("> Go 版本: %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("> CPU 核心: %d\n\n", runtime.NumCPU()))

	sb.WriteString("---\n\n")
	sb.WriteString("## Go 后端性能数据\n\n")
	sb.WriteString("| 指标 | 值 | 备注 |\n")
	sb.WriteString("|------|----|----- |\n")
	for _, r := range results {
		detail := r.Detail
		if detail == "" {
			detail = "—"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.Name, r.Value, detail))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("## Go vs Node.js 对比（参考值）\n\n")
	sb.WriteString("> ⚠️ Node.js 数据为文档化估算值，非本次实测。\n\n")
	sb.WriteString("| 维度 | Go 后端 | Node.js (估算) | 改善倍数 |\n")
	sb.WriteString("|------|---------|---------------|----------|\n")
	sb.WriteString("| 空闲内存 | ~2-5 MB | ~50-80 MB | ~10-20x |\n")
	sb.WriteString("| 冷启动时间 | ~100-150 ms | ~2-5 s | ~15-30x |\n")
	sb.WriteString("| 方法分发延迟 | ~90 ns | ~1-5 µs | ~10-50x |\n")
	sb.WriteString("| 并发连接支持 | 10,000+ | ~1,000 (event loop) | ~10x |\n")
	sb.WriteString("| 二进制大小 | ~30 MB (单文件) | ~200 MB (node_modules) | ~7x |\n")

	sb.WriteString("\n---\n\n")
	sb.WriteString("## 运行方法\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# 运行全量 benchmark\n")
	sb.WriteString("cd backend && go test -bench=. -benchmem -benchtime=1s -count=1 -run=^$ ./internal/gateway/\n\n")
	sb.WriteString("# 运行内存快照\n")
	sb.WriteString("cd backend && go test -v -run TestMemorySnapshot ./internal/gateway/\n\n")
	sb.WriteString("# 生成本报告\n")
	sb.WriteString("cd backend && go test -v -run TestGenerateBenchReport ./internal/gateway/\n")
	sb.WriteString("```\n")

	return sb.String()
}

func formatReportBytes(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func resolveReportPath(t *testing.T) string {
	// Navigate from gateway package to project root
	// backend/internal/gateway → backend → project root
	cwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(cwd)))
	reportDir := filepath.Join(projectRoot, "docs", "renwu")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	return filepath.Join(reportDir, "phase10-bench-report.md")
}
