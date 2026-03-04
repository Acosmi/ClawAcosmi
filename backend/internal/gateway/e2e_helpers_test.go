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

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// ============================================================================
// E2E 测试辅助基础设施 — 封装通用 setup/teardown
// ============================================================================

// E2ETestHarness 封装 E2E 测试的通用基础设施。
type E2ETestHarness struct {
	t           *testing.T
	StorePath   string
	Store       *SessionStore
	ChatState   *ChatRunState
	Broadcaster *mockBroadcaster
	Registry    *MethodRegistry

	mu         sync.Mutex
	dispatcher func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error)
}

// NewE2EHarness 创建完整 E2E 测试基础设施。
func NewE2EHarness(t *testing.T) *E2ETestHarness {
	t.Helper()
	storePath := t.TempDir()
	store := NewSessionStore("")
	chatState := NewChatRunState()
	broadcaster := &mockBroadcaster{}

	r := NewMethodRegistry()
	r.RegisterAll(ChatHandlers())

	return &E2ETestHarness{
		t:           t,
		StorePath:   storePath,
		Store:       store,
		ChatState:   chatState,
		Broadcaster: broadcaster,
		Registry:    r,
	}
}

// SetDispatcher 设置消息分发器。
func (h *E2ETestHarness) SetDispatcher(fn func(ctx context.Context, msgCtx *autoreply.MsgContext, opts *autoreply.GetReplyOptions) ([]autoreply.ReplyPayload, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dispatcher = fn
}

// CreateSession 创建测试会话。
func (h *E2ETestHarness) CreateSession(sessionKey, sessionId string) {
	h.t.Helper()
	h.Store.Save(&SessionEntry{
		SessionKey:  sessionKey,
		SessionId:   sessionId,
		SessionFile: "",
		Label:       "Test Session: " + sessionKey,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	})
}

// SendMethod 发送 gateway 方法请求并返回结果。
func (h *E2ETestHarness) SendMethod(method string, params map[string]interface{}) (bool, interface{}) {
	h.t.Helper()
	req := &RequestFrame{Method: method, Params: params}

	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}

	h.mu.Lock()
	dispatcher := h.dispatcher
	h.mu.Unlock()

	HandleGatewayRequest(h.Registry, req, nil, &GatewayMethodContext{
		SessionStore:       h.Store,
		StorePath:          h.StorePath,
		ChatState:          h.ChatState,
		PipelineDispatcher: dispatcher,
	}, respond)

	return gotOK, gotPayload
}

// WaitForTranscript 轮询 transcript 文件，等待至少 minMessages 条消息。
func (h *E2ETestHarness) WaitForTranscript(sessionId string, minMessages int, timeout time.Duration) []map[string]interface{} {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		messages := ReadTranscriptMessages(sessionId, h.StorePath, "")
		if len(messages) >= minMessages {
			return messages
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for %d messages in transcript %s", minMessages, sessionId)
	return nil
}

// GetBroadcastEvents 返回广播事件列表。
func (h *E2ETestHarness) GetBroadcastEvents() []mockBroadcastEvent {
	return h.Broadcaster.getEvents()
}

// AssertMessageInTranscript 在 transcript 中查找指定 role 和文本包含的消息。
func (h *E2ETestHarness) AssertMessageInTranscript(sessionId, role, textContains string) {
	h.t.Helper()
	messages := ReadTranscriptMessages(sessionId, h.StorePath, "")
	for _, msg := range messages {
		if msg["role"] == role {
			content, _ := msg["content"].([]interface{})
			for _, c := range content {
				block, _ := c.(map[string]interface{})
				text, _ := block["text"].(string)
				if strings.Contains(text, textContains) {
					return
				}
			}
		}
	}
	h.t.Errorf("expected message with role=%q containing %q, not found in transcript %s", role, textContains, sessionId)
}

// ---------- Transcript 辅助 ----------

// EnsureTranscriptWithTypedMessages 创建带类型消息的 transcript。
func EnsureTranscriptWithTypedMessages(t *testing.T, sessionId, storePath string, messages []TranscriptMessage) {
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

	// header
	header := map[string]interface{}{
		"__header":  true,
		"sessionId": sessionId,
		"createdAt": time.Now().UnixMilli(),
	}
	hBytes, _ := json.Marshal(header)
	f.Write(hBytes)
	f.WriteString("\n")

	for _, msg := range messages {
		mBytes, _ := json.Marshal(msg)
		f.Write(mBytes)
		f.WriteString("\n")
	}
}

// TranscriptMessage 用于构建测试 transcript 的消息类型。
type TranscriptMessage struct {
	Role      string         `json:"role"`
	Content   []ContentBlock `json:"content"`
	Timestamp int64          `json:"timestamp"`
}

// ContentBlock 内容块。
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewTextMessage 创建文本消息。
func NewTextMessage(role, text string) TranscriptMessage {
	return TranscriptMessage{
		Role:      role,
		Content:   []ContentBlock{{Type: "text", Text: text}},
		Timestamp: time.Now().UnixMilli(),
	}
}
