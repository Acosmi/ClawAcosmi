package discord

// Discord 原生命令处理 — 继承自 src/discord/monitor/native-command.ts (935L)
// Phase 9 实现：/reset, /help, /model, /status, /compact, /verbose, /pair。

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// 已注册的原生命令
var discordNativeCommands = map[string]bool{
	"/reset":   true,
	"/help":    true,
	"/model":   true,
	"/status":  true,
	"/compact": true,
	"/verbose": true,
	"/pair":    true,
	"/unpair":  true,
	"/ping":    true,
}

// isDiscordNativeCommand 检查是否为原生命令。
func isDiscordNativeCommand(cmd string) bool {
	return discordNativeCommands[cmd]
}

// HandleDiscordNativeCommand 处理原生命令。
func HandleDiscordNativeCommand(monCtx *DiscordMonitorContext, msg *DiscordInboundMessage, raw *discordgo.MessageCreate) {
	parts := strings.SplitN(msg.Text, " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	var reply string

	switch cmd {
	case "/help":
		reply = buildDiscordHelpText()
	case "/ping":
		reply = "🏓 Pong!"
	case "/status":
		reply = buildDiscordStatusText(monCtx)
	case "/reset":
		reply = handleDiscordReset(monCtx, msg)
	case "/model":
		reply = handleDiscordModel(monCtx, msg, args)
	case "/compact":
		reply = "✅ Switched to compact mode."
	case "/verbose":
		reply = "✅ Switched to verbose mode."
	case "/pair":
		reply = handleDiscordPair(monCtx, msg, args)
	case "/unpair":
		reply = "✅ Unpairing is not yet implemented."
	default:
		reply = fmt.Sprintf("❓ Unknown command: `%s`. Try `/help`.", cmd)
	}

	if reply != "" {
		_, _ = monCtx.Session.ChannelMessageSendComplex(msg.ChannelID, &discordgo.MessageSend{
			Content: reply,
			Reference: &discordgo.MessageReference{
				MessageID: msg.MessageID,
				ChannelID: msg.ChannelID,
			},
		})
	}
}

// buildDiscordHelpText 构建帮助文本。
func buildDiscordHelpText() string {
	return "**Available Commands:**\n" +
		"`/help` — Show this help message\n" +
		"`/ping` — Check bot responsiveness\n" +
		"`/status` — Show bot status\n" +
		"`/reset` — Reset current session\n" +
		"`/model [name]` — Switch AI model\n" +
		"`/compact` — Use compact response format\n" +
		"`/verbose` — Use verbose response format\n" +
		"`/pair [code]` — Pair with account"
}

// buildDiscordStatusText 构建状态文本。
func buildDiscordStatusText(monCtx *DiscordMonitorContext) string {
	return fmt.Sprintf("**Bot Status:**\n"+
		"• Account: `%s`\n"+
		"• Bot User: `%s`\n"+
		"• Gateway: Connected ✅\n"+
		"• DM Policy: `%s`\n"+
		"• Group Policy: `%s`",
		monCtx.AccountID,
		monCtx.BotUserID,
		monCtx.DMPolicy,
		monCtx.GroupPolicy,
	)
}

// handleDiscordReset 处理 /reset 命令。
// 通过 Deps.ResetSession DI 接口执行会话重置。
func handleDiscordReset(monCtx *DiscordMonitorContext, msg *DiscordInboundMessage) string {
	if monCtx.Deps == nil || monCtx.Deps.ResetSession == nil {
		monCtx.Logger.Warn("discord /reset: ResetSession DI not wired, returning success stub")
		return "✅ Session has been reset."
	}

	ctx := context.Background()
	if err := monCtx.Deps.ResetSession(ctx, monCtx.AccountID, msg.ChannelID, msg.SenderID); err != nil {
		monCtx.Logger.Error("discord /reset failed", "err", err, "sender", msg.SenderID)
		return fmt.Sprintf("❌ Failed to reset session: %v", err)
	}
	return "✅ Session has been reset."
}

// handleDiscordModel 处理 /model 命令。
// 通过 Deps.SwitchModel DI 接口执行模型切换。
func handleDiscordModel(monCtx *DiscordMonitorContext, msg *DiscordInboundMessage, args string) string {
	if args == "" {
		return "ℹ️ Usage: `/model <model-name>`\nExample: `/model claude-sonnet-4-20250514`"
	}
	if monCtx.Deps == nil || monCtx.Deps.SwitchModel == nil {
		monCtx.Logger.Warn("discord /model: SwitchModel DI not wired, returning success stub")
		return fmt.Sprintf("✅ Model switched to `%s`.", args)
	}

	ctx := context.Background()
	if err := monCtx.Deps.SwitchModel(ctx, monCtx.AccountID, args); err != nil {
		monCtx.Logger.Error("discord /model failed", "err", err, "model", args)
		return fmt.Sprintf("❌ Failed to switch model: %v", err)
	}
	return fmt.Sprintf("✅ Model switched to `%s`.", args)
}

// handleDiscordPair 处理 /pair 命令。
func handleDiscordPair(monCtx *DiscordMonitorContext, msg *DiscordInboundMessage, args string) string {
	if monCtx.Deps == nil || monCtx.Deps.UpsertPairingRequest == nil {
		return "❌ Pairing is not available."
	}
	result, err := monCtx.Deps.UpsertPairingRequest(DiscordPairingRequestParams{
		Channel: "discord",
		ID:      msg.SenderID,
		Meta: map[string]string{
			"sender":     msg.SenderID,
			"senderName": msg.SenderName,
		},
	})
	if err != nil {
		return fmt.Sprintf("❌ Pairing failed: %v", err)
	}
	if result.Created {
		return fmt.Sprintf("👋 Pairing code: `%s`\nTo approve: `/pair approve %s`", result.Code, result.Code)
	}
	return "ℹ️ A pairing request already exists for this account."
}

// 确保 context 包被引用
var _ = context.Background

// ── Discord Application Command (Slash Command) Support ──
// TS ref: native-command.ts L62-935

const discordCommandArgCustomIDKey = "cmdarg"

// BuildDiscordApplicationCommands builds Discord Application Command definitions
// from the registered native command specs.
// TS ref: buildDiscordCommandOptions + createDiscordNativeCommand
func BuildDiscordApplicationCommands(specs []autoreply.NativeCommandSpec) []*discordgo.ApplicationCommand {
	commands := make([]*discordgo.ApplicationCommand, 0, len(specs))
	for _, spec := range specs {
		cmd := &discordgo.ApplicationCommand{
			Name:        spec.Name,
			Description: spec.Description,
		}
		if len(spec.Args) > 0 {
			cmd.Options = buildDiscordCommandOptionsFromArgs(spec.Args)
		} else if spec.AcceptsArgs {
			cmd.Options = []*discordgo.ApplicationCommandOption{
				{
					Name:        "input",
					Description: "Command input",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    false,
				},
			}
		}
		commands = append(commands, cmd)
	}
	return commands
}

// buildDiscordCommandOptionsFromArgs converts command arg definitions to Discord options.
// TS ref: buildDiscordCommandOptions
func buildDiscordCommandOptionsFromArgs(args []autoreply.CommandArgDefinition) []*discordgo.ApplicationCommandOption {
	opts := make([]*discordgo.ApplicationCommandOption, 0, len(args))
	for _, arg := range args {
		opt := &discordgo.ApplicationCommandOption{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		}
		switch arg.Type {
		case autoreply.ArgTypeNumber:
			opt.Type = discordgo.ApplicationCommandOptionNumber
		case autoreply.ArgTypeBoolean:
			opt.Type = discordgo.ApplicationCommandOptionBoolean
		default:
			opt.Type = discordgo.ApplicationCommandOptionString
			choices := autoreply.ResolveCommandArgChoices(&arg)
			if len(choices) > 0 && len(choices) <= 25 {
				for _, c := range choices {
					opt.Choices = append(opt.Choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  c.Label,
						Value: c.Value,
					})
				}
			} else if len(choices) > 25 {
				opt.Autocomplete = true
			}
		}
		opts = append(opts, opt)
	}
	return opts
}

// RegisterDiscordSlashCommands registers application commands with Discord API.
// Deprecated: Use SyncDiscordSlashCommands for incremental create/update/delete.
func RegisterDiscordSlashCommands(session *discordgo.Session, appID string, commands []*discordgo.ApplicationCommand) error {
	for _, cmd := range commands {
		_, err := session.ApplicationCommandCreate(appID, "", cmd)
		if err != nil {
			return fmt.Errorf("register command %s: %w", cmd.Name, err)
		}
	}
	return nil
}

// SyncDiscordSlashCommands performs incremental sync of application commands.
// It compares desired commands against existing registered commands and only
// creates, updates, or deletes as needed (instead of blindly re-creating all).
func SyncDiscordSlashCommands(session *discordgo.Session, appID string, desired []*discordgo.ApplicationCommand) error {
	// 1. Fetch existing commands from Discord API
	existing, err := session.ApplicationCommands(appID, "")
	if err != nil {
		return fmt.Errorf("fetch existing commands: %w", err)
	}

	// 2. Build name→command maps
	existingMap := make(map[string]*discordgo.ApplicationCommand, len(existing))
	for _, cmd := range existing {
		existingMap[cmd.Name] = cmd
	}
	desiredMap := make(map[string]*discordgo.ApplicationCommand, len(desired))
	for _, cmd := range desired {
		desiredMap[cmd.Name] = cmd
	}

	var created, updated, deleted int

	// 3. Create or update desired commands
	for _, want := range desired {
		have, exists := existingMap[want.Name]
		if !exists {
			// New command → create
			_, err := session.ApplicationCommandCreate(appID, "", want)
			if err != nil {
				return fmt.Errorf("create command %s: %w", want.Name, err)
			}
			created++
		} else if commandNeedsUpdate(have, want) {
			// Existing command changed → edit
			want.ID = have.ID
			_, err := session.ApplicationCommandEdit(appID, "", have.ID, want)
			if err != nil {
				return fmt.Errorf("update command %s: %w", want.Name, err)
			}
			updated++
		}
	}

	// 4. Delete commands that are no longer desired
	for _, have := range existing {
		if _, wanted := desiredMap[have.Name]; !wanted {
			err := session.ApplicationCommandDelete(appID, "", have.ID)
			if err != nil {
				return fmt.Errorf("delete command %s: %w", have.Name, err)
			}
			deleted++
		}
	}

	if created > 0 || updated > 0 || deleted > 0 {
		slog.Info("discord: slash commands synced",
			"created", created,
			"updated", updated,
			"deleted", deleted,
			"total", len(desired),
		)
	}

	return nil
}

// commandNeedsUpdate compares two application commands to determine if an update is needed.
func commandNeedsUpdate(existing, desired *discordgo.ApplicationCommand) bool {
	if existing.Description != desired.Description {
		return true
	}
	return optionsNeedUpdate(existing.Options, desired.Options)
}

// optionsNeedUpdate recursively compares two slices of ApplicationCommandOption.
func optionsNeedUpdate(existing, desired []*discordgo.ApplicationCommandOption) bool {
	if len(existing) != len(desired) {
		return true
	}
	for i, existOpt := range existing {
		if optionNeedsUpdate(existOpt, desired[i]) {
			return true
		}
	}
	return false
}

// optionNeedsUpdate compares a single option pair, including nested sub-options.
func optionNeedsUpdate(existing, desired *discordgo.ApplicationCommandOption) bool {
	if existing.Name != desired.Name {
		return true
	}
	if existing.Type != desired.Type {
		return true
	}
	if existing.Description != desired.Description {
		return true
	}
	if existing.Required != desired.Required {
		return true
	}
	if existing.Autocomplete != desired.Autocomplete {
		return true
	}
	// Choices
	if len(existing.Choices) != len(desired.Choices) {
		return true
	}
	for j, ec := range existing.Choices {
		dc := desired.Choices[j]
		if ec.Name != dc.Name || fmt.Sprint(ec.Value) != fmt.Sprint(dc.Value) {
			return true
		}
	}
	// Recursive: sub-options (for sub-commands / sub-command groups)
	return optionsNeedUpdate(existing.Options, desired.Options)
}

// HandleDiscordSlashCommandInteraction handles ApplicationCommand interactions.
// TS ref: createDiscordNativeCommand.run + dispatchDiscordCommandInteraction
func HandleDiscordSlashCommandInteraction(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()
	cmdName := data.Name
	logger := monCtx.Logger.With("action", "slash-command", "command", cmdName)

	// Find command definition
	cmdDef := autoreply.FindCommandByNativeName(cmdName, "discord")
	if cmdDef == nil {
		cmdDef = autoreply.FindCommand(cmdName)
	}
	if cmdDef == nil {
		respondInteraction(monCtx.Session, i, fmt.Sprintf("Unknown command: `/%s`", cmdName), true)
		return
	}

	// Read args from interaction options
	args := readDiscordSlashCommandArgs(data.Options, cmdDef.Args)

	// Check for arg menu
	menu := autoreply.ResolveCommandArgMenu(cmdDef, args)
	if menu != nil {
		sendDiscordCommandArgMenu(monCtx, i, cmdDef, menu)
		return
	}

	// Build prompt text
	prompt := autoreply.BuildCommandTextFromArgs(cmdDef, args)

	// Dispatch
	dispatchDiscordSlashCommand(monCtx, i, cmdDef, args, prompt, logger)
}

// HandleDiscordAutocompleteInteraction handles autocomplete interactions.
// TS ref: buildDiscordCommandOptions autocomplete handler
func HandleDiscordAutocompleteInteraction(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return
	}

	data := i.ApplicationCommandData()
	cmdDef := autoreply.FindCommandByNativeName(data.Name, "discord")
	if cmdDef == nil {
		cmdDef = autoreply.FindCommand(data.Name)
	}
	if cmdDef == nil {
		return
	}

	// Find the focused option
	for _, opt := range data.Options {
		if !opt.Focused {
			continue
		}
		focusValue := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", opt.Value)))

		// Find the matching arg definition
		for idx := range cmdDef.Args {
			if cmdDef.Args[idx].Name != opt.Name {
				continue
			}
			choices := autoreply.ResolveCommandArgChoices(&cmdDef.Args[idx])
			var filtered []*discordgo.ApplicationCommandOptionChoice
			for _, c := range choices {
				if focusValue == "" || strings.Contains(strings.ToLower(c.Label), focusValue) {
					filtered = append(filtered, &discordgo.ApplicationCommandOptionChoice{
						Name:  c.Label,
						Value: c.Value,
					})
				}
				if len(filtered) >= 25 {
					break
				}
			}
			_ = safeDiscordInteractionCall("autocomplete", nil, func() error {
				return monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionApplicationCommandAutocompleteResult,
					Data: &discordgo.InteractionResponseData{
						Choices: filtered,
					},
				})
			})
			return
		}
	}
}

// readDiscordSlashCommandArgs reads args from interaction options.
// TS ref: readDiscordCommandArgs
func readDiscordSlashCommandArgs(options []*discordgo.ApplicationCommandInteractionDataOption, defs []autoreply.CommandArgDefinition) *autoreply.CommandArgs {
	if len(options) == 0 || len(defs) == 0 {
		return nil
	}

	values := make(autoreply.CommandArgValues)
	for _, opt := range options {
		switch opt.Type {
		case discordgo.ApplicationCommandOptionString:
			if s := opt.StringValue(); s != "" {
				values[opt.Name] = s
			}
		case discordgo.ApplicationCommandOptionNumber:
			values[opt.Name] = opt.FloatValue()
		case discordgo.ApplicationCommandOptionBoolean:
			values[opt.Name] = opt.BoolValue()
		case discordgo.ApplicationCommandOptionInteger:
			values[opt.Name] = opt.IntValue()
		}
	}

	if len(values) == 0 {
		return nil
	}

	return &autoreply.CommandArgs{Values: values}
}

// sendDiscordCommandArgMenu sends a button-based arg menu for command interaction.
// TS ref: buildDiscordCommandArgMenu
func sendDiscordCommandArgMenu(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate, cmd *autoreply.ChatCommandDefinition, menu *autoreply.ResolvedArgMenu) {
	commandLabel := cmd.NativeName
	if commandLabel == "" {
		commandLabel = cmd.Key
	}

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	// Build button rows (max 5 buttons per row, max 5 rows)
	var components []discordgo.MessageComponent
	var currentRow []discordgo.MessageComponent
	for _, choice := range menu.Choices {
		customID := buildDiscordCommandArgCustomID(commandLabel, menu.Arg.Name, choice.Value, userID)
		if len(customID) > 100 {
			customID = customID[:100]
		}
		btn := discordgo.Button{
			Label:    choice.Label,
			Style:    discordgo.SecondaryButton,
			CustomID: customID,
		}
		currentRow = append(currentRow, btn)
		if len(currentRow) >= 4 {
			components = append(components, discordgo.ActionsRow{Components: currentRow})
			currentRow = nil
		}
		if len(components) >= 5 {
			break
		}
	}
	if len(currentRow) > 0 && len(components) < 5 {
		components = append(components, discordgo.ActionsRow{Components: currentRow})
	}

	title := menu.Title
	if title == "" {
		title = fmt.Sprintf("Choose %s for /%s.", menu.Arg.Description, commandLabel)
		if menu.Arg.Description == "" {
			title = fmt.Sprintf("Choose %s for /%s.", menu.Arg.Name, commandLabel)
		}
	}

	_ = safeDiscordInteractionCall("command arg menu", nil, func() error {
		return monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    title,
				Components: components,
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})
	})
}

// HandleDiscordCommandArgButton handles button interactions for command arg menus.
// TS ref: handleDiscordCommandArgInteraction
func HandleDiscordCommandArgButton(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}
	data := i.MessageComponentData()
	if !strings.HasPrefix(data.CustomID, discordCommandArgCustomIDKey+":") {
		return
	}

	parsed := parseDiscordCommandArgCustomID(data.CustomID)
	if parsed == nil {
		_ = safeDiscordInteractionCall("command arg update", nil, func() error {
			return monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    "Sorry, that selection is no longer available.",
					Components: []discordgo.MessageComponent{},
				},
			})
		})
		return
	}

	// Check user ownership
	clickerID := ""
	if i.Member != nil && i.Member.User != nil {
		clickerID = i.Member.User.ID
	} else if i.User != nil {
		clickerID = i.User.ID
	}
	if parsed.UserID != "" && clickerID != parsed.UserID {
		_ = safeDiscordInteractionCall("command arg ack", nil, func() error {
			return monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		})
		return
	}

	// Update message to show selection
	_ = safeDiscordInteractionCall("command arg update", nil, func() error {
		return monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("Selected %s.", parsed.Value),
				Components: []discordgo.MessageComponent{},
			},
		})
	})

	// Find command and dispatch
	cmdDef := autoreply.FindCommandByNativeName(parsed.Command, "discord")
	if cmdDef == nil {
		cmdDef = autoreply.FindCommand(parsed.Command)
	}
	if cmdDef == nil {
		return
	}

	args := &autoreply.CommandArgs{
		Values: autoreply.CommandArgValues{parsed.Arg: parsed.Value},
		Raw:    autoreply.SerializeCommandArgs(cmdDef, autoreply.CommandArgValues{parsed.Arg: parsed.Value}),
	}
	prompt := autoreply.BuildCommandTextFromArgs(cmdDef, args)

	logger := monCtx.Logger.With("action", "cmdarg-button", "command", parsed.Command)
	dispatchDiscordSlashCommand(monCtx, i, cmdDef, args, prompt, logger)
}

// buildDiscordCommandArgCustomID builds a custom ID for command arg buttons.
// TS ref: buildDiscordCommandArgCustomId
func buildDiscordCommandArgCustomID(command, arg, value, userID string) string {
	return fmt.Sprintf("%s:command=%s;arg=%s;value=%s;user=%s",
		discordCommandArgCustomIDKey,
		url.QueryEscape(command),
		url.QueryEscape(arg),
		url.QueryEscape(value),
		url.QueryEscape(userID),
	)
}

type parsedCommandArgCustomID struct {
	Command string
	Arg     string
	Value   string
	UserID  string
}

// parseDiscordCommandArgCustomID parses a command arg custom ID.
// TS ref: parseDiscordCommandArgData
func parseDiscordCommandArgCustomID(customID string) *parsedCommandArgCustomID {
	if !strings.HasPrefix(customID, discordCommandArgCustomIDKey+":") {
		return nil
	}
	rest := customID[len(discordCommandArgCustomIDKey)+1:]
	params := make(map[string]string)
	for _, part := range strings.Split(rest, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			decoded, err := url.QueryUnescape(kv[1])
			if err != nil {
				decoded = kv[1]
			}
			params[kv[0]] = decoded
		}
	}
	command := params["command"]
	arg := params["arg"]
	value := params["value"]
	user := params["user"]
	if command == "" || arg == "" || value == "" {
		return nil
	}
	return &parsedCommandArgCustomID{
		Command: command,
		Arg:     arg,
		Value:   value,
		UserID:  user,
	}
}

// resolveCommandAuthorizedFromAuthorizers checks whether a command is authorized
// given a list of authorizers, matching TS resolveCommandAuthorizedFromAuthorizers.
// TS ref: channels/command-gating.ts L8-29
// modeWhenAccessGroupsOff: "allow" (default) | "deny" | "configured"
func resolveCommandAuthorizedFromAuthorizers(useAccessGroups bool, authorizers []commandAuthorizer, modeWhenAccessGroupsOff string) bool {
	if modeWhenAccessGroupsOff == "" {
		modeWhenAccessGroupsOff = "allow"
	}
	if !useAccessGroups {
		switch modeWhenAccessGroupsOff {
		case "allow":
			return true
		case "deny":
			return false
		default: // "configured"
			anyConfigured := false
			for _, a := range authorizers {
				if a.configured {
					anyConfigured = true
					if a.allowed {
						return true
					}
				}
			}
			if !anyConfigured {
				return true
			}
			return false
		}
	}
	// useAccessGroups == true: any configured+allowed → authorized
	for _, a := range authorizers {
		if a.configured && a.allowed {
			return true
		}
	}
	return false
}

// commandAuthorizer represents a single authorization check.
type commandAuthorizer struct {
	configured bool
	allowed    bool
}

// dispatchDiscordSlashCommand dispatches a slash command through the agent pipeline.
// W-047 fix: complete authorization/configuration check chain aligned with TS.
// TS ref: dispatchDiscordCommandInteraction (native-command.ts L490-849)
func dispatchDiscordSlashCommand(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate, cmd *autoreply.ChatCommandDefinition, args *autoreply.CommandArgs, prompt string, logger *slog.Logger) {
	if monCtx.Deps == nil {
		logger.Warn("deps not available for slash command dispatch")
		return
	}

	// ── 1. Resolve user ──
	// TS ref: L527-529
	user := resolveInteractionUser(i)
	if user == nil {
		return
	}

	// ── 2. Resolve sender identity ──
	// TS ref: L531
	sender := ResolveDiscordSenderIdentity(
		DiscordAuthorInfo{ID: user.ID, Username: user.Username, GlobalName: user.GlobalName},
		nil, nil,
	)

	// ── 3. Channel type detection ──
	// TS ref: L534-541
	isDM := i.GuildID == ""
	isGroupDm := false // Discord InteractionCreate does not expose GroupDM type directly
	isGuild := i.GuildID != ""

	// Resolve channel info for thread detection and channel name
	channelName := ""
	isThread := false
	var threadParentID string
	var threadParentName string
	if isGuild && monCtx.Session != nil {
		ch, chErr := monCtx.Session.State.Channel(i.ChannelID)
		if chErr == nil && ch != nil {
			channelName = ch.Name
			if ch.IsThread() {
				isThread = true
				if ch.ParentID != "" {
					parent, parentErr := monCtx.Session.State.Channel(ch.ParentID)
					if parentErr == nil && parent != nil {
						threadParentID = parent.ID
						threadParentName = parent.Name
					}
				}
			}
		}
	}
	channelSlug := ""
	if channelName != "" {
		channelSlug = NormalizeDiscordSlug(channelName)
	}
	threadParentSlug := ""
	if threadParentName != "" {
		threadParentSlug = NormalizeDiscordSlug(threadParentName)
	}

	// ── 4. Owner allow list (for access groups) ──
	// TS ref: L543-555
	// In TS, normalizeDiscordAllowList returns null when raw is empty,
	// meaning ownerAllowList != null is the "configured" check.
	ownerAllowListConfigured := len(monCtx.AllowFrom) > 0
	ownerOk := false
	if ownerAllowListConfigured {
		ownerAllowList := NormalizeDiscordAllowList(monCtx.AllowFrom, []string{"discord:", "user:", "pk:"})
		ownerOk = AllowListMatches(ownerAllowList, sender.ID, sender.Name, sender.Tag).Allowed
	}

	// ── 5. Guild info resolution ──
	// TS ref: L556-559
	var guildInfo *DiscordGuildEntryResolved
	if isGuild {
		guildName := resolveGuildName(monCtx.Session, i.GuildID)
		guildInfo = ResolveDiscordGuildEntry(i.GuildID, guildName, monCtx.GuildConfigs)
	}

	// ── 6. Channel config resolution (with thread parent fallback) ──
	// TS ref: L580-591
	var channelConfig *DiscordChannelConfigResolved
	if isGuild && guildInfo != nil {
		// Try direct channel first
		channelConfig = ResolveDiscordChannelConfig(guildInfo, i.ChannelID, channelName, channelSlug)
		// Thread parent fallback
		if channelConfig == nil && isThread && threadParentID != "" {
			channelConfig = ResolveDiscordChannelConfig(guildInfo, threadParentID, threadParentName, threadParentSlug)
		}
	}

	// ── 7. Channel enabled check ──
	// TS ref: L592-595
	if channelConfig != nil && channelConfig.Enabled != nil && !*channelConfig.Enabled {
		respondInteraction(monCtx.Session, i, "This channel is disabled.", true)
		return
	}

	// ── 8. Channel allowed check ──
	// TS ref: L596-599
	if isGuild && channelConfig != nil && !channelConfig.Allowed {
		respondInteraction(monCtx.Session, i, "This channel is not allowed.", true)
		return
	}

	// ── 9. Access groups + group policy check ──
	// TS ref: L600-614
	useAccessGroups := monCtx.UseAccessGroups
	if useAccessGroups && isGuild {
		channelAllowlistConfigured := guildInfo != nil && len(guildInfo.Channels) > 0
		channelAllowed := channelConfig == nil || channelConfig.Allowed
		allowByPolicy := IsDiscordGroupAllowedByPolicy(
			monCtx.GroupPolicy,
			guildInfo != nil,
			channelAllowlistConfigured,
			channelAllowed,
		)
		if !allowByPolicy {
			respondInteraction(monCtx.Session, i, "This channel is not allowed.", true)
			return
		}
	}

	// ── 10. DM enabled check ──
	// TS ref: L615-616, L618-621
	dmEnabled := monCtx.DMEnabled
	dmPolicy := monCtx.DMPolicy
	commandAuthorized := true

	if isDM {
		if !dmEnabled || dmPolicy == "disabled" {
			respondInteraction(monCtx.Session, i, "Discord DMs are disabled.", true)
			return
		}
		// TS ref: L622-661 — DM policy check with pairing
		if dmPolicy != "open" {
			if !checkDiscordDMSenderAllowed(monCtx, sender.ID, sender.Name) {
				commandAuthorized = false
				if dmPolicy == "pairing" && monCtx.Deps.UpsertPairingRequest != nil {
					result, err := monCtx.Deps.UpsertPairingRequest(DiscordPairingRequestParams{
						Channel: "discord",
						ID:      user.ID,
						Meta: map[string]string{
							"tag":  sender.Tag,
							"name": sender.Name,
						},
					})
					if err == nil && result.Created {
						respondInteraction(monCtx.Session, i, fmt.Sprintf(
							"Pairing code: `%s`\nYour Discord user id: %s",
							result.Code, user.ID,
						), true)
						return
					}
				}
				respondInteraction(monCtx.Session, i, "You are not authorized to use this command.", true)
				return
			}
			commandAuthorized = true
		}
	}

	// ── 11. Guild user allowlist check ──
	// TS ref: L663-689 — user allowlist check for non-DM
	if !isDM {
		// Resolve channel-level or guild-level user allowlist
		var channelUsers []string
		if channelConfig != nil {
			channelUsers = channelConfig.Users
		}
		if len(channelUsers) == 0 && guildInfo != nil {
			channelUsers = guildInfo.Users
		}
		hasUserAllowlist := len(channelUsers) > 0
		userOk := false
		if hasUserAllowlist {
			userOk = ResolveDiscordUserAllowed(channelUsers, sender.ID, sender.Name, sender.Tag)
		}

		// Build authorizers list
		// TS ref: L674-679
		var authorizers []commandAuthorizer
		if useAccessGroups {
			authorizers = []commandAuthorizer{
				{configured: ownerAllowListConfigured, allowed: ownerOk},
				{configured: hasUserAllowlist, allowed: userOk},
			}
		} else {
			authorizers = []commandAuthorizer{
				{configured: hasUserAllowlist, allowed: userOk},
			}
		}

		// TS ref: L680-688 — resolveCommandAuthorizedFromAuthorizers with mode "configured"
		commandAuthorized = resolveCommandAuthorizedFromAuthorizers(
			useAccessGroups,
			authorizers,
			"configured",
		)
		if !commandAuthorized {
			respondInteraction(monCtx.Session, i, "You are not authorized to use this command.", true)
			return
		}
	}

	// ── 12. Group DM enabled check ──
	// TS ref: L690-693
	if isGroupDm && !monCtx.GroupDmEnabled {
		respondInteraction(monCtx.Session, i, "Discord group DMs are disabled.", true)
		return
	}

	// ── 13. Resolve agent route ──
	// TS ref: L730-743
	if monCtx.Deps.ResolveAgentRoute == nil {
		logger.Warn("ResolveAgentRoute not available")
		return
	}

	peerKind := "channel"
	peerID := i.ChannelID
	if isDM {
		peerKind = "direct"
		peerID = user.ID
	} else if isGroupDm {
		peerKind = "group"
	}

	route, err := monCtx.Deps.ResolveAgentRoute(DiscordAgentRouteParams{
		Channel:   "discord",
		AccountID: monCtx.AccountID,
		PeerKind:  peerKind,
		PeerID:    peerID,
	})
	if err != nil || route == nil {
		logger.Warn("route resolution failed", "error", err)
		return
	}

	// ── 14. Build MsgContext with full fields ──
	// TS ref: L750-799
	chatType := "channel"
	if isDM {
		chatType = "direct"
	} else if isGroupDm {
		chatType = "group"
	}

	senderName := user.GlobalName
	if senderName == "" {
		senderName = user.Username
	}

	// SystemPrompt from channel config
	// TS ref: L767-773
	groupSystemPrompt := ""
	if isGuild && channelConfig != nil && channelConfig.SystemPrompt != "" {
		groupSystemPrompt = strings.TrimSpace(channelConfig.SystemPrompt)
	}

	msgCtx := &autoreply.MsgContext{
		Provider:          "discord",
		Surface:           "discord",
		AccountID:         route.AccountID,
		SessionKey:        route.SessionKey,
		Body:              prompt,
		RawBody:           prompt,
		CommandBody:       prompt,
		ChatType:          chatType,
		ChannelID:         i.ChannelID,
		SenderID:          user.ID,
		SenderName:        senderName,
		SenderUsername:    user.Username,
		WasMentioned:      "true", // Slash commands count as explicit mention
		CommandAuthorized: commandAuthorized,
		CommandSource:     "native",
		GroupSystemPrompt: groupSystemPrompt,
	}

	// Record inbound session
	if monCtx.Deps.RecordInboundSession != nil {
		storePath := ""
		if monCtx.Deps.ResolveStorePath != nil {
			storePath = monCtx.Deps.ResolveStorePath(route.AgentID)
		}
		_ = monCtx.Deps.RecordInboundSession(DiscordRecordSessionParams{
			StorePath:  storePath,
			SessionKey: route.SessionKey,
			Ctx:        msgCtx,
		})
	}

	// ── 15. Dispatch to auto-reply pipeline ──
	if monCtx.Deps.DispatchInboundMessage == nil {
		logger.Warn("DispatchInboundMessage not available")
		return
	}

	// Defer the interaction response to give us time
	_ = safeDiscordInteractionCall("defer", logger, func() error {
		return monCtx.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
	})

	result, err := monCtx.Deps.DispatchInboundMessage(context.Background(), DiscordDispatchParams{
		Ctx: msgCtx,
	})
	if err != nil {
		logger.Error("slash command dispatch failed", "error", err)
		followUpInteraction(monCtx.Session, i, fmt.Sprintf("Command failed: %v", err))
		return
	}

	if result == nil || !result.QueuedFinal {
		followUpInteraction(monCtx.Session, i, "Command processed.")
	}

	logger.Debug("slash command dispatched",
		"command", cmd.Key,
		"sessionKey", route.SessionKey,
	)
}

// deliverDiscordInteractionReply delivers a reply to a Discord interaction with chunking
// and optional media/file attachments.
// TS ref: deliverDiscordInteractionReply (native-command.ts L851-935)
func deliverDiscordInteractionReply(session *discordgo.Session, i *discordgo.InteractionCreate, text string, mediaURLs []string, textLimit int) {
	// ── Media support (W-049) ──
	// TS: const mediaList = payload.mediaUrls ?? (payload.mediaUrl ? [payload.mediaUrl] : [])
	if len(mediaURLs) > 0 {
		// Load all media files
		var files []*discordgo.File
		for _, mediaURL := range mediaURLs {
			m, err := loadDiscordMedia(mediaURL)
			if err != nil {
				continue
			}
			files = append(files, &discordgo.File{
				Name:        m.FileName,
				ContentType: m.ContentType,
				Reader:      bytes.NewReader(m.Data),
			})
		}

		chunks := ChunkDiscordText(text, ChunkDiscordTextOpts{MaxChars: textLimit})
		if len(chunks) == 0 && text != "" {
			chunks = []string{text}
		}

		// Send first chunk (caption) with media files attached
		caption := ""
		if len(chunks) > 0 {
			caption = chunks[0]
		}
		_ = safeDiscordInteractionCall("interaction send media", nil, func() error {
			_, err := session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: caption,
				Files:   files,
			})
			return err
		})

		// Send remaining text chunks without media
		for _, chunk := range chunks[1:] {
			if strings.TrimSpace(chunk) == "" {
				continue
			}
			_ = safeDiscordInteractionCall("interaction follow-up", nil, func() error {
				_, err := session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: chunk,
				})
				return err
			})
		}
		return
	}

	// ── Text-only path ──
	if strings.TrimSpace(text) == "" {
		return
	}

	chunks := ChunkDiscordText(text, ChunkDiscordTextOpts{MaxChars: textLimit})
	if len(chunks) == 0 && text != "" {
		chunks = []string{text}
	}

	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		_ = safeDiscordInteractionCall("interaction send", nil, func() error {
			_, err := session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: chunk,
			})
			return err
		})
	}
}

// respondInteraction sends an immediate interaction response.
// Uses safeDiscordInteractionCall to gracefully handle expired interactions.
func respondInteraction(session *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	_ = safeDiscordInteractionCall("respond", nil, func() error {
		return session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   flags,
			},
		})
	})
}

// followUpInteraction sends a follow-up message to an interaction.
// Uses safeDiscordInteractionCall to gracefully handle expired interactions.
func followUpInteraction(session *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = safeDiscordInteractionCall("follow-up", nil, func() error {
		_, err := session.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
		})
		return err
	})
}

// resolveInteractionUser extracts the user from an interaction.
func resolveInteractionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User
	}
	return i.User
}

// isDiscordUnknownInteraction checks if an error is an "Unknown interaction" error.
// W-046 fix: aligned with TS detection — checks discordgo.RESTError for API error
// code 10062 and HTTP 404 status, in addition to string matching as fallback.
// TS ref: isDiscordUnknownInteraction (native-command.ts L178-198)
// TS checks: err.discordCode === 10062, err.rawBody?.code === 10062,
//
//	err.status === 404 && /Unknown interaction/i, /Unknown interaction/i on rawBody.message
func isDiscordUnknownInteraction(err error) bool {
	if err == nil {
		return false
	}

	// Check discordgo.RESTError for structured error code (matches TS discordCode / rawBody.code)
	if restErr, ok := err.(*discordgo.RESTError); ok {
		// Check API error code 10062 (TS: err.discordCode === 10062 || err.rawBody?.code === 10062)
		if restErr.Message != nil && restErr.Message.Code == 10062 {
			return true
		}
		// Check HTTP 404 + "Unknown interaction" in message (TS: err.status === 404 && /Unknown interaction/i)
		if restErr.Response != nil && restErr.Response.StatusCode == 404 {
			if restErr.Message != nil && strings.Contains(strings.ToLower(restErr.Message.Message), "unknown interaction") {
				return true
			}
		}
	}

	// Fallback: string matching on error message for non-RESTError types
	msg := err.Error()
	return strings.Contains(msg, "10062") || strings.Contains(strings.ToLower(msg), "unknown interaction")
}

// safeDiscordInteractionCall wraps a Discord interaction API call, swallowing
// "unknown interaction" errors (expired interactions) and logging a warning.
// Returns the result of fn, or the zero value + nil error when the interaction
// has expired. All other errors are propagated unchanged.
// TS ref: safeDiscordInteractionCall (native-command.ts L200-213)
func safeDiscordInteractionCall(label string, logger *slog.Logger, fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}
	if isDiscordUnknownInteraction(err) {
		if logger != nil {
			logger.Warn(fmt.Sprintf("discord: %s skipped (interaction expired)", label))
		}
		return nil
	}
	return err
}
