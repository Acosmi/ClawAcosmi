package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-tts.ts (280L)

// TTSService TTS 服务接口。
// TS 对照: tts/tts.ts 中的各种 TTS 函数
type TTSService interface {
	IsEnabled() bool
	SetEnabled(enabled bool)
	GetProvider() string
	SetProvider(provider string) error
	GetMaxLength() int
	SetMaxLength(length int)
	IsSummarizationEnabled() bool
	SetSummarizationEnabled(enabled bool)
	TextToSpeech(ctx context.Context, text string) (audioURL string, err error)
	ListProviders() []string
	IsProviderConfigured(provider string) bool
}

// ParsedTtsCommand /tts 命令解析结果。
type ParsedTtsCommand struct {
	Action   string // "toggle" | "on" | "off" | "say" | "provider" | "maxlen" | "summarize" | "status" | "providers"
	Text     string // /tts say <text>
	Provider string // /tts provider <name>
	MaxLen   int    // /tts maxlen <n>
}

// parseTtsCommand 解析 /tts 命令。
// TS 对照: commands-tts.ts parseTtsCommand
func parseTtsCommand(body string) *ParsedTtsCommand {
	lower := strings.ToLower(strings.TrimSpace(body))
	if lower == "/tts" {
		return &ParsedTtsCommand{Action: "toggle"}
	}
	if !strings.HasPrefix(lower, "/tts ") {
		return nil
	}

	rest := strings.TrimSpace(body[len("/tts"):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedTtsCommand{Action: "toggle"}
	}

	action := strings.ToLower(parts[0])
	switch action {
	case "on", "enable":
		return &ParsedTtsCommand{Action: "on"}
	case "off", "disable":
		return &ParsedTtsCommand{Action: "off"}
	case "say", "speak":
		text := ""
		if len(parts) > 1 {
			text = strings.Join(parts[1:], " ")
		}
		return &ParsedTtsCommand{Action: "say", Text: text}
	case "provider":
		provider := ""
		if len(parts) > 1 {
			provider = parts[1]
		}
		return &ParsedTtsCommand{Action: "provider", Provider: provider}
	case "maxlen", "max", "length":
		maxLen := 0
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &maxLen)
		}
		return &ParsedTtsCommand{Action: "maxlen", MaxLen: maxLen}
	case "summarize", "summary":
		return &ParsedTtsCommand{Action: "summarize"}
	case "status", "info":
		return &ParsedTtsCommand{Action: "status"}
	case "providers", "list":
		return &ParsedTtsCommand{Action: "providers"}
	default:
		// 无子命令 → 当作 say <text>
		return &ParsedTtsCommand{Action: "say", Text: rest}
	}
}

// HandleTTSCommand /tts 命令处理器。
// TS 对照: commands-tts.ts handleTtsCommand
func HandleTTSCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	if !strings.HasPrefix(lower, "/tts") {
		return nil, nil
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	parsed := parseTtsCommand(body)
	if parsed == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Invalid TTS command."},
		}, nil
	}

	switch parsed.Action {
	case "toggle":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔊 TTS toggled."},
		}, nil
	case "on":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔊 TTS enabled."},
		}, nil
	case "off":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔇 TTS disabled."},
		}, nil
	case "say":
		if parsed.Text == "" {
			return &CommandHandlerResult{
				ShouldContinue: false,
				Reply:          &ReplyPayload{Text: "⚠️ Usage: /tts say <text>"},
			}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply: &ReplyPayload{
				Text:         fmt.Sprintf("🔊 Speaking: %s", truncateForDisplay(parsed.Text, 80)),
				AudioAsVoice: true,
			},
		}, nil
	case "provider":
		if parsed.Provider == "" {
			return &CommandHandlerResult{
				ShouldContinue: false,
				Reply:          &ReplyPayload{Text: "⚠️ Usage: /tts provider <name>"},
			}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🔊 TTS provider set to: %s", parsed.Provider)},
		}, nil
	case "maxlen":
		if parsed.MaxLen <= 0 {
			return &CommandHandlerResult{
				ShouldContinue: false,
				Reply:          &ReplyPayload{Text: "⚠️ Usage: /tts maxlen <number>"},
			}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🔊 TTS max length set to: %d", parsed.MaxLen)},
		}, nil
	case "summarize":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔊 TTS summarization toggled."},
		}, nil
	case "status":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔊 TTS status: (pending TTSService implementation)"},
		}, nil
	case "providers":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔊 TTS providers: (pending TTSService implementation)"},
		}, nil
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Unknown TTS sub-command."},
		}, nil
	}
}

// truncateForDisplay 截断字符串用于显示。
func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
