package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Mock implementations
// ──────────────────────────────────────────────────────────────

// mockCapturer returns a fixed frame.
type mockCapturer struct {
	frame *capture.Frame
}

func (m *mockCapturer) Start(fps int) error                  { return nil }
func (m *mockCapturer) Stop() error                          { return nil }
func (m *mockCapturer) LatestFrame() *capture.Frame          { return m.frame }
func (m *mockCapturer) FrameChan() <-chan *capture.Frame     { return nil }
func (m *mockCapturer) Subscribe() <-chan *capture.Frame     { return nil }
func (m *mockCapturer) Unsubscribe(ch <-chan *capture.Frame) {}
func (m *mockCapturer) IsRunning() bool                      { return true }
func (m *mockCapturer) DisplayInfo() capture.DisplayInfo {
	return capture.DisplayInfo{Width: 100, Height: 100}
}
func (m *mockCapturer) ListWindows() ([]capture.WindowInfo, error) { return nil, nil }
func (m *mockCapturer) SetExcludedWindows(ids []uint32) error      { return nil }
func (m *mockCapturer) ExcludeApp(bundleID string) error           { return nil }
func (m *mockCapturer) GetExcludedWindows() []uint32               { return nil }

// mockVLM returns a fixed response.
type mockVLM struct {
	response string
	err      error
}

func (m *mockVLM) ChatCompletion(_ context.Context, _ vlm.ChatRequest) (*vlm.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return vlm.NewChatResponse("test", "test-model", m.response, "stop"), nil
}
func (m *mockVLM) ChatCompletionStream(_ context.Context, _ vlm.ChatRequest) (<-chan vlm.StreamChunk, error) {
	return nil, nil
}
func (m *mockVLM) Name() string { return "mock" }
func (m *mockVLM) Close() error { return nil }

// newTestFrame creates a small 4x4 BGRA test frame.
func newTestFrame() *capture.Frame {
	w, h := 4, 4
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 0xFF   // B
		pixels[i+1] = 0x00 // G
		pixels[i+2] = 0x00 // R
		pixels[i+3] = 0xFF // A
	}
	return &capture.Frame{
		Width:    w,
		Height:   h,
		Stride:   w * 4,
		Channels: 4,
		Pixels:   pixels,
		FrameNo:  1,
	}
}

// ──────────────────────────────────────────────────────────────
// Registry Tests
// ──────────────────────────────────────────────────────────────

func TestRegistry_Register_And_Get(t *testing.T) {
	r := NewRegistry()
	tool := &CaptureScreenTool{}

	r.Register(tool)
	got := r.Get("capture_screen")
	if got == nil {
		t.Fatal("Get returned nil for registered tool")
	}
	if got.Name() != "capture_screen" {
		t.Errorf("Got name %q, want capture_screen", got.Name())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	if r.Get("nonexistent") != nil {
		t.Error("Get should return nil for unknown tool")
	}
}

func TestRegistry_Register_Duplicate_Panics(t *testing.T) {
	r := NewRegistry()
	r.Register(&CaptureScreenTool{})

	defer func() {
		if rc := recover(); rc == nil {
			t.Error("Expected panic on duplicate registration")
		}
	}()
	r.Register(&CaptureScreenTool{})
}

func TestRegistry_List_All(t *testing.T) {
	r := NewRegistry()
	r.Register(&CaptureScreenTool{})
	r.Register(&DescribeSceneTool{})
	r.Register(&ReadTextTool{})

	tools := r.List()
	if len(tools) != 3 {
		t.Errorf("List() returned %d tools, want 3", len(tools))
	}
}

func TestRegistry_List_ByCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(&CaptureScreenTool{})
	r.Register(&DescribeSceneTool{})

	tools := r.List(CategoryPerception)
	if len(tools) != 2 {
		t.Errorf("List(perception) returned %d tools, want 2", len(tools))
	}

	tools = r.List(CategoryAction)
	if len(tools) != 0 {
		t.Errorf("List(action) returned %d tools, want 0", len(tools))
	}
}

func TestRegistry_Capabilities(t *testing.T) {
	r := NewRegistry()
	r.Register(&CaptureScreenTool{})
	r.Register(&DetectDialogTool{})

	caps := r.Capabilities()
	if len(caps) != 2 {
		t.Fatalf("Capabilities returned %d items, want 2", len(caps))
	}

	// Check structure
	for _, c := range caps {
		if c.Name == "" || c.Description == "" {
			t.Errorf("Capability missing name or description: %+v", c)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Perception Tool Tests
// ──────────────────────────────────────────────────────────────

func TestCaptureScreen_VLMQuality(t *testing.T) {
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		Scaler:   newTestScaler(),
	}
	tool := &CaptureScreenTool{deps: deps}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"quality":"vlm"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}

	content, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatal("Content is not a map")
	}
	if content["image_b64"] == nil || content["image_b64"] == "" {
		t.Error("image_b64 is empty")
	}
	if content["quality"] != "vlm" {
		t.Errorf("quality = %v, want vlm", content["quality"])
	}
}

func TestCaptureScreen_NoFrame(t *testing.T) {
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: nil},
		Scaler:   newTestScaler(),
	}
	tool := &CaptureScreenTool{deps: deps}

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected IsError for no frame")
	}
}

func TestDescribeScene_WithFocus(t *testing.T) {
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: "A blue screen with a terminal open"},
		Scaler:   newTestScaler(),
	}
	tool := &DescribeSceneTool{deps: deps}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"focus":"terminal"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["description"] != "A blue screen with a terminal open" {
		t.Errorf("Unexpected description: %v", content["description"])
	}
}

func TestReadText_DefaultRegion(t *testing.T) {
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: "Hello World\nLine 2"},
		Scaler:   newTestScaler(),
	}
	tool := &ReadTextTool{deps: deps}

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	text := content["text"].(string)
	if !strings.Contains(text, "Hello World") {
		t.Errorf("Expected 'Hello World' in text, got %q", text)
	}
	if content["region"] != "full" {
		t.Errorf("region = %v, want full", content["region"])
	}
}

func TestDetectDialog_Found(t *testing.T) {
	vlmResp := `{"has_dialog": true, "dialogs": [{"type": "confirm", "title": "Save?", "buttons": ["Yes", "No"]}]}`
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: vlmResp},
		Scaler:   newTestScaler(),
	}
	tool := &DetectDialogTool{deps: deps}

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["has_dialog"] != true {
		t.Errorf("has_dialog = %v, want true", content["has_dialog"])
	}
}

func TestDetectDialog_MarkdownFences(t *testing.T) {
	vlmResp := "```json\n{\"has_dialog\": false, \"dialogs\": []}\n```"
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: vlmResp},
		Scaler:   newTestScaler(),
	}
	tool := &DetectDialogTool{deps: deps}

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["has_dialog"] != false {
		t.Errorf("has_dialog = %v, want false", content["has_dialog"])
	}
}

func TestWatchForChange_Detected(t *testing.T) {
	vlmResp := `{"detected": true, "description": "Dialog appeared", "confidence": 0.95}`
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: vlmResp},
		Scaler:   newTestScaler(),
	}
	tool := &WatchForChangeTool{deps: deps}

	params := `{"watch_for": "dialog appears", "timeout_seconds": 2, "interval_ms": 500}`
	result, err := tool.Execute(context.Background(), json.RawMessage(params))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["detected"] != true {
		t.Errorf("detected = %v, want true", content["detected"])
	}
}

func TestWatchForChange_Timeout(t *testing.T) {
	vlmResp := `{"detected": false, "description": "No change", "confidence": 0.1}`
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: vlmResp},
		Scaler:   newTestScaler(),
	}
	tool := &WatchForChangeTool{deps: deps}

	// Short timeout so test finishes fast
	params := `{"watch_for": "something", "timeout_seconds": 1, "interval_ms": 500}`
	start := time.Now()
	result, err := tool.Execute(context.Background(), json.RawMessage(params))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	content := result.Content.(map[string]any)
	if content["detected"] != false {
		t.Errorf("detected = %v, want false", content["detected"])
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("Returned too fast: %v (expected ≥1s)", elapsed)
	}
}

func TestAllPerceptionTools_RiskLow(t *testing.T) {
	tools := []Tool{
		&CaptureScreenTool{},
		&DescribeSceneTool{},
		&LocateElementTool{},
		&ReadTextTool{},
		&DetectDialogTool{},
		&WatchForChangeTool{},
	}
	for _, tool := range tools {
		if tool.Risk() != RiskLow {
			t.Errorf("%s.Risk() = %v, want RiskLow", tool.Name(), tool.Risk())
		}
		if tool.Category() != CategoryPerception {
			t.Errorf("%s.Category() = %v, want perception", tool.Name(), tool.Category())
		}
	}
}

func TestAllPerceptionTools_HaveSchema(t *testing.T) {
	tools := []Tool{
		&CaptureScreenTool{},
		&DescribeSceneTool{},
		&LocateElementTool{},
		&ReadTextTool{},
		&DetectDialogTool{},
		&WatchForChangeTool{},
	}
	for _, tool := range tools {
		schema := tool.InputSchema()
		if schema.Type != "object" {
			t.Errorf("%s schema type = %q, want object", tool.Name(), schema.Type)
		}
	}
}

func TestRegisterPerceptionTools(t *testing.T) {
	r := NewRegistry()
	deps := PerceptionDeps{
		Capturer: &mockCapturer{frame: newTestFrame()},
		VLM:      &mockVLM{response: "ok"},
		Scaler:   newTestScaler(),
	}
	RegisterPerceptionTools(r, deps)

	expected := []string{
		"capture_screen", "describe_scene", "locate_element",
		"read_text", "detect_dialog", "watch_for_change",
	}
	for _, name := range expected {
		if r.Get(name) == nil {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────

func newTestScaler() *imaging.Scaler {
	return &imaging.Scaler{
		VLMMaxDim:      64,
		VLMQuality:     50,
		DisplayQuality: 80,
	}
}
