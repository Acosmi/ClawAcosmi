package autoreply

import "context"

// TS 对照: auto-reply/reply/commands-types.ts (64L)

// ---------- 命令上下文 ----------

// CommandContext 命令上下文（从 MsgContext + Config 构建）。
// TS 对照: commands-types.ts CommandContext
type CommandContext struct {
	Surface               string
	Channel               string
	ChannelID             string
	OwnerList             []string
	SenderIsOwner         bool
	IsAuthorizedSender    bool
	SenderID              string
	AbortKey              string
	RawBodyNormalized     string
	CommandBodyNormalized string
	From                  string
	To                    string
}

// ---------- 命令处理器签名 ----------

// CommandHandlerResult 命令处理结果。
// TS 对照: commands-types.ts CommandHandlerResult
type CommandHandlerResult struct {
	Reply          *ReplyPayload
	ShouldContinue bool
}

// CommandHandler 命令处理器函数签名。
// Go 版本多接受一个 context.Context，对齐 Go 惯例。
// 返回 (nil, nil) 表示该 handler 不处理该命令（TS 中返回 null）。
// TS 对照: commands-types.ts CommandHandler
type CommandHandler func(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error)

// ---------- 命令处理参数 ----------

// HandleCommandsParams 命令处理参数包。
// TS 对照: commands-types.ts HandleCommandsParams
type HandleCommandsParams struct {
	MsgCtx     *MsgContext
	Command    *CommandContext
	AgentID    string
	IsGroup    bool
	SessionKey string

	// 以下字段暂用简单类型/DI 接口；对应模块完全移植后替换
	Provider string
	Model    string

	// DI — 外部依赖通过接口注入
	GatewayCaller  GatewayCaller
	SessionUpdater SessionStoreUpdater
	BashExecutor   BashExecutor
	PluginMatcher  PluginCommandMatcher
}

// ---------- DI 接口（占位） ----------

// GatewayCaller 网关调用接口。
// TS 对照: gateway/call.ts callGateway
type GatewayCaller interface {
	CallGateway(ctx context.Context, method string, params map[string]any) (map[string]any, error)
}

// SessionStoreUpdater 会话存储更新接口。
// TS 对照: config/sessions.ts updateSessionStore
type SessionStoreUpdater interface {
	UpdateSessionStore(path string, fn func(store map[string]any)) error
}

// BashExecutor Bash 命令执行接口。
// TS 对照: auto-reply/reply/commands-bash.ts handleBashChatCommand
type BashExecutor interface {
	HandleBashChatCommand(ctx context.Context, params map[string]any) (*ReplyPayload, error)
}

// PluginCommandMatch 插件命令匹配结果。
type PluginCommandMatch struct {
	Command string
	Args    string
}

// PluginCommandMatcher 插件命令匹配接口。
// TS 对照: auto-reply/reply/commands-plugin.ts matchPluginCommand / executePluginCommand
type PluginCommandMatcher interface {
	MatchPluginCommand(body string) *PluginCommandMatch
	ExecutePluginCommand(ctx context.Context, match *PluginCommandMatch, params map[string]any) (*ReplyPayload, error)
}
