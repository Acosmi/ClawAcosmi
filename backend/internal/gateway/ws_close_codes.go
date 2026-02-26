package gateway

// ws_close_codes.go — WebSocket Close Code 常量
// 对齐 TS gateway/server/ws-connection/message-handler.ts
//
// RFC 6455 定义的标准 close codes:
//   1000 — Normal Closure
//   1001 — Going Away
//   1002 — Protocol Error
//   1008 — Policy Violation
//   1012 — Service Restart (not in original RFC, added in IANA registry)

const (
	// WsCloseNormal 正常关闭。
	WsCloseNormal = 1000

	// WsCloseGoingAway 服务端关闭或客户端导航离开。
	WsCloseGoingAway = 1001

	// WsCloseProtocolError 协议错误（如版本不匹配）。
	WsCloseProtocolError = 1002

	// WsClosePolicyViolation 策略违规（认证失败、设备验证失败等）。
	// TS 对照: message-handler.ts 中所有认证/设备/配对相关关闭均使用 1008。
	WsClosePolicyViolation = 1008

	// WsCloseServiceRestart 服务重启（优雅关闭时通知客户端）。
	// TS 对照: server-close.ts L101: close(1012, "service restart")
	WsCloseServiceRestart = 1012
)

// 常用 close reason 字符串常量。
// 对齐 TS message-handler.ts 中的 close reason。
const (
	WsReasonPolicyViolation     = "policy violation"
	WsReasonAuthFailed          = "authentication failed"
	WsReasonProtocolMismatch    = "protocol mismatch"
	WsReasonNonceMismatch       = "device nonce mismatch"
	WsReasonDeviceAuthFailed    = "device auth failed"
	WsReasonPairingRequired     = "pairing required"
	WsReasonInvalidRole         = "invalid role"
	WsReasonDeviceIdentRequired = "device identity required"
	WsReasonDeviceIdentMismatch = "device identity mismatch"
	WsReasonDeviceSigExpired    = "device signature expired"
	WsReasonDeviceSigInvalid    = "device signature invalid"
	WsReasonDevicePubKeyInvalid = "device public key invalid"
	WsReasonDeviceNonceRequired = "device nonce required"
	WsReasonSlowConsumer        = "slow consumer"
	WsReasonServiceRestart      = "service restart"
	WsReasonConnectFailed       = "connect failed"
)
