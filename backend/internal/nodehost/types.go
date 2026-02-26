package nodehost

// types.go — Node Host 类型定义和常量
// 对应 TS: runner.ts L49-158 (类型) + L156-174 (常量)

// ---------- 常量 ----------

const (
	// OutputCap 命令输出字节上限。
	OutputCap = 200_000
	// OutputEventTail 事件通知中输出截断尾部字节数。
	OutputEventTail = 20_000
	// DefaultNodePath 默认 PATH 环境变量。
	DefaultNodePath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	// BrowserProxyMaxFileBytes 浏览器代理文件大小上限 (10MB)。
	BrowserProxyMaxFileBytes = 10 * 1024 * 1024
)

// ---------- 请求/响应类型 ----------

// RunOptions 启动 node host 的选项。
type RunOptions struct {
	GatewayHost           string
	GatewayPort           int
	GatewayTLS            bool
	GatewayTLSFingerprint string
	NodeID                string
	DisplayName           string
}

// SystemRunParams system.run 命令参数。
type SystemRunParams struct {
	Command              []string          `json:"command"`
	RawCommand           *string           `json:"rawCommand,omitempty"`
	Cwd                  *string           `json:"cwd,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	TimeoutMs            *int              `json:"timeoutMs,omitempty"`
	NeedsScreenRecording *bool             `json:"needsScreenRecording,omitempty"`
	AgentID              *string           `json:"agentId,omitempty"`
	SessionKey           *string           `json:"sessionKey,omitempty"`
	Approved             *bool             `json:"approved,omitempty"`
	ApprovalDecision     *string           `json:"approvalDecision,omitempty"`
	RunID                *string           `json:"runId,omitempty"`
}

// SystemWhichParams system.which 命令参数。
type SystemWhichParams struct {
	Bins []string `json:"bins"`
}

// BrowserProxyParams browser.proxy 命令参数。
type BrowserProxyParams struct {
	Method    string                 `json:"method,omitempty"`
	Path      string                 `json:"path,omitempty"`
	Query     map[string]interface{} `json:"query,omitempty"`
	Body      interface{}            `json:"body,omitempty"`
	TimeoutMs *int                   `json:"timeoutMs,omitempty"`
	Profile   *string                `json:"profile,omitempty"`
}

// BrowserProxyFile 浏览器代理返回的文件。
type BrowserProxyFile struct {
	Path     string `json:"path"`
	Base64   string `json:"base64"`
	MimeType string `json:"mimeType,omitempty"`
}

// BrowserProxyResult 浏览器代理返回结果。
type BrowserProxyResult struct {
	Result interface{}         `json:"result"`
	Files  []*BrowserProxyFile `json:"files,omitempty"`
}

// RunResult 命令执行结果。
type RunResult struct {
	ExitCode  *int   `json:"exitCode,omitempty"`
	TimedOut  bool   `json:"timedOut"`
	Success   bool   `json:"success"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error,omitempty"`
	Truncated bool   `json:"-"` // 内部使用，不序列化
}

// ExecEventPayload 执行事件载荷。
type ExecEventPayload struct {
	SessionKey string `json:"sessionKey"`
	RunID      string `json:"runId"`
	Host       string `json:"host"`
	Command    string `json:"command,omitempty"`
	ExitCode   *int   `json:"exitCode,omitempty"`
	TimedOut   *bool  `json:"timedOut,omitempty"`
	Success    *bool  `json:"success,omitempty"`
	Output     string `json:"output,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// NodeInvokeRequest node.invoke.request 载荷。
type NodeInvokeRequest struct {
	ID             string `json:"id"`
	NodeID         string `json:"nodeId"`
	Command        string `json:"command"`
	ParamsJSON     string `json:"paramsJSON,omitempty"`
	TimeoutMs      *int   `json:"timeoutMs,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// InvokeResult node.invoke.result 参数。
type InvokeResult struct {
	ID          string            `json:"id"`
	NodeID      string            `json:"nodeId"`
	OK          bool              `json:"ok"`
	Payload     interface{}       `json:"payload,omitempty"`
	PayloadJSON string            `json:"payloadJSON,omitempty"`
	Error       *InvokeErrorShape `json:"error,omitempty"`
}

// InvokeErrorShape 调用错误。
type InvokeErrorShape struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ExecApprovalsSetParams system.execApprovals.set 参数。
type ExecApprovalsSetParams struct {
	File     interface{} `json:"file"`
	BaseHash *string     `json:"baseHash,omitempty"`
}
