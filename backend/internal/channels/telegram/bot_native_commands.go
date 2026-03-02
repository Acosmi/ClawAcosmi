package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/internal/plugins"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Telegram 原生命令 — 继承自 src/telegram/bot-native-commands.ts (727L)
// 动态发现 native/skill/custom/plugin 命令并注册到 Telegram Bot 菜单。
// 命令执行通过 agent dispatch 管线分发（非静态回复）。

// TelegramCommandAuthResult 命令鉴权结果
type TelegramCommandAuthResult struct {
	ChatID            int64
	IsGroup           bool
	IsForum           bool
	ResolvedThreadID  int
	SenderID          string
	SenderUsername    string
	GroupConfig       *types.TelegramGroupConfig
	TopicConfig       *types.TelegramTopicConfig
	CommandAuthorized bool
}

// RegisterTelegramNativeCommandsParams 注册原生命令参数
type RegisterTelegramNativeCommandsParams struct {
	Client         *http.Client
	Token          string
	AccountID      string
	Config         *types.OpenAcosmiConfig
	TelegramCfg    types.TelegramAccountConfig
	AllowFrom      NormalizedAllowFrom
	GroupAllowFrom NormalizedAllowFrom
	ReplyToMode    string
	TextLimit      int
	Deps           *TelegramMonitorDeps
}

// TelegramRegisteredCommands 注册结果，保存已注册的命令供 HandleUpdate 路由
type TelegramRegisteredCommands struct {
	NativeCommands  []autoreply.NativeCommandSpec
	CustomCommands  []config.TelegramResolvedCommand
	PluginCommands  []pluginCommandEntry
	AllCommandNames map[string]string // name → source ("native"|"custom"|"plugin")
}

type pluginCommandEntry struct {
	Command     string
	Description string
}

// ResolveTelegramCommandAuth 解析命令鉴权
func ResolveTelegramCommandAuth(msg *TelegramMessage, botID int64, botUsername string,
	allowFrom, groupAllowFrom NormalizedAllowFrom,
	resolveGroupCfg func(chatID string, threadID int) (*types.TelegramGroupConfig, *types.TelegramTopicConfig),
	requireAuth bool,
) *TelegramCommandAuthResult {

	if msg == nil {
		return nil
	}

	chatID := msg.Chat.ID
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"
	isForum := msg.Chat.IsForum

	senderID := ""
	senderUsername := ""
	if msg.From != nil {
		senderID = fmt.Sprintf("%d", msg.From.ID)
		senderUsername = msg.From.Username
	}

	// 群组命令 — 检查群组权限
	allow := allowFrom
	if isGroup {
		allow = groupAllowFrom
	}
	authorized := IsSenderAllowed(allow, senderID, senderUsername)

	if requireAuth && !authorized {
		return nil
	}

	threadSpec := ResolveTelegramThreadSpec(isGroup, isForum, msg.MessageThreadID)
	threadID := 0
	if threadSpec.ID != nil {
		threadID = *threadSpec.ID
	}

	var groupCfg *types.TelegramGroupConfig
	var topicCfg *types.TelegramTopicConfig
	if resolveGroupCfg != nil {
		groupCfg, topicCfg = resolveGroupCfg(fmt.Sprintf("%d", chatID), threadID)
	}

	return &TelegramCommandAuthResult{
		ChatID:            chatID,
		IsGroup:           isGroup,
		IsForum:           isForum,
		ResolvedThreadID:  threadID,
		SenderID:          senderID,
		SenderUsername:    senderUsername,
		GroupConfig:       groupCfg,
		TopicConfig:       topicCfg,
		CommandAuthorized: authorized,
	}
}

// resolveNativeEnabled 解析原生命令启用配置
func resolveNativeEnabled(cfg *types.OpenAcosmiConfig, telegramCfg types.TelegramAccountConfig) (nativeEnabled, nativeSkillsEnabled bool) {
	// 默认启用
	nativeEnabled = true
	nativeSkillsEnabled = true

	// 检查 telegram 账户级命令配置
	if telegramCfg.Commands != nil {
		switch v := telegramCfg.Commands.Native.(type) {
		case bool:
			nativeEnabled = v
		case string:
			nativeEnabled = v != "false"
		}
		switch v := telegramCfg.Commands.NativeSkills.(type) {
		case bool:
			nativeSkillsEnabled = v
		case string:
			nativeSkillsEnabled = v != "false"
		}
	}

	return
}

// DiscoverTelegramCommands 动态发现所有要注册的 Telegram 命令。
// 对齐 TS registerTelegramNativeCommands 中的命令发现逻辑。
func DiscoverTelegramCommands(cfg *types.OpenAcosmiConfig, telegramCfg types.TelegramAccountConfig) *TelegramRegisteredCommands {
	nativeEnabled, _ := resolveNativeEnabled(cfg, telegramCfg)

	result := &TelegramRegisteredCommands{
		AllCommandNames: make(map[string]string),
	}

	// 1. Native 命令（从 autoreply 命令注册中心发现）
	if nativeEnabled {
		// 通过 config 过滤获取 native 命令规格
		var cmdsCfg *autoreply.CommandsEnabledConfig
		if cfg.Commands != nil {
			cmdsCfg = &autoreply.CommandsEnabledConfig{
				Config: cfg.Commands.Config,
				Debug:  cfg.Commands.Debug,
				Bash:   cfg.Commands.Bash,
			}
		}
		result.NativeCommands = autoreply.ListNativeCommandSpecsForConfig(cmdsCfg, "telegram")
	}

	// 记录 native 命令名
	for _, cmd := range result.NativeCommands {
		result.AllCommandNames[strings.ToLower(cmd.Name)] = "native"
	}

	// 2. 自定义命令（从 telegram 账户配置发现）
	if len(telegramCfg.CustomCommands) > 0 {
		reservedSet := make(map[string]bool)
		for name := range result.AllCommandNames {
			reservedSet[name] = true
		}
		inputs := make([]config.TelegramCustomCommandInput, 0, len(telegramCfg.CustomCommands))
		for _, cc := range telegramCfg.CustomCommands {
			inputs = append(inputs, config.TelegramCustomCommandInput{
				Command:     cc.Command,
				Description: cc.Description,
			})
		}
		resolved, issues := config.ResolveTelegramCustomCommands(config.ResolveTelegramCustomCommandsOptions{
			Commands:         inputs,
			ReservedCommands: reservedSet,
			CheckReserved:    true,
			CheckDuplicates:  true,
		})
		for _, issue := range issues {
			slog.Warn("telegram: custom command issue", "msg", issue.Message)
		}
		result.CustomCommands = resolved
		for _, cmd := range resolved {
			result.AllCommandNames[strings.ToLower(cmd.Command)] = "custom"
		}
	}

	// 3. Plugin 命令（从全局插件注册中心发现）
	pluginSpecs := plugins.GetPluginCommandSpecs()
	for _, spec := range pluginSpecs {
		normalized := config.NormalizeTelegramCommandName(spec.Name)
		if normalized == "" || !config.TelegramCommandNamePattern.MatchString(normalized) {
			slog.Warn("telegram: plugin command invalid", "name", spec.Name)
			continue
		}
		description := strings.TrimSpace(spec.Description)
		if description == "" {
			slog.Warn("telegram: plugin command missing description", "name", normalized)
			continue
		}
		if _, exists := result.AllCommandNames[normalized]; exists {
			slog.Warn("telegram: plugin command conflicts", "name", normalized)
			continue
		}
		result.PluginCommands = append(result.PluginCommands, pluginCommandEntry{
			Command:     normalized,
			Description: description,
		})
		result.AllCommandNames[normalized] = "plugin"
	}

	return result
}

// RegisterTelegramNativeCommands 注册 Telegram Bot 原生命令。
// 动态发现 native/custom/plugin 命令并通过 setMyCommands 注册菜单。
// 对齐 TS registerTelegramNativeCommands。
func RegisterTelegramNativeCommands(ctx context.Context, params RegisterTelegramNativeCommandsParams) (*TelegramRegisteredCommands, error) {
	discovered := DiscoverTelegramCommands(params.Config, params.TelegramCfg)

	// 组装 Telegram API setMyCommands payload
	var allCommands []map[string]string
	for _, cmd := range discovered.NativeCommands {
		allCommands = append(allCommands, map[string]string{
			"command":     cmd.Name,
			"description": cmd.Description,
		})
	}
	for _, cmd := range discovered.PluginCommands {
		allCommands = append(allCommands, map[string]string{
			"command":     cmd.Command,
			"description": cmd.Description,
		})
	}
	for _, cmd := range discovered.CustomCommands {
		allCommands = append(allCommands, map[string]string{
			"command":     cmd.Command,
			"description": cmd.Description,
		})
	}

	// 先清除旧命令再注册新命令（防止删除的技能命令残留 — 对齐 TS #5717 fix）
	_, delErr := callTelegramAPI(ctx, params.Client, params.Token, "deleteMyCommands", nil)
	if delErr != nil {
		slog.Debug("telegram: deleteMyCommands failed (non-critical)", "err", delErr)
	}

	if len(allCommands) > 0 {
		cmdPayload := map[string]interface{}{
			"commands": allCommands,
		}
		_, err := callTelegramAPI(ctx, params.Client, params.Token, "setMyCommands", cmdPayload)
		if err != nil {
			slog.Warn("telegram: setMyCommands failed", "err", err)
		}
	}

	slog.Info("telegram native commands registered",
		"account", params.AccountID,
		"native", len(discovered.NativeCommands),
		"custom", len(discovered.CustomCommands),
		"plugin", len(discovered.PluginCommands),
		"total", len(allCommands),
	)

	return discovered, nil
}

// HandleTelegramCommand 处理命令消息。
// 原生命令通过 dispatch 管线分发到 agent；自定义/plugin 命令有各自的处理路径。
func HandleTelegramCommand(ctx context.Context, params HandleTelegramCommandParams) error {
	cmd := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(params.Command)), "/")

	// 忽略 @ 后缀
	if idx := strings.Index(cmd, "@"); idx > 0 {
		cmd = cmd[:idx]
	}

	// 提取命令后的参数文本
	rawArgs := ""
	if params.Msg != nil {
		fullText := params.Msg.Text
		if fullText == "" {
			fullText = params.Msg.Caption
		}
		// 跳过命令部分（包括 @bot 后缀），提取参数
		parts := strings.SplitN(fullText, " ", 2)
		if len(parts) == 2 {
			rawArgs = strings.TrimSpace(parts[1])
		}
	}

	commands := params.RegisteredCommands
	if commands == nil {
		// 无已注册命令信息时回退到基本处理
		return handleCommandFallback(ctx, params, cmd)
	}

	source, known := commands.AllCommandNames[cmd]
	if !known {
		return nil // 未知命令忽略
	}

	switch source {
	case "native":
		return handleNativeCommandViaDispatch(ctx, params, cmd, rawArgs)
	case "custom":
		// 自定义命令：构建完整命令文本作为 body 发送到 dispatch
		prompt := "/" + cmd
		if rawArgs != "" {
			prompt += " " + rawArgs
		}
		return handleCommandViaDispatch(ctx, params, prompt)
	case "plugin":
		return handlePluginCommand(ctx, params, cmd, rawArgs)
	default:
		return nil
	}
}

// HandleTelegramCommandParams 命令处理参数
type HandleTelegramCommandParams struct {
	Client             *http.Client
	Token              string
	ChatID             int64
	Command            string
	Thread             *TelegramThreadSpec
	Config             *types.OpenAcosmiConfig
	Deps               *TelegramMonitorDeps
	AccountID          string
	AllowFrom          NormalizedAllowFrom
	GroupAllowFrom     NormalizedAllowFrom
	ReplyToMode        string
	TextLimit          int
	Msg                *TelegramMessage
	RegisteredCommands *TelegramRegisteredCommands
}

// handleNativeCommandViaDispatch 通过 dispatch 管线处理原生命令。
// 对齐 TS: 原生命令通过 findCommandByNativeName → parseCommandArgs → finalizeInboundContext → dispatchReply。
func handleNativeCommandViaDispatch(ctx context.Context, params HandleTelegramCommandParams, cmd, rawArgs string) error {
	// 查找命令定义
	cmdDef := autoreply.FindCommandByNativeName(cmd, "telegram")
	if cmdDef == nil {
		// 命令已注册但无定义 — 作为文本命令分发
		prompt := "/" + cmd
		if rawArgs != "" {
			prompt += " " + rawArgs
		}
		return handleCommandViaDispatch(ctx, params, prompt)
	}

	// 解析命令参数
	cmdArgs := autoreply.ParseCommandArgs(cmdDef, rawArgs)

	// 检查是否需要参数选择菜单
	menu := autoreply.ResolveCommandArgMenu(cmdDef, &cmdArgs)
	if menu != nil {
		return sendCommandArgMenu(ctx, params, cmdDef, menu)
	}

	// 构建完整命令文本
	prompt := autoreply.BuildCommandTextFromArgs(cmdDef, &cmdArgs)
	return handleCommandViaDispatch(ctx, params, prompt)
}

// sendCommandArgMenu 发送命令参数选择 inline keyboard
func sendCommandArgMenu(ctx context.Context, params HandleTelegramCommandParams,
	cmdDef *autoreply.ChatCommandDefinition, menu *autoreply.ResolvedArgMenu,
) error {
	title := menu.Title
	if title == "" {
		nativeName := cmdDef.NativeName
		if nativeName == "" {
			nativeName = cmdDef.Key
		}
		argDesc := menu.Arg.Description
		if argDesc == "" {
			argDesc = menu.Arg.Name
		}
		title = fmt.Sprintf("Choose %s for /%s.", argDesc, nativeName)
	}

	var rows [][]map[string]string
	for i := 0; i < len(menu.Choices); i += 2 {
		end := i + 2
		if end > len(menu.Choices) {
			end = len(menu.Choices)
		}
		var row []map[string]string
		for _, choice := range menu.Choices[i:end] {
			args := &autoreply.CommandArgs{
				Values: autoreply.CommandArgValues{
					menu.Arg.Name: choice.Value,
				},
			}
			callbackData := autoreply.BuildCommandTextFromArgs(cmdDef, args)
			row = append(row, map[string]string{
				"text":          choice.Label,
				"callback_data": callbackData,
			})
		}
		rows = append(rows, row)
	}

	apiParams := map[string]interface{}{
		"chat_id": params.ChatID,
		"text":    title,
		"reply_markup": map[string]interface{}{
			"inline_keyboard": rows,
		},
	}
	applyThreadParams(apiParams, params.Thread)
	_, err := callTelegramAPI(ctx, params.Client, params.Token, "sendMessage", apiParams)
	return err
}

// handleCommandViaDispatch 通过 agent dispatch 管线处理命令。
// 构建 MsgContext 并分发到 auto-reply 管线，投递回复。
func handleCommandViaDispatch(ctx context.Context, params HandleTelegramCommandParams, prompt string) error {
	deps := params.Deps
	if deps == nil || deps.DispatchInboundMessage == nil {
		// 无 dispatch 时降级为静态回复
		return sendCommandReply(ctx, params.Client, params.Token, params.ChatID,
			"Command received: "+prompt, params.Thread)
	}

	msg := params.Msg
	chatID := params.ChatID
	isGroup := msg != nil && (msg.Chat.Type == "group" || msg.Chat.Type == "supergroup")

	// 解析 agent 路由
	peerKind := "direct"
	if isGroup {
		peerKind = "group"
	}
	var route *TelegramAgentRoute
	if deps.ResolveAgentRoute != nil {
		threadID := ""
		if params.Thread != nil && params.Thread.ID != nil {
			threadID = strconv.Itoa(*params.Thread.ID)
		}
		var err error
		route, err = deps.ResolveAgentRoute(TelegramAgentRouteParams{
			Channel:   "telegram",
			AccountID: params.AccountID,
			PeerKind:  peerKind,
			PeerID:    strconv.FormatInt(chatID, 10),
			ThreadID:  threadID,
		})
		if err != nil {
			slog.Warn("telegram: command route failed", "err", err)
		}
	}

	sessionKey := fmt.Sprintf("telegram:%s:%d", params.AccountID, chatID)
	agentID := ""
	if route != nil {
		sessionKey = route.SessionKey
		agentID = route.AgentID
	}

	senderID := ""
	senderName := ""
	senderUsername := ""
	if msg != nil && msg.From != nil {
		senderID = strconv.FormatInt(msg.From.ID, 10)
		senderName = buildSenderDisplayName(msg.From)
		senderUsername = msg.From.Username
	}

	chatType := "dm"
	if isGroup {
		chatType = "group"
	}

	messageID := 0
	var timestamp int64
	if msg != nil {
		messageID = msg.MessageID
		if msg.Date > 0 {
			timestamp = int64(msg.Date) * 1000
		}
	}

	// 构建 MsgContext
	arCtx := &autoreply.MsgContext{
		Body:                    prompt,
		RawBody:                 prompt,
		CommandBody:             prompt,
		ChatType:                chatType,
		ChannelType:             "telegram",
		ChannelID:               strconv.FormatInt(chatID, 10),
		From:                    senderID,
		To:                      fmt.Sprintf("slash:%s", senderID),
		SenderID:                senderID,
		SenderName:              senderName,
		SenderUsername:          senderUsername,
		IsGroup:                 isGroup,
		SessionKey:              fmt.Sprintf("telegram:slash:%s", senderID),
		AccountID:               params.AccountID,
		WasMentioned:            "true",
		CommandAuthorized:       true,
		CommandSource:           "native",
		Timestamp:               timestamp,
		MessageSid:              strconv.Itoa(messageID),
		CommandTargetSessionKey: sessionKey,
	}
	_ = agentID // agentID 通过 route 传递给 dispatch

	// 发送 typing indicator
	go sendTypingAction(ctx, params.Client, params.Token, chatID, params.Thread)

	// 分发到 auto-reply 管线
	dispatchResult, err := deps.DispatchInboundMessage(ctx, TelegramDispatchParams{
		Ctx: arCtx,
	})
	if err != nil {
		slog.Warn("telegram: command dispatch failed", "err", err, "cmd", prompt)
		return sendCommandReply(ctx, params.Client, params.Token, chatID,
			"An error occurred processing your command.", params.Thread)
	}

	// 投递回复
	if dispatchResult != nil && len(dispatchResult.Replies) > 0 {
		replies := convertAutoReplyPayloads(dispatchResult.Replies)
		delivered, deliverErr := DeliverReplies(ctx, DeliverRepliesParams{
			Client:           params.Client,
			Token:            params.Token,
			ChatID:           strconv.FormatInt(chatID, 10),
			Replies:          replies,
			ReplyToMode:      params.ReplyToMode,
			ReplyToMessageID: messageID,
			TextLimit:        params.TextLimit,
			Thread:           params.Thread,
		})
		if deliverErr != nil {
			slog.Warn("telegram: command reply delivery failed", "err", deliverErr)
		}
		if !delivered {
			// 空回复回退（对齐 TS EMPTY_RESPONSE_FALLBACK）
			return sendCommandReply(ctx, params.Client, params.Token, chatID,
				"No response generated. Please try again.", params.Thread)
		}
	}

	return nil
}

// handlePluginCommand 处理插件命令
func handlePluginCommand(ctx context.Context, params HandleTelegramCommandParams, cmd, rawArgs string) error {
	commandBody := "/" + cmd
	if rawArgs != "" {
		commandBody += " " + rawArgs
	}
	match := plugins.MatchPluginCommand(commandBody)
	if match == nil {
		return sendCommandReply(ctx, params.Client, params.Token, params.ChatID,
			"Command not found.", params.Thread)
	}

	senderID := ""
	if params.Msg != nil && params.Msg.From != nil {
		senderID = strconv.FormatInt(params.Msg.From.ID, 10)
	}

	result, err := plugins.ExecutePluginCommand(match.Command, plugins.PluginCommandContext{
		SenderID:    senderID,
		Channel:     "telegram",
		CommandBody: commandBody,
		From:        fmt.Sprintf("telegram:%d", params.ChatID),
		To:          fmt.Sprintf("telegram:%d", params.ChatID),
		AccountID:   params.AccountID,
		Args:        match.Args,
	})
	if err != nil {
		slog.Warn("telegram: plugin command failed", "cmd", cmd, "err", err)
		return sendCommandReply(ctx, params.Client, params.Token, params.ChatID,
			"Plugin command failed.", params.Thread)
	}

	// 投递 plugin 回复
	if result.Text != "" {
		replies := []ReplyPayload{{Text: result.Text, TextMode: "markdown"}}
		_, deliverErr := DeliverReplies(ctx, DeliverRepliesParams{
			Client:      params.Client,
			Token:       params.Token,
			ChatID:      strconv.FormatInt(params.ChatID, 10),
			Replies:     replies,
			ReplyToMode: params.ReplyToMode,
			TextLimit:   params.TextLimit,
			Thread:      params.Thread,
		})
		if deliverErr != nil {
			slog.Warn("telegram: plugin reply delivery failed", "err", deliverErr)
		}
	}

	return nil
}

// handleCommandFallback 无已注册命令时的降级处理（保留基本命令功能）
func handleCommandFallback(ctx context.Context, params HandleTelegramCommandParams, cmd string) error {
	switch cmd {
	case "start":
		return sendCommandReply(ctx, params.Client, params.Token, params.ChatID,
			"Welcome! I'm ready to chat. Send me a message to start.", params.Thread)
	case "help":
		return sendCommandReply(ctx, params.Client, params.Token, params.ChatID,
			"Send me a message to start chatting. Use /reset to clear the conversation.", params.Thread)
	case "reset":
		return handleResetCommand(ctx, params.Client, params.Token, params.ChatID, params.Thread, params.Deps, params.AccountID)
	case "model":
		return handleModelCommand(ctx, params.Client, params.Token, params.ChatID, params.Thread, params.Deps, params.AccountID)
	default:
		return nil
	}
}

// handleResetCommand 处理 /reset 命令（降级路径）
func handleResetCommand(ctx context.Context, client *http.Client, token string, chatID int64, thread *TelegramThreadSpec,
	deps *TelegramMonitorDeps, accountID string,
) error {
	if deps != nil && deps.ResetSession != nil {
		sessionKey := "telegram:" + accountID + ":" + strconv.FormatInt(chatID, 10)
		storePath := ""
		if deps.ResolveStorePath != nil {
			storePath = deps.ResolveStorePath("")
		}
		if err := deps.ResetSession(ctx, sessionKey, storePath); err != nil {
			slog.Warn("telegram: session reset failed", "err", err)
			return sendCommandReply(ctx, client, token, chatID, "Failed to reset conversation.", thread)
		}
	}
	return sendCommandReply(ctx, client, token, chatID, "Conversation has been reset.", thread)
}

// handleModelCommand 处理 /model 命令（降级路径）
func handleModelCommand(ctx context.Context, client *http.Client, token string, chatID int64, thread *TelegramThreadSpec,
	deps *TelegramMonitorDeps, accountID string,
) error {
	keyboard := buildModelSelectionKeyboard()
	apiParams := map[string]interface{}{
		"chat_id":      chatID,
		"text":         "Select a model:",
		"reply_markup": keyboard,
	}
	applyThreadParams(apiParams, thread)
	_, err := callTelegramAPI(ctx, client, token, "sendMessage", apiParams)
	return err
}

// buildModelSelectionKeyboard 构建模型选择 inline keyboard
func buildModelSelectionKeyboard() map[string]interface{} {
	models := []struct {
		label string
		value string
	}{
		{"Claude Sonnet", "model:claude-sonnet"},
		{"Claude Haiku", "model:claude-haiku"},
		{"GPT-4o", "model:gpt-4o"},
		{"GPT-4o mini", "model:gpt-4o-mini"},
	}

	var rows [][]map[string]string
	for _, m := range models {
		rows = append(rows, []map[string]string{
			{"text": m.label, "callback_data": m.value},
		})
	}

	return map[string]interface{}{
		"inline_keyboard": rows,
	}
}

func sendCommandReply(ctx context.Context, client *http.Client, token string, chatID int64, text string, thread *TelegramThreadSpec) error {
	apiParams := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	applyThreadParams(apiParams, thread)
	_, err := callTelegramAPI(ctx, client, token, "sendMessage", apiParams)
	return err
}

// buildSenderDisplayName 构建发送者显示名
func buildSenderDisplayName(user *TelegramUser) string {
	if user == nil {
		return ""
	}
	name := strings.TrimSpace(user.FirstName)
	if user.LastName != "" {
		name += " " + strings.TrimSpace(user.LastName)
	}
	name = strings.TrimSpace(name)
	if name == "" && user.Username != "" {
		return user.Username
	}
	return name
}
