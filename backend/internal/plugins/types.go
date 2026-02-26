package plugins

// PluginLogger 插件日志接口
// 对应 TS: types.ts PluginLogger
type PluginLogger struct {
	Debug func(message string)
	Info  func(message string)
	Warn  func(message string)
	Error func(message string)
}

// PluginKind 插件类型
type PluginKind string

const (
	PluginKindMemory PluginKind = "memory"
)

// PluginOrigin 插件来源
type PluginOrigin string

const (
	PluginOriginBundled   PluginOrigin = "bundled"
	PluginOriginGlobal    PluginOrigin = "global"
	PluginOriginWorkspace PluginOrigin = "workspace"
	PluginOriginConfig    PluginOrigin = "config"
)

// PluginConfigUiHint 配置 UI 提示
type PluginConfigUiHint struct {
	Label       string `json:"label,omitempty"`
	Help        string `json:"help,omitempty"`
	Advanced    bool   `json:"advanced,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// PluginConfigValidation 配置校验结果
type PluginConfigValidation struct {
	OK     bool
	Value  interface{}
	Errors []string
}

// PluginConfigSchema 插件配置 schema
// 对应 TS: types.ts OpenAcosmiPluginConfigSchema
type PluginConfigSchema struct {
	SafeParse  func(value interface{}) PluginConfigValidation
	JsonSchema map[string]interface{}        `json:"jsonSchema,omitempty"`
	UiHints    map[string]PluginConfigUiHint `json:"uiHints,omitempty"`
}

// PluginDiagnostic 插件诊断信息
type PluginDiagnostic struct {
	Level    string `json:"level"` // "warn" | "error"
	Message  string `json:"message"`
	PluginID string `json:"pluginId,omitempty"`
	Source   string `json:"source,omitempty"`
}

// PluginToolContext 工具注册上下文
// 对应 TS: types.ts OpenAcosmiPluginToolContext
type PluginToolContext struct {
	WorkspaceDir   string
	AgentDir       string
	AgentID        string
	SessionKey     string
	MessageChannel string
	AgentAccountID string
	Sandboxed      bool
}

// PluginToolOptions 工具注册选项
type PluginToolOptions struct {
	Name     string
	Names    []string
	Optional bool
}

// PluginHookOptions 钩子注册选项
type PluginHookOptions struct {
	Name        string
	Description string
	Register    *bool // nil = default (true)
}

// ProviderAuthKind 认证方式
type ProviderAuthKind string

const (
	ProviderAuthOAuth      ProviderAuthKind = "oauth"
	ProviderAuthAPIKey     ProviderAuthKind = "api_key"
	ProviderAuthToken      ProviderAuthKind = "token"
	ProviderAuthDeviceCode ProviderAuthKind = "device_code"
	ProviderAuthCustom     ProviderAuthKind = "custom"
)

// ProviderAuthResult 认证结果
type ProviderAuthResult struct {
	Profiles     []ProviderAuthProfile
	ConfigPatch  map[string]interface{}
	DefaultModel string
	Notes        []string
}

// ProviderAuthProfile 认证 profile
type ProviderAuthProfile struct {
	ProfileID  string
	Credential map[string]interface{}
}

// ProviderAuthMethod 认证方法
type ProviderAuthMethod struct {
	ID    string
	Label string
	Hint  string
	Kind  ProviderAuthKind
	// Run 在调用层实现，此处仅记录元数据
}

// ProviderPlugin 提供商插件
type ProviderPlugin struct {
	ID       string
	Label    string
	DocsPath string
	Aliases  []string
	EnvVars  []string
	Models   map[string]interface{}
	Auth     []ProviderAuthMethod
}

// PluginCommandContext 命令上下文
// 对应 TS: types.ts PluginCommandContext
type PluginCommandContext struct {
	SenderID           string
	Channel            string
	ChannelID          string
	IsAuthorizedSender bool
	Args               string
	CommandBody        string
	From               string
	To                 string
	AccountID          string
	MessageThreadID    int
}

// PluginCommandResult 命令结果
type PluginCommandResult struct {
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// PluginCommandHandler 命令处理函数
type PluginCommandHandler func(ctx PluginCommandContext) (PluginCommandResult, error)

// PluginCommandDefinition 命令定义
// 对应 TS: types.ts OpenAcosmiPluginCommandDefinition
type PluginCommandDefinition struct {
	Name        string
	Description string
	AcceptsArgs bool
	RequireAuth *bool // nil = default (true)
	Handler     PluginCommandHandler
}

// PluginServiceContext 服务上下文
type PluginServiceContext struct {
	WorkspaceDir string
	StateDir     string
	Logger       PluginLogger
}

// PluginService 插件服务
type PluginService struct {
	ID    string
	Start func(ctx PluginServiceContext) error
	Stop  func(ctx PluginServiceContext) error
}

// PluginDefinition 插件定义
// 对应 TS: types.ts OpenAcosmiPluginDefinition
type PluginDefinition struct {
	ID           string
	Name         string
	Description  string
	Version      string
	Kind         PluginKind
	ConfigSchema *PluginConfigSchema
}

// =============================================================================
// Plugin Hook Types
// =============================================================================

// PluginHookName 钩子名称
type PluginHookName string

const (
	HookBeforeAgentStart  PluginHookName = "before_agent_start"
	HookAgentEnd          PluginHookName = "agent_end"
	HookBeforeCompaction  PluginHookName = "before_compaction"
	HookAfterCompaction   PluginHookName = "after_compaction"
	HookMessageReceived   PluginHookName = "message_received"
	HookMessageSending    PluginHookName = "message_sending"
	HookMessageSent       PluginHookName = "message_sent"
	HookBeforeToolCall    PluginHookName = "before_tool_call"
	HookAfterToolCall     PluginHookName = "after_tool_call"
	HookToolResultPersist PluginHookName = "tool_result_persist"
	HookSessionStart      PluginHookName = "session_start"
	HookSessionEnd        PluginHookName = "session_end"
	HookGatewayStart      PluginHookName = "gateway_start"
	HookGatewayStop       PluginHookName = "gateway_stop"
)

// AllPluginHookNames 所有钩子名称
var AllPluginHookNames = []PluginHookName{
	HookBeforeAgentStart, HookAgentEnd,
	HookBeforeCompaction, HookAfterCompaction,
	HookMessageReceived, HookMessageSending, HookMessageSent,
	HookBeforeToolCall, HookAfterToolCall, HookToolResultPersist,
	HookSessionStart, HookSessionEnd,
	HookGatewayStart, HookGatewayStop,
}

// PluginHookAgentContext Agent 级钩子上下文
type PluginHookAgentContext struct {
	AgentID         string
	SessionKey      string
	WorkspaceDir    string
	MessageProvider string
}

// PluginHookBeforeAgentStartEvent before_agent_start
type PluginHookBeforeAgentStartEvent struct {
	Prompt   string
	Messages []interface{}
}

// PluginHookBeforeAgentStartResult before_agent_start 结果
type PluginHookBeforeAgentStartResult struct {
	SystemPrompt   string
	PrependContext string
}

// PluginHookAgentEndEvent agent_end
type PluginHookAgentEndEvent struct {
	Messages   []interface{}
	Success    bool
	Error      string
	DurationMs int64
}

// PluginHookBeforeCompactionEvent before_compaction
type PluginHookBeforeCompactionEvent struct {
	MessageCount int
	TokenCount   *int
}

// PluginHookAfterCompactionEvent after_compaction
type PluginHookAfterCompactionEvent struct {
	MessageCount   int
	TokenCount     *int
	CompactedCount int
}

// PluginHookMessageContext 消息级上下文
type PluginHookMessageContext struct {
	ChannelID      string
	AccountID      string
	ConversationID string
}

// PluginHookMessageReceivedEvent message_received
type PluginHookMessageReceivedEvent struct {
	From      string
	Content   string
	Timestamp *int64
	Metadata  map[string]interface{}
}

// PluginHookMessageSendingEvent message_sending
type PluginHookMessageSendingEvent struct {
	To       string
	Content  string
	Metadata map[string]interface{}
}

// PluginHookMessageSendingResult message_sending 结果
type PluginHookMessageSendingResult struct {
	Content string
	Cancel  bool
}

// PluginHookMessageSentEvent message_sent
type PluginHookMessageSentEvent struct {
	To      string
	Content string
	Success bool
	Error   string
}

// PluginHookToolContext 工具级上下文
type PluginHookToolContext struct {
	AgentID    string
	SessionKey string
	ToolName   string
}

// PluginHookBeforeToolCallEvent before_tool_call
type PluginHookBeforeToolCallEvent struct {
	ToolName string
	Params   map[string]interface{}
}

// PluginHookBeforeToolCallResult before_tool_call 结果
type PluginHookBeforeToolCallResult struct {
	Params      map[string]interface{}
	Block       bool
	BlockReason string
}

// PluginHookAfterToolCallEvent after_tool_call
type PluginHookAfterToolCallEvent struct {
	ToolName   string
	Params     map[string]interface{}
	Result     interface{}
	Error      string
	DurationMs int64
}

// PluginHookToolResultPersistContext tool_result_persist 上下文
type PluginHookToolResultPersistContext struct {
	AgentID    string
	SessionKey string
	ToolName   string
	ToolCallID string
}

// PluginHookToolResultPersistEvent tool_result_persist
type PluginHookToolResultPersistEvent struct {
	ToolName    string
	ToolCallID  string
	Message     map[string]interface{} // AgentMessage
	IsSynthetic bool
}

// PluginHookToolResultPersistResult tool_result_persist 结果
type PluginHookToolResultPersistResult struct {
	Message map[string]interface{}
}

// PluginHookSessionContext session 级上下文
type PluginHookSessionContext struct {
	AgentID   string
	SessionID string
}

// PluginHookSessionStartEvent session_start
type PluginHookSessionStartEvent struct {
	SessionID   string
	ResumedFrom string
}

// PluginHookSessionEndEvent session_end
type PluginHookSessionEndEvent struct {
	SessionID    string
	MessageCount int
	DurationMs   *int64
}

// PluginHookGatewayContext gateway 级上下文
type PluginHookGatewayContext struct {
	Port int
}

// PluginHookGatewayStartEvent gateway_start
type PluginHookGatewayStartEvent struct {
	Port int
}

// PluginHookGatewayStopEvent gateway_stop
type PluginHookGatewayStopEvent struct {
	Reason string
}

// PluginHookRegistration 类型化钩子注册
type PluginHookRegistration struct {
	PluginID string
	HookName PluginHookName
	Handler  interface{} // 具体 handler func 类型在运行时确定
	Priority *int
	Source   string
}
