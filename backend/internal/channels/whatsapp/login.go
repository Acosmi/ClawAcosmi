package whatsapp

import (
	"fmt"
)

// WhatsApp CLI 登录 — 继承自 src/web/login.ts (79L)
// Baileys SDK WebSocket/Session 实际集成延迟到 Phase 6 Gateway 阶段

// DisconnectReason WhatsApp 断开原因码
type DisconnectReason int

const (
	DisconnectReasonLoggedOut          DisconnectReason = 401
	DisconnectReasonConnectionReplaced DisconnectReason = 440
	DisconnectReasonTimedOut           DisconnectReason = 408
	DisconnectReasonRestartRequired    DisconnectReason = 515
)

// LoginResult 登录结果
type LoginResult struct {
	Connected bool
	Message   string
	Error     error
}

// LoginWebOptions 登录选项
type LoginWebOptions struct {
	Verbose   bool
	AuthDir   string
	AccountID string
}

// LoginWeb 执行 WhatsApp Web 登录
// Phase 6 Gateway: 此函数将集成实际的 Baileys WebSocket 连接
func LoginWeb(opts LoginWebOptions) (*LoginResult, error) {
	if opts.AuthDir == "" {
		opts.AuthDir = ResolveDefaultWebAuthDir()
	}

	// 检查是否已有认证
	if WebAuthExists(opts.AuthDir) {
		selfID := ReadWebSelfId(opts.AuthDir)
		who := selfID.E164
		if who == "" {
			who = selfID.JID
		}
		if who == "" {
			who = "unknown"
		}
		return &LoginResult{
			Connected: true,
			Message:   fmt.Sprintf("WhatsApp already linked (%s)", who),
		}, nil
	}

	// Phase 6: 实际 WebSocket 连接和 QR 码扫描
	return &LoginResult{
		Connected: false,
		Message:   "WhatsApp Web login requires gateway runtime (Phase 6)",
	}, nil
}
