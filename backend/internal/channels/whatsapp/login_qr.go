package whatsapp

import (
	"fmt"
	"sync"
	"time"
)

// WhatsApp QR 码登录 — 继承自 src/web/login-qr.ts (296L)
// 定义状态管理和接口骨架，实际 Baileys QR 生成延迟到 Phase 6

const activeLoginTTLMs = 3 * 60 * 1000 // 3 分钟

// ActiveLogin 活跃登录会话状态
type ActiveLogin struct {
	AccountID        string
	AuthDir          string
	IsLegacyAuthDir  bool
	ID               string
	StartedAt        time.Time
	QR               string
	QRDataURL        string
	Connected        bool
	Error            string
	ErrorStatus      int
	RestartAttempted bool
	Verbose          bool
}

var (
	activeLoginsMu sync.Mutex
	activeLogins   = make(map[string]*ActiveLogin)
)

// isLoginFresh 检查登录是否在 TTL 内
func isLoginFresh(login *ActiveLogin) bool {
	return time.Since(login.StartedAt).Milliseconds() < int64(activeLoginTTLMs)
}

// resetActiveLogin 重置活跃登录
func resetActiveLogin(accountID string) {
	activeLoginsMu.Lock()
	defer activeLoginsMu.Unlock()
	delete(activeLogins, accountID)
}

// getActiveLogin 获取活跃登录
func getActiveLogin(accountID string) *ActiveLogin {
	activeLoginsMu.Lock()
	defer activeLoginsMu.Unlock()
	return activeLogins[accountID]
}

// QRLoginResult QR 登录启动结果
type QRLoginResult struct {
	QRDataURL string
	Message   string
}

// StartWebLoginWithQROptions QR 登录选项
type StartWebLoginWithQROptions struct {
	Verbose   bool
	TimeoutMs int
	Force     bool
	AccountID string
}

// StartWebLoginWithQR 启动 QR 码登录流程
// Phase 6 Gateway: 需要集成实际的 Baileys WebSocket 和 QR 生成
func StartWebLoginWithQR(opts StartWebLoginWithQROptions) (*QRLoginResult, error) {
	accountID := ResolveWebAccountID(opts.AccountID)

	// 检查已有认证
	authResult := ResolveWhatsAppAuthDir(nil, accountID)
	if !opts.Force && WebAuthExists(authResult.AuthDir) {
		selfID := ReadWebSelfId(authResult.AuthDir)
		who := selfID.E164
		if who == "" {
			who = selfID.JID
		}
		if who == "" {
			who = "unknown"
		}
		return &QRLoginResult{
			Message: fmt.Sprintf("WhatsApp is already linked (%s). Say \"relink\" if you want a fresh QR.", who),
		}, nil
	}

	// 检查是否有活跃登录
	existing := getActiveLogin(accountID)
	if existing != nil && isLoginFresh(existing) && existing.QRDataURL != "" {
		return &QRLoginResult{
			QRDataURL: existing.QRDataURL,
			Message:   "QR already active. Scan it in WhatsApp → Linked Devices.",
		}, nil
	}

	// Phase 6: 创建 Baileys socket、生成 QR、等待扫描
	return &QRLoginResult{
		Message: "QR login requires gateway runtime (Phase 6)",
	}, nil
}

// WaitForWebLoginOptions 等待登录选项
type WaitForWebLoginOptions struct {
	TimeoutMs int
	AccountID string
}

// WaitForWebLoginResult 等待登录结果
type WaitForWebLoginResult struct {
	Connected bool
	Message   string
}

// WaitForWebLogin 等待 QR 扫描完成
func WaitForWebLogin(opts WaitForWebLoginOptions) *WaitForWebLoginResult {
	accountID := ResolveWebAccountID(opts.AccountID)

	login := getActiveLogin(accountID)
	if login == nil {
		return &WaitForWebLoginResult{
			Connected: false,
			Message:   "No active WhatsApp login in progress.",
		}
	}

	if !isLoginFresh(login) {
		resetActiveLogin(accountID)
		return &WaitForWebLoginResult{
			Connected: false,
			Message:   "The login QR expired. Ask me to generate a new one.",
		}
	}

	if login.Connected {
		resetActiveLogin(accountID)
		return &WaitForWebLoginResult{
			Connected: true,
			Message:   "✅ Linked! WhatsApp is ready.",
		}
	}

	if login.Error != "" {
		message := fmt.Sprintf("WhatsApp login failed: %s", login.Error)
		resetActiveLogin(accountID)
		return &WaitForWebLoginResult{
			Connected: false,
			Message:   message,
		}
	}

	return &WaitForWebLoginResult{
		Connected: false,
		Message:   "Still waiting for the QR scan. Let me know when you've scanned it.",
	}
}
