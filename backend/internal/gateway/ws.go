package gateway

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ---------- WebSocket 客户端配置 ----------

// WsClientConfig WebSocket 客户端连接配置。
type WsClientConfig struct {
	URL            string
	Token          string
	Password       string
	ConnectParams  ConnectParams
	TLSSkipVerify  bool
	MaxReconnectMs int // 最大重连间隔，默认 30000
	PingIntervalMs int // 心跳间隔，默认 30000
	PongTimeoutMs  int // pong 超时，默认 60000
}

// WsClientHandler 处理 WebSocket 事件的回调。
type WsClientHandler struct {
	OnOpen    func()
	OnClose   func(code int, reason string)
	OnError   func(err error)
	OnMessage func(msgType int, data []byte)
}

// ---------- WebSocket 客户端 ----------

// GatewayWsClient WebSocket 客户端，自动重连、心跳管理。
type GatewayWsClient struct {
	config  WsClientConfig
	handler WsClientHandler

	mu       sync.Mutex
	writeMu  sync.Mutex // 保护所有写操作，防止并发写竞态
	conn     *websocket.Conn
	closed   atomic.Bool
	stopChan chan struct{}

	// 重连状态
	reconnectAttempts int
}

// NewGatewayWsClient 创建一个新的 WebSocket 客户端。
func NewGatewayWsClient(config WsClientConfig, handler WsClientHandler) *GatewayWsClient {
	if config.MaxReconnectMs <= 0 {
		config.MaxReconnectMs = 30000
	}
	if config.PingIntervalMs <= 0 {
		config.PingIntervalMs = 30000
	}
	if config.PongTimeoutMs <= 0 {
		config.PongTimeoutMs = 60000
	}
	return &GatewayWsClient{
		config:   config,
		handler:  handler,
		stopChan: make(chan struct{}),
	}
}

// Connect 建立 WebSocket 连接。
func (c *GatewayWsClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("client is closed")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	if c.config.Token != "" {
		header.Set("Authorization", "Bearer "+c.config.Token)
	}

	conn, _, err := dialer.Dial(c.config.URL, header)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}

	conn.SetReadLimit(MaxPayloadBytes)
	c.conn = conn
	c.reconnectAttempts = 0

	// 发送 connect 帧
	connectFrame := map[string]interface{}{
		"type":   "connect",
		"connId": uuid.New().String(),
		"role":   c.config.ConnectParams.Role,
		"scopes": c.config.ConnectParams.Scopes,
	}
	if c.config.Token != "" {
		connectFrame["token"] = c.config.Token
	}
	if c.config.Password != "" {
		connectFrame["password"] = c.config.Password
	}
	data, _ := json.Marshal(connectFrame)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.Close()
		return fmt.Errorf("ws connect frame: %w", err)
	}

	// 启动 ping/pong 和消息读取
	go c.readLoop(conn)
	go c.pingLoop(conn)

	if c.handler.OnOpen != nil {
		c.handler.OnOpen()
	}

	return nil
}

// Send 发送消息。
func (c *GatewayWsClient) Send(data []byte) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}

// SendJSON 发送 JSON 消息。
func (c *GatewayWsClient) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Send(data)
}

// Close 关闭连接并停止重连。
func (c *GatewayWsClient) Close() {
	if c.closed.Swap(true) {
		return
	}
	close(c.stopChan)
	c.mu.Lock()
	if c.conn != nil {
		c.writeMu.Lock()
		c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		)
		c.writeMu.Unlock()
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
}

// IsConnected 是否已连接。
func (c *GatewayWsClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// ---------- 内部循环 ----------

func (c *GatewayWsClient) readLoop(conn *websocket.Conn) {
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		conn.Close()
		if !c.closed.Load() {
			go c.reconnect()
		}
	}()

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if !c.closed.Load() && c.handler.OnError != nil {
				c.handler.OnError(err)
			}
			return
		}
		if c.handler.OnMessage != nil {
			c.handler.OnMessage(msgType, data)
		}
	}
}

func (c *GatewayWsClient) pingLoop(conn *websocket.Conn) {
	interval := time.Duration(c.config.PingIntervalMs) * time.Millisecond
	pongTimeout := time.Duration(c.config.PongTimeoutMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 设置 pong 处理器：每次收到 pong 延长读超时
	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.mu.Lock()
			currentConn := c.conn
			c.mu.Unlock()
			if currentConn != conn {
				return
			}
			c.writeMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			c.writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// ---------- 指数退避重连 ----------

func (c *GatewayWsClient) reconnect() {
	for {
		if c.closed.Load() {
			return
		}
		c.reconnectAttempts++
		delay := c.reconnectDelay()

		if c.handler.OnClose != nil {
			c.handler.OnClose(0, "disconnected")
		}

		select {
		case <-time.After(delay):
		case <-c.stopChan:
			return
		}

		if c.closed.Load() {
			return
		}

		if err := c.Connect(); err != nil {
			if c.handler.OnError != nil {
				c.handler.OnError(fmt.Errorf("reconnect failed: %w", err))
			}
			// 继续循环重试，不再递归产生新 goroutine
			continue
		}
		// 连接成功，退出循环
		return
	}
}

func (c *GatewayWsClient) reconnectDelay() time.Duration {
	// 指数退避: 1s, 2s, 4s, 8s... 最大 maxReconnectMs
	baseMs := 1000.0
	maxMs := float64(c.config.MaxReconnectMs)
	delayMs := math.Min(baseMs*math.Pow(2, float64(c.reconnectAttempts-1)), maxMs)
	return time.Duration(delayMs) * time.Millisecond
}

// ---------- WebSocket 服务端升级器 ----------

// WsUpgrader 用于将 HTTP 请求升级为 WebSocket 连接。
var WsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
