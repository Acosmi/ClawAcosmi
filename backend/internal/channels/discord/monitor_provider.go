package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/internal/config"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Discord 监控入口 — 继承自 src/discord/monitor/provider.ts (690L) + listeners.ts (322L)
// Phase 9 实现：discordgo Gateway v10 + 事件绑定。

// MonitorDiscordMode 监控模式
type MonitorDiscordMode string

const (
	MonitorDiscordModeGateway MonitorDiscordMode = "gateway"
	MonitorDiscordModeHTTP    MonitorDiscordMode = "http" // 未来 Interactions-only 模式
)

// MonitorDiscordOpts 监控选项
// W-014 fix: 补全 TS MonitorDiscordOpts 中的运行时覆盖字段
// TS ref: provider.ts L44-53
type MonitorDiscordOpts struct {
	Token     string
	AccountID string
	Mode      MonitorDiscordMode
	Deps      *DiscordMonitorDeps

	// 运行时覆盖字段 — 对齐 TS MonitorDiscordOpts
	MediaMaxMb   *int              // opts.mediaMaxMb — 覆盖配置中的 mediaMaxMb
	HistoryLimit *int              // opts.historyLimit — 覆盖配置中的 historyLimit
	ReplyToMode  types.ReplyToMode // opts.replyToMode — 覆盖配置中的 replyToMode
}

// DiscordMonitorContext 监控上下文
type DiscordMonitorContext struct {
	mu sync.RWMutex

	// 身份
	BotUserID string
	AccountID string
	Token     string

	// Guild 配置
	GuildConfigs map[string]DiscordGuildEntryResolved

	// 策略
	DMPolicy        string // "open" | "allowlist" | "pairing" | "disabled"
	DMEnabled       bool   // dm.enabled — default true
	GroupDmEnabled  bool   // dm.groupEnabled — default true
	AllowFrom       []string
	GroupPolicy     string // "open" | "allowlist" | "disabled"
	RequireMention  bool
	UseAccessGroups bool // commands.useAccessGroups — default true

	// discordgo 会话
	Session *discordgo.Session

	// DI 依赖
	Deps *DiscordMonitorDeps

	// 在线状态缓存
	PresenceCache *DiscordPresenceCache

	// 回复上下文
	ReplyCtxMap sync.Map // channelID → *DiscordReplyCtx

	// 运行时参数 — W-014 fix: 对齐 TS provider 局部变量
	MediaMaxBytes int               // mediaMaxMb * 1024 * 1024
	HistoryLimit  int               // 历史消息限制
	ReplyToMode   types.ReplyToMode // 回复引用模式

	// W-021 fix: exec approvals handler
	ExecApprovalsHandler *DiscordExecApprovalHandler

	// 日志
	Logger *slog.Logger
}

// resolveDiscordGatewayIntents 根据配置条件启用 Gateway intents。
// 对齐 TS: src/discord/monitor/provider.ts — resolveDiscordGatewayIntents()
// 基础 intents 总是启用；GuildPresences / GuildMembers 是特权 intent，
// 仅在配置中显式启用时才添加，否则 bot 无特权 intent 权限时会导致连接失败。
func resolveDiscordGatewayIntents(intentsCfg *types.DiscordIntentsConfig) discordgo.Intent {
	intents := discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsDirectMessageReactions

	if intentsCfg != nil && intentsCfg.Presence != nil && *intentsCfg.Presence {
		intents |= discordgo.IntentsGuildPresences
	}
	if intentsCfg != nil && intentsCfg.GuildMembers != nil && *intentsCfg.GuildMembers {
		intents |= discordgo.IntentsGuildMembers
	}
	return intents
}

// MonitorDiscordProvider 启动 Discord 监控提供者。
func MonitorDiscordProvider(ctx context.Context, cfg *types.OpenAcosmiConfig, opts MonitorDiscordOpts) error {
	if opts.Token == "" {
		return fmt.Errorf("discord monitor: bot token is required")
	}

	logger := slog.Default().With("channel", "discord", "account", opts.AccountID)

	// 解析账户配置以获取 intents 等设置
	account := ResolveDiscordAccount(cfg, opts.AccountID)
	discordCfg := account.Config

	// 创建 discordgo 会话
	session, err := discordgo.New("Bot " + opts.Token)
	if err != nil {
		return fmt.Errorf("discord monitor: create session: %w", err)
	}

	// 设置 intents — 基础 intents 总是启用；特权 intents（GuildPresences / GuildMembers）
	// 仅在配置中显式启用时才添加（对齐 TS 条件逻辑）。
	session.Identify.Intents = resolveDiscordGatewayIntents(discordCfg.Intents)

	// W-014 fix: 解析运行时覆盖字段
	// TS ref: provider.ts L182-190
	mediaMaxMb := 8 // default
	if opts.MediaMaxMb != nil {
		mediaMaxMb = *opts.MediaMaxMb
	} else if discordCfg.MediaMaxMB != nil {
		mediaMaxMb = *discordCfg.MediaMaxMB
	}
	mediaMaxBytes := mediaMaxMb * 1024 * 1024

	historyLimit := 20 // default
	if opts.HistoryLimit != nil {
		historyLimit = *opts.HistoryLimit
	} else if discordCfg.HistoryLimit != nil {
		historyLimit = *discordCfg.HistoryLimit
	} else if cfg != nil && cfg.Messages != nil && cfg.Messages.GroupChat != nil && cfg.Messages.GroupChat.HistoryLimit != nil {
		historyLimit = *cfg.Messages.GroupChat.HistoryLimit
	}
	if historyLimit < 0 {
		historyLimit = 0
	}

	replyToMode := types.ReplyToOff // default "off"
	if opts.ReplyToMode != "" {
		replyToMode = opts.ReplyToMode
	} else if discordCfg.DiscordAccountConfigReplyToMode() != "" {
		replyToMode = discordCfg.DiscordAccountConfigReplyToMode()
	}

	// 构建监控上下文（defaults matching TS)
	monCtx := &DiscordMonitorContext{
		AccountID:       opts.AccountID,
		Token:           opts.Token,
		DMEnabled:       true, // TS default: dm.enabled ?? true
		GroupDmEnabled:  true, // TS default: dm.groupEnabled ?? true (not false)
		UseAccessGroups: true, // TS default: commands.useAccessGroups !== false
		Session:         session,
		Deps:            opts.Deps,
		PresenceCache:   NewDiscordPresenceCache(),
		MediaMaxBytes:   mediaMaxBytes,
		HistoryLimit:    historyLimit,
		ReplyToMode:     replyToMode,
		Logger:          logger,
	}

	// 从 cfg 解析 Discord 配置
	if cfg != nil && cfg.Channels != nil && cfg.Channels.Discord != nil {
		monCtx.applyDiscordConfig(cfg)
	}

	// W-016 fix: 在 Gateway 连接前解析 allowlist 中的名称→ID
	// 对齐 TS: provider.ts L213-417 — 使用 Discord API 将配置中基于名称的
	// guild/channel/user 条目解析为 Discord snowflake ID。
	monCtx.resolveAllowlistNames(ctx)

	// W-021 fix: 初始化 execApprovalsHandler
	// TS ref: provider.ts L466-475 — 在 client 创建前初始化 handler
	monCtx.ExecApprovalsHandler = initExecApprovalsHandler(discordCfg, opts, session)

	// W-018 fix: 解析 slash command 配置并获取 applicationId
	// TS ref: provider.ts L426-515
	var nativeEnabled bool
	var nativeDisabledExplicit bool
	var applicationID string
	var slashCommands []*discordgo.ApplicationCommand

	if discordCfg.Commands != nil {
		provNative := toNativeCommandsSetting(discordCfg.Commands.Native)
		globalNative := resolveGlobalNativeSetting(cfg)
		nativeEnabled = config.ResolveNativeCommandsEnabled("discord", provNative, globalNative)
		nativeDisabledExplicit = config.IsNativeCommandsExplicitlyDisabled(provNative, globalNative)
	} else {
		nativeEnabled = config.ResolveNativeCommandsEnabled("discord", nil, resolveGlobalNativeSetting(cfg))
	}

	if nativeEnabled || nativeDisabledExplicit {
		applicationID = FetchDiscordApplicationId(ctx, opts.Token, 4000, nil)
		if applicationID == "" {
			logger.Warn("discord: failed to resolve application id; slash commands will not be deployed")
		}
	}

	if nativeEnabled && applicationID != "" {
		specs := autoreply.ListNativeCommandSpecsForConfig(nil, "discord")
		slashCommands = BuildDiscordApplicationCommands(specs)
		if err := SyncDiscordSlashCommands(session, applicationID, slashCommands); err != nil {
			logger.Error("discord: failed to sync slash commands", "error", err)
		} else {
			logger.Info("discord: slash commands synced", "count", len(slashCommands))
		}
	}

	// W-018 fix: 若 native 被显式禁用，清空已注册的 slash commands
	// TS ref: provider.ts L521-527
	if nativeDisabledExplicit && applicationID != "" {
		if err := ClearDiscordSlashCommands(session, applicationID); err != nil {
			logger.Error("discord: failed to clear slash commands", "error", err)
		} else {
			logger.Info("discord: cleared native commands (commands.native=false)")
		}
	}

	// 绑定事件处理器（传入 intents 配置以条件注册特权 intent 处理器）
	bindDiscordEventHandlers(monCtx, discordCfg.Intents)

	// 打开 Gateway 连接
	if err := session.Open(); err != nil {
		return fmt.Errorf("discord monitor: open gateway: %w", err)
	}

	logger.Info("Discord gateway connected",
		"botUserID", session.State.User.ID,
		"mode", opts.Mode,
	)
	monCtx.BotUserID = session.State.User.ID

	// W-019 fix: 30s HELLO timeout — 僵尸连接检测
	// 对齐 TS: provider.ts L619-642
	// TS 在 gateway 连接后检测是否在 30s 内收到 HELLO (Ready) 事件，
	// 若未收到则视为僵尸连接并强制重连。
	// discordgo 的 Connect 事件 ≈ TS 的 "WebSocket connection opened"，
	// Ready 事件 ≈ TS 的 HELLO 处理完成。
	const helloTimeoutDuration = 30 * time.Second
	helloReceived := make(chan struct{}, 1)
	var helloOnce sync.Once

	// 注册 Ready handler 来取消超时
	removeReadyHandler := session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		helloOnce.Do(func() {
			close(helloReceived)
		})
	})

	// 注册 Connect handler 来启动超时检测（每次 WebSocket 重连都会触发）
	removeConnectHandler := session.AddHandler(func(s *discordgo.Session, c *discordgo.Connect) {
		helloOnce = sync.Once{} // reset for reconnection
		helloReceived = make(chan struct{}, 1)

		go func() {
			timer := time.NewTimer(helloTimeoutDuration)
			defer timer.Stop()
			select {
			case <-helloReceived:
				// HELLO received in time, all good
			case <-timer.C:
				logger.Warn("connection stalled: no HELLO received within 30s, forcing reconnect")
				// discordgo 会自动处理重连; Close+Open 强制重新连接
				_ = s.Close()
				if err := s.Open(); err != nil {
					logger.Error("failed to reopen gateway after HELLO timeout", "error", err)
				}
			case <-ctx.Done():
				// shutting down, ignore
			}
		}()
	})

	// W-017 fix: 注册 Gateway 到注册表
	// TS ref: provider.ts L596-597
	RegisterGateway(opts.AccountID, session)

	// W-021 fix: 启动 exec approvals handler（gateway 就绪后）
	// TS ref: provider.ts L590-592
	if monCtx.ExecApprovalsHandler != nil {
		monCtx.ExecApprovalsHandler.started = true
		logger.Info("discord: exec approvals handler started")
	}

	// W-020 fix: 完整关闭语义 — 对齐 TS abortSignal + waitForDiscordGatewayStop
	// TS ref: provider.ts L603-674
	// 使用 context 取消 + gatewayDone channel 实现 TS 中 abortSignal +
	// waitForDiscordGatewayStop 的等价语义：
	//   1. ctx 取消时触发 gateway 断连（等价于 TS onAbort）
	//   2. gatewayDone channel 等待 Disconnect 事件确认关闭完成
	//      （等价于 TS waitForDiscordGatewayStop）
	//   3. finally 块清理资源（等价于 TS finally）
	gatewayDone := make(chan struct{})

	// 监听 Disconnect 事件 — 当 ctx 已取消时，Disconnect 表示关闭流程完成
	removeDisconnectHandler := session.AddHandler(func(s *discordgo.Session, d *discordgo.Disconnect) {
		select {
		case <-ctx.Done():
			// ctx 已取消，这是预期的关闭 disconnect；通知等待方
			select {
			case gatewayDone <- struct{}{}:
			default:
			}
		default:
			// ctx 未取消，这是意外断连（discordgo 会自动重连）
			logger.Warn("discord gateway disconnected unexpectedly, awaiting auto-reconnect")
		}
	})

	// 等待 ctx 取消 → 触发优雅关闭
	<-ctx.Done()
	logger.Info("Discord gateway shutting down")

	// 关闭 session（触发 Disconnect 事件）
	_ = session.Close()

	// 等待 Disconnect 确认或超时
	shutdownTimeout := 5 * time.Second
	select {
	case <-gatewayDone:
		logger.Info("Discord gateway shutdown confirmed")
	case <-time.After(shutdownTimeout):
		logger.Warn("Discord gateway shutdown timed out, proceeding with cleanup")
	}

	// finally: 清理资源（对齐 TS finally 块）
	removeReadyHandler()
	removeConnectHandler()
	removeDisconnectHandler()

	// W-017 fix: 注销 Gateway
	// TS ref: provider.ts L664
	UnregisterGateway(opts.AccountID)

	// W-021 fix: 停止 exec approvals handler
	// TS ref: provider.ts L671-673
	if monCtx.ExecApprovalsHandler != nil {
		monCtx.ExecApprovalsHandler.Stop()
		logger.Info("discord: exec approvals handler stopped")
	}

	return nil
}

// resolveGlobalNativeSetting 从全局配置中获取 commands.native 设置。
func resolveGlobalNativeSetting(cfg *types.OpenAcosmiConfig) config.NativeCommandsSetting {
	if cfg == nil || cfg.Commands == nil {
		return nil
	}
	return toNativeCommandsSetting(cfg.Commands.Native)
}

// toNativeCommandsSetting 将 types.NativeCommandsSetting (interface{}) 转换为 config.NativeCommandsSetting (*bool)。
func toNativeCommandsSetting(v types.NativeCommandsSetting) config.NativeCommandsSetting {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case bool:
		return &val
	case *bool:
		return val
	default:
		return nil
	}
}

// initExecApprovalsHandler 初始化执行审批处理器。
// W-021 fix: 对齐 TS provider.ts L466-475
func initExecApprovalsHandler(discordCfg types.DiscordAccountConfig, opts MonitorDiscordOpts, session *discordgo.Session) *DiscordExecApprovalHandler {
	if discordCfg.ExecApprovals == nil {
		return nil
	}
	execCfg := discordCfg.ExecApprovals
	if execCfg.Enabled == nil || !*execCfg.Enabled {
		return nil
	}

	// 将 []interface{} approvers 转换为 []string
	var approvers []string
	for _, v := range execCfg.Approvers {
		if s, ok := v.(string); ok {
			approvers = append(approvers, s)
		} else {
			approvers = append(approvers, fmt.Sprintf("%v", v))
		}
	}
	if len(approvers) == 0 {
		return nil
	}

	return NewDiscordExecApprovalHandler(DiscordExecApprovalHandlerOpts{
		Token:     opts.Token,
		AccountID: opts.AccountID,
		Config: ExecApprovalConfig{
			Enabled:       true,
			Approvers:     approvers,
			AgentFilter:   execCfg.AgentFilter,
			SessionFilter: execCfg.SessionFilter,
		},
		Session: session,
	})
}

// ClearDiscordSlashCommands clears all registered slash commands for the application.
// W-018 fix: 对齐 TS clearDiscordNativeCommands (provider.ts L677-690)
func ClearDiscordSlashCommands(session *discordgo.Session, appID string) error {
	// Fetch existing commands and delete them
	existingCmds, err := session.ApplicationCommands(appID, "")
	if err != nil {
		return fmt.Errorf("fetch existing commands: %w", err)
	}
	for _, cmd := range existingCmds {
		if err := session.ApplicationCommandDelete(appID, "", cmd.ID); err != nil {
			return fmt.Errorf("delete command %s: %w", cmd.Name, err)
		}
	}
	return nil
}

// applyDiscordConfig 从 OpenAcosmiConfig 应用 Discord 配置。
func (monCtx *DiscordMonitorContext) applyDiscordConfig(cfg *types.OpenAcosmiConfig) {
	dc := cfg.Channels.Discord
	if dc == nil {
		return
	}

	// DM 配置
	if dc.DM != nil {
		if dc.DM.Policy != "" {
			monCtx.DMPolicy = string(dc.DM.Policy)
		}
		// dm.enabled — TS default true
		if dc.DM.Enabled != nil {
			monCtx.DMEnabled = *dc.DM.Enabled
		}
		// dm.groupEnabled — TS default true (not false)
		if dc.DM.GroupEnabled != nil {
			monCtx.GroupDmEnabled = *dc.DM.GroupEnabled
		}
	}
	if monCtx.DMPolicy == "" {
		monCtx.DMPolicy = "pairing"
	}

	// AllowFrom（从 dc.DM.AllowFrom 读取，转为 []string）
	if dc.DM != nil && len(dc.DM.AllowFrom) > 0 {
		for _, v := range dc.DM.AllowFrom {
			if s, ok := v.(string); ok {
				monCtx.AllowFrom = append(monCtx.AllowFrom, s)
			}
		}
	}

	// 群组策略
	if dc.DiscordAccountConfigGroupPolicy() != "" {
		monCtx.GroupPolicy = string(dc.DiscordAccountConfigGroupPolicy())
	}
	if monCtx.GroupPolicy == "" {
		monCtx.GroupPolicy = "open"
	}

	// commands.useAccessGroups — TS: cfg.commands?.useAccessGroups !== false → default true
	// Only the global CommandsConfig carries UseAccessGroups; ProviderCommandsConfig does not.
	if cfg.Commands != nil && cfg.Commands.UseAccessGroups != nil {
		monCtx.UseAccessGroups = *cfg.Commands.UseAccessGroups
	}

	// W-016 fix: Guild 配置 — 将 types.DiscordGuildEntry map 转换为
	// DiscordGuildEntryResolved map 并存储到 GuildConfigs
	// 对齐 TS: provider.ts L166 (let guildEntries = discordCfg.guilds)
	if len(dc.Guilds) > 0 {
		monCtx.GuildConfigs = make(map[string]DiscordGuildEntryResolved, len(dc.Guilds))
		for key, guildEntry := range dc.Guilds {
			if guildEntry == nil {
				monCtx.GuildConfigs[key] = DiscordGuildEntryResolved{}
				continue
			}
			resolved := DiscordGuildEntryResolved{
				Slug:                 guildEntry.Slug,
				RequireMention:       guildEntry.RequireMention,
				ReactionNotification: string(guildEntry.ReactionNotifications),
			}
			// Convert Users []interface{} → []string
			if len(guildEntry.Users) > 0 {
				for _, v := range guildEntry.Users {
					s := strings.TrimSpace(fmt.Sprintf("%v", v))
					if s != "" {
						resolved.Users = append(resolved.Users, s)
					}
				}
			}
			// Convert Channels map[string]*DiscordGuildChannelConfig → map[string]DiscordChannelEntryResolved
			if len(guildEntry.Channels) > 0 {
				resolved.Channels = make(map[string]DiscordChannelEntryResolved, len(guildEntry.Channels))
				for chKey, chCfg := range guildEntry.Channels {
					if chCfg == nil {
						resolved.Channels[chKey] = DiscordChannelEntryResolved{}
						continue
					}
					chResolved := DiscordChannelEntryResolved{
						Allow:                chCfg.Allow,
						RequireMention:       chCfg.RequireMention,
						Skills:               chCfg.Skills,
						Enabled:              chCfg.Enabled,
						SystemPrompt:         chCfg.SystemPrompt,
						IncludeThreadStarter: chCfg.IncludeThreadStarter,
					}
					// Convert channel Users []interface{} → []string
					for _, v := range chCfg.Users {
						s := strings.TrimSpace(fmt.Sprintf("%v", v))
						if s != "" {
							chResolved.Users = append(chResolved.Users, s)
						}
					}
					resolved.Channels[chKey] = chResolved
				}
			}
			monCtx.GuildConfigs[key] = resolved
		}
	}
}

// mergeAllowlist 合并允许列表（去重，保留顺序）
// 对齐 TS: src/channels/allowlists/resolve-utils.ts — mergeAllowlist()
func mergeAllowlist(existing []string, additions []string) []string {
	seen := make(map[string]bool)
	var merged []string
	push := func(val string) {
		normalized := strings.TrimSpace(val)
		if normalized == "" {
			return
		}
		key := strings.ToLower(normalized)
		if seen[key] {
			return
		}
		seen[key] = true
		merged = append(merged, normalized)
	}
	for _, e := range existing {
		push(e)
	}
	for _, a := range additions {
		push(a)
	}
	return merged
}

// summarizeResolveMapping 打印 resolve 结果摘要日志
// 对齐 TS: src/channels/allowlists/resolve-utils.ts — summarizeMapping()
func summarizeResolveMapping(logger *slog.Logger, label string, mapping, unresolved []string) {
	if len(mapping) > 0 {
		sample := mapping
		suffix := ""
		if len(sample) > 6 {
			suffix = fmt.Sprintf(" (+%d)", len(sample)-6)
			sample = sample[:6]
		}
		logger.Info(fmt.Sprintf("%s resolved: %s%s", label, strings.Join(sample, ", "), suffix))
	}
	if len(unresolved) > 0 {
		sample := unresolved
		suffix := ""
		if len(sample) > 6 {
			suffix = fmt.Sprintf(" (+%d)", len(sample)-6)
			sample = sample[:6]
		}
		logger.Warn(fmt.Sprintf("%s unresolved: %s%s", label, strings.Join(sample, ", "), suffix))
	}
}

// resolveAllowlistNames 通过 Discord API 将配置中基于名称的 guild/channel/user 条目
// 解析为对应的 Discord ID，然后将解析结果更新回 monCtx。
// 对齐 TS: src/discord/monitor/provider.ts L213-417 — 三阶段 resolve 逻辑：
//  1. guild + channel name → ID (resolveDiscordChannelAllowlist)
//  2. dm.allowFrom username → user ID (resolveDiscordUserAllowlist)
//  3. guild/channel level users username → user ID (resolveDiscordUserAllowlist)
func (monCtx *DiscordMonitorContext) resolveAllowlistNames(ctx context.Context) {
	token := monCtx.Token
	if token == "" {
		return
	}
	logger := monCtx.Logger

	// ── Phase 1: Resolve guild/channel names → IDs ──
	// 对齐 TS: provider.ts L214-286
	if len(monCtx.GuildConfigs) > 0 {
		type resolveEntry struct {
			input      string
			guildKey   string
			channelKey string
		}
		var entries []resolveEntry
		for guildKey, guildCfg := range monCtx.GuildConfigs {
			if guildKey == "*" {
				continue
			}
			channelKeys := make([]string, 0)
			for chKey := range guildCfg.Channels {
				if chKey != "*" {
					channelKeys = append(channelKeys, chKey)
				}
			}
			if len(channelKeys) == 0 {
				entries = append(entries, resolveEntry{input: guildKey, guildKey: guildKey})
				continue
			}
			for _, chKey := range channelKeys {
				entries = append(entries, resolveEntry{
					input:      guildKey + "/" + chKey,
					guildKey:   guildKey,
					channelKey: chKey,
				})
			}
		}

		if len(entries) > 0 {
			inputStrings := make([]string, len(entries))
			for i, e := range entries {
				inputStrings[i] = e.input
			}
			resolved, err := ResolveDiscordChannelAllowlist(ctx, token, inputStrings, nil)
			if err != nil {
				logger.Warn("discord channel resolve failed; using config entries",
					"error", err.Error())
			} else {
				// Build next guild map starting from current entries
				nextGuilds := make(map[string]DiscordGuildEntryResolved)
				for k, v := range monCtx.GuildConfigs {
					nextGuilds[k] = v
				}

				var mapping []string
				var unresolvedEntries []string

				for _, res := range resolved {
					// Find the source entry
					var source *resolveEntry
					for i := range entries {
						if entries[i].input == res.Input {
							source = &entries[i]
							break
						}
					}
					if source == nil {
						continue
					}

					sourceGuild := monCtx.GuildConfigs[source.guildKey]

					if !res.Resolved || res.GuildID == "" {
						unresolvedEntries = append(unresolvedEntries, res.Input)
						continue
					}

					// Build mapping description
					if res.ChannelID != "" {
						mapping = append(mapping, fmt.Sprintf("%s→%s/%s", res.Input, res.GuildID, res.ChannelID))
					} else {
						mapping = append(mapping, fmt.Sprintf("%s→%s", res.Input, res.GuildID))
					}

					// Merge resolved entry into nextGuilds keyed by resolved guild ID
					existing, hasExisting := nextGuilds[res.GuildID]
					if !hasExisting {
						existing = DiscordGuildEntryResolved{}
					}
					mergedChannels := mergeChannelMaps(sourceGuild.Channels, existing.Channels)
					merged := mergeGuildEntry(sourceGuild, existing)
					merged.Channels = mergedChannels
					merged.ID = res.GuildID
					nextGuilds[res.GuildID] = merged

					// If a specific channel was resolved, merge its config under the resolved ID key
					if source.channelKey != "" && res.ChannelID != "" {
						sourceChannel, hasSrc := sourceGuild.Channels[source.channelKey]
						if hasSrc {
							if merged.Channels == nil {
								merged.Channels = make(map[string]DiscordChannelEntryResolved)
							}
							existingCh, hasExCh := merged.Channels[res.ChannelID]
							if hasExCh {
								merged.Channels[res.ChannelID] = mergeChannelEntry(sourceChannel, existingCh)
							} else {
								merged.Channels[res.ChannelID] = sourceChannel
							}
							nextGuilds[res.GuildID] = merged
						}
					}
				}
				monCtx.GuildConfigs = nextGuilds
				summarizeResolveMapping(logger, "discord channels", mapping, unresolvedEntries)
			}
		}
	}

	// ── Phase 2: Resolve dm.allowFrom usernames → user IDs ──
	// 对齐 TS: provider.ts L288-314
	var allowEntries []string
	for _, entry := range monCtx.AllowFrom {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" && trimmed != "*" {
			allowEntries = append(allowEntries, trimmed)
		}
	}
	if len(allowEntries) > 0 {
		resolvedUsers, err := ResolveDiscordUserAllowlist(ctx, token, allowEntries, nil)
		if err != nil {
			logger.Warn("discord user resolve failed; using config entries",
				"error", err.Error())
		} else {
			var mapping []string
			var unresolvedEntries []string
			var additions []string
			for _, entry := range resolvedUsers {
				if entry.Resolved && entry.ID != "" {
					mapping = append(mapping, fmt.Sprintf("%s→%s", entry.Input, entry.ID))
					additions = append(additions, entry.ID)
				} else {
					unresolvedEntries = append(unresolvedEntries, entry.Input)
				}
			}
			monCtx.AllowFrom = mergeAllowlist(monCtx.AllowFrom, additions)
			summarizeResolveMapping(logger, "discord users", mapping, unresolvedEntries)
		}
	}

	// ── Phase 3: Resolve guild/channel level user names → user IDs ──
	// 对齐 TS: provider.ts L316-417
	if len(monCtx.GuildConfigs) > 0 {
		// Collect all unique user entries across all guilds and channels
		userEntrySet := make(map[string]bool)
		for _, guild := range monCtx.GuildConfigs {
			for _, u := range guild.Users {
				trimmed := strings.TrimSpace(u)
				if trimmed != "" && trimmed != "*" {
					userEntrySet[trimmed] = true
				}
			}
			for _, ch := range guild.Channels {
				for _, u := range ch.Users {
					trimmed := strings.TrimSpace(u)
					if trimmed != "" && trimmed != "*" {
						userEntrySet[trimmed] = true
					}
				}
			}
		}

		if len(userEntrySet) > 0 {
			userEntries := make([]string, 0, len(userEntrySet))
			for k := range userEntrySet {
				userEntries = append(userEntries, k)
			}
			resolvedUsers, err := ResolveDiscordUserAllowlist(ctx, token, userEntries, nil)
			if err != nil {
				logger.Warn("discord channel user resolve failed; using config entries",
					"error", err.Error())
			} else {
				resolvedMap := make(map[string]DiscordUserResolution)
				var mapping []string
				var unresolvedEntries []string
				for _, entry := range resolvedUsers {
					resolvedMap[entry.Input] = entry
					if entry.Resolved && entry.ID != "" {
						mapping = append(mapping, fmt.Sprintf("%s→%s", entry.Input, entry.ID))
					} else {
						unresolvedEntries = append(unresolvedEntries, entry.Input)
					}
				}

				nextGuilds := make(map[string]DiscordGuildEntryResolved)
				for guildKey, guildConfig := range monCtx.GuildConfigs {
					nextGuild := guildConfig

					// Resolve guild-level users
					if len(guildConfig.Users) > 0 {
						var additions []string
						for _, u := range guildConfig.Users {
							trimmed := strings.TrimSpace(u)
							if res, ok := resolvedMap[trimmed]; ok && res.Resolved && res.ID != "" {
								additions = append(additions, res.ID)
							}
						}
						nextGuild.Users = mergeAllowlist(guildConfig.Users, additions)
					}

					// Resolve channel-level users
					if len(guildConfig.Channels) > 0 {
						nextChannels := make(map[string]DiscordChannelEntryResolved, len(guildConfig.Channels))
						for chKey, chConfig := range guildConfig.Channels {
							nextCh := chConfig
							if len(chConfig.Users) > 0 {
								var additions []string
								for _, u := range chConfig.Users {
									trimmed := strings.TrimSpace(u)
									if res, ok := resolvedMap[trimmed]; ok && res.Resolved && res.ID != "" {
										additions = append(additions, res.ID)
									}
								}
								nextCh.Users = mergeAllowlist(chConfig.Users, additions)
							}
							nextChannels[chKey] = nextCh
						}
						nextGuild.Channels = nextChannels
					}

					nextGuilds[guildKey] = nextGuild
				}
				monCtx.GuildConfigs = nextGuilds
				summarizeResolveMapping(logger, "discord channel users", mapping, unresolvedEntries)
			}
		}
	}
}

// mergeGuildEntry 合并两个 guild entry（src 优先填充空值，dst 已有值保留）
func mergeGuildEntry(src, dst DiscordGuildEntryResolved) DiscordGuildEntryResolved {
	result := dst
	if result.ID == "" {
		result.ID = src.ID
	}
	if result.Slug == "" {
		result.Slug = src.Slug
	}
	if result.RequireMention == nil {
		result.RequireMention = src.RequireMention
	}
	if result.ReactionNotification == "" {
		result.ReactionNotification = src.ReactionNotification
	}
	if len(result.Users) == 0 {
		result.Users = src.Users
	}
	if result.Allow == nil {
		result.Allow = src.Allow
	}
	if len(result.Skills) == 0 {
		result.Skills = src.Skills
	}
	if result.Enabled == nil {
		result.Enabled = src.Enabled
	}
	if result.SystemPrompt == "" {
		result.SystemPrompt = src.SystemPrompt
	}
	if result.IncludeThreadStarter == nil {
		result.IncludeThreadStarter = src.IncludeThreadStarter
	}
	if result.AutoThread == nil {
		result.AutoThread = src.AutoThread
	}
	return result
}

// mergeChannelEntry 合并两个 channel entry（src 为基础，dst 优先覆盖）
func mergeChannelEntry(src, dst DiscordChannelEntryResolved) DiscordChannelEntryResolved {
	result := src
	if dst.Allow != nil {
		result.Allow = dst.Allow
	}
	if dst.RequireMention != nil {
		result.RequireMention = dst.RequireMention
	}
	if len(dst.Skills) > 0 {
		result.Skills = dst.Skills
	}
	if dst.Enabled != nil {
		result.Enabled = dst.Enabled
	}
	if len(dst.Users) > 0 {
		result.Users = dst.Users
	}
	if dst.SystemPrompt != "" {
		result.SystemPrompt = dst.SystemPrompt
	}
	if dst.IncludeThreadStarter != nil {
		result.IncludeThreadStarter = dst.IncludeThreadStarter
	}
	if dst.AutoThread != nil {
		result.AutoThread = dst.AutoThread
	}
	return result
}

// mergeChannelMaps 合并两个 channel map
func mergeChannelMaps(src, dst map[string]DiscordChannelEntryResolved) map[string]DiscordChannelEntryResolved {
	if len(src) == 0 && len(dst) == 0 {
		return nil
	}
	result := make(map[string]DiscordChannelEntryResolved)
	for k, v := range src {
		result[k] = v
	}
	for k, v := range dst {
		if existing, ok := result[k]; ok {
			result[k] = mergeChannelEntry(existing, v)
		} else {
			result[k] = v
		}
	}
	return result
}

// bindDiscordEventHandlers 绑定 discordgo 事件处理器。
// intentsCfg 控制特权 intent 相关处理器的注册（对齐 TS 条件逻辑）。
func bindDiscordEventHandlers(monCtx *DiscordMonitorContext, intentsCfg *types.DiscordIntentsConfig) {
	s := monCtx.Session

	// 消息创建
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil {
			return
		}
		handleDiscordMessageCreate(monCtx, m)
	})

	// 反应添加
	s.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		handleDiscordReactionAdd(monCtx, r)
	})

	// 反应移除
	s.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
		handleDiscordReactionRemove(monCtx, r)
	})

	// 交互（审批按钮等）
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleDiscordInteractionCreate(monCtx, i)
	})

	// 在线状态更新 — 仅在 GuildPresences 特权 intent 启用时注册
	// 对齐 TS: provider.ts L579-585
	if intentsCfg != nil && intentsCfg.Presence != nil && *intentsCfg.Presence {
		s.AddHandler(func(s *discordgo.Session, p *discordgo.PresenceUpdate) {
			if p.User != nil {
				monCtx.PresenceCache.Update(monCtx.AccountID, p.User.ID, PresenceData{
					Status:       string(p.Status),
					Activities:   p.Activities,
					ClientStatus: &p.ClientStatus,
				})
			}
		})
		monCtx.Logger.Info("GuildPresences intent enabled — presence listener registered")
	}

	// W-022 note: [Go 扩展] Guild 成员加入/离开 → system event
	// TS (provider.ts) 中不存在此处理器。Go 版额外添加了 GuildMemberAdd/Remove
	// 事件监听以支持 system event 推送，属于 Go 扩展功能，非 TS 对齐项。
	// 仅在 GuildMembers 特权 intent 启用时注册。
	if intentsCfg != nil && intentsCfg.GuildMembers != nil && *intentsCfg.GuildMembers {
		s.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
			handleDiscordMemberAdd(monCtx, m)
		})
		s.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
			handleDiscordMemberRemove(monCtx, m)
		})
		monCtx.Logger.Info("GuildMembers intent enabled — member listeners registered")
	}

	// W-022 note: [Go 扩展] 频道创建/删除 → system event
	// TS (provider.ts) 中不存在此处理器。Go 版额外添加了 ChannelCreate/Delete
	// 事件监听以支持 system event 推送，属于 Go 扩展功能，非 TS 对齐项。
	s.AddHandler(func(s *discordgo.Session, c *discordgo.ChannelCreate) {
		handleDiscordChannelCreate(monCtx, c)
	})
	s.AddHandler(func(s *discordgo.Session, c *discordgo.ChannelDelete) {
		handleDiscordChannelDelete(monCtx, c)
	})
}

// ── 事件处理桩（具体实现在后续文件中） ──

func handleDiscordMessageCreate(monCtx *DiscordMonitorContext, m *discordgo.MessageCreate) {
	// 自身消息跳过
	if m.Author.ID == monCtx.BotUserID {
		return
	}

	// 预处理入站消息
	msg, skipReason := PrepareDiscordInboundMessage(monCtx, m)
	if skipReason != "" {
		monCtx.Logger.Debug("message skipped",
			"reason", skipReason,
			"user", m.Author.ID,
			"channel", m.ChannelID,
		)
		return
	}

	// 检查原生命令
	if msg != nil && strings.HasPrefix(msg.Text, "/") {
		parts := strings.SplitN(msg.Text, " ", 2)
		cmd := strings.ToLower(parts[0])
		if isDiscordNativeCommand(cmd) {
			HandleDiscordNativeCommand(monCtx, msg, m)
			return
		}
	}

	// 分发到 agent 管线
	if msg != nil {
		go DispatchDiscordInbound(context.Background(), monCtx, msg, m)
	}
}

func handleDiscordReactionAdd(monCtx *DiscordMonitorContext, r *discordgo.MessageReactionAdd) {
	if r.UserID == monCtx.BotUserID {
		return
	}
	enqueueDiscordSystemEvent(monCtx, fmt.Sprintf(
		"Reaction added: %s by user %s on message %s",
		FormatDiscordReactionEmoji(r.Emoji.ID, r.Emoji.Name),
		r.UserID, r.MessageID,
	), r.ChannelID)
}

func handleDiscordReactionRemove(monCtx *DiscordMonitorContext, r *discordgo.MessageReactionRemove) {
	if r.UserID == monCtx.BotUserID {
		return
	}
	enqueueDiscordSystemEvent(monCtx, fmt.Sprintf(
		"Reaction removed: %s by user %s on message %s",
		FormatDiscordReactionEmoji(r.Emoji.ID, r.Emoji.Name),
		r.UserID, r.MessageID,
	), r.ChannelID)
}

func handleDiscordInteractionCreate(monCtx *DiscordMonitorContext, i *discordgo.InteractionCreate) {
	HandleDiscordInteraction(monCtx, i)
}

// W-022 note: [Go 扩展] TS 中不存在 GuildMemberAdd 处理，此为 Go 扩展功能。
func handleDiscordMemberAdd(monCtx *DiscordMonitorContext, m *discordgo.GuildMemberAdd) {
	name := ""
	if m.User != nil {
		name = m.User.Username
	}
	enqueueDiscordSystemEvent(monCtx, fmt.Sprintf(
		"Member joined guild %s: %s", m.GuildID, name,
	), "guild:"+m.GuildID)
}

// W-022 note: [Go 扩展] TS 中不存在 GuildMemberRemove 处理，此为 Go 扩展功能。
func handleDiscordMemberRemove(monCtx *DiscordMonitorContext, m *discordgo.GuildMemberRemove) {
	name := ""
	if m.User != nil {
		name = m.User.Username
	}
	enqueueDiscordSystemEvent(monCtx, fmt.Sprintf(
		"Member left guild %s: %s", m.GuildID, name,
	), "guild:"+m.GuildID)
}

// W-022 note: [Go 扩展] TS 中不存在 ChannelCreate 处理，此为 Go 扩展功能。
func handleDiscordChannelCreate(monCtx *DiscordMonitorContext, c *discordgo.ChannelCreate) {
	enqueueDiscordSystemEvent(monCtx, fmt.Sprintf(
		"Channel created: #%s (%s)", c.Name, c.ID,
	), "guild:"+c.GuildID)
}

// W-022 note: [Go 扩展] TS 中不存在 ChannelDelete 处理，此为 Go 扩展功能。
func handleDiscordChannelDelete(monCtx *DiscordMonitorContext, c *discordgo.ChannelDelete) {
	enqueueDiscordSystemEvent(monCtx, fmt.Sprintf(
		"Channel deleted: #%s (%s)", c.Name, c.ID,
	), "guild:"+c.GuildID)
}

// enqueueDiscordSystemEvent 入队系统事件到 DI。
func enqueueDiscordSystemEvent(monCtx *DiscordMonitorContext, text, contextKey string) {
	if monCtx.Deps == nil || monCtx.Deps.EnqueueSystemEvent == nil {
		return
	}
	// sessionKey 为空时使用 contextKey
	_ = monCtx.Deps.EnqueueSystemEvent(text, "", contextKey)
}
