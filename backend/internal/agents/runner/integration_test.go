package runner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/open-acosmi/internal/agents/llmclient"
	"github.com/anthropic/open-acosmi/internal/agents/runner"
)

// ============================================================================
// llmclient + tool_executor 集成测试
// 验证 LLM 流式响应 → 工具调用解析 → 工具执行 → 结果回传的完整链路。
// ============================================================================

// --- Anthropic SSE 辅助 ---

// buildAnthropicTextSSE 构造纯文本 Anthropic SSE 响应。
func buildAnthropicTextSSE(text string) string {
	return fmt.Sprintf(`event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":10}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`, text)
}

// buildAnthropicToolUseSSE 构造包含工具调用的 Anthropic SSE 响应。
func buildAnthropicToolUseSSE(toolID, toolName, inputJSON string) string {
	return fmt.Sprintf(`event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":20}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me do that."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":%q,"name":%q}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":%q}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}

`, toolID, toolName, inputJSON)
}

// ---------- Test 1: 纯文本对话 ----------

func TestIntegration_TextOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildAnthropicTextSSE("Hello from integration test!"))
	}))
	defer server.Close()

	result, err := llmclient.StreamChat(context.Background(), llmclient.ChatRequest{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		SystemPrompt: "be helpful",
		Messages:     []llmclient.ChatMessage{llmclient.TextMessage("user", "hi")},
		APIKey:       "test-key",
		BaseURL:      server.URL,
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.StopReason != "end_turn" {
		t.Errorf("expected end_turn, got %q", result.StopReason)
	}
	if len(result.AssistantMessage.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.AssistantMessage.Content))
	}
	if result.AssistantMessage.Content[0].Text != "Hello from integration test!" {
		t.Errorf("unexpected text: %q", result.AssistantMessage.Content[0].Text)
	}
}

// ---------- Test 2: 工具调用循环 ----------
// mock SSE 返回 tool_use → ExecuteToolCall → 验证工具执行结果 → mock 第二轮 end_turn

func TestIntegration_ToolCallLoop(t *testing.T) {
	workDir := t.TempDir()

	// 在 workDir 中创建一个测试文件供 read_file 工具读取
	testContent := "integration test file content"
	testFile := filepath.Join(workDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		callCount++
		if callCount == 1 {
			// 第一轮：返回 read_file 工具调用
			fmt.Fprint(w, buildAnthropicToolUseSSE(
				"toolu_read1",
				"read_file",
				`{"path":"test.txt"}`,
			))
		} else {
			// 第二轮：收到工具结果后返回最终文本
			fmt.Fprint(w, buildAnthropicTextSSE("The file contains: "+testContent))
		}
	}))
	defer server.Close()

	// --- Round 1: 获取 tool_use 响应 ---
	r1, err := llmclient.StreamChat(context.Background(), llmclient.ChatRequest{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		SystemPrompt: "read files when asked",
		Messages:     []llmclient.ChatMessage{llmclient.TextMessage("user", "read test.txt")},
		APIKey:       "test-key",
		BaseURL:      server.URL,
	}, nil)
	if err != nil {
		t.Fatalf("round 1 error: %v", err)
	}
	if r1.StopReason != "tool_use" {
		t.Fatalf("expected tool_use, got %q", r1.StopReason)
	}

	// 提取工具调用
	var toolBlock *llmclient.ContentBlock
	for i := range r1.AssistantMessage.Content {
		if r1.AssistantMessage.Content[i].Type == "tool_use" {
			toolBlock = &r1.AssistantMessage.Content[i]
			break
		}
	}
	if toolBlock == nil {
		t.Fatal("no tool_use block found")
	}
	if toolBlock.Name != "read_file" {
		t.Fatalf("expected read_file, got %q", toolBlock.Name)
	}

	// --- 执行工具 ---
	toolResult, err := runner.ExecuteToolCall(
		context.Background(),
		toolBlock.Name,
		toolBlock.Input,
		runner.ToolExecParams{WorkspaceDir: workDir, AllowExec: true, AllowWrite: true},
	)
	if err != nil {
		t.Fatalf("tool execution error: %v", err)
	}
	if toolResult != testContent {
		t.Errorf("expected %q, got %q", testContent, toolResult)
	}

	// --- Round 2: 发送工具结果，获取最终回复 ---
	r2Messages := []llmclient.ChatMessage{
		llmclient.TextMessage("user", "read test.txt"),
		r1.AssistantMessage,
		{
			Role: "user",
			Content: []llmclient.ContentBlock{
				{Type: "tool_result", ToolUseID: toolBlock.ID, ResultText: toolResult},
			},
		},
	}

	r2, err := llmclient.StreamChat(context.Background(), llmclient.ChatRequest{
		Provider:     "anthropic",
		Model:        "claude-3-sonnet",
		SystemPrompt: "read files when asked",
		Messages:     r2Messages,
		APIKey:       "test-key",
		BaseURL:      server.URL,
	}, nil)
	if err != nil {
		t.Fatalf("round 2 error: %v", err)
	}
	if r2.StopReason != "end_turn" {
		t.Errorf("expected end_turn, got %q", r2.StopReason)
	}
	if len(r2.AssistantMessage.Content) == 0 {
		t.Fatal("expected content in round 2")
	}

	finalText := r2.AssistantMessage.Content[0].Text
	if !strings.Contains(finalText, testContent) {
		t.Errorf("expected final text to contain %q, got %q", testContent, finalText)
	}
}

// ---------- Test 3: 工具执行错误传递 ----------

func TestIntegration_ToolError(t *testing.T) {
	workDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// 请求读取不存在的文件
		fmt.Fprint(w, buildAnthropicToolUseSSE(
			"toolu_notfound",
			"read_file",
			`{"path":"nonexistent.txt"}`,
		))
	}))
	defer server.Close()

	r1, err := llmclient.StreamChat(context.Background(), llmclient.ChatRequest{
		Provider: "anthropic",
		Model:    "claude-3",
		Messages: []llmclient.ChatMessage{llmclient.TextMessage("user", "read file")},
		APIKey:   "test-key",
		BaseURL:  server.URL,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 提取 tool_use
	var tb *llmclient.ContentBlock
	for i := range r1.AssistantMessage.Content {
		if r1.AssistantMessage.Content[i].Type == "tool_use" {
			tb = &r1.AssistantMessage.Content[i]
			break
		}
	}
	if tb == nil {
		t.Fatal("no tool_use block")
	}

	// 执行 → 应该返回错误消息（不返回 Go error）
	toolResult, err := runner.ExecuteToolCall(
		context.Background(),
		tb.Name,
		tb.Input,
		runner.ToolExecParams{WorkspaceDir: workDir, AllowExec: true, AllowWrite: true},
	)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if !strings.Contains(toolResult, "Error reading file") {
		t.Errorf("expected 'Error reading file', got: %q", toolResult)
	}
}

// ---------- Test 4: Context 取消 ----------

func TestIntegration_ContextCancel(t *testing.T) {
	// 服务器延迟响应
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		time.Sleep(2 * time.Second)
		fmt.Fprint(w, buildAnthropicTextSSE("should not arrive"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := llmclient.StreamChat(ctx, llmclient.ChatRequest{
		Provider: "anthropic",
		Model:    "claude-3",
		Messages: []llmclient.ChatMessage{llmclient.TextMessage("user", "hi")},
		APIKey:   "test-key",
		BaseURL:  server.URL,
	}, nil)

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

// ---------- Test 5: bash 工具 → 写入文件 → 读取验证 ----------

func TestIntegration_BashWriteReadChain(t *testing.T) {
	workDir := t.TempDir()

	// bash: 写入文件
	writeInput, _ := json.Marshal(map[string]string{
		"command": "echo 'hello integration' > chain_test.txt",
	})
	result, err := runner.ExecuteToolCall(
		context.Background(),
		"bash",
		writeInput,
		runner.ToolExecParams{WorkspaceDir: workDir, TimeoutMs: 5000, AllowExec: true, AllowWrite: true},
	)
	if err != nil {
		t.Fatalf("bash write error: %v", err)
	}
	_ = result // bash 输出可能为空

	// read_file: 验证文件内容
	readInput, _ := json.Marshal(map[string]string{
		"path": "chain_test.txt",
	})
	content, err := runner.ExecuteToolCall(
		context.Background(),
		"read_file",
		readInput,
		runner.ToolExecParams{WorkspaceDir: workDir, AllowExec: true, AllowWrite: true},
	)
	if err != nil {
		t.Fatalf("read_file error: %v", err)
	}
	if !strings.Contains(content, "hello integration") {
		t.Errorf("expected 'hello integration', got %q", content)
	}

	// list_dir: 验证文件存在
	listInput, _ := json.Marshal(map[string]string{
		"path": ".",
	})
	listing, err := runner.ExecuteToolCall(
		context.Background(),
		"list_dir",
		listInput,
		runner.ToolExecParams{WorkspaceDir: workDir, AllowExec: true, AllowWrite: true},
	)
	if err != nil {
		t.Fatalf("list_dir error: %v", err)
	}
	if !strings.Contains(listing, "chain_test.txt") {
		t.Errorf("expected chain_test.txt in listing, got %q", listing)
	}
}
