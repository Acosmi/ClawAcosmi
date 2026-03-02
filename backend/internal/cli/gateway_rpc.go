package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/openacosmi/claw-acismi/internal/config"
)

// 对应 TS src/cli/gateway-rpc.ts — Gateway WebSocket RPC 封装

// GatewayRPCOpts Gateway RPC 调用选项
type GatewayRPCOpts struct {
	URL         string // Gateway WebSocket URL
	Token       string // Gateway token
	TimeoutMs   int    // 超时（毫秒）
	ExpectFinal bool   // 等待最终响应
	JSON        bool   // JSON 输出模式
}

// DefaultGatewayRPCOpts 默认 RPC 选项
var DefaultGatewayRPCOpts = GatewayRPCOpts{
	TimeoutMs: 30000,
}

// rpcConnectFrame CLI→Gateway connect 帧
type rpcConnectFrame struct {
	Type   string              `json:"type"`
	Client rpcConnectClient    `json:"client"`
	Auth   *rpcConnectAuthCred `json:"auth,omitempty"`
	Role   string              `json:"role"`
}

type rpcConnectClient struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Mode     string `json:"mode"`
}

type rpcConnectAuthCred struct {
	Token string `json:"token,omitempty"`
}

// rpcRequestFrame CLI 请求帧
type rpcRequestFrame struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// rpcResponseFrame Gateway 响应帧
type rpcResponseFrame struct {
	Type    string           `json:"type"`
	ID      string           `json:"id"`
	OK      bool             `json:"ok"`
	Payload json.RawMessage  `json:"payload,omitempty"`
	Error   *rpcErrorPayload `json:"error,omitempty"`
}

type rpcErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// resolveGatewayURL 解析 Gateway WebSocket URL
func resolveGatewayURL(opts GatewayRPCOpts) string {
	if opts.URL != "" {
		return opts.URL
	}
	port := config.ResolveGatewayPort(nil)
	return fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
}

// CallGatewayFromCLI 通过 CLI 调用 Gateway RPC。
// 对应 TS callGatewayFromCli()。
// 流程: WebSocket connect → hello-ok → method request → response。
func CallGatewayFromCLI(method string, opts GatewayRPCOpts, params interface{}) (interface{}, error) {
	showProgress := !opts.JSON

	if showProgress {
		p := CreateProgress(ProgressOptions{
			Label:         fmt.Sprintf("Gateway %s", method),
			Indeterminate: true,
			Enabled:       true,
		})
		defer p.Done()
	}

	wsURL := resolveGatewayURL(opts)
	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultGatewayRPCOpts.TimeoutMs
	}

	// ---------- 1. 建立 WebSocket 连接 ----------
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, http.Header{})
	if err != nil {
		return nil, fmt.Errorf("无法连接 Gateway (%s): %w", wsURL, err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))

	// ---------- 2. 发送 connect 帧 ----------
	connectFrame := rpcConnectFrame{
		Type: "connect",
		Client: rpcConnectClient{
			ID:       GatewayClientNames.CLI,
			Version:  Version,
			Platform: runtime.GOOS,
			Mode:     GatewayClientModes.CLI,
		},
		Role: "operator",
	}
	if opts.Token != "" {
		connectFrame.Auth = &rpcConnectAuthCred{Token: opts.Token}
	}

	if err := conn.WriteJSON(connectFrame); err != nil {
		return nil, fmt.Errorf("发送 connect 帧失败: %w", err)
	}

	// ---------- 3. 等待 hello-ok ----------
	_, helloRaw, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("等待 hello-ok 失败: %w", err)
	}

	var helloMap map[string]json.RawMessage
	if err := json.Unmarshal(helloRaw, &helloMap); err != nil {
		return nil, fmt.Errorf("解析 hello-ok 失败: %w", err)
	}
	var helloType string
	if raw, ok := helloMap["type"]; ok {
		json.Unmarshal(raw, &helloType)
	}
	if helloType == "error" {
		return nil, fmt.Errorf("Gateway 拒绝连接: %s", string(helloRaw))
	}
	if helloType != "hello-ok" {
		return nil, fmt.Errorf("期望 hello-ok，收到: %s", helloType)
	}

	// ---------- 4. 发送 method 请求 ----------
	reqID := uuid.NewString()
	reqFrame := rpcRequestFrame{
		Type:   "req",
		ID:     reqID,
		Method: method,
		Params: params,
	}
	if err := conn.WriteJSON(reqFrame); err != nil {
		return nil, fmt.Errorf("发送请求失败 (method=%s): %w", method, err)
	}

	// ---------- 5. 等待响应 ----------
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("等待响应超时或连接中断 (method=%s): %w", method, err)
		}

		var resp rpcResponseFrame
		if err := json.Unmarshal(rawMsg, &resp); err != nil {
			continue // 跳过无法解析的帧
		}

		// 忽略 event 帧
		if resp.Type == "event" {
			continue
		}

		// 匹配请求 ID
		if resp.Type == "res" && resp.ID == reqID {
			if !resp.OK {
				errMsg := "未知错误"
				if resp.Error != nil {
					errMsg = fmt.Sprintf("[%s] %s", resp.Error.Code, resp.Error.Message)
				}
				return nil, fmt.Errorf("Gateway RPC 错误 (method=%s): %s", method, errMsg)
			}

			// 解析 payload
			if resp.Payload == nil {
				return nil, nil
			}
			var result interface{}
			if err := json.Unmarshal(resp.Payload, &result); err != nil {
				return nil, fmt.Errorf("解析 payload 失败: %w", err)
			}
			return result, nil
		}
	}
}

// GatewayClientModes Gateway 客户端模式常量
var GatewayClientModes = struct {
	CLI     string
	Web     string
	Channel string
}{
	CLI:     "cli",
	Web:     "web",
	Channel: "channel",
}

// GatewayClientNames Gateway 客户端名称常量
var GatewayClientNames = struct {
	CLI     string
	Web     string
	Channel string
}{
	CLI:     "openacosmi-cli",
	Web:     "openacosmi-web",
	Channel: "openacosmi-channel",
}
