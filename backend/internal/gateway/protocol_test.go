package gateway

import (
	"encoding/json"
	"testing"
)

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion != 3 {
		t.Errorf("ProtocolVersion = %d, want 3", ProtocolVersion)
	}
}

func TestRequestFrame_JSON(t *testing.T) {
	frame := RequestFrame{
		Type:   FrameTypeRequest,
		ID:     "req-1",
		Method: "chat.send",
		Params: map[string]string{"text": "hello"},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded RequestFrame
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "req" || decoded.ID != "req-1" || decoded.Method != "chat.send" {
		t.Errorf("decoded = %+v", decoded)
	}
}

func TestResponseFrame_JSON(t *testing.T) {
	frame := ResponseFrame{
		Type:    FrameTypeResponse,
		ID:      "req-1",
		OK:      true,
		Payload: map[string]string{"status": "ok"},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ResponseFrame
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.OK || decoded.ID != "req-1" {
		t.Errorf("decoded = %+v", decoded)
	}
}

func TestResponseFrame_Error(t *testing.T) {
	frame := ResponseFrame{
		Type:  FrameTypeResponse,
		ID:    "req-2",
		OK:    false,
		Error: NewErrorShape(ErrCodeBadRequest, "invalid params"),
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ResponseFrame
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.OK {
		t.Error("expected ok=false")
	}
	if decoded.Error == nil || decoded.Error.Code != ErrCodeBadRequest {
		t.Errorf("error = %+v", decoded.Error)
	}
}

func TestEventFrame_JSON(t *testing.T) {
	seq := int64(42)
	frame := EventFrame{
		Type:  FrameTypeEvent,
		Event: "chat.delta",
		Seq:   &seq,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded EventFrame
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Event != "chat.delta" || decoded.Seq == nil || *decoded.Seq != 42 {
		t.Errorf("decoded = %+v", decoded)
	}
}

func TestErrorShape_Builder(t *testing.T) {
	err := NewErrorShape(ErrCodeTooManyRequests, "rate limited").
		WithRetryable(5000).
		WithDetails(map[string]int{"remaining": 0})

	if err.Code != ErrCodeTooManyRequests {
		t.Errorf("code = %s", err.Code)
	}
	if err.Retryable == nil || !*err.Retryable {
		t.Error("expected retryable=true")
	}
	if err.RetryAfterMs == nil || *err.RetryAfterMs != 5000 {
		t.Errorf("retryAfterMs = %v", err.RetryAfterMs)
	}
	if err.Details == nil {
		t.Error("expected details")
	}
}

func TestConnectParamsFull_JSON(t *testing.T) {
	params := ConnectParamsFull{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client: ConnectClientInfo{
			ID:       "openacosmi-desktop",
			Version:  "1.0.0",
			Platform: "macos",
			Mode:     "operator",
		},
		Role:   "operator",
		Scopes: []string{"operator.admin"},
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ConnectParamsFull
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Client.ID != "openacosmi-desktop" || decoded.Role != "operator" {
		t.Errorf("decoded = %+v", decoded)
	}
}

func TestHelloOk_JSON(t *testing.T) {
	hello := HelloOk{
		Type:     FrameTypeHelloOk,
		Protocol: ProtocolVersion,
		Server: HelloOkServer{
			Version: "1.0.0",
			ConnID:  "conn-abc",
		},
		Features: HelloOkFeatures{
			Methods: []string{"chat.send", "config.get"},
			Events:  []string{"chat.delta", "tick"},
		},
		Policy: DefaultHelloOkPolicy(),
	}
	data, err := json.Marshal(hello)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded HelloOk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Protocol != 3 {
		t.Errorf("protocol = %d", decoded.Protocol)
	}
	if decoded.Policy.MaxPayload != MaxPayloadBytes {
		t.Errorf("maxPayload = %d", decoded.Policy.MaxPayload)
	}
}

func TestDefaultHelloOkPolicy(t *testing.T) {
	p := DefaultHelloOkPolicy()
	if p.MaxPayload != MaxPayloadBytes {
		t.Errorf("maxPayload = %d", p.MaxPayload)
	}
	if p.MaxBufferedBytes != MaxBufferedBytes {
		t.Errorf("maxBufferedBytes = %d", p.MaxBufferedBytes)
	}
	if p.TickIntervalMs != 30000 {
		t.Errorf("tickIntervalMs = %d", p.TickIntervalMs)
	}
}
