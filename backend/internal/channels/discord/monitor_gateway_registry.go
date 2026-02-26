package discord

import "sync"

// Discord Gateway 注册表 — 继承自 src/discord/monitor/gateway-registry.ts (37L)
// 模块级 Gateway 会话实例注册表，允许跨模块访问 WebSocket 连接。

const defaultGatewayAccountKey = "\x00__default__"

var (
	gatewayRegistryMu sync.RWMutex
	gatewayRegistry   = make(map[string]GatewaySession)
)

// GatewaySession 抽象 Gateway 会话接口。
// discordgo.Session 满足此接口。
type GatewaySession interface {
	// RequestGuildMembers requests guild members from the gateway.
	RequestGuildMembers(guildID, query string, limit int, nonce string, presences bool) error
}

func resolveGatewayAccountKey(accountID string) string {
	if accountID == "" {
		return defaultGatewayAccountKey
	}
	return accountID
}

// RegisterGateway registers a Gateway session for an account.
// TS ref: registerGateway()
func RegisterGateway(accountID string, gw GatewaySession) {
	gatewayRegistryMu.Lock()
	defer gatewayRegistryMu.Unlock()
	gatewayRegistry[resolveGatewayAccountKey(accountID)] = gw
}

// UnregisterGateway removes a Gateway session for an account.
// TS ref: unregisterGateway()
func UnregisterGateway(accountID string) {
	gatewayRegistryMu.Lock()
	defer gatewayRegistryMu.Unlock()
	delete(gatewayRegistry, resolveGatewayAccountKey(accountID))
}

// GetGateway returns the Gateway session for an account, or nil if not registered.
// TS ref: getGateway()
func GetGateway(accountID string) GatewaySession {
	gatewayRegistryMu.RLock()
	defer gatewayRegistryMu.RUnlock()
	return gatewayRegistry[resolveGatewayAccountKey(accountID)]
}

// ClearGateways removes all registered gateways (for testing).
// TS ref: clearGateways()
func ClearGateways() {
	gatewayRegistryMu.Lock()
	defer gatewayRegistryMu.Unlock()
	gatewayRegistry = make(map[string]GatewaySession)
}
