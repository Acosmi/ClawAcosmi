package gateway

import (
	"testing"
)

func TestParseVoiceTranscript_Valid(t *testing.T) {
	text, sk, err := ParseVoiceTranscript(`{"text":"hello world","sessionKey":"main"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q", text)
	}
	if sk != "main" {
		t.Errorf("sessionKey = %q", sk)
	}
}

func TestParseVoiceTranscript_Empty(t *testing.T) {
	_, _, err := ParseVoiceTranscript("")
	if err == nil {
		t.Error("empty should error")
	}
	_, _, err = ParseVoiceTranscript(`{"text":""}`)
	if err == nil {
		t.Error("empty text should error")
	}
}

func TestParseAgentRequest_Valid(t *testing.T) {
	link, err := ParseAgentRequest(`{"message":"do something","channel":"discord"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Message != "do something" {
		t.Errorf("message = %q", link.Message)
	}
}

func TestParseAgentRequest_Empty(t *testing.T) {
	_, err := ParseAgentRequest(`{"message":""}`)
	if err == nil {
		t.Error("empty message should error")
	}
}

func TestNodeEventDispatcher(t *testing.T) {
	d := NewNodeEventDispatcher()
	var called bool
	d.Register("test.event", func(ctx *NodeEventContext, nodeID string, evt *NodeEvent) error {
		called = true
		if nodeID != "node1" {
			t.Errorf("nodeID = %q", nodeID)
		}
		return nil
	})
	err := d.Dispatch(&NodeEventContext{}, "node1", &NodeEvent{Event: "test.event"})
	if err != nil {
		t.Errorf("dispatch error: %v", err)
	}
	if !called {
		t.Error("handler not called")
	}
	// 未注册的事件静默通过
	err = d.Dispatch(&NodeEventContext{}, "node1", &NodeEvent{Event: "unknown"})
	if err != nil {
		t.Errorf("unknown event should not error: %v", err)
	}
}
