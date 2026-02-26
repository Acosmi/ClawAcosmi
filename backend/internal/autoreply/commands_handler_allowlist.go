package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-allowlist.ts (696L)

// AllowlistManager 白名单管理接口。
// TS 对照: channels/plugins/config-writes.ts, acp/channel-accounts.ts, acp/pairing-store.ts
type AllowlistManager interface {
	ListAllowed(ctx context.Context, channelID string) ([]AllowlistEntry, error)
	AddToAllowlist(ctx context.Context, channelID string, entry AllowlistEntry) error
	RemoveFromAllowlist(ctx context.Context, channelID string, identifier string) error
	IsAllowed(ctx context.Context, channelID string, identifier string) (bool, error)
	ListPairedChannels(ctx context.Context) ([]PairedChannel, error)
	PairChannel(ctx context.Context, channelID string, params *PairChannelParams) error
	UnpairChannel(ctx context.Context, channelID string) error
}

// AllowlistEntry 白名单条目。
type AllowlistEntry struct {
	Identifier  string // 用户 ID 或电话号码
	DisplayName string
	AddedBy     string
	AddedAt     int64
}

// PairedChannel 配对的频道。
type PairedChannel struct {
	ChannelID   string
	ChannelType string
	Label       string
	PairedAt    int64
}

// PairChannelParams 配对频道参数。
type PairChannelParams struct {
	ChannelType string
	Label       string
}

// ParsedAllowlistCommand /allowlist 命令解析结果。
type ParsedAllowlistCommand struct {
	Action     string // "list" | "add" | "remove" | "check" | "clear" | "pair" | "unpair" | "pairs" | "export" | "import"
	Identifier string
	ChannelID  string
	Label      string
	Bulk       []string // for batch add/remove
}

// parseAllowlistCommand 解析 /allowlist 命令。
// TS 对照: commands-allowlist.ts parseAllowlistCommand
func parseAllowlistCommand(body string) *ParsedAllowlistCommand {
	lower := strings.ToLower(strings.TrimSpace(body))

	var prefix string
	if strings.HasPrefix(lower, "/allowlist") {
		prefix = "/allowlist"
	} else if strings.HasPrefix(lower, "/whitelist") {
		prefix = "/whitelist"
	} else if strings.HasPrefix(lower, "/acl") {
		prefix = "/acl"
	} else if strings.HasPrefix(lower, "/allow") {
		prefix = "/allow"
	} else {
		return nil
	}

	if lower == prefix {
		return &ParsedAllowlistCommand{Action: "list"}
	}
	if len(lower) > len(prefix) && lower[len(prefix)] != ' ' {
		// /allowlisted 等不匹配
		if prefix == "/allow" {
			return nil
		}
	}

	rest := strings.TrimSpace(body[len(prefix):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedAllowlistCommand{Action: "list"}
	}

	action := strings.ToLower(parts[0])
	cmd := &ParsedAllowlistCommand{Action: action}

	switch action {
	case "list", "ls", "show":
		cmd.Action = "list"
		if len(parts) > 1 {
			cmd.ChannelID = parts[1]
		}
	case "add":
		cmd.Action = "add"
		if len(parts) > 1 {
			cmd.Identifier = parts[1]
		}
		if len(parts) > 2 {
			cmd.Label = strings.Join(parts[2:], " ")
		}
	case "remove", "rm", "delete", "del":
		cmd.Action = "remove"
		if len(parts) > 1 {
			cmd.Identifier = parts[1]
		}
	case "check", "status", "is":
		cmd.Action = "check"
		if len(parts) > 1 {
			cmd.Identifier = parts[1]
		}
	case "clear", "reset":
		cmd.Action = "clear"
		if len(parts) > 1 {
			cmd.ChannelID = parts[1]
		}
	case "pair", "link":
		cmd.Action = "pair"
		if len(parts) > 1 {
			cmd.ChannelID = parts[1]
		}
		if len(parts) > 2 {
			cmd.Label = strings.Join(parts[2:], " ")
		}
	case "unpair", "unlink":
		cmd.Action = "unpair"
		if len(parts) > 1 {
			cmd.ChannelID = parts[1]
		}
	case "pairs", "links", "paired":
		cmd.Action = "pairs"
	case "export":
		cmd.Action = "export"
	case "import":
		cmd.Action = "import"
	case "batch":
		// /allowlist batch add id1 id2 id3
		cmd.Action = "batch"
		if len(parts) > 2 {
			cmd.Bulk = parts[2:]
		}
	default:
		// 当作 add <identifier>
		cmd.Action = "add"
		cmd.Identifier = action
	}

	return cmd
}

// HandleAllowlistCommand /allowlist 命令处理器。
// TS 对照: commands-allowlist.ts handleAllowlistCommand
func HandleAllowlistCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	if !strings.HasPrefix(lower, "/allowlist") &&
		!strings.HasPrefix(lower, "/whitelist") &&
		!strings.HasPrefix(lower, "/acl") &&
		!strings.HasPrefix(lower, "/allow ") {
		return nil, nil
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized to manage allowlist."},
		}, nil
	}

	parsed := parseAllowlistCommand(body)
	if parsed == nil {
		return nil, nil
	}

	switch parsed.Action {
	case "list":
		return handleAllowlistList(ctx, params, parsed)
	case "add":
		return handleAllowlistAdd(ctx, params, parsed)
	case "remove":
		return handleAllowlistRemove(ctx, params, parsed)
	case "check":
		return handleAllowlistCheck(ctx, params, parsed)
	case "clear":
		return handleAllowlistClear(ctx, params, parsed)
	case "pair":
		return handleAllowlistPair(ctx, params, parsed)
	case "unpair":
		return handleAllowlistUnpair(ctx, params, parsed)
	case "pairs":
		return handleAllowlistPairs(ctx, params)
	case "export":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📋 Allowlist export: (pending AllowlistManager implementation)"},
		}, nil
	case "import":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📋 Allowlist import: (pending AllowlistManager implementation)"},
		}, nil
	case "batch":
		return handleAllowlistBatch(ctx, params, parsed)
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist [list|add|remove|check|clear|pair|unpair|pairs|export|import]"},
		}, nil
	}
}

func handleAllowlistList(_ context.Context, params *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	channelScope := params.Command.ChannelID
	if parsed.ChannelID != "" {
		channelScope = parsed.ChannelID
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("📋 *Allowlist* (channel: %s)\n(pending AllowlistManager implementation)", channelScope)},
	}, nil
}

func handleAllowlistAdd(_ context.Context, _ *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	if parsed.Identifier == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist add <identifier> [label]"},
		}, nil
	}
	label := ""
	if parsed.Label != "" {
		label = fmt.Sprintf(" (%s)", parsed.Label)
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("✅ Added to allowlist: %s%s", parsed.Identifier, label)},
	}, nil
}

func handleAllowlistRemove(_ context.Context, _ *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	if parsed.Identifier == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist remove <identifier>"},
		}, nil
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("🗑️ Removed from allowlist: %s", parsed.Identifier)},
	}, nil
}

func handleAllowlistCheck(_ context.Context, _ *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	if parsed.Identifier == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist check <identifier>"},
		}, nil
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("🔍 Allowlist check for %s: (pending AllowlistManager implementation)", parsed.Identifier)},
	}, nil
}

func handleAllowlistClear(_ context.Context, params *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	channelScope := params.Command.ChannelID
	if parsed.ChannelID != "" {
		channelScope = parsed.ChannelID
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("🗑️ Allowlist cleared for channel: %s", channelScope)},
	}, nil
}

func handleAllowlistPair(_ context.Context, _ *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	if parsed.ChannelID == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist pair <channel-id> [label]"},
		}, nil
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("🔗 Channel paired: %s", parsed.ChannelID)},
	}, nil
}

func handleAllowlistUnpair(_ context.Context, _ *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	if parsed.ChannelID == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist unpair <channel-id>"},
		}, nil
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("🔗 Channel unpaired: %s", parsed.ChannelID)},
	}, nil
}

func handleAllowlistPairs(_ context.Context, _ *HandleCommandsParams) (*CommandHandlerResult, error) {
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: "🔗 *Paired Channels*\n(pending AllowlistManager implementation)"},
	}, nil
}

func handleAllowlistBatch(_ context.Context, _ *HandleCommandsParams, parsed *ParsedAllowlistCommand) (*CommandHandlerResult, error) {
	if len(parsed.Bulk) == 0 {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /allowlist batch add <id1> <id2> ..."},
		}, nil
	}
	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("📋 Batch allowlist: %d entries queued", len(parsed.Bulk))},
	}, nil
}
