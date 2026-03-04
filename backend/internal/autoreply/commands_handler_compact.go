package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-compact.ts (135L)

// SessionCompactor 会话压缩接口。
// TS 对照: agents/pi-embedded.ts compactEmbeddedPiSession / abortEmbeddedPiRun
type SessionCompactor interface {
	CompactSession(ctx context.Context, sessionKey string) (compactedTokens int, err error)
	AbortActiveRun(ctx context.Context, sessionKey string) error
	IsRunActive(sessionKey string) bool
	WaitForRunEnd(ctx context.Context, sessionKey string) error
}

// HandleCompactCommand /compact 命令处理器。
// 处理 /compact [full|summary|hard|soft] 命令，压缩当前会话上下文。
// TS 对照: commands-compact.ts handleCompactCommand
func HandleCompactCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	if body != "/compact" && !strings.HasPrefix(body, "/compact ") {
		return nil, nil
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	// 解析子命令
	rest := strings.TrimSpace(strings.TrimPrefix(body, "/compact"))
	mode := "default"
	if rest != "" {
		switch rest {
		case "full", "hard":
			mode = "full"
		case "summary", "soft":
			mode = "summary"
		default:
			return &CommandHandlerResult{
				ShouldContinue: false,
				Reply:          &ReplyPayload{Text: "⚠️ Unknown compact mode. Use: /compact [full|summary]"},
			}, nil
		}
	}

	// CompactFn 是 /compact 的核心依赖，优先检查
	if params.CompactFn == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Compression not available in current mode."},
		}, nil
	}

	original, compressed, err := params.CompactFn(ctx, params.SessionKey, mode)
	if err != nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("⚠️ Compression failed: %s", err.Error())},
		}, nil
	}

	if original == compressed {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("✅ Context is within threshold (%d messages). No compression needed.", original)},
		}, nil
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("🗜️ Compressed: %d → %d messages.", original, compressed)},
	}, nil
}
