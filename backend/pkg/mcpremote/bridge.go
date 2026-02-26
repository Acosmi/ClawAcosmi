package mcpremote

// bridge.go — 远程 MCP 工具桥接
//
// 状态机: init → connecting → ready → degraded → stopped
// 参考 internal/argus/bridge.go 模式:
//   - 健康检查: 定期 ping，3 次失败 → degraded
//   - 自动重连: 指数退避 (1s→2s→4s...→60s)，最多 5 次
//   - 工具缓存: 连接时 ListTools 并缓存

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ---------- 常量 ----------

const (
	bridgeHealthInterval  = 30 * time.Second
	bridgeMaxPingFailures = 3
	bridgeMaxReconnects   = 5
	bridgeInitialBackoff  = 1 * time.Second
	bridgeMaxBackoff      = 60 * time.Second
)

// ---------- 状态 ----------

// BridgeState RemoteBridge 状态。
type BridgeState string

const (
	BridgeStateInit       BridgeState = "init"
	BridgeStateConnecting BridgeState = "connecting"
	BridgeStateReady      BridgeState = "ready"
	BridgeStateDegraded   BridgeState = "degraded"
	BridgeStateStopped    BridgeState = "stopped"
)

// ---------- 配置 ----------

// RemoteBridgeConfig 远程 MCP Bridge 配置。
type RemoteBridgeConfig struct {
	Endpoint       string // MCP Streamable HTTP 端点
	TokenManager   *OAuthTokenManager
	HealthInterval time.Duration // 健康检查间隔（0 = 默认 30s）
}

// ---------- Bridge ----------

// RemoteBridge 远程 MCP 工具桥接器。
type RemoteBridge struct {
	cfg RemoteBridgeConfig

	mu       sync.RWMutex
	state    BridgeState
	client   *RemoteClient
	tools    []Tool
	lastPing time.Time
	lastRTT  time.Duration

	cancel context.CancelFunc
	done   chan struct{}
}

// NewRemoteBridge 创建远程 MCP Bridge。
func NewRemoteBridge(cfg RemoteBridgeConfig) *RemoteBridge {
	if cfg.HealthInterval <= 0 {
		cfg.HealthInterval = bridgeHealthInterval
	}
	return &RemoteBridge{
		cfg:   cfg,
		state: BridgeStateInit,
		done:  make(chan struct{}),
	}
}

// State 返回当前状态。
func (b *RemoteBridge) State() BridgeState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Tools 返回缓存的远程工具列表。
func (b *RemoteBridge) Tools() []Tool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cp := make([]Tool, len(b.tools))
	copy(cp, b.tools)
	return cp
}

// Endpoint 返回 MCP 端点 URL。
func (b *RemoteBridge) Endpoint() string { return b.cfg.Endpoint }

// LastPing 返回最近健康检查信息。
func (b *RemoteBridge) LastPing() (time.Time, time.Duration) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastPing, b.lastRTT
}

// Start 连接到远程 MCP Server 并发现工具。
// cancel + done 在连接前创建，确保 Stop() 随时可安全调用。
func (b *RemoteBridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.state != BridgeStateInit && b.state != BridgeStateStopped {
		b.mu.Unlock()
		return fmt.Errorf("mcpremote: cannot start in state %s", b.state)
	}
	b.state = BridgeStateConnecting
	// 先创建 cancel + done，避免 Stop() 在连接期间遇到 nil cancel
	bgCtx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.done = make(chan struct{})
	b.mu.Unlock()

	if err := b.connectAndDiscover(ctx); err != nil {
		cancel() // 清理 background context
		b.mu.Lock()
		b.state = BridgeStateStopped
		b.cancel = nil
		close(b.done) // 标记 done，让 Stop() 的等待不会死锁
		b.mu.Unlock()
		return err
	}

	go b.healthLoop(bgCtx)

	return nil
}

// connectAndDiscover 连接并发现工具。
func (b *RemoteBridge) connectAndDiscover(ctx context.Context) error {
	client := NewRemoteClient(b.cfg.Endpoint, b.cfg.TokenManager)

	// MCP 握手 (10s 超时)
	initCtx, initCancel := context.WithTimeout(ctx, 10*time.Second)
	defer initCancel()

	_, err := client.Connect(initCtx)
	if err != nil {
		client.Close()
		return fmt.Errorf("mcpremote: connect: %w", err)
	}

	// 工具发现 (10s 超时)
	listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
	defer listCancel()

	tools, err := client.ListTools(listCtx)
	if err != nil {
		client.Close()
		return fmt.Errorf("mcpremote: tools/list: %w", err)
	}

	b.mu.Lock()
	b.client = client
	b.tools = tools
	b.state = BridgeStateReady
	b.lastPing = time.Now()
	b.mu.Unlock()

	slog.Info("mcpremote: bridge ready",
		"endpoint", b.cfg.Endpoint,
		"tools", len(tools),
	)

	return nil
}

// healthLoop 定期 ping 检查连接健康状态。
func (b *RemoteBridge) healthLoop(ctx context.Context) {
	defer close(b.done)

	ticker := time.NewTicker(b.cfg.HealthInterval)
	defer ticker.Stop()
	failures := 0
	backoff := bridgeInitialBackoff
	reconnects := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.mu.RLock()
			client := b.client
			state := b.state
			b.mu.RUnlock()

			if state == BridgeStateStopped {
				return
			}

			if client == nil {
				// 尝试重连
				if reconnects >= bridgeMaxReconnects {
					slog.Error("mcpremote: max reconnect attempts exceeded")
					b.mu.Lock()
					b.state = BridgeStateStopped
					b.mu.Unlock()
					return
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}

				reconnects++
				slog.Info("mcpremote: reconnecting", "attempt", reconnects, "backoff", backoff)

				if err := b.connectAndDiscover(ctx); err != nil {
					slog.Warn("mcpremote: reconnect failed", "error", err, "attempt", reconnects)
					backoff = min(backoff*2, bridgeMaxBackoff)
					continue
				}

				// 重连成功
				reconnects = 0
				failures = 0
				backoff = bridgeInitialBackoff
				continue
			}

			// Ping 健康检查
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			rtt, err := client.Ping(pingCtx)
			cancel()

			if err != nil {
				failures++
				slog.Warn("mcpremote: ping failed", "failures", failures, "error", err)
				if failures >= bridgeMaxPingFailures {
					b.mu.Lock()
					if b.state == BridgeStateReady {
						b.state = BridgeStateDegraded
						slog.Warn("mcpremote: degraded — ping failures exceeded threshold")
					}
					// 关闭当前客户端，下一轮将尝试重连
					if b.client != nil {
						b.client.Close()
						b.client = nil
					}
					b.mu.Unlock()
					failures = 0
				}
			} else {
				failures = 0
				b.mu.Lock()
				b.lastPing = time.Now()
				b.lastRTT = rtt
				if b.state == BridgeStateDegraded {
					b.state = BridgeStateReady
					slog.Info("mcpremote: recovered from degraded state", "rtt", rtt)
				}
				b.mu.Unlock()
			}
		}
	}
}

// CallTool 调用远程 MCP 工具。
func (b *RemoteBridge) CallTool(ctx context.Context, name string, arguments json.RawMessage, timeout time.Duration) (*ToolCallResult, error) {
	b.mu.RLock()
	client := b.client
	state := b.state
	b.mu.RUnlock()

	if client == nil || (state != BridgeStateReady && state != BridgeStateDegraded) {
		return nil, fmt.Errorf("mcpremote: bridge not available (state: %s)", state)
	}

	return client.CallTool(ctx, name, arguments, timeout)
}

// Refresh 重新发现远程工具列表。
func (b *RemoteBridge) Refresh(ctx context.Context) error {
	b.mu.RLock()
	client := b.client
	b.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("mcpremote: not connected")
	}

	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tools, err := client.ListTools(listCtx)
	if err != nil {
		return fmt.Errorf("mcpremote: refresh tools/list: %w", err)
	}

	b.mu.Lock()
	b.tools = tools
	b.mu.Unlock()

	slog.Info("mcpremote: tools refreshed", "count", len(tools))
	return nil
}

// Stop 优雅关闭 Bridge，等待 healthLoop 退出。
func (b *RemoteBridge) Stop() {
	b.mu.Lock()
	if b.state == BridgeStateStopped {
		b.mu.Unlock()
		return
	}
	cancel := b.cancel
	client := b.client
	done := b.done
	b.state = BridgeStateStopped
	b.mu.Unlock()

	// 取消后台循环
	if cancel != nil {
		cancel()
	}

	// 等待 healthLoop 退出（最多 5s）
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			slog.Warn("mcpremote: healthLoop did not exit within 5s")
		}
	}

	// 关闭客户端
	if client != nil {
		client.Close()
	}

	b.mu.Lock()
	b.client = nil
	b.tools = nil
	b.mu.Unlock()

	slog.Info("mcpremote: bridge stopped")
}

// ---------- Agent 集成辅助 ----------

// ToolCallResultToText 将 ToolCallResult 转为纯文本（供 Agent tool executor 使用）。
func ToolCallResultToText(result *ToolCallResult) string {
	if result == nil {
		return ""
	}
	var sb strings.Builder
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(c.Text)
		case "image":
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("[image: %s, %d bytes base64]", c.MIME, len(c.Data)))
		}
	}
	if result.IsError {
		return fmt.Sprintf("[Remote tool error] %s", sb.String())
	}
	return sb.String()
}
