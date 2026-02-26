package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-models.ts (327L)

// ModelCatalogProvider 模型目录接口。
// TS 对照: model-catalog/model-catalog.ts
type ModelCatalogProvider interface {
	ListModels(ctx context.Context) ([]ModelInfo, error)
	ListModelsForProvider(ctx context.Context, provider string) ([]ModelInfo, error)
	ListProviders(ctx context.Context) ([]string, error)
	GetCurrentModel() (provider string, model string)
	SetModel(ctx context.Context, provider string, model string) error
}

// ModelInfo 模型信息。
type ModelInfo struct {
	ID          string
	Name        string
	Provider    string
	Description string
	ContextSize int
	IsDefault   bool
}

// ParsedModelsCommand /model(s) 命令解析结果。
type ParsedModelsCommand struct {
	Action   string // "list" | "set" | "info" | "providers" | "search"
	Provider string
	Model    string
	Query    string
	Page     int
}

// parseModelsCommand 解析 /model 或 /models 命令。
// TS 对照: commands-models.ts parseModelsArgs
func parseModelsCommand(body string) *ParsedModelsCommand {
	lower := strings.ToLower(strings.TrimSpace(body))

	// 匹配 /model 或 /models
	var prefix string
	if strings.HasPrefix(lower, "/models") {
		prefix = "/models"
	} else if strings.HasPrefix(lower, "/model") {
		prefix = "/model"
	} else {
		return nil
	}

	if lower == prefix {
		return &ParsedModelsCommand{Action: "list", Page: 1}
	}
	if len(lower) > len(prefix) && lower[len(prefix)] != ' ' {
		return nil
	}

	rest := strings.TrimSpace(body[len(prefix):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedModelsCommand{Action: "list", Page: 1}
	}

	action := strings.ToLower(parts[0])
	switch action {
	case "list", "ls":
		cmd := &ParsedModelsCommand{Action: "list", Page: 1}
		if len(parts) > 1 {
			cmd.Provider = parts[1]
		}
		return cmd
	case "set", "use", "switch":
		cmd := &ParsedModelsCommand{Action: "set"}
		if len(parts) > 1 {
			// 可能是 provider/model 格式
			modelSpec := parts[1]
			if idx := strings.LastIndex(modelSpec, "/"); idx >= 0 {
				cmd.Provider = modelSpec[:idx]
				cmd.Model = modelSpec[idx+1:]
			} else {
				cmd.Model = modelSpec
			}
		}
		if len(parts) > 2 && cmd.Provider == "" {
			cmd.Provider = parts[2]
		}
		return cmd
	case "info", "details":
		cmd := &ParsedModelsCommand{Action: "info"}
		if len(parts) > 1 {
			cmd.Model = parts[1]
		}
		return cmd
	case "providers":
		return &ParsedModelsCommand{Action: "providers"}
	case "search", "find":
		query := ""
		if len(parts) > 1 {
			query = strings.Join(parts[1:], " ")
		}
		return &ParsedModelsCommand{Action: "search", Query: query}
	default:
		// 无子命令 → 当作 set <model>
		cmd := &ParsedModelsCommand{Action: "set"}
		modelSpec := action
		if idx := strings.LastIndex(modelSpec, "/"); idx >= 0 {
			cmd.Provider = modelSpec[:idx]
			cmd.Model = modelSpec[idx+1:]
		} else {
			cmd.Model = modelSpec
		}
		return cmd
	}
}

// HandleModelsCommand /model 或 /models 命令处理器。
// TS 对照: commands-models.ts handleModelsCommand
func HandleModelsCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	if !strings.HasPrefix(lower, "/model") {
		return nil, nil
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	parsed := parseModelsCommand(body)
	if parsed == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Invalid model command."},
		}, nil
	}

	switch parsed.Action {
	case "list":
		return handleModelsListCommand(ctx, params, parsed)
	case "set":
		return handleModelsSetCommand(ctx, params, parsed)
	case "info":
		return handleModelsInfoCommand(ctx, params, parsed)
	case "providers":
		return handleModelsProvidersCommand(ctx, params)
	case "search":
		return handleModelsSearchCommand(ctx, params, parsed)
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /model [list|set|info|providers|search]"},
		}, nil
	}
}

func handleModelsListCommand(_ context.Context, params *HandleCommandsParams, parsed *ParsedModelsCommand) (*CommandHandlerResult, error) {
	qualifier := ""
	if parsed.Provider != "" {
		qualifier = fmt.Sprintf(" for %s", parsed.Provider)
	}
	currentProvider, currentModel := params.Provider, params.Model
	replyText := fmt.Sprintf("🤖 *Models%s*\nCurrent: %s/%s\n(pending ModelCatalogProvider implementation)",
		qualifier, currentProvider, currentModel)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleModelsSetCommand(_ context.Context, _ *HandleCommandsParams, parsed *ParsedModelsCommand) (*CommandHandlerResult, error) {
	if parsed.Model == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /model set <provider/model> or /model set <model>"},
		}, nil
	}
	ident := parsed.Model
	if parsed.Provider != "" {
		ident = parsed.Provider + "/" + parsed.Model
	}
	replyText := fmt.Sprintf("🤖 Model set to: %s", ident)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleModelsInfoCommand(_ context.Context, _ *HandleCommandsParams, parsed *ParsedModelsCommand) (*CommandHandlerResult, error) {
	model := parsed.Model
	if model == "" {
		model = "(current)"
	}
	replyText := fmt.Sprintf("🤖 *Model Info: %s*\n(pending ModelCatalogProvider implementation)", model)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleModelsProvidersCommand(_ context.Context, _ *HandleCommandsParams) (*CommandHandlerResult, error) {
	replyText := "🤖 *Providers*\n(pending ModelCatalogProvider implementation)"
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleModelsSearchCommand(_ context.Context, _ *HandleCommandsParams, parsed *ParsedModelsCommand) (*CommandHandlerResult, error) {
	if parsed.Query == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /model search <query>"},
		}, nil
	}
	replyText := fmt.Sprintf("🤖 Search: \"%s\"\n(pending ModelCatalogProvider implementation)", parsed.Query)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}
