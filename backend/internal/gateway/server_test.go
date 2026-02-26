package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestStartGatewayServer 验证网关启动与关闭。
func TestStartGatewayServer(t *testing.T) {
	runtime, err := StartGatewayServer(0, GatewayServerOptions{})
	if err != nil {
		t.Fatalf("StartGatewayServer failed: %v", err)
	}
	defer runtime.Close("test")

	if runtime.State.Phase() != BootPhaseReady {
		t.Errorf("expected phase Ready, got %s", runtime.State.Phase())
	}
	if runtime.HTTPServer == nil {
		t.Fatal("HTTPServer is nil")
	}
}

// TestStartGatewayServer_DoubleClose 验证重复关闭安全。
func TestStartGatewayServer_DoubleClose(t *testing.T) {
	runtime, err := StartGatewayServer(0, GatewayServerOptions{})
	if err != nil {
		t.Fatalf("StartGatewayServer failed: %v", err)
	}

	if err := runtime.Close("first"); err != nil {
		t.Errorf("first close failed: %v", err)
	}
	if err := runtime.Close("second"); err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

// TestDefaultGatewayEvents 验证事件列表非空。
func TestDefaultGatewayEvents(t *testing.T) {
	events := defaultGatewayEvents()
	if len(events) == 0 {
		t.Fatal("defaultGatewayEvents returned empty list")
	}
	// 检查关键事件存在
	found := make(map[string]bool)
	for _, e := range events {
		found[e] = true
	}
	required := []string{"chat.delta", "presence.changed", "gateway.shutdown"}
	for _, r := range required {
		if !found[r] {
			t.Errorf("missing required event: %s", r)
		}
	}
}

// TestHandleWebSocketUpgrade 验证 WS 升级 + connect → hello-ok 握手。
func TestHandleWebSocketUpgrade(t *testing.T) {
	state := NewGatewayState()
	state.SetPhase(BootPhaseReady)
	registry := NewMethodRegistry()
	registry.Register("health", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, map[string]string{"status": "ok"}, nil)
	})

	cfg := WsServerConfig{
		Auth: ResolvedGatewayAuth{
			Mode:  AuthModeToken,
			Token: "test-token-123",
		},
		State:        state,
		Registry:     registry,
		SessionStore: NewSessionStore(""),
		Version:      "test-1.0",
	}

	handler := HandleWebSocketUpgrade(cfg)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial failed: %v", err)
	}
	defer conn.Close()

	// 读取 connect.challenge 事件（E5 新增）
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, challengeMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read connect.challenge: %v", err)
	}
	var challengeFrame map[string]interface{}
	if err := json.Unmarshal(challengeMsg, &challengeFrame); err != nil {
		t.Fatalf("failed to parse challenge: %v", err)
	}
	if challengeFrame["event"] != "connect.challenge" {
		t.Errorf("expected connect.challenge event, got %v", challengeFrame["event"])
	}

	// 发送 connect 帧 (包含 token)
	connectFrame := map[string]interface{}{
		"type":        "connect",
		"minProtocol": 3,
		"maxProtocol": 3,
		"client": map[string]interface{}{
			"id":       "test-client",
			"version":  "1.0",
			"platform": "test",
			"mode":     "operator",
		},
		"role":   "operator",
		"scopes": []string{"operator.read", "operator.write", "operator.admin"},
		"auth": map[string]string{
			"token": "test-token-123",
		},
	}
	if err := conn.WriteJSON(connectFrame); err != nil {
		t.Fatalf("failed to send connect: %v", err)
	}

	// 读取 hello-ok 响应
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read hello-ok: %v", err)
	}

	var helloOk HelloOk
	if err := json.Unmarshal(msg, &helloOk); err != nil {
		t.Fatalf("failed to parse hello-ok: %v", err)
	}
	if helloOk.Type != FrameTypeHelloOk {
		t.Errorf("expected type %q, got %q", FrameTypeHelloOk, helloOk.Type)
	}
	if helloOk.Protocol != ProtocolVersion {
		t.Errorf("expected protocol %d, got %d", ProtocolVersion, helloOk.Protocol)
	}
	if helloOk.Server.Version != "test-1.0" {
		t.Errorf("expected server version %q, got %q", "test-1.0", helloOk.Server.Version)
	}
	if helloOk.Server.ConnID == "" {
		t.Error("connId should not be empty")
	}

	// 发送 request 帧
	reqFrame := map[string]interface{}{
		"type":   "req",
		"id":     "test-1",
		"method": "health",
	}
	if err := conn.WriteJSON(reqFrame); err != nil {
		t.Fatalf("failed to send request: %v", err)
	}

	// 读取 response 帧 (跳过可能的 event 帧)
	var resp ResponseFrame
	for i := 0; i < 5; i++ { // 最多读 5 帧
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		var frame map[string]interface{}
		json.Unmarshal(msg, &frame)
		if frame["type"] == FrameTypeResponse {
			if err := json.Unmarshal(msg, &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			break
		}
		// 跳过 event 帧
		t.Logf("skipping event frame: %s", string(msg))
	}
	if resp.Type != FrameTypeResponse {
		t.Errorf("expected type %q, got %q", FrameTypeResponse, resp.Type)
	}
	if resp.ID != "test-1" {
		t.Errorf("expected id %q, got %q", "test-1", resp.ID)
	}
	if !resp.OK {
		t.Errorf("expected ok=true, got false, error: %+v", resp.Error)
	}
}

// TestHandleWebSocketUpgrade_BadConnect 验证无效 connect 帧被拒绝。
func TestHandleWebSocketUpgrade_BadConnect(t *testing.T) {
	state := NewGatewayState()
	cfg := WsServerConfig{
		Auth:         ResolvedGatewayAuth{},
		State:        state,
		Registry:     NewMethodRegistry(),
		SessionStore: NewSessionStore(""),
		Version:      "test",
	}

	handler := HandleWebSocketUpgrade(cfg)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial failed: %v", err)
	}
	defer conn.Close()

	// 读取 connect.challenge 事件（E5）
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, challengeMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read connect.challenge: %v", err)
	}
	var cFrame map[string]interface{}
	json.Unmarshal(challengeMsg, &cFrame)
	if cFrame["event"] != "connect.challenge" {
		t.Errorf("expected connect.challenge, got %v", cFrame["event"])
	}

	// 发送错误帧类型
	if err := conn.WriteJSON(map[string]string{"type": "wrong"}); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// 应收到错误响应
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	var resp ResponseFrame
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if resp.OK {
		t.Error("expected ok=false for bad connect frame")
	}
}

// TestHealthEndpoint 验证 /health HTTP 端点。
func TestHealthEndpoint(t *testing.T) {
	runtime, err := StartGatewayServer(0, GatewayServerOptions{})
	if err != nil {
		t.Fatalf("StartGatewayServer failed: %v", err)
	}
	defer runtime.Close("test")

	// 由于 port=0 时绑定到随机端口，我们通过 httptest 测试
	state := runtime.State
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		SendJSON(w, http.StatusOK, GetHealthStatus(state, "test"))
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}
