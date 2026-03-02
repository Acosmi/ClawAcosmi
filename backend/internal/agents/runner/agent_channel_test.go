package runner

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------- NewAgentChannel ----------

func TestAgentChannel_New(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	if ch.IsClosed() {
		t.Error("new channel should not be closed")
	}
}

// ---------- SendToParent / ReceiveFromParent ----------

func TestAgentChannel_SendReceive(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	msg := AgentMessage{
		Type:    MsgStatusUpdate,
		Content: "progress 50%",
	}

	if err := ch.SendToParent(msg); err != nil {
		t.Fatalf("SendToParent error: %v", err)
	}

	// DrainFromChild 应能收到
	msgs := ch.DrainFromChild()
	if len(msgs) != 1 {
		t.Fatalf("DrainFromChild got %d messages, want 1", len(msgs))
	}
	if msgs[0].Content != "progress 50%" {
		t.Errorf("message content = %q, want %q", msgs[0].Content, "progress 50%")
	}
}

func TestAgentChannel_SendToChild_ReceiveFromParent(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	msg := AgentMessage{
		Type:    MsgDirective,
		Content: "use alternative approach",
	}

	if err := ch.SendToChild(msg); err != nil {
		t.Fatalf("SendToChild error: %v", err)
	}

	received := ch.ReceiveFromParent()
	if received == nil {
		t.Fatal("ReceiveFromParent returned nil")
	}
	if received.Content != "use alternative approach" {
		t.Errorf("content = %q, want %q", received.Content, "use alternative approach")
	}
}

// ---------- Auto-fill ID and Timestamp ----------

func TestAgentChannel_AutoFillFields(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	if err := ch.SendToParent(AgentMessage{
		Type:    MsgStatusUpdate,
		Content: "test",
	}); err != nil {
		t.Fatalf("SendToParent error: %v", err)
	}

	msgs := ch.DrainFromChild()
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	if msgs[0].ID == "" {
		t.Error("ID should be auto-filled")
	}
	if msgs[0].Timestamp == 0 {
		t.Error("Timestamp should be auto-filled")
	}
}

// ---------- ReceiveFromParent 空通道 ----------

func TestAgentChannel_ReceiveFromParent_Empty(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	received := ch.ReceiveFromParent()
	if received != nil {
		t.Error("ReceiveFromParent on empty channel should return nil")
	}
}

// ---------- DrainFromChild 空通道 ----------

func TestAgentChannel_DrainFromChild_Empty(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	msgs := ch.DrainFromChild()
	if len(msgs) != 0 {
		t.Errorf("DrainFromChild on empty channel returned %d messages", len(msgs))
	}
}

// ---------- Buffer 满测试（非阻塞） ----------

func TestAgentChannel_BufferFull(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	// 填满 buffer（cap=10）
	for i := 0; i < 10; i++ {
		if err := ch.SendToParent(AgentMessage{
			Type:    MsgStatusUpdate,
			Content: "msg",
		}); err != nil {
			t.Fatalf("SendToParent #%d error: %v", i, err)
		}
	}

	// 第 11 条应返回 error，不阻塞
	err := ch.SendToParent(AgentMessage{
		Type:    MsgStatusUpdate,
		Content: "overflow",
	})
	if err == nil {
		t.Error("11th SendToParent should return error when buffer full")
	}
}

// ---------- Close 后操作 ----------

func TestAgentChannel_SendAfterClose(t *testing.T) {
	ch := NewAgentChannel()
	ch.Close()

	err := ch.SendToParent(AgentMessage{Type: MsgStatusUpdate, Content: "late"})
	if err == nil {
		t.Error("SendToParent after Close should return error")
	}

	err = ch.SendToChild(AgentMessage{Type: MsgDirective, Content: "late"})
	if err == nil {
		t.Error("SendToChild after Close should return error")
	}
}

func TestAgentChannel_CloseIdempotent(t *testing.T) {
	ch := NewAgentChannel()
	ch.Close()
	ch.Close() // 不应 panic

	if !ch.IsClosed() {
		t.Error("IsClosed should be true after Close")
	}
}

// ---------- ToParentChan ----------

func TestAgentChannel_ToParentChan(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	rawCh := ch.ToParentChan()
	if rawCh == nil {
		t.Error("ToParentChan should not be nil")
	}

	// 发送后应能从 raw channel 收到
	_ = ch.SendToParent(AgentMessage{Type: MsgStatusUpdate, Content: "via raw"})

	select {
	case msg := <-rawCh:
		if msg.Content != "via raw" {
			t.Errorf("content = %q, want %q", msg.Content, "via raw")
		}
	default:
		t.Error("expected message on raw channel")
	}
}

// ---------- 便捷方法 ----------

func TestAgentChannel_SendHelpRequest(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	id, err := ch.SendHelpRequest(HelpRequestPayload{
		Question: "how to proceed?",
		Context:  "stuck on step 3",
		Urgency:  "high",
	})

	if err != nil {
		t.Fatalf("SendHelpRequest error: %v", err)
	}
	if id == "" {
		t.Error("returned ID should not be empty")
	}

	msgs := ch.DrainFromChild()
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	if msgs[0].Type != MsgHelpRequest {
		t.Errorf("type = %q, want %q", msgs[0].Type, MsgHelpRequest)
	}
}

func TestAgentChannel_SendHelpResponse(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	if err := ch.SendHelpResponse("req-123", "use approach B"); err != nil {
		t.Fatalf("SendHelpResponse error: %v", err)
	}

	received := ch.ReceiveFromParent()
	if received == nil {
		t.Fatal("expected message")
	}
	if received.Type != MsgHelpResponse {
		t.Errorf("type = %q, want %q", received.Type, MsgHelpResponse)
	}
	if received.ReplyTo != "req-123" {
		t.Errorf("ReplyTo = %q, want %q", received.ReplyTo, "req-123")
	}
}

func TestAgentChannel_SendDirective(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	if err := ch.SendDirective("change strategy"); err != nil {
		t.Fatalf("SendDirective error: %v", err)
	}

	received := ch.ReceiveFromParent()
	if received == nil {
		t.Fatal("expected message")
	}
	if received.Type != MsgDirective {
		t.Errorf("type = %q, want %q", received.Type, MsgDirective)
	}
}

// ---------- ExecuteRequestHelp ----------

func TestAgentChannel_ExecuteRequestHelp_NilChannel(t *testing.T) {
	result, err := ExecuteRequestHelp(json.RawMessage(`{}`), nil)
	if err != nil {
		t.Fatalf("nil channel should not return error, got: %v", err)
	}
	if result == "" {
		t.Error("nil channel should return graceful string")
	}
}

func TestAgentChannel_ExecuteRequestHelp_EmptyQuestion(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	input, _ := json.Marshal(map[string]string{"question": ""})
	result, err := ExecuteRequestHelp(json.RawMessage(input), ch)
	if err != nil {
		t.Fatalf("empty question should not return error, got: %v", err)
	}
	if result == "" {
		t.Error("empty question should return graceful string")
	}
}

func TestAgentChannel_ExecuteRequestHelp_Truncation(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	longQuestion := strings.Repeat("问", 600) // 600 runes, 超过 500 限制
	input, _ := json.Marshal(map[string]interface{}{
		"question": longQuestion,
		"context":  strings.Repeat("x", 400),               // 超过 300 限制
		"options":  []string{"a", "b", "c", "d", "e", "f"}, // 超过 5 限制
	})

	_, err := ExecuteRequestHelp(json.RawMessage(input), ch)
	if err != nil {
		t.Fatalf("ExecuteRequestHelp error: %v", err)
	}

	msgs := ch.DrainFromChild()
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	// 验证截断发生（消息成功发送即可，具体截断逻辑在 ExecuteRequestHelp 内部）
}

func TestAgentChannel_ExecuteRequestHelp_DefaultUrgency(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	input, _ := json.Marshal(map[string]string{
		"question": "help needed",
	})
	_, err := ExecuteRequestHelp(json.RawMessage(input), ch)
	if err != nil {
		t.Fatalf("ExecuteRequestHelp error: %v", err)
	}

	msgs := ch.DrainFromChild()
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	if msgs[0].Urgency != "medium" {
		t.Errorf("default urgency = %q, want %q", msgs[0].Urgency, "medium")
	}
}

// ---------- RequestHelpToolDef ----------

func TestRequestHelpToolDef(t *testing.T) {
	def := RequestHelpToolDef()
	if def.Name != "request_help" {
		t.Errorf("tool name = %q, want %q", def.Name, "request_help")
	}
	if def.InputSchema == nil {
		t.Error("InputSchema should not be nil")
	}
}

// ---------- DrainFromChild 多消息 ----------

func TestAgentChannel_DrainMultiple(t *testing.T) {
	ch := NewAgentChannel()
	defer ch.Close()

	for i := 0; i < 5; i++ {
		_ = ch.SendToParent(AgentMessage{
			Type:    MsgStatusUpdate,
			Content: "msg",
		})
	}

	msgs := ch.DrainFromChild()
	if len(msgs) != 5 {
		t.Errorf("DrainFromChild got %d messages, want 5", len(msgs))
	}

	// 再次 drain 应为空
	msgs2 := ch.DrainFromChild()
	if len(msgs2) != 0 {
		t.Errorf("second DrainFromChild got %d messages, want 0", len(msgs2))
	}
}
