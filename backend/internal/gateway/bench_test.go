package gateway

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/gorilla/websocket"
)

// ============================================================================
// Gateway 性能基准测试
// 测量方法分发延迟、并发连接吞吐、Transcript 读写性能、启动时间。
// ============================================================================

// ---------- BenchmarkMethodDispatch ----------

func BenchmarkMethodDispatch_Health(b *testing.B) {
	r := NewMethodRegistry()
	r.Register("health", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, map[string]interface{}{"status": "ok"}, nil)
	})
	req := &RequestFrame{Method: "health", Params: map[string]interface{}{}}
	mctx := &GatewayMethodContext{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
	}
}

func BenchmarkMethodDispatch_SessionsList(b *testing.B) {
	r := NewMethodRegistry()
	r.RegisterAll(map[string]GatewayMethodHandler{
		"sessions.list": handleSessionsList,
	})
	store := NewSessionStore("")
	req := &RequestFrame{Method: "sessions.list", Params: map[string]interface{}{}}
	mctx := &GatewayMethodContext{SessionStore: store}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
	}
}

func BenchmarkMethodDispatch_SystemPresence(b *testing.B) {
	r := NewMethodRegistry()
	r.RegisterAll(SystemHandlers())
	store := NewSystemPresenceStore()
	mctx := &GatewayMethodContext{PresenceStore: store}
	req := &RequestFrame{Method: "system-presence", Params: map[string]interface{}{}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
	}
}

// ---------- BenchmarkRegistryLookup ----------

func BenchmarkRegistryLookup_70Methods(b *testing.B) {
	r := NewMethodRegistry()
	r.RegisterAll(map[string]GatewayMethodHandler{
		"sessions.list":    handleSessionsList,
		"sessions.preview": handleSessionsPreview,
		"sessions.resolve": handleSessionsResolve,
		"sessions.patch":   handleSessionsPatch,
		"sessions.reset":   handleSessionsReset,
		"sessions.delete":  handleSessionsDelete,
		"sessions.compact": handleSessionsCompact,
	})
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

	methods := []string{
		"sessions.list", "config.get", "models.list",
		"agents.list", "channels.status", "logs.tail",
		"chat.send", "chat.history", "chat.abort",
		"wizard.start", "cron.list",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Get(methods[i%len(methods)])
	}
}

// ---------- BenchmarkTranscriptReadWrite ----------

func BenchmarkTranscriptWrite(b *testing.B) {
	dir := b.TempDir()
	sessionId := "bench-session"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AppendAssistantTranscriptMessage(AppendTranscriptParams{
			Message:         fmt.Sprintf("Test response %d with some content", i),
			SessionID:       sessionId,
			StorePath:       dir,
			CreateIfMissing: true,
		})
	}
}

func BenchmarkTranscriptRead(b *testing.B) {
	dir := b.TempDir()
	sessionId := "bench-read"

	for i := 0; i < 100; i++ {
		AppendAssistantTranscriptMessage(AppendTranscriptParams{
			Message:         fmt.Sprintf("Message %d with some meaningful content to simulate real data", i),
			SessionID:       sessionId,
			StorePath:       dir,
			CreateIfMissing: true,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReadTranscriptMessages(sessionId, dir, "")
	}
}

func BenchmarkTranscriptRead_LargeHistory(b *testing.B) {
	dir := b.TempDir()
	sessionId := "bench-large"

	for i := 0; i < 1000; i++ {
		AppendAssistantTranscriptMessage(AppendTranscriptParams{
			Message:         fmt.Sprintf("Message %d: Lorem ipsum dolor sit amet, consectetur adipiscing elit.", i),
			SessionID:       sessionId,
			StorePath:       dir,
			CreateIfMissing: true,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgs := ReadTranscriptMessages(sessionId, dir, "")
		StripEnvelopeFromMessages(msgs)
	}
}

// ---------- BenchmarkSessionStore ----------

func BenchmarkSessionStore_Load(b *testing.B) {
	store := NewSessionStore("")
	store.Save(&SessionEntry{
		SessionId:  "bench-sess-001",
		SessionKey: "agent:main",
		Label:      "Test Session",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.LoadSessionEntry("agent:main")
	}
}

func BenchmarkSessionStore_ConcurrentAccess(b *testing.B) {
	store := NewSessionStore("")
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("agent-%d:main", i)
		store.Save(&SessionEntry{
			SessionId:  fmt.Sprintf("sess-%d", i),
			SessionKey: key,
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("agent-%d:main", i%50)
			store.LoadSessionEntry(key)
			i++
		}
	})
}

// ---------- BenchmarkConcurrentChatSend ----------

func BenchmarkConcurrentChatSend(b *testing.B) {
	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())
	chatState := NewChatRunState()

	mctx := &GatewayMethodContext{
		ChatState:    chatState,
		SessionStore: NewSessionStore(""),
		StorePath:    b.TempDir(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
				"text":       "benchmark message",
				"sessionKey": "bench:main",
			}}
			HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
		}
	})
}

// ---------- BenchmarkCombineReplyPayloads ----------

func BenchmarkCombineReplyPayloads_Small(b *testing.B) {
	replies := []autoreply.ReplyPayload{
		{Text: "Hello, how can I help you?"},
		{Text: "I see you have a question."},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CombineReplyPayloads(replies)
	}
}

// ---------- BenchmarkCapArrayByJSONBytes ----------

func BenchmarkCapArrayByJSONBytes(b *testing.B) {
	msgs := make([]map[string]interface{}, 200)
	for i := range msgs {
		msgs[i] = map[string]interface{}{
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": fmt.Sprintf("Response %d with content", i)},
			},
			"timestamp": time.Now().UnixMilli(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CapArrayByJSONBytes(msgs, 5*1024*1024)
	}
}

// ---------- BenchmarkChatRunRegistry ----------

func BenchmarkChatRunRegistry(b *testing.B) {
	state := NewChatRunState()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("session-%d:main", i%100)
			runId := fmt.Sprintf("run-%d", i)
			state.Registry.Add(key, ChatRunEntry{SessionKey: key, ClientRunID: runId})
			state.Registry.Remove(key, runId, key)
			i++
		}
	})
}

// ---------- BenchmarkGatewayStartup ----------

func BenchmarkGatewayStartup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := NewGatewayState()
		state.SetPhase(BootPhaseStarting)

		registry := NewMethodRegistry()
		registry.RegisterAll(map[string]GatewayMethodHandler{
			"sessions.list":    handleSessionsList,
			"sessions.preview": handleSessionsPreview,
		})
		registry.RegisterAll(ConfigHandlers())
		registry.RegisterAll(ModelsHandlers())
		registry.RegisterAll(AgentsHandlers())
		registry.RegisterAll(AgentHandlers())
		registry.RegisterAll(ChannelsHandlers())
		registry.RegisterAll(LogsHandlers())
		registry.RegisterAll(SystemHandlers())
		registry.RegisterAll(CronHandlers())
		registry.RegisterAll(TtsHandlers())
		registry.RegisterAll(SkillsHandlers())
		registry.RegisterAll(NodeHandlers())
		registry.RegisterAll(DeviceHandlers())
		registry.RegisterAll(VoiceWakeHandlers())
		registry.RegisterAll(UpdateHandlers())
		registry.RegisterAll(BrowserHandlers())
		registry.RegisterAll(TalkHandlers())
		registry.RegisterAll(WebHandlers())
		registry.RegisterAll(ChatHandlers())
		registry.RegisterAll(SendHandlers())

		_ = NewSessionStore("")
		_ = NewSystemPresenceStore()
		_ = NewHeartbeatState()
		_ = NewSystemEventQueue()

		state.SetPhase(BootPhaseReady)
	}
}

// ---------- BenchmarkIdleMemory ----------

func BenchmarkIdleMemoryAllocation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewGatewayState()
		_ = NewMethodRegistry()
		_ = NewSessionStore("")
		_ = NewSystemPresenceStore()
		_ = NewHeartbeatState()
		_ = NewSystemEventQueue()
	}
}

// ---------- BenchmarkWebSocketConcurrent ----------
// 真实 WebSocket 连接压测: connect → hello-ok → health request → response

func BenchmarkWebSocketConcurrent(b *testing.B) {
	state := NewGatewayState()
	state.SetPhase(BootPhaseReady)

	registry := NewMethodRegistry()
	registry.Register("health", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, map[string]interface{}{"status": "ok"}, nil)
	})

	cfg := WsServerConfig{
		Auth: ResolvedGatewayAuth{
			Mode:  AuthModeToken,
			Token: "bench-token",
		},
		State:        state,
		Registry:     registry,
		SessionStore: NewSessionStore(""),
		Version:      "bench",
	}

	handler := HandleWebSocketUpgrade(cfg)
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchWebSocketRoundTrip(b, wsURL)
		}
	})
}

// benchWebSocketRoundTrip executes a single WS connect → hello-ok → request → response cycle.
func benchWebSocketRoundTrip(b *testing.B, wsURL string) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Fatalf("WS dial failed: %v", err)
	}
	defer conn.Close()

	// Send connect frame
	connectFrame := map[string]interface{}{
		"type":        "connect",
		"minProtocol": 3,
		"maxProtocol": 3,
		"client": map[string]interface{}{
			"id": "bench-client", "version": "1.0", "platform": "test", "mode": "operator",
		},
		"role":   "operator",
		"scopes": []string{"operator.read", "operator.write"},
		"auth":   map[string]string{"token": "bench-token"},
	}
	if err := conn.WriteJSON(connectFrame); err != nil {
		b.Fatalf("connect write failed: %v", err)
	}

	// Read hello-ok
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		b.Fatalf("hello-ok read failed: %v", err)
	}

	// Send health request + read response (skip event frames)
	if err := conn.WriteJSON(map[string]interface{}{
		"type": "req", "id": "b-1", "method": "health",
	}); err != nil {
		b.Fatalf("req write failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			b.Fatalf("response read failed: %v", err)
		}
		var frame map[string]interface{}
		json.Unmarshal(msg, &frame)
		if frame["type"] == FrameTypeResponse {
			break
		}
	}
}

// ---------- BenchmarkColdStartFull ----------
// 完整冷启动计时: StartGatewayServer → Close (含 HTTP 监听 + WS + 全量注册)

func BenchmarkColdStartFull(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rt, err := StartGatewayServer(0, GatewayServerOptions{})
		if err != nil {
			b.Fatalf("start failed: %v", err)
		}
		rt.Close("bench")
	}
}

// ---------- BenchmarkChatSendLatencyHistogram ----------
// 聊天发送延迟直方图，计算 P50/P95/P99 分位数。

func BenchmarkChatSendLatencyHistogram(b *testing.B) {
	const sampleSize = 1000

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())
	chatState := NewChatRunState()
	storePath := b.TempDir()

	mctx := &GatewayMethodContext{
		ChatState:    chatState,
		SessionStore: NewSessionStore(""),
		StorePath:    storePath,
	}

	// Collect latency samples
	latencies := make([]time.Duration, sampleSize)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < sampleSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
				"text":       fmt.Sprintf("bench message %d", idx),
				"sessionKey": fmt.Sprintf("bench-%d:main", idx%10),
			}}
			HandleGatewayRequest(r, req, nil, mctx, func(ok bool, _ interface{}, _ *ErrorShape) {})
			elapsed := time.Since(start)
			mu.Lock()
			latencies[idx] = elapsed
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Sort and compute percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[sampleSize*50/100]
	p95 := latencies[sampleSize*95/100]
	p99 := latencies[sampleSize*99/100]

	b.ReportMetric(float64(p50.Microseconds()), "p50-µs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-µs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-µs")
	b.ReportMetric(float64(latencies[sampleSize-1].Microseconds()), "max-µs")
}
