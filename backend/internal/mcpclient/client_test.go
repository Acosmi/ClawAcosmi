package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

// mockMCPServer 模拟 MCP 服务端：从 clientStdin 读请求，向 clientStdout 写响应。
// handler 接收解析后的请求，返回 result JSON 和可选 error。
type mockMCPServer struct {
	clientStdin  io.ReadCloser  // 读客户端写入的请求
	clientStdout io.WriteCloser // 向客户端写响应
	handler      func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError)
}

func (m *mockMCPServer) run() {
	scanner := bufio.NewScanner(m.clientStdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		// 通知（无 ID）不需要响应 — 通过检测 ID==0 跳过
		// 但 JSON 默认 int64 为 0，所以我们检测 method 前缀
		if req.ID == 0 {
			continue
		}

		result, rpcErr := m.handler(req)
		resp := JSONRPCResponse{
			JSONRPC: JSONRPC2,
			ID:      req.ID,
			Result:  result,
			Error:   rpcErr,
		}

		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		m.clientStdout.Write(data)
	}
}

// setupMockPair 创建 mock MCP client + server 管道对。
func setupMockPair(handler func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError)) (*Client, *mockMCPServer) {
	// client → server（客户端写入 stdin，服务端读取）
	serverReader, clientWriter := io.Pipe()
	// server → client（服务端写入 stdout，客户端读取）
	clientReader, serverWriter := io.Pipe()

	server := &mockMCPServer{
		clientStdin:  serverReader,
		clientStdout: serverWriter,
		handler:      handler,
	}
	go server.run()

	client := NewClient(clientWriter, clientReader)
	return client, server
}

// ---------- 测试用例 ----------

func TestInitialize(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		if req.Method != "initialize" {
			return nil, nil
		}
		result := MCPInitializeResult{
			ProtocolVersion: MCPProtocolVersion,
			Capabilities:    json.RawMessage(`{}`),
			ServerInfo: MCPImplementation{
				Name:    "test-server",
				Version: "0.1.0",
			},
		}
		data, _ := json.Marshal(result)
		return data, nil
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %q", result.ServerInfo.Name)
	}
	if result.ProtocolVersion != MCPProtocolVersion {
		t.Errorf("expected protocol %q, got %q", MCPProtocolVersion, result.ProtocolVersion)
	}
}

func TestListTools(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		switch req.Method {
		case "initialize":
			result := MCPInitializeResult{
				ProtocolVersion: MCPProtocolVersion,
				Capabilities:    json.RawMessage(`{}`),
				ServerInfo:      MCPImplementation{Name: "test", Version: "1.0"},
			}
			data, _ := json.Marshal(result)
			return data, nil
		case "tools/list":
			result := MCPToolsListResult{
				Tools: []MCPToolDef{
					{Name: "capture_screen", Description: "Capture screenshot", InputSchema: json.RawMessage(`{"type":"object"}`)},
					{Name: "click", Description: "Click at position", InputSchema: json.RawMessage(`{"type":"object"}`)},
				},
			}
			data, _ := json.Marshal(result)
			return data, nil
		}
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx := context.Background()

	// 先握手
	_, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 列出工具
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "capture_screen" {
		t.Errorf("expected first tool 'capture_screen', got %q", tools[0].Name)
	}
	if tools[1].Name != "click" {
		t.Errorf("expected second tool 'click', got %q", tools[1].Name)
	}
}

func TestCallTool_Success(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		if req.Method == "tools/call" {
			result := MCPToolsCallResult{
				Content: []MCPContent{
					{Type: "text", Text: `{"x":100,"y":200}`},
				},
				IsError: false,
			}
			data, _ := json.Marshal(result)
			return data, nil
		}
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx := context.Background()
	args := json.RawMessage(`{"description":"search button"}`)

	result, err := client.CallTool(ctx, "locate_element", args, 5*time.Second)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Error("expected IsError=false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got %q", result.Content[0].Type)
	}
}

func TestCallTool_MCPError(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		if req.Method == "tools/call" {
			result := MCPToolsCallResult{
				Content: []MCPContent{
					{Type: "text", Text: "element not found"},
				},
				IsError: true,
			}
			data, _ := json.Marshal(result)
			return data, nil
		}
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx := context.Background()
	result, err := client.CallTool(ctx, "locate_element", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("CallTool should not return transport error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for tool-level error")
	}
}

func TestCallTool_JSONRPCError(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		if req.Method == "tools/call" {
			return nil, &JSONRPCError{Code: -32601, Message: "method not found"}
		}
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx := context.Background()
	_, err := client.CallTool(ctx, "nonexistent", nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for JSON-RPC error response")
	}
}

func TestPing(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		if req.Method == "ping" {
			return json.RawMessage(`{}`), nil
		}
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx := context.Background()
	rtt, err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if rtt <= 0 {
		t.Errorf("expected positive RTT, got %v", rtt)
	}
}

func TestContextCancellation(t *testing.T) {
	// 服务端故意不响应，测试 context 取消
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		// 故意不返回任何东西 — 通过阻塞模拟
		time.Sleep(10 * time.Second)
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Ping(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestClosePreventsSend(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		return json.RawMessage(`{}`), nil
	})

	client.Close()

	ctx := context.Background()
	_, err := client.Ping(ctx)
	if err == nil {
		t.Fatal("expected error after client close")
	}
}

func TestConcurrentRequests(t *testing.T) {
	client, _ := setupMockPair(func(req JSONRPCRequest) (json.RawMessage, *JSONRPCError) {
		// 为不同方法返回不同结果
		if req.Method == "ping" {
			return json.RawMessage(`{}`), nil
		}
		if req.Method == "tools/list" {
			result := MCPToolsListResult{
				Tools: []MCPToolDef{
					{Name: "test_tool", Description: "test"},
				},
			}
			data, _ := json.Marshal(result)
			return data, nil
		}
		return json.RawMessage(`{}`), nil
	})
	defer client.Close()

	ctx := context.Background()
	errs := make(chan error, 10)

	// 并发发送 5 个 ping + 5 个 tools/list
	for i := 0; i < 5; i++ {
		go func() {
			_, err := client.Ping(ctx)
			errs <- err
		}()
		go func() {
			_, err := client.ListTools(ctx)
			errs <- err
		}()
	}

	for i := 0; i < 10; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent request %d failed: %v", i, err)
		}
	}
}
