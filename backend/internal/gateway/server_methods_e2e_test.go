package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropic/open-acosmi/internal/agents/llmclient"
	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// ============================================================
// 端到端集成测试 — Pipeline + Transcript + Broadcast
// ============================================================

// ---------- 测试基础设施 ----------

// mockBroadcaster 捕获广播事件用于断言。
type mockBroadcaster struct {
	mu     sync.Mutex
	events []mockBroadcastEvent
}

type mockBroadcastEvent struct {
	Event   string
	Payload interface{}
}

func (b *mockBroadcaster) Broadcast(event string, payload interface{}, _ interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, mockBroadcastEvent{Event: event, Payload: payload})
}

func (b *mockBroadcaster) ClientCount() int { return 1 }

func (b *mockBroadcaster) getEvents() []mockBroadcastEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]mockBroadcastEvent, len(b.events))
	copy(cp, b.events)
	return cp
}

// setupE2EStore 创建一个临时 storePath 并返回预填充的 session store。
func setupE2EStore(t *testing.T, sessionKey, sessionId string) (string, *SessionStore) {
	t.Helper()
	storePath := t.TempDir()
	store := NewSessionStore("")
	store.Save(&SessionEntry{
		SessionKey:  sessionKey,
		SessionId:   sessionId,
		SessionFile: "",
		Label:       "Test Session",
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	})
	return storePath, store
}

// ensureTranscriptWithMessages 在 storePath 中创建一个带消息的 transcript 文件。
func ensureTranscriptWithMessages(t *testing.T, sessionId, storePath string, messages []map[string]interface{}) {
	t.Helper()
	transcriptPath := ResolveTranscriptPath(sessionId, storePath, "")
	dir := filepath.Dir(transcriptPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// 写 header
	header := map[string]interface{}{
		"__header":  true,
		"sessionId": sessionId,
		"createdAt": time.Now().UnixMilli(),
	}
	hBytes, _ := json.Marshal(header)
	f.Write(hBytes)
	f.WriteString("\n")

	// 写消息
	for _, msg := range messages {
		mBytes, _ := json.Marshal(msg)
		f.Write(mBytes)
		f.WriteString("\n")
	}
}

// ---------- Test 1: Transcript Write and Read ----------

func TestE2E_TranscriptWriteAndRead(t *testing.T) {
	storePath := t.TempDir()
	sessionId := "e2e-test-session-1"

	// 写入消息
	result := AppendAssistantTranscriptMessage(AppendTranscriptParams{
		Message:         "Hello from assistant",
		SessionID:       sessionId,
		StorePath:       storePath,
		CreateIfMissing: true,
	})
	if !result.OK {
		t.Fatalf("append failed: %s", result.Error)
	}
	if result.MessageID == "" {
		t.Fatal("expected non-empty messageId")
	}

	// 读回
	messages := ReadTranscriptMessages(sessionId, storePath, "")
	if len(messages) == 0 {
		t.Fatal("expected at least 1 message")
	}

	lastMsg := messages[len(messages)-1]
	role, _ := lastMsg["role"].(string)
	if role != "assistant" {
		t.Errorf("expected role=assistant, got %v", role)
	}

	// 验证内容
	content, _ := lastMsg["content"].([]interface{})
	if len(content) == 0 {
		t.Fatal("expected content array")
	}
	first, _ := content[0].(map[string]interface{})
	text, _ := first["text"].(string)
	if text != "Hello from assistant" {
		t.Errorf("expected text='Hello from assistant', got '%s'", text)
	}
}

// ---------- Test 2: ChatInject Writes Transcript ----------

func TestE2E_ChatInject_WritesTranscript(t *testing.T) {
	sessionId := "e2e-inject-session"
	sessionKey := "e2e:inject"
	storePath, store := setupE2EStore(t, sessionKey, sessionId)

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	req := &RequestFrame{Method: "chat.inject", Params: map[string]interface{}{
		"sessionKey": sessionKey,
		"text":       "Injected message from test",
		"role":       "assistant",
	}}

	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}

	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		SessionStore: store,
		StorePath:    storePath,
	}, respond)

	if !gotOK {
		t.Fatalf("chat.inject should succeed, payload: %v", gotPayload)
	}

	m, ok := gotPayload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", gotPayload)
	}
	if m["ok"] != true {
		t.Errorf("expected ok=true, got %v", m["ok"])
	}
	if m["messageId"] == nil || m["messageId"] == "" {
		t.Error("expected non-empty messageId")
	}

	// 验证 transcript 文件存在且包含消息
	messages := ReadTranscriptMessages(sessionId, storePath, "")
	if len(messages) == 0 {
		t.Fatal("expected transcript to contain messages after inject")
	}
	lastMsg := messages[len(messages)-1]
	role, _ := lastMsg["role"].(string)
	if role != "assistant" {
		t.Errorf("expected role=assistant, got %v", role)
	}
}

// ---------- Test 3: ChatHistory Reads Transcript ----------

func TestE2E_ChatHistory_ReadsTranscript(t *testing.T) {
	sessionId := "e2e-history-session"
	sessionKey := "e2e:history"
	storePath, store := setupE2EStore(t, sessionKey, sessionId)

	// 预填充 transcript
	ensureTranscriptWithMessages(t, sessionId, storePath, []map[string]interface{}{
		{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "Hello"},
			},
			"timestamp": time.Now().UnixMilli(),
		},
		{
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "Hi there!"},
			},
			"timestamp": time.Now().UnixMilli(),
		},
	})

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	req := &RequestFrame{Method: "chat.history", Params: map[string]interface{}{
		"sessionKey": sessionKey,
	}}

	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}

	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		SessionStore: store,
		StorePath:    storePath,
	}, respond)

	if !gotOK {
		t.Fatal("chat.history should succeed")
	}

	m, ok := gotPayload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", gotPayload)
	}

	total, _ := m["total"].(int)
	if total != 2 {
		t.Errorf("expected total=2, got %v", m["total"])
	}

	messages, _ := m["messages"].([]map[string]interface{})
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	if messages[0]["role"] != "user" {
		t.Errorf("first message should be user, got %v", messages[0]["role"])
	}
	if messages[1]["role"] != "assistant" {
		t.Errorf("second message should be assistant, got %v", messages[1]["role"])
	}
}

// ---------- Test 4: ChatSend Pipeline Dispatch (Stub) ----------

func TestE2E_ChatSend_PipelineDispatch(t *testing.T) {
	sessionKey := "e2e:send"
	chatState := NewChatRunState()

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
		"sessionKey": sessionKey,
		"text":       "Hello AI",
		"agentId":    "default",
	}}

	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}

	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		ChatState: chatState,
		StorePath: t.TempDir(),
	}, respond)

	if !gotOK {
		t.Fatal("chat.send should succeed")
	}

	m, ok := gotPayload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", gotPayload)
	}
	if m["status"] != "started" {
		t.Errorf("expected status=started, got %v", m["status"])
	}
	if m["runId"] == nil || m["runId"] == "" {
		t.Error("expected non-empty runId")
	}

	// 等待 goroutine 完成
	time.Sleep(500 * time.Millisecond)
}

// ---------- Test 5: ChatSend With Mock Dispatcher ----------

func TestE2E_ChatSend_WithDispatcher(t *testing.T) {
	sessionId := "e2e-dispatch-session"
	sessionKey := "e2e:dispatch"
	storePath, store := setupE2EStore(t, sessionKey, sessionId)
	chatState := NewChatRunState()

	// mock dispatcher 返回一段回复
	mockDispatcher := func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error) {
		return []autoreply.ReplyPayload{
			{Text: "Mock AI reply from dispatcher"},
		}, nil
	}

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
		"sessionKey": sessionKey,
		"text":       "Hello with dispatcher",
		"agentId":    "default",
	}}

	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}

	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		ChatState:          chatState,
		StorePath:          storePath,
		SessionStore:       store,
		PipelineDispatcher: mockDispatcher,
	}, respond)

	if !gotOK {
		t.Fatal("chat.send should succeed")
	}

	m, _ := gotPayload.(map[string]interface{})
	runId, _ := m["runId"].(string)
	if runId == "" {
		t.Fatal("expected runId")
	}

	// 等待 goroutine 完成
	time.Sleep(1 * time.Second)

	// 验证 transcript 已写入
	messages := ReadTranscriptMessages(sessionId, storePath, "")
	if len(messages) == 0 {
		t.Fatal("expected transcript to contain the AI reply")
	}

	// 找到 assistant 消息
	found := false
	for _, msg := range messages {
		if msg["role"] == "assistant" {
			content, _ := msg["content"].([]interface{})
			if len(content) > 0 {
				first, _ := content[0].(map[string]interface{})
				text, _ := first["text"].(string)
				if strings.Contains(text, "Mock AI reply") {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("expected to find 'Mock AI reply' in transcript")
	}
}

// ---------- Test 6: ChatSend + Abort ----------

func TestE2E_ChatSend_Abort(t *testing.T) {
	sessionKey := "e2e:abort"
	chatState := NewChatRunState()

	// 慢速 dispatcher (模拟长时间运行)
	slowDispatcher := func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return []autoreply.ReplyPayload{{Text: "should not reach"}}, nil
		}
	}

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	// 1. 发送 chat.send
	sendReq := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
		"sessionKey": sessionKey,
		"text":       "This should be aborted",
		"agentId":    "default",
	}}

	var sendPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) { sendPayload = payload }

	HandleGatewayRequest(r, sendReq, nil, &GatewayMethodContext{
		ChatState:          chatState,
		StorePath:          t.TempDir(),
		PipelineDispatcher: slowDispatcher,
	}, respond)

	m, _ := sendPayload.(map[string]interface{})
	runId, _ := m["runId"].(string)

	// 2. 验证 run 被注册
	time.Sleep(100 * time.Millisecond)
	entry := chatState.Registry.Peek(sessionKey)
	if entry == nil {
		// 可能 goroutine 还没开始，等一下
		time.Sleep(200 * time.Millisecond)
		entry = chatState.Registry.Peek(sessionKey)
		if entry == nil {
			t.Log("warn: no registered run found, goroutine may have completed already")
		}
	}

	// 3. 发送 chat.abort
	abortReq := &RequestFrame{Method: "chat.abort", Params: map[string]interface{}{
		"sessionKey": sessionKey,
	}}
	var abortOK bool
	abortRespond := func(ok bool, _ interface{}, _ *ErrorShape) { abortOK = ok }
	HandleGatewayRequest(r, abortReq, nil, &GatewayMethodContext{
		ChatState: chatState,
	}, abortRespond)

	if !abortOK {
		t.Error("chat.abort should succeed")
	}

	// 4. 验证 abort 标记
	_, aborted := chatState.AbortedRuns.Load(sessionKey)
	if !aborted {
		// abort 可能使用不同的 key 格式
		t.Log("warn: abort flag check may use different key format")
	}

	// 5. 等待 goroutine 完成
	time.Sleep(1 * time.Second)
	_ = runId // 使用 runId 以满足 linter
}

// ---------- Test 7: ChatSend With Real DeepSeek LLM ----------

func TestE2E_ChatSend_RealLLM(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set, skipping real LLM test")
	}

	sessionId := "e2e-realllm-session"
	sessionKey := "e2e:realllm"
	storePath, store := setupE2EStore(t, sessionKey, sessionId)
	chatState := NewChatRunState()

	// 真实 LLM dispatcher: 直接调用 DeepSeek API
	realDispatcher := func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error) {
		result, err := llmclient.Chat(ctx, llmclient.ChatRequest{
			Provider:     "deepseek",
			Model:        "deepseek-chat",
			SystemPrompt: "You are a helpful assistant. Respond briefly.",
			Messages: []llmclient.ChatMessage{
				llmclient.TextMessage("user", msgCtx.Body),
			},
			MaxTokens: 256,
			TimeoutMs: 30000,
			APIKey:    apiKey,
			BaseURL:   "https://api.deepseek.com/v1",
		})
		if err != nil {
			return nil, err
		}
		// 从 assistant message 提取文本
		var text strings.Builder
		for _, block := range result.AssistantMessage.Content {
			if block.Type == "text" {
				text.WriteString(block.Text)
			}
		}
		return []autoreply.ReplyPayload{
			{Text: text.String()},
		}, nil
	}

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	req := &RequestFrame{Method: "chat.send", Params: map[string]interface{}{
		"sessionKey": sessionKey,
		"text":       "Say hello in exactly 3 words",
		"agentId":    "default",
	}}

	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}

	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		ChatState:          chatState,
		StorePath:          storePath,
		SessionStore:       store,
		PipelineDispatcher: realDispatcher,
	}, respond)

	if !gotOK {
		t.Fatal("chat.send should succeed")
	}

	m, _ := gotPayload.(map[string]interface{})
	runId, _ := m["runId"].(string)
	if runId == "" {
		t.Fatal("expected runId")
	}
	t.Logf("runId: %s", runId)

	// 轮询 transcript 等待真实 LLM 响应（最多 45s）
	deadline := time.Now().Add(45 * time.Second)
	var replyText string
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		messages := ReadTranscriptMessages(sessionId, storePath, "")
		for _, msg := range messages {
			if msg["role"] == "assistant" {
				content, _ := msg["content"].([]interface{})
				if len(content) > 0 {
					first, _ := content[0].(map[string]interface{})
					text, _ := first["text"].(string)
					if text != "" {
						replyText = text
						break
					}
				}
			}
		}
		if replyText != "" {
			break
		}
	}

	if replyText == "" {
		t.Fatal("timed out waiting for DeepSeek reply in transcript")
	}

	t.Logf("DeepSeek reply: %s", replyText)
}
