package acp

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
)

// UUID v4 格式正则
var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ---------- meta_test ----------

func TestReadString(t *testing.T) {
	meta := map[string]interface{}{
		"sessionKey":   "agent:main:test",
		"sessionLabel": "  support  ",
	}

	got := ReadString(meta, []string{"sessionLabel", "session_label"})
	if got != "support" {
		t.Errorf("expected 'support', got %q", got)
	}

	got = ReadString(meta, []string{"nonexistent"})
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	got = ReadString(nil, []string{"sessionKey"})
	if got != "" {
		t.Errorf("expected empty for nil meta, got %q", got)
	}
}

func TestReadBool(t *testing.T) {
	meta := map[string]interface{}{
		"resetSession": true,
	}

	got := ReadBool(meta, []string{"resetSession"})
	if got == nil || !*got {
		t.Error("expected true")
	}

	got = ReadBool(meta, []string{"nonexistent"})
	if got != nil {
		t.Error("expected nil for missing key")
	}
}

func TestReadNumber(t *testing.T) {
	meta := map[string]interface{}{
		"count": float64(42),
	}

	got := ReadNumber(meta, []string{"count"})
	if got == nil || *got != 42 {
		t.Errorf("expected 42, got %v", got)
	}
}

// ---------- session_test ----------

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewInMemorySessionStore()
	session := store.CreateSession(CreateSessionOpts{
		SessionKey: "acp:test",
		Cwd:        "/tmp",
	})

	if session.SessionID == "" {
		t.Fatal("session ID should not be empty")
	}
	if session.SessionKey != "acp:test" {
		t.Errorf("expected key 'acp:test', got %q", session.SessionKey)
	}

	got := store.GetSession(session.SessionID)
	if got == nil {
		t.Fatal("GetSession returned nil")
	}

	got = store.GetSessionByKey("acp:test")
	if got == nil {
		t.Fatal("GetSessionByKey returned nil")
	}
	if got.SessionID != session.SessionID {
		t.Error("session ID mismatch via key lookup")
	}
}

func TestSessionStore_ActiveRunTracking(t *testing.T) {
	store := NewInMemorySessionStore()
	defer store.ClearAllSessionsForTest()

	session := store.CreateSession(CreateSessionOpts{
		SessionKey: "acp:test",
		Cwd:        "/tmp",
	})

	_, cancel := context.WithCancel(context.Background())
	store.SetActiveRun(session.SessionID, "run-1", cancel)

	got := store.GetSessionByRunId("run-1")
	if got == nil {
		t.Fatal("expected session by run ID")
	}
	if got.SessionID != session.SessionID {
		t.Error("session ID mismatch")
	}

	cancelled := store.CancelActiveRun(session.SessionID)
	if !cancelled {
		t.Error("expected cancel to succeed")
	}
	got = store.GetSessionByRunId("run-1")
	if got != nil {
		t.Error("expected nil after cancel")
	}
}

// ---------- event_mapper_test ----------

func TestExtractTextFromPrompt(t *testing.T) {
	text := ExtractTextFromPrompt([]ContentBlock{
		{Type: "text", Text: "Hello"},
		{Type: "resource", Resource: &ResourceContent{Text: "File contents"}},
		{Type: "resource_link", URI: "https://example.com", Title: "Spec"},
		{Type: "image", Data: "abc", MimeType: "image/png"},
	})

	expected := "Hello\nFile contents\n[Resource link (Spec)] https://example.com"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestExtractAttachmentsFromPrompt(t *testing.T) {
	attachments := ExtractAttachmentsFromPrompt([]ContentBlock{
		{Type: "image", Data: "abc", MimeType: "image/png"},
		{Type: "image", Data: "", MimeType: "image/png"},
		{Type: "text", Text: "ignored"},
	})

	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].Type != "image" {
		t.Error("expected type 'image'")
	}
	if attachments[0].MimeType != "image/png" {
		t.Error("expected mimeType 'image/png'")
	}
	if attachments[0].Content != "abc" {
		t.Error("expected content 'abc'")
	}
}

func TestInferToolKind(t *testing.T) {
	tests := []struct {
		name     string
		expected ToolKind
	}{
		{"readFile", ToolKindRead},
		{"writeFile", ToolKindEdit},
		{"deleteItem", ToolKindDelete},
		{"moveFile", ToolKindMove},
		{"searchCode", ToolKindSearch},
		{"executeCommand", ToolKindExecute},
		{"fetchURL", ToolKindFetch},
		{"unknownTool", ToolKindOther},
	}

	for _, tc := range tests {
		got := InferToolKind(tc.name)
		if got != tc.expected {
			t.Errorf("InferToolKind(%q) = %q, want %q", tc.name, got, tc.expected)
		}
	}
}

// ---------- session_mapper_test ----------

type mockGatewayRequester struct {
	calls   []mockCall
	handler func(method string, params interface{}) (interface{}, error)
}

type mockCall struct {
	Method string
	Params interface{}
}

func (m *mockGatewayRequester) Request(method string, params interface{}, result interface{}) error {
	m.calls = append(m.calls, mockCall{Method: method, Params: params})
	if m.handler != nil {
		resp, err := m.handler(method, params)
		if err != nil {
			return err
		}
		if resp != nil {
			data, _ := json.Marshal(resp)
			_ = json.Unmarshal(data, result)
		}
	}
	return nil
}

func TestResolveSessionKey_PrefersLabel(t *testing.T) {
	gw := &mockGatewayRequester{
		handler: func(method string, params interface{}) (interface{}, error) {
			return map[string]interface{}{"ok": true, "key": "agent:main:label"}, nil
		},
	}

	meta := ParseSessionMeta(map[string]interface{}{
		"sessionLabel": "support",
		"sessionKey":   "agent:main:main",
	})

	key, err := ResolveSessionKey(ResolveSessionKeyParams{
		Meta:        meta,
		FallbackKey: "acp:fallback",
		Gateway:     gw,
		Opts:        &AcpServerOptions{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "agent:main:label" {
		t.Errorf("expected 'agent:main:label', got %q", key)
	}
	if len(gw.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(gw.calls))
	}
}

func TestResolveSessionKey_FallsBackToSessionKey(t *testing.T) {
	gw := &mockGatewayRequester{}

	meta := ParseSessionMeta(map[string]interface{}{
		"sessionKey": "agent:main:override",
	})

	key, err := ResolveSessionKey(ResolveSessionKeyParams{
		Meta:        meta,
		FallbackKey: "acp:fallback",
		Gateway:     gw,
		Opts:        &AcpServerOptions{DefaultSessionLabel: "default-label"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "agent:main:override" {
		t.Errorf("expected 'agent:main:override', got %q", key)
	}
	if len(gw.calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(gw.calls))
	}
}

// ---------- ACP-P2-1: UUID session ID ----------

func TestSessionStore_CreateSession_UUID(t *testing.T) {
	store := NewInMemorySessionStore()
	defer store.ClearAllSessionsForTest()

	s1 := store.CreateSession(CreateSessionOpts{SessionKey: "a:test:1", Cwd: "/tmp"})
	s2 := store.CreateSession(CreateSessionOpts{SessionKey: "a:test:2", Cwd: "/tmp"})

	// 验证 UUID v4 格式
	if !uuidRe.MatchString(s1.SessionID) {
		t.Errorf("session 1 ID is not UUID v4: %q", s1.SessionID)
	}
	if !uuidRe.MatchString(s2.SessionID) {
		t.Errorf("session 2 ID is not UUID v4: %q", s2.SessionID)
	}

	// 验证唯一性
	if s1.SessionID == s2.SessionID {
		t.Error("session IDs should be unique")
	}
}

// ---------- ACP-P2-2: ListSessions limit ----------

func TestListSessions_LimitParam(t *testing.T) {
	gw := &mockGatewayRequester{
		handler: func(method string, params interface{}) (interface{}, error) {
			// 验证传入的 limit 参数
			m, _ := json.Marshal(params)
			var p map[string]interface{}
			_ = json.Unmarshal(m, &p)
			if limit, ok := p["limit"]; ok {
				if limit != float64(25) {
					t.Errorf("expected limit 25, got %v", limit)
				}
			}
			return map[string]interface{}{"sessions": []interface{}{}}, nil
		},
	}

	agent := &AcpGatewayAgent{gateway: gw}
	_, err := agent.ListSessions(ListSessionsRequest{
		Meta: map[string]interface{}{
			"limit": float64(25),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gw.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(gw.calls))
	}
}

func TestListSessions_DefaultLimit(t *testing.T) {
	gw := &mockGatewayRequester{
		handler: func(method string, params interface{}) (interface{}, error) {
			m, _ := json.Marshal(params)
			var p map[string]interface{}
			_ = json.Unmarshal(m, &p)
			if limit, ok := p["limit"]; ok {
				if limit != float64(100) {
					t.Errorf("expected default limit 100, got %v", limit)
				}
			}
			return map[string]interface{}{"sessions": []interface{}{}}, nil
		},
	}

	agent := &AcpGatewayAgent{gateway: gw}
	_, err := agent.ListSessions(ListSessionsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
