package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"Argus-compound/go-sensory/internal/input"
)

// ──────────────────────────────────────────────────────────────
// Mock InputController
// ──────────────────────────────────────────────────────────────

type mockInput struct {
	lastAction   string
	lastX, lastY int
	lastText     string
	lastKeys     []input.Key
	err          error
}

func (m *mockInput) Click(x, y int, b input.MouseButton) error {
	m.lastAction = "click"
	m.lastX = x
	m.lastY = y
	return m.err
}
func (m *mockInput) DoubleClick(x, y int) error {
	m.lastAction = "double_click"
	m.lastX = x
	m.lastY = y
	return m.err
}
func (m *mockInput) MoveTo(x, y int) error            { return m.err }
func (m *mockInput) MoveToSmooth(x, y, dur int) error { return m.err }
func (m *mockInput) Drag(x1, y1, x2, y2 int) error    { return m.err }
func (m *mockInput) Type(text string) error           { m.lastAction = "type"; m.lastText = text; return m.err }
func (m *mockInput) KeyDown(key input.Key) error      { return m.err }
func (m *mockInput) KeyUp(key input.Key) error        { return m.err }
func (m *mockInput) KeyPress(key input.Key) error {
	m.lastAction = "press_key"
	m.lastKeys = []input.Key{key}
	return m.err
}
func (m *mockInput) Hotkey(keys ...input.Key) error {
	m.lastAction = "hotkey"
	m.lastKeys = keys
	return m.err
}
func (m *mockInput) Scroll(x, y, dx, dy int) error {
	m.lastAction = "scroll"
	m.lastX = x
	m.lastY = y
	return m.err
}
func (m *mockInput) GetMousePosition() (int, int, error) { return 123, 456, m.err }

// Mock ApprovalNotifier that always approves
type alwaysApprove struct{}

func (a *alwaysApprove) RequestApproval(_ context.Context, _ input.ApprovalRequest) (input.ApprovalResponse, error) {
	return input.ApprovalResponse{Approved: true, Reason: "auto-test"}, nil
}

// Mock notifier that always denies
type alwaysDeny struct{}

func (a *alwaysDeny) RequestApproval(_ context.Context, _ input.ApprovalRequest) (input.ApprovalResponse, error) {
	return input.ApprovalResponse{Approved: false, Reason: "denied-test"}, nil
}

// Mock notifier that modifies params
type modifyNotifier struct {
	modified json.RawMessage
}

func (m *modifyNotifier) RequestApproval(_ context.Context, _ input.ApprovalRequest) (input.ApprovalResponse, error) {
	return input.ApprovalResponse{Approved: true, ModifiedParams: m.modified}, nil
}

func newTestGateway(notifier input.ApprovalNotifier) *input.ApprovalGateway {
	return input.NewApprovalGateway(input.GatewayConfig{
		Enabled:  true,
		Notifier: notifier,
	})
}

func newAutoModeGateway() *input.ApprovalGateway {
	return input.NewApprovalGateway(input.GatewayConfig{
		Enabled:  true,
		AutoMode: true,
	})
}

// ──────────────────────────────────────────────────────────────
// Tests: Registration
// ──────────────────────────────────────────────────────────────

func TestRegisterActionTools(t *testing.T) {
	r := NewRegistry()
	deps := ActionDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}
	RegisterActionTools(r, deps)

	expected := []string{"click", "double_click", "type_text", "press_key", "hotkey", "scroll", "mouse_position"}
	for _, name := range expected {
		if r.Get(name) == nil {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Click
// ──────────────────────────────────────────────────────────────

func TestClick_AutoMode(t *testing.T) {
	mi := &mockInput{}
	tool := &ClickTool{deps: ActionDeps{Input: mi, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"x":100,"y":200,"button":0}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastAction != "click" || mi.lastX != 100 || mi.lastY != 200 {
		t.Errorf("Expected click(100,200), got %s(%d,%d)", mi.lastAction, mi.lastX, mi.lastY)
	}
}

func TestClick_Denied(t *testing.T) {
	tool := &ClickTool{deps: ActionDeps{
		Input:   &mockInput{},
		Gateway: newTestGateway(&alwaysDeny{}),
	}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"x":100,"y":200}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected denial error")
	}
}

func TestClick_InvalidParams(t *testing.T) {
	tool := &ClickTool{deps: ActionDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`invalid`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected param parse error")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: TypeText
// ──────────────────────────────────────────────────────────────

func TestTypeText_AutoMode(t *testing.T) {
	mi := &mockInput{}
	tool := &TypeTextTool{deps: ActionDeps{Input: mi, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"hello world"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastText != "hello world" {
		t.Errorf("Typed %q, want 'hello world'", mi.lastText)
	}
}

func TestTypeText_ModifiedByHuman(t *testing.T) {
	mi := &mockInput{}
	tool := &TypeTextTool{deps: ActionDeps{
		Input:   mi,
		Gateway: newTestGateway(&modifyNotifier{modified: json.RawMessage(`{"text":"safe text"}`)}),
	}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"sudo rm -rf /"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	// Human modified the dangerous text
	if mi.lastText != "safe text" {
		t.Errorf("Typed %q, want 'safe text' (human-modified)", mi.lastText)
	}
}

func TestTypeText_EmptyText(t *testing.T) {
	tool := &TypeTextTool{deps: ActionDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"text":""}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty text")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Hotkey
// ──────────────────────────────────────────────────────────────

func TestHotkey_AutoMode(t *testing.T) {
	mi := &mockInput{}
	tool := &HotkeyTool{deps: ActionDeps{Input: mi, Gateway: newAutoModeGateway()}}

	// Cmd+C
	keys := []uint16{uint16(input.KeyCommand), uint16(input.KeyC)}
	params, _ := json.Marshal(map[string][]uint16{"keys": keys})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastAction != "hotkey" {
		t.Errorf("Action = %s, want hotkey", mi.lastAction)
	}
}

func TestHotkey_EmptyKeys(t *testing.T) {
	tool := &HotkeyTool{deps: ActionDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"keys":[]}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty keys")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Scroll
// ──────────────────────────────────────────────────────────────

func TestScroll_AutoApproved(t *testing.T) {
	mi := &mockInput{}
	tool := &ScrollTool{deps: ActionDeps{Input: mi, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"x":500,"y":300,"delta_y":-3}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastAction != "scroll" {
		t.Errorf("Action = %s, want scroll", mi.lastAction)
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: MousePosition
// ──────────────────────────────────────────────────────────────

func TestMousePosition_Success(t *testing.T) {
	tool := &MousePositionTool{deps: ActionDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["x"] != 123 || content["y"] != 456 {
		t.Errorf("Position = (%v,%v), want (123,456)", content["x"], content["y"])
	}
}

func TestMousePosition_Error(t *testing.T) {
	tool := &MousePositionTool{deps: ActionDeps{
		Input:   &mockInput{err: fmt.Errorf("no display")},
		Gateway: newAutoModeGateway(),
	}}

	_, err := tool.Execute(context.Background(), nil)
	if err == nil {
		t.Error("Expected error from GetMousePosition")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Risk Levels
// ──────────────────────────────────────────────────────────────

func TestActionTools_RiskLevels(t *testing.T) {
	tests := []struct {
		tool Tool
		want RiskLevel
	}{
		{&ClickTool{}, RiskMedium},
		{&DoubleClickTool{}, RiskMedium},
		{&TypeTextTool{}, RiskMedium},
		{&PressKeyTool{}, RiskMedium},
		{&HotkeyTool{}, RiskMedium},
		{&ScrollTool{}, RiskLow},
		{&MousePositionTool{}, RiskLow},
	}
	for _, tt := range tests {
		if tt.tool.Risk() != tt.want {
			t.Errorf("%s.Risk() = %v, want %v", tt.tool.Name(), tt.tool.Risk(), tt.want)
		}
	}
}

func TestActionTools_CategoryAction(t *testing.T) {
	tools := []Tool{
		&ClickTool{}, &DoubleClickTool{}, &TypeTextTool{},
		&PressKeyTool{}, &HotkeyTool{}, &ScrollTool{}, &MousePositionTool{},
	}
	for _, tool := range tools {
		if tool.Category() != CategoryAction {
			t.Errorf("%s.Category() = %v, want action", tool.Name(), tool.Category())
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: No Gateway (nil gateway, direct execution)
// ──────────────────────────────────────────────────────────────

func TestClick_NoGateway(t *testing.T) {
	mi := &mockInput{}
	tool := &ClickTool{deps: ActionDeps{Input: mi, Gateway: nil}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"x":50,"y":60}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastAction != "click" {
		t.Error("Expected click to execute without gateway")
	}
}
