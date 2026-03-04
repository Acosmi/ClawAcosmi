package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/infra"
)

// ---------- buildResponsesAgentPrompt 测试 ----------

func TestBuildResponsesAgentPrompt_StringInput(t *testing.T) {
	input := json.RawMessage(`"Hello, how are you?"`)
	prompt, err := buildResponsesAgentPrompt(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt.message != "Hello, how are you?" {
		t.Errorf("unexpected message: %q", prompt.message)
	}
	if prompt.extraSystemPrompt != "" {
		t.Errorf("unexpected extraSystemPrompt: %q", prompt.extraSystemPrompt)
	}
}

func TestBuildResponsesAgentPrompt_EmptyString(t *testing.T) {
	input := json.RawMessage(`""`)
	_, err := buildResponsesAgentPrompt(input)
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestBuildResponsesAgentPrompt_SimpleMessage(t *testing.T) {
	input := json.RawMessage(`[
		{"type": "message", "role": "user", "content": "what is 2+2?"}
	]`)
	prompt, err := buildResponsesAgentPrompt(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt.message != "what is 2+2?" {
		t.Errorf("unexpected message: %q", prompt.message)
	}
}

func TestBuildResponsesAgentPrompt_WithSystemAndHistory(t *testing.T) {
	input := json.RawMessage(`[
		{"type": "message", "role": "system", "content": "Be concise."},
		{"type": "message", "role": "user", "content": "hello"},
		{"type": "message", "role": "assistant", "content": "hi there"},
		{"type": "message", "role": "user", "content": "how are you?"}
	]`)
	prompt, err := buildResponsesAgentPrompt(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt.extraSystemPrompt != "Be concise." {
		t.Errorf("unexpected extraSystemPrompt: %q", prompt.extraSystemPrompt)
	}
	if !strings.Contains(prompt.message, "User: hello") {
		t.Errorf("expected history context, got: %q", prompt.message)
	}
	if !strings.Contains(prompt.message, "how are you?") {
		t.Errorf("expected current message, got: %q", prompt.message)
	}
}

func TestBuildResponsesAgentPrompt_FunctionCallOutput(t *testing.T) {
	input := json.RawMessage(`[
		{"type": "message", "role": "user", "content": "run ls"},
		{"type": "function_call_output", "call_id": "call_1", "output": "file1.txt\nfile2.txt"}
	]`)
	prompt, err := buildResponsesAgentPrompt(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt.message, "Tool:call_1") {
		t.Errorf("expected tool output in message, got: %q", prompt.message)
	}
}

func TestBuildResponsesAgentPrompt_ContentParts(t *testing.T) {
	input := json.RawMessage(`[
		{"type": "message", "role": "user", "content": [
			{"type": "input_text", "text": "Hello "},
			{"type": "input_text", "text": "world!"}
		]}
	]`)
	prompt, err := buildResponsesAgentPrompt(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt.message, "Hello") || !strings.Contains(prompt.message, "world!") {
		t.Errorf("unexpected message: %q", prompt.message)
	}
}

// ---------- applyORToolChoice 测试 ----------

func TestApplyORToolChoice_Auto(t *testing.T) {
	result := applyORToolChoice(nil, json.RawMessage(`"auto"`))
	if result != "" {
		t.Errorf("expected empty prompt for auto, got: %q", result)
	}
}

func TestApplyORToolChoice_None(t *testing.T) {
	result := applyORToolChoice(nil, json.RawMessage(`"none"`))
	if result != "" {
		t.Errorf("expected empty prompt for none, got: %q", result)
	}
}

func TestApplyORToolChoice_Required(t *testing.T) {
	tools := []ToolDefinition{{Type: "function", Function: ToolDefinitionFunction{Name: "bash"}}}
	result := applyORToolChoice(tools, json.RawMessage(`"required"`))
	if !strings.Contains(result, "must call") {
		t.Errorf("expected 'must call' prompt, got: %q", result)
	}
}

func TestApplyORToolChoice_FunctionName(t *testing.T) {
	tools := []ToolDefinition{{Type: "function", Function: ToolDefinitionFunction{Name: "bash"}}}
	result := applyORToolChoice(tools, json.RawMessage(`{"type":"function","function":{"name":"bash"}}`))
	if !strings.Contains(result, "bash") {
		t.Errorf("expected 'bash' in prompt, got: %q", result)
	}
}

func TestApplyORToolChoice_Empty(t *testing.T) {
	result := applyORToolChoice(nil, nil)
	if result != "" {
		t.Errorf("expected empty prompt for nil tool_choice, got: %q", result)
	}
}

// ---------- resolveOpenResponsesSessionKey 测试 ----------

func TestResolveOpenResponsesSessionKey(t *testing.T) {
	tests := []struct {
		agentID string
		user    string
		want    string
	}{
		{"myagent", "user1", "openresponses:myagent:user1"},
		{"myagent", "", "openresponses:myagent"},
		{"", "user1", "openresponses:main:user1"},
		{"", "", "openresponses:main"},
	}

	for _, tt := range tests {
		got := resolveOpenResponsesSessionKey(tt.agentID, tt.user)
		if got != tt.want {
			t.Errorf("resolveOpenResponsesSessionKey(%q, %q) = %q, want %q",
				tt.agentID, tt.user, got, tt.want)
		}
	}
}

// ---------- extractORTextContent 测试 ----------

func TestExtractORTextContent_String(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	result := extractORTextContent(raw)
	if result != "hello world" {
		t.Errorf("expected 'hello world', got: %q", result)
	}
}

func TestExtractORTextContent_Parts(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_text", "text": "part1"},
		{"type": "output_text", "text": "part2"},
		{"type": "input_image", "text": "should_skip"}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "part1") || !strings.Contains(result, "part2") {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExtractORTextContent_Empty(t *testing.T) {
	result := extractORTextContent(nil)
	if result != "" {
		t.Errorf("expected empty, got: %q", result)
	}
}

// ---------- 类型创建测试 ----------

func TestCreateResponseResource(t *testing.T) {
	resp := createResponseResource("resp_1", "gpt-4", "completed",
		[]OutputItem{createAssistantOutputItem("msg_1", "hello", "completed")},
		emptyUsage(), nil)

	if resp.ID != "resp_1" {
		t.Errorf("unexpected ID: %q", resp.ID)
	}
	if resp.Object != "response" {
		t.Errorf("unexpected object: %q", resp.Object)
	}
	if resp.Status != "completed" {
		t.Errorf("unexpected status: %q", resp.Status)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "message" || resp.Output[0].Content[0].Text != "hello" {
		t.Errorf("unexpected output: %+v", resp.Output[0])
	}
}

func TestCreateAssistantOutputItem(t *testing.T) {
	item := createAssistantOutputItem("msg_1", "test text", "in_progress")
	if item.Type != "message" || item.Role != "assistant" {
		t.Errorf("unexpected item: %+v", item)
	}
	if len(item.Content) != 1 || item.Content[0].Type != "output_text" || item.Content[0].Text != "test text" {
		t.Errorf("unexpected content: %+v", item.Content)
	}
	if item.Status != "in_progress" {
		t.Errorf("unexpected status: %q", item.Status)
	}
}

// ---------- input_image 测试 ----------

func TestExtractORTextContent_InputImage_Base64(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_text", "text": "What is this?"},
		{"type": "input_image", "source": {"type": "base64", "data": "iVBORw0KGgo=", "media_type": "image/png"}}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "What is this?") {
		t.Errorf("expected text content, got: %q", result)
	}
	if !strings.Contains(result, "[Image: base64 image/png") {
		t.Errorf("expected image description, got: %q", result)
	}
}

func TestExtractORTextContent_InputImage_URL(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_image", "source": {"type": "url", "url": "https://example.com/photo.jpg"}}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "https://example.com/photo.jpg") {
		t.Errorf("expected image URL, got: %q", result)
	}
}

func TestExtractORTextContent_InputImage_Shorthand(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_image", "image_url": "https://example.com/pic.png"}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "https://example.com/pic.png") {
		t.Errorf("expected image URL shorthand, got: %q", result)
	}
}

// ---------- input_file 测试 ----------

func TestExtractORTextContent_InputFile_TextBase64(t *testing.T) {
	// "Hello, World!" in base64
	raw := json.RawMessage(`[
		{"type": "input_file", "filename": "test.txt", "source": {"type": "base64", "data": "SGVsbG8sIFdvcmxkIQ==", "media_type": "text/plain"}}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "Hello, World!") {
		t.Errorf("expected decoded text content, got: %q", result)
	}
	if !strings.Contains(result, "test.txt") {
		t.Errorf("expected filename in output, got: %q", result)
	}
}

func TestExtractORTextContent_InputFile_URL(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_file", "source": {"type": "url", "url": "https://example.com/report.pdf", "filename": "report.pdf"}}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "report.pdf") {
		t.Errorf("expected filename, got: %q", result)
	}
	if !strings.Contains(result, "https://example.com/report.pdf") {
		t.Errorf("expected URL, got: %q", result)
	}
}

func TestExtractORTextContent_InputFile_BinaryBase64(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_file", "filename": "image.png", "source": {"type": "base64", "data": "iVBORw0KGgo=", "media_type": "image/png"}}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "[File: image.png, image/png") {
		t.Errorf("expected binary file description, got: %q", result)
	}
}

// ---------- 混合内容测试 ----------

func TestExtractORTextContent_Mixed(t *testing.T) {
	raw := json.RawMessage(`[
		{"type": "input_text", "text": "Analyze this image and file:"},
		{"type": "input_image", "source": {"type": "url", "url": "https://example.com/chart.png"}},
		{"type": "input_file", "filename": "data.json", "source": {"type": "base64", "data": "eyJrZXkiOiAidmFsdWUifQ==", "media_type": "application/json"}}
	]`)
	result := extractORTextContent(raw)
	if !strings.Contains(result, "Analyze this image and file:") {
		t.Errorf("expected text, got: %q", result)
	}
	if !strings.Contains(result, "[Image: https://example.com/chart.png]") {
		t.Errorf("expected image desc, got: %q", result)
	}
	if !strings.Contains(result, "data.json") {
		t.Errorf("expected file name, got: %q", result)
	}
}

func TestBuildResponsesAgentPrompt_WithImage(t *testing.T) {
	input := json.RawMessage(`[
		{"type": "message", "role": "user", "content": [
			{"type": "input_text", "text": "What is in this image?"},
			{"type": "input_image", "source": {"type": "url", "url": "https://example.com/photo.jpg"}}
		]}
	]`)
	prompt, err := buildResponsesAgentPrompt(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt.message, "What is in this image?") {
		t.Errorf("expected text in message, got: %q", prompt.message)
	}
	if !strings.Contains(prompt.message, "[Image: https://example.com/photo.jpg]") {
		t.Errorf("expected image description in message, got: %q", prompt.message)
	}
}

// ---------- usage 提取测试 ----------

func TestExtractUsageFromAgentEvent(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want *ORUsage
	}{
		{
			name: "direct usage",
			data: map[string]interface{}{
				"usage": map[string]interface{}{
					"input":  float64(100),
					"output": float64(50),
					"total":  float64(150),
				},
			},
			want: &ORUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
		},
		{
			name: "agentMeta usage",
			data: map[string]interface{}{
				"agentMeta": map[string]interface{}{
					"usage": map[string]interface{}{
						"input":  float64(200),
						"output": float64(100),
					},
				},
			},
			want: &ORUsage{InputTokens: 200, OutputTokens: 100, TotalTokens: 300},
		},
		{
			name: "usage with cacheRead and cacheWrite",
			data: map[string]interface{}{
				"usage": map[string]interface{}{
					"input":      float64(100),
					"output":     float64(50),
					"cacheRead":  float64(30),
					"cacheWrite": float64(20),
				},
			},
			want: &ORUsage{InputTokens: 100, OutputTokens: 50, TotalTokens: 200},
		},
		{
			name: "no usage",
			data: map[string]interface{}{"phase": "end"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := infra.AgentEventPayload{Data: tt.data}
			got := extractUsageFromAgentEvent(evt)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil usage")
			}
			if got.InputTokens != tt.want.InputTokens || got.OutputTokens != tt.want.OutputTokens || got.TotalTokens != tt.want.TotalTokens {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---------- isTextMime 测试 ----------

func TestIsTextMime(t *testing.T) {
	textMimes := []string{"text/plain", "text/html", "text/csv", "application/json", "application/xml", "application/yaml"}
	for _, m := range textMimes {
		if !isTextMime(m) {
			t.Errorf("expected %q to be text mime", m)
		}
	}
	nonTextMimes := []string{"image/png", "application/pdf", "application/octet-stream", "video/mp4"}
	for _, m := range nonTextMimes {
		if isTextMime(m) {
			t.Errorf("expected %q to NOT be text mime", m)
		}
	}
}
