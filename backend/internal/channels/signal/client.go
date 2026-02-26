package signal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Signal JSON-RPC 客户端 + SSE 事件流 — 继承自 src/signal/client.ts (195L)

// randomRpcID 生成 UUID v4 作为 RPC request id（对齐 TS randomUUID()）
func randomRpcID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// DefaultRpcTimeoutMs 默认 RPC 超时（与 TS 端一致 = 10s）
const DefaultRpcTimeoutMs = 10_000

// SignalRpcRequest JSON-RPC 2.0 请求（使用默认超时 10s）
func SignalRpcRequest(ctx context.Context, baseURL string, method string, params interface{}, account string) (json.RawMessage, error) {
	return SignalRpcRequestWithTimeout(ctx, baseURL, method, params, account, DefaultRpcTimeoutMs)
}

// SignalRpcRequestWithTimeout JSON-RPC 2.0 请求（可配置超时，单位 ms）
func SignalRpcRequestWithTimeout(ctx context.Context, baseURL string, method string, params interface{}, account string, timeoutMs int) (json.RawMessage, error) {
	if timeoutMs <= 0 {
		timeoutMs = DefaultRpcTimeoutMs
	}
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      randomRpcID(),
	}
	if params != nil {
		reqBody["params"] = params
	}
	if account != "" {
		p, ok := reqBody["params"].(map[string]interface{})
		if !ok {
			p = make(map[string]interface{})
		}
		p["account"] = account
		reqBody["params"] = p
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/api/v1/rpc"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rpc request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read rpc response: %w", err)
	}

	// HTTP 201 Created 也视为成功（signal-cli 部分操作返回 201）
	if resp.StatusCode == http.StatusCreated {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rpc HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal rpc response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// SignalCheck 检查 signal-cli daemon 连通性（GET /api/v1/check）
// 注：TS 端使用 /api/v1/check 端点，非 /api/v1/about
func SignalCheck(ctx context.Context, baseURL string) error {
	url := strings.TrimRight(baseURL, "/") + "/api/v1/check"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create check request: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("signal check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("signal check HTTP %d", resp.StatusCode)
	}
	return nil
}

// SignalSSEvent SSE 事件
type SignalSSEvent struct {
	Event string
	Data  string
}

// StreamSignalEvents 流式读取 SSE 事件
// 通过 ctx 控制生命周期，阻塞调用直到 ctx 取消或连接关闭
func StreamSignalEvents(ctx context.Context, baseURL string, account string, handler func(SignalSSEvent)) error {
	url := strings.TrimRight(baseURL, "/") + "/api/v1/events"
	if account != "" {
		url += "?account=" + account
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		Timeout: 0, // SSE 无超时
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SSE HTTP %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 最大 1MB 行

	var currentEvent string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// 空行触发事件分发
			if len(dataLines) > 0 {
				handler(SignalSSEvent{
					Event: currentEvent,
					Data:  strings.Join(dataLines, "\n"),
				})
			}
			currentEvent = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(line[len("event:"):])
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[len("data:"):]))
		}
		// 忽略 comment 行（以 : 开头）和其他不认识的字段
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("SSE read error: %w", err)
	}
	return nil
}
