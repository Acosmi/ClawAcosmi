package autoreply

import (
	"context"
	"strings"
)

// TS 对照: auto-reply/reply/commands-core.ts (135L)

// ShouldHandleTextCommands 判断是否应处理文本命令。
// TS 对照: commands-core.ts shouldHandleTextCommands
func ShouldHandleTextCommands(params *ShouldHandleTextCommandsParams) bool {
	if params == nil {
		return false
	}
	// native 命令源始终处理
	if params.CommandSource == "native" {
		return true
	}
	// 文本命令源需要显式允许
	return params.CommandSource == "text"
}

// HandleCommands 主命令分发器。
// 遍历注册的命令处理器链，按顺序尝试匹配，直到某个处理器返回非 nil 结果。
// TS 对照: commands-core.ts handleCommands
func HandleCommands(ctx context.Context, params *HandleCommandsParams) (*CommandHandlerResult, error) {
	if params == nil || params.Command == nil {
		return &CommandHandlerResult{ShouldContinue: true}, nil
	}

	// 特殊处理 /reset 和 /new 命令
	cmd := strings.ToLower(strings.TrimSpace(params.Command.CommandBodyNormalized))
	if cmd == "/reset" || cmd == "/new" {
		if params.Command.IsAuthorizedSender {
			return &CommandHandlerResult{
				ShouldContinue: false,
				Reply:          &ReplyPayload{Text: "⚙️ Session reset."},
			}, nil
		}
		return &CommandHandlerResult{ShouldContinue: false}, nil
	}

	// 判断是否允许处理文本命令
	allowTextCommands := ShouldHandleTextCommands(&ShouldHandleTextCommandsParams{
		Surface:       params.Command.Surface,
		CommandSource: params.MsgCtx.CommandSource,
	})

	// 获取注册的处理器列表
	handlers := GetRegisteredHandlers()

	// 遍历处理器链
	for _, handler := range handlers {
		result, err := handler(ctx, params, allowTextCommands)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	// 所有处理器都未匹配 → 继续消息处理
	return &CommandHandlerResult{ShouldContinue: true}, nil
}

// ---------- 命令处理器注册 ----------

// registeredHandlers 已注册的命令处理器列表（有序）。
// 插件命令优先于内置命令。
var registeredHandlers []CommandHandler

// RegisterHandler 注册一个命令处理器到链中。
// 注意：注册顺序决定优先级。
func RegisterHandler(handler CommandHandler) {
	registeredHandlers = append(registeredHandlers, handler)
}

// GetRegisteredHandlers 获取所有注册的处理器。
func GetRegisteredHandlers() []CommandHandler {
	return registeredHandlers
}

// ResetHandlers 清除所有注册的处理器（用于测试）。
func ResetHandlers() {
	registeredHandlers = nil
}
