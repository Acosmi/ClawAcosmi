package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func startEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(mt, data)
		}
	}))
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func TestGatewayWsClient_ConnectAndSend(t *testing.T) {
	server := startEchoServer(t)
	defer server.Close()

	var mu sync.Mutex
	var messages []string
	done := make(chan struct{}, 10)

	client := NewGatewayWsClient(
		WsClientConfig{
			URL:            wsURL(server.URL),
			Token:          "test-token",
			PingIntervalMs: 5000,
			PongTimeoutMs:  10000,
		},
		WsClientHandler{
			OnMessage: func(msgType int, data []byte) {
				mu.Lock()
				messages = append(messages, string(data))
				mu.Unlock()
				done <- struct{}{}
			},
		},
	)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	// 等待 connect 帧回显
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for connect echo")
	}

	// 发送自定义消息
	if err := client.Send([]byte(`{"type":"ping"}`)); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// 等待自定义消息回显
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ping echo")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(messages) < 2 {
		t.Fatalf("expected >=2 messages, got %d", len(messages))
	}
	lastMsg := messages[len(messages)-1]
	if lastMsg != `{"type":"ping"}` {
		t.Errorf("last message = %q, want ping", lastMsg)
	}
}

func TestGatewayWsClient_Close(t *testing.T) {
	server := startEchoServer(t)
	defer server.Close()

	client := NewGatewayWsClient(
		WsClientConfig{URL: wsURL(server.URL), PingIntervalMs: 5000, PongTimeoutMs: 10000},
		WsClientHandler{},
	)
	if err := client.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client.Close()
	if client.IsConnected() {
		t.Error("should be disconnected after Close")
	}
	// Close 应该是幂等的
	client.Close()
}

func TestGatewayWsClient_ReconnectDelay(t *testing.T) {
	client := NewGatewayWsClient(
		WsClientConfig{MaxReconnectMs: 30000},
		WsClientHandler{},
	)
	client.reconnectAttempts = 1
	d := client.reconnectDelay()
	if d != 1*time.Second {
		t.Errorf("attempt 1: delay = %v, want 1s", d)
	}
	client.reconnectAttempts = 2
	d = client.reconnectDelay()
	if d != 2*time.Second {
		t.Errorf("attempt 2: delay = %v, want 2s", d)
	}
	client.reconnectAttempts = 3
	d = client.reconnectDelay()
	if d != 4*time.Second {
		t.Errorf("attempt 3: delay = %v, want 4s", d)
	}
	client.reconnectAttempts = 20
	d = client.reconnectDelay()
	if d != 30*time.Second {
		t.Errorf("attempt 20: delay = %v, want 30s (cap)", d)
	}
}

func TestWsUpgrader_Defined(t *testing.T) {
	if WsUpgrader.ReadBufferSize != 4096 {
		t.Errorf("ReadBufferSize = %d", WsUpgrader.ReadBufferSize)
	}
	if WsUpgrader.WriteBufferSize != 4096 {
		t.Errorf("WriteBufferSize = %d", WsUpgrader.WriteBufferSize)
	}
}
