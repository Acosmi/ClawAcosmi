package tui

import (
	"testing"
	"time"
)

func TestPruneRunMapUnderLimit(t *testing.T) {
	runs := make(map[string]int64)
	for i := 0; i < 200; i++ {
		runs[string(rune('a'+i%26))+string(rune('0'+i/26))] = time.Now().UnixMilli()
	}
	pruneRunMap(runs)
	if len(runs) != 200 {
		t.Errorf("under limit: got %d, want 200", len(runs))
	}
}

func TestPruneRunMapExpired(t *testing.T) {
	runs := make(map[string]int64)
	expired := time.Now().UnixMilli() - 11*60*1000 // 11 分钟前

	// 150 条过期 + 60 条新鲜 = 210 条
	for i := 0; i < 150; i++ {
		runs["expired-"+string(rune('a'+i%26))+string(rune('0'+i/26))] = expired
	}
	for i := 0; i < 60; i++ {
		runs["fresh-"+string(rune('a'+i%26))+string(rune('0'+i/26))] = time.Now().UnixMilli()
	}

	pruneRunMap(runs)

	if len(runs) > 200 {
		t.Errorf("after prune: got %d, want ≤200", len(runs))
	}
}

func TestPruneRunMapForceDelete(t *testing.T) {
	runs := make(map[string]int64)
	recent := time.Now().UnixMilli()

	// 210 条全部新鲜 — 无法通过过期删除
	for i := 0; i < 210; i++ {
		runs["r-"+string(rune('a'+i%26))+string(rune('0'+i/26))+string(rune('A'+i%10))] = recent
	}

	pruneRunMap(runs)

	if len(runs) > 150 {
		t.Errorf("force prune: got %d, want ≤150", len(runs))
	}
}

func TestParseChatEvent(t *testing.T) {
	record := map[string]interface{}{
		"sessionKey":   "agent:main:default",
		"runId":        "run-123",
		"state":        "delta",
		"message":      map[string]interface{}{"content": "hello"},
		"errorMessage": "something went wrong",
	}
	evt := parseChatEvent(record)

	if evt.SessionKey != "agent:main:default" {
		t.Errorf("SessionKey: got %q", evt.SessionKey)
	}
	if evt.RunID != "run-123" {
		t.Errorf("RunID: got %q", evt.RunID)
	}
	if evt.State != "delta" {
		t.Errorf("State: got %q", evt.State)
	}
	if evt.ErrorMessage != "something went wrong" {
		t.Errorf("ErrorMessage: got %q", evt.ErrorMessage)
	}
	if evt.Message == nil {
		t.Error("Message should not be nil")
	}
}

func TestParseChatEventMissingFields(t *testing.T) {
	record := map[string]interface{}{}
	evt := parseChatEvent(record)

	if evt.SessionKey != "" {
		t.Errorf("SessionKey: got %q, want empty", evt.SessionKey)
	}
	if evt.RunID != "" {
		t.Errorf("RunID: got %q, want empty", evt.RunID)
	}
	if evt.State != "" {
		t.Errorf("State: got %q, want empty", evt.State)
	}
}

func TestParseAgentEvent(t *testing.T) {
	record := map[string]interface{}{
		"runId":  "run-456",
		"stream": "tool",
		"data": map[string]interface{}{
			"phase":      "start",
			"toolCallId": "tc-1",
			"name":       "read_file",
		},
	}
	evt := parseAgentEvent(record)

	if evt.RunID != "run-456" {
		t.Errorf("RunID: got %q", evt.RunID)
	}
	if evt.Stream != "tool" {
		t.Errorf("Stream: got %q", evt.Stream)
	}
	if evt.Data == nil {
		t.Fatal("Data should not be nil")
	}
	if evt.Data["phase"] != "start" {
		t.Errorf("Data.phase: got %v", evt.Data["phase"])
	}
}

func TestParseAgentEventMissingData(t *testing.T) {
	record := map[string]interface{}{
		"runId":  "run-789",
		"stream": "lifecycle",
	}
	evt := parseAgentEvent(record)

	if evt.RunID != "run-789" {
		t.Errorf("RunID: got %q", evt.RunID)
	}
	if evt.Data != nil {
		t.Errorf("Data: got %v, want nil", evt.Data)
	}
}
