package gateway

import (
	"encoding/json"
	"testing"
)

// ---------- 事件常量 ----------

func TestTaskEventConstants(t *testing.T) {
	expected := map[string]string{
		"task.queued":    EventTaskQueued,
		"task.started":   EventTaskStarted,
		"task.progress":  EventTaskProgress,
		"task.completed": EventTaskCompleted,
		"task.failed":    EventTaskFailed,
	}
	for want, got := range expected {
		if got != want {
			t.Errorf("constant mismatch: got %q, want %q", got, want)
		}
	}
}

// ---------- 序列化 ----------

func TestTaskQueuedEvent_JSON(t *testing.T) {
	evt := TaskQueuedEvent{
		TaskID:     "run_123",
		SessionKey: "agent:main",
		Text:       "帮我写个函数",
		Ts:         1709337600000,
		Async:      true,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded TaskQueuedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.TaskID != evt.TaskID {
		t.Errorf("taskId = %q, want %q", decoded.TaskID, evt.TaskID)
	}
	if decoded.SessionKey != evt.SessionKey {
		t.Errorf("sessionKey = %q, want %q", decoded.SessionKey, evt.SessionKey)
	}
	if decoded.Text != evt.Text {
		t.Errorf("text = %q, want %q", decoded.Text, evt.Text)
	}
	if decoded.Ts != evt.Ts {
		t.Errorf("ts = %d, want %d", decoded.Ts, evt.Ts)
	}
	if decoded.Async != evt.Async {
		t.Errorf("async = %v, want %v", decoded.Async, evt.Async)
	}
}

func TestTaskStartedEvent_JSON(t *testing.T) {
	evt := TaskStartedEvent{
		TaskID:     "run_456",
		SessionKey: "agent:main",
		Ts:         1709337600000,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded TaskStartedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.TaskID != evt.TaskID {
		t.Errorf("taskId = %q, want %q", decoded.TaskID, evt.TaskID)
	}
	if decoded.SessionKey != evt.SessionKey {
		t.Errorf("sessionKey = %q, want %q", decoded.SessionKey, evt.SessionKey)
	}
	if decoded.Ts != evt.Ts {
		t.Errorf("ts = %d, want %d", decoded.Ts, evt.Ts)
	}
}

func TestTaskProgressEvent_JSON(t *testing.T) {
	evt := TaskProgressEvent{
		TaskID:     "run_789",
		SessionKey: "agent:main",
		ToolName:   "bash",
		ToolID:     "call_abc",
		Phase:      "end",
		Text:       "[结果] ls completed (42ms)",
		IsError:    false,
		Duration:   42,
		Ts:         1709337600000,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded TaskProgressEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.TaskID != evt.TaskID {
		t.Errorf("taskId = %q", decoded.TaskID)
	}
	if decoded.ToolName != "bash" {
		t.Errorf("toolName = %q", decoded.ToolName)
	}
	if decoded.ToolID != "call_abc" {
		t.Errorf("toolId = %q", decoded.ToolID)
	}
	if decoded.Phase != "end" {
		t.Errorf("phase = %q", decoded.Phase)
	}
	if decoded.IsError {
		t.Error("isError should be false")
	}
	if decoded.Duration != 42 {
		t.Errorf("duration = %d", decoded.Duration)
	}
}

func TestTaskProgressEvent_ErrorPhase(t *testing.T) {
	evt := TaskProgressEvent{
		TaskID:   "run_err",
		ToolName: "bash",
		Phase:    "end",
		Text:     "[错误] command not found",
		IsError:  true,
		Duration: 100,
		Ts:       1709337600000,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded TaskProgressEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !decoded.IsError {
		t.Error("isError should be true")
	}
}

func TestTaskCompletedEvent_JSON(t *testing.T) {
	evt := TaskCompletedEvent{
		TaskID:     "run_done",
		SessionKey: "agent:main",
		Summary:    "函数已写好",
		Ts:         1709337600000,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded TaskCompletedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.TaskID != evt.TaskID {
		t.Errorf("taskId = %q", decoded.TaskID)
	}
	if decoded.Summary != evt.Summary {
		t.Errorf("summary = %q", decoded.Summary)
	}
}

func TestTaskCompletedEvent_EmptySummary(t *testing.T) {
	evt := TaskCompletedEvent{
		TaskID: "run_nosummary",
		Ts:     1709337600000,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	// summary 是 omitempty，空值应该不出现在 JSON 中
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, hasSummary := raw["summary"]; hasSummary {
		t.Error("empty summary should be omitted from JSON")
	}
}

func TestTaskFailedEvent_JSON(t *testing.T) {
	evt := TaskFailedEvent{
		TaskID:     "run_fail",
		SessionKey: "agent:main",
		Error:      "pipeline timeout",
		Ts:         1709337600000,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded TaskFailedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Error != "pipeline timeout" {
		t.Errorf("error = %q", decoded.Error)
	}
}

// ---------- defaultGatewayEvents 包含 task.* ----------

func TestDefaultGatewayEvents_ContainsTaskEvents(t *testing.T) {
	events := defaultGatewayEvents()
	required := []string{
		EventTaskQueued,
		EventTaskStarted,
		EventTaskProgress,
		EventTaskCompleted,
		EventTaskFailed,
	}
	eventSet := make(map[string]bool, len(events))
	for _, e := range events {
		eventSet[e] = true
	}
	for _, req := range required {
		if !eventSet[req] {
			t.Errorf("defaultGatewayEvents missing %q", req)
		}
	}
}
