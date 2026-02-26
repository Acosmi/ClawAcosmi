package session

import (
	"path/filepath"
	"testing"
)

// ---------- TestValidateHistoryMessages ----------

func TestValidateHistoryMessages_Valid(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": []interface{}{
			map[string]interface{}{"type": "text", "text": "hi"},
		}},
	}

	result := ValidateHistoryMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("有效消息数 = %d, want 2", len(result))
	}
}

func TestValidateHistoryMessages_InvalidRole(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "ok"},
		{"role": "invalid_role", "content": "bad"},
		{"role": "assistant", "content": "good"},
	}

	result := ValidateHistoryMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("应过滤 invalid_role, got %d messages", len(result))
	}
}

func TestValidateHistoryMessages_MissingRole(t *testing.T) {
	msgs := []map[string]interface{}{
		{"content": "no role at all"},
		{"role": "user", "content": "has role"},
	}

	result := ValidateHistoryMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("应过滤无 role 消息, got %d", len(result))
	}
}

func TestValidateHistoryMessages_EmptyContent(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": ""},
		{"role": "user", "content": "ok"},
	}

	result := ValidateHistoryMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("应过滤空 content, got %d", len(result))
	}
}

func TestValidateHistoryMessages_StringToArray(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "simple string"},
	}

	result := ValidateHistoryMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("消息数 = %d, want 1", len(result))
	}

	// 验证 content 被转换为数组
	content, ok := result[0]["content"].([]interface{})
	if !ok {
		t.Fatal("content 应被转换为 []interface{}")
	}
	if len(content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(content))
	}

	block, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("content block 应为 map")
	}
	if block["text"] != "simple string" {
		t.Errorf("text = %v, want %q", block["text"], "simple string")
	}
}

func TestValidateHistoryMessages_TextFieldFallback(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "text": "old format text"},
	}

	result := ValidateHistoryMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("text 字段回退应保留, got %d", len(result))
	}

	// 应有 content 而无 text
	if _, hasText := result[0]["text"]; hasText {
		t.Error("text 字段应被移除")
	}
	if _, hasContent := result[0]["content"]; !hasContent {
		t.Error("应生成 content 字段")
	}
}

func TestValidateHistoryMessages_Empty(t *testing.T) {
	result := ValidateHistoryMessages(nil)
	if result != nil {
		t.Errorf("nil 输入应返回 nil, got %v", result)
	}

	result = ValidateHistoryMessages([]map[string]interface{}{})
	if result != nil {
		t.Errorf("空切片应返回 nil, got %v", result)
	}
}

// ---------- TestTruncateByTokenBudget ----------

func TestTruncateByTokenBudget_WithinBudget(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hi"},
		{"role": "assistant", "content": "hello"},
	}

	result := TruncateByTokenBudget(msgs, 100000)
	if len(result) != 2 {
		t.Errorf("预算内应全部保留, got %d", len(result))
	}
}

func TestTruncateByTokenBudget_ExceedsBudget(t *testing.T) {
	// 创建很多消息
	msgs := make([]map[string]interface{}, 50)
	for i := 0; i < 50; i++ {
		msgs[i] = map[string]interface{}{
			"role":    "user",
			"content": "This is a fairly long message that takes up some tokens in the context window. It contains enough text to be meaningful.",
		}
	}

	result := TruncateByTokenBudget(msgs, 100) // 很小的预算
	if len(result) >= 50 {
		t.Errorf("应截断消息, got %d (原始 50)", len(result))
	}
	if len(result) == 0 {
		t.Error("至少保留一些消息")
	}
}

func TestTruncateByTokenBudget_PreservesLatest(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "first message"},
		{"role": "assistant", "content": "second message"},
		{"role": "user", "content": "third message"},
	}

	// 给一个只够容纳 1-2 条消息的预算
	result := TruncateByTokenBudget(msgs, 15)
	if len(result) == 0 {
		t.Fatal("至少保留一条消息")
	}
	// 最后一条消息应是 "third message"
	lastMsg := result[len(result)-1]
	if lastMsg["content"] != "third message" {
		t.Errorf("最新消息应被保留, got %v", lastMsg["content"])
	}
}

func TestTruncateByTokenBudget_ZeroBudget(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
	}
	result := TruncateByTokenBudget(msgs, 0)
	if len(result) != 1 {
		t.Errorf("zero budget 应返回全部, got %d", len(result))
	}
}

// ---------- TestEstimateMessageTokens ----------

func TestEstimateMessageTokens_StringContent(t *testing.T) {
	msg := map[string]interface{}{
		"role":    "user",
		"content": "Hello, this is a test message",
	}
	tokens := EstimateMessageTokens(msg)
	if tokens < 5 {
		t.Errorf("token 估算过低: %d", tokens)
	}
}

func TestEstimateMessageTokens_ArrayContent(t *testing.T) {
	msg := map[string]interface{}{
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Hello world"},
			map[string]interface{}{"type": "text", "text": "More content here"},
		},
	}
	tokens := EstimateMessageTokens(msg)
	if tokens < 5 {
		t.Errorf("token 估算过低: %d", tokens)
	}
}

// ---------- TestPrepareMessagesForAttempt ----------

func TestPrepareMessagesForAttempt_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "e2e.jsonl")
	mgr := NewSessionManager("")

	// 创建文件并写入消息
	if _, err := mgr.EnsureSessionFile("e2e", sessionFile); err != nil {
		t.Fatal(err)
	}

	msgs := []TranscriptEntry{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Q1"}}},
		{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "A1"}}},
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Q2"}}},
	}
	for _, m := range msgs {
		if err := mgr.AppendMessage("e2e", sessionFile, m); err != nil {
			t.Fatal(err)
		}
	}

	// 准备消息
	result, err := PrepareMessagesForAttempt(mgr, PrepareParams{
		SessionID:    "e2e",
		SessionFile:  sessionFile,
		MaxTokens:    100000,
		SystemTokens: 1000,
	})
	if err != nil {
		t.Fatalf("PrepareMessagesForAttempt 失败: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("消息数 = %d, want 3", len(result))
	}
}

func TestPrepareMessagesForAttempt_EmptySession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "empty.jsonl")
	mgr := NewSessionManager("")

	if _, err := mgr.EnsureSessionFile("empty", sessionFile); err != nil {
		t.Fatal(err)
	}

	result, err := PrepareMessagesForAttempt(mgr, PrepareParams{
		SessionID:   "empty",
		SessionFile: sessionFile,
		MaxTokens:   100000,
	})
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if result != nil {
		t.Errorf("空 session 应返回 nil, got %d messages", len(result))
	}
}

func TestPrepareMessagesForAttempt_FileNotFound(t *testing.T) {
	mgr := NewSessionManager("")

	result, err := PrepareMessagesForAttempt(mgr, PrepareParams{
		SessionID:   "missing",
		SessionFile: "/tmp/does-not-exist-ever.jsonl",
		MaxTokens:   100000,
	})
	if err != nil {
		t.Fatalf("文件不存在时不应 error: %v", err)
	}
	if result != nil {
		t.Errorf("文件不存在应返回 nil")
	}
}
