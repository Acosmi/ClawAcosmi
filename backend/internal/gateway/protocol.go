package gateway

// ---------- 协议版本 ----------

// ProtocolVersion WS 协议版本号。
const ProtocolVersion = 3

// MinSupportedProtocol 支持的最低协议版本。
// F3: 用于版本协商 — 客户端 maxProtocol 必须 >= MinSupportedProtocol。
const MinSupportedProtocol = 1

// ---------- WS 帧类型 ----------

// RequestFrame WS 请求帧 (客户端 → 服务端)。
type RequestFrame struct {
	Type   string      `json:"type"`   // 固定 "req"
	ID     string      `json:"id"`     // 请求 ID，用于关联响应
	Method string      `json:"method"` // 方法名
	Params interface{} `json:"params,omitempty"`
}

// ResponseFrame WS 响应帧 (服务端 → 客户端)。
type ResponseFrame struct {
	Type    string      `json:"type"`              // 固定 "res"
	ID      string      `json:"id"`                // 匹配 RequestFrame.ID
	OK      bool        `json:"ok"`                // 是否成功
	Payload interface{} `json:"payload,omitempty"` // 成功载荷
	Error   *ErrorShape `json:"error,omitempty"`   // 失败错误
}

// EventFrame WS 事件帧 (服务端 → 客户端)。
// 注意: broadcast.go 中已有 eventFrame (小写，内部使用)，
// 此类型为公开的协议定义，供协议层消费。
type EventFrame struct {
	Type         string        `json:"type"`                   // 固定 "event"
	Event        string        `json:"event"`                  // 事件名
	Payload      interface{}   `json:"payload,omitempty"`      // 事件载荷
	Seq          *int64        `json:"seq,omitempty"`          // 全局递增序号
	StateVersion *StateVersion `json:"stateVersion,omitempty"` // 状态版本
}

// ---------- 错误类型 ----------

// ErrorShape 统一错误结构。
type ErrorShape struct {
	Code         string      `json:"code"`
	Message      string      `json:"message"`
	Details      interface{} `json:"details,omitempty"`
	Retryable    *bool       `json:"retryable,omitempty"`
	RetryAfterMs *int        `json:"retryAfterMs,omitempty"`
}

// NewErrorShape 创建错误结构。
func NewErrorShape(code, message string) *ErrorShape {
	return &ErrorShape{Code: code, Message: message}
}

// WithDetails 附加详情。
func (e *ErrorShape) WithDetails(details interface{}) *ErrorShape {
	e.Details = details
	return e
}

// WithRetryable 标记为可重试。
func (e *ErrorShape) WithRetryable(retryAfterMs int) *ErrorShape {
	t := true
	e.Retryable = &t
	e.RetryAfterMs = &retryAfterMs
	return e
}

// ---------- 错误码常量 ----------
//
// GW-13 向下兼容注释：
// TS 原版定义 5 个错误码（bad_request, unauthorized, not_found, internal_error, not_implemented）。
// Go 端新增 16 个细粒度错误码。TS 客户端应将未识别的错误码视为 internal_error 降级处理。

const (
	// --- TS 原版共有错误码（5 个） ---
	ErrCodeBadRequest     = "bad_request"     // TS 共有
	ErrCodeUnauthorized   = "unauthorized"    // TS 共有
	ErrCodeNotFound       = "not_found"       // TS 共有
	ErrCodeInternalError  = "internal_error"  // TS 共有
	ErrCodeNotImplemented = "not_implemented" // TS 共有

	// --- Go 新增错误码（16 个，TS 客户端需向下兼容） ---
	ErrCodeForbidden          = "forbidden"           // Go 新增
	ErrCodeMethodNotAllowed   = "method_not_allowed"  // Go 新增
	ErrCodeConflict           = "conflict"            // Go 新增
	ErrCodePayloadTooLarge    = "payload_too_large"   // Go 新增
	ErrCodeTooManyRequests    = "too_many_requests"   // Go 新增
	ErrCodeServiceUnavailable = "service_unavailable" // Go 新增
	ErrCodeProtocolMismatch   = "protocol_mismatch"   // Go 新增
	ErrCodeConnectionRejected = "connection_rejected" // Go 新增
	ErrCodeSessionNotFound    = "session_not_found"   // Go 新增
	ErrCodeAgentNotFound      = "agent_not_found"     // Go 新增
	ErrCodeAgentBusy          = "agent_busy"          // Go 新增
	ErrCodeInvalidParams      = "invalid_params"      // Go 新增
	ErrCodeAborted            = "aborted"             // Go 新增
	ErrCodeTimeout            = "timeout"             // Go 新增
	ErrCodeValidationFailed   = "validation_failed"   // Go 新增
	ErrCodeConfigInvalid      = "config_invalid"      // Go 新增
	ErrCodeAlreadyExists      = "already_exists"      // Go 新增
	ErrCodePreconditionFailed = "precondition_failed" // Go 新增
	ErrCodeUnsupportedFeature = "unsupported_feature" // Go 新增
)

// ---------- 连接参数 (完整版) ----------

// ConnectClientInfo 客户端信息。
type ConnectClientInfo struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName,omitempty"`
	Version         string `json:"version"`
	Platform        string `json:"platform"`
	DeviceFamily    string `json:"deviceFamily,omitempty"`
	ModelIdentifier string `json:"modelIdentifier,omitempty"`
	Mode            string `json:"mode"` // "operator" | "node" | "device"
	InstanceID      string `json:"instanceId,omitempty"`
}

// ConnectDeviceAuth 设备认证。
type ConnectDeviceAuth struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce,omitempty"`
}

// ConnectAuthCredentials 连接认证凭据。
type ConnectAuthCredentials struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// ConnectParamsFull 完整连接参数 (对应 TS ConnectParams)。
// 注意: broadcast.go 中的 ConnectParams 仅含 Role/Scopes，此为完整版。
type ConnectParamsFull struct {
	MinProtocol int                     `json:"minProtocol"`
	MaxProtocol int                     `json:"maxProtocol"`
	Client      ConnectClientInfo       `json:"client"`
	Caps        []string                `json:"caps,omitempty"`
	Commands    []string                `json:"commands,omitempty"`
	Permissions map[string]bool         `json:"permissions,omitempty"`
	PathEnv     string                  `json:"pathEnv,omitempty"`
	Role        string                  `json:"role,omitempty"`
	Scopes      []string                `json:"scopes,omitempty"`
	Device      *ConnectDeviceAuth      `json:"device,omitempty"`
	Auth        *ConnectAuthCredentials `json:"auth,omitempty"`
	Locale      string                  `json:"locale,omitempty"`
	UserAgent   string                  `json:"userAgent,omitempty"`
}

// ---------- HelloOk 握手响应 ----------

// HelloOkServer 服务端信息。
type HelloOkServer struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Host    string `json:"host,omitempty"`
	ConnID  string `json:"connId"`
}

// HelloOkFeatures 特性声明。
type HelloOkFeatures struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// HelloOkPolicy 连接策略。
type HelloOkPolicy struct {
	MaxPayload       int `json:"maxPayload"`
	MaxBufferedBytes int `json:"maxBufferedBytes"`
	TickIntervalMs   int `json:"tickIntervalMs"`
}

// HelloOkAuth 认证结果。
type HelloOkAuth struct {
	DeviceToken string   `json:"deviceToken"`
	Role        string   `json:"role"`
	Scopes      []string `json:"scopes"`
	IssuedAtMs  *int64   `json:"issuedAtMs,omitempty"`
}

// HelloOk 握手成功响应帧。
type HelloOk struct {
	Type          string          `json:"type"` // "hello-ok"
	Protocol      int             `json:"protocol"`
	Server        HelloOkServer   `json:"server"`
	Features      HelloOkFeatures `json:"features"`
	Snapshot      interface{}     `json:"snapshot"`
	CanvasHostURL string          `json:"canvasHostUrl,omitempty"`
	Auth          *HelloOkAuth    `json:"auth,omitempty"`
	Policy        HelloOkPolicy   `json:"policy"`
}

// ---------- 在线状态 ----------

// PresenceEntry 在线状态条目 (P-R1: 对齐 TS PresenceEntrySchema — 16 字段)。
type PresenceEntry struct {
	Host             string   `json:"host,omitempty"`
	IP               string   `json:"ip,omitempty"`
	Version          string   `json:"version,omitempty"`
	Platform         string   `json:"platform,omitempty"`
	DeviceFamily     string   `json:"deviceFamily,omitempty"`
	ModelIdentifier  string   `json:"modelIdentifier,omitempty"`
	Mode             string   `json:"mode,omitempty"`
	LastInputSeconds int      `json:"lastInputSeconds,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Text             string   `json:"text,omitempty"`
	Ts               int64    `json:"ts"`
	DeviceID         string   `json:"deviceId,omitempty"`
	Roles            []string `json:"roles,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	InstanceID       string   `json:"instanceId,omitempty"`
}

// SnapshotStateVersion 快照状态版本 (P-R2: 对齐 TS StateVersionSchema)。
type SnapshotStateVersion struct {
	Presence int64 `json:"presence"` // presence 变更计数
	Health   int64 `json:"health"`   // health 变更计数
}

// SessionDefaults 会话默认值 (P-R4: 对齐 TS SessionDefaultsSchema)。
type SessionDefaults struct {
	DefaultAgentID string `json:"defaultAgentId"`
	MainKey        string `json:"mainKey"`
	MainSessionKey string `json:"mainSessionKey"`
	Scope          string `json:"scope,omitempty"`
}

// SnapshotData 快照数据 (P-R3: 对齐 TS SnapshotSchema)。
type SnapshotData struct {
	Presence        []PresenceEntry        `json:"presence,omitempty"`
	Health          map[string]interface{} `json:"health,omitempty"`
	StateVersion    *SnapshotStateVersion  `json:"stateVersion,omitempty"`
	UptimeMs        int64                  `json:"uptimeMs,omitempty"`
	ConfigPath      string                 `json:"configPath,omitempty"`
	StateDir        string                 `json:"stateDir,omitempty"`
	SessionDefaults *SessionDefaults       `json:"sessionDefaults,omitempty"`
}

// ---------- Tick / Shutdown 事件 ----------

// TickEvent 心跳 tick 事件。
type TickEvent struct {
	Ts int64 `json:"ts"`
}

// ShutdownEvent 关闭事件。
type ShutdownEvent struct {
	Reason            string `json:"reason"`
	RestartExpectedMs *int   `json:"restartExpectedMs,omitempty"`
}

// ---------- 帧类型常量 ----------

const (
	FrameTypeRequest  = "req"
	FrameTypeResponse = "res"
	FrameTypeEvent    = "event"
	FrameTypeConnect  = "connect"
	FrameTypeHelloOk  = "hello-ok"
)

// ---------- 默认策略 ----------

// DefaultHelloOkPolicy 默认连接策略。
func DefaultHelloOkPolicy() HelloOkPolicy {
	return HelloOkPolicy{
		MaxPayload:       MaxPayloadBytes,
		MaxBufferedBytes: MaxBufferedBytes,
		TickIntervalMs:   30000,
	}
}
