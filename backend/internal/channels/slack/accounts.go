package slack

import (
	"os"
	"sort"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Slack 账户解析 — 继承自 src/slack/accounts.ts (134L)

// defaultAccountID 默认账户 ID
const defaultAccountID = "default"

// SlackTokenSource Token 来源
type SlackTokenSource string

const (
	TokenSourceEnv    SlackTokenSource = "env"
	TokenSourceConfig SlackTokenSource = "config"
	TokenSourceNone   SlackTokenSource = "none"
)

// ResolvedSlackAccount 解析后的 Slack 账户配置
type ResolvedSlackAccount struct {
	AccountID             string
	Enabled               bool
	Name                  string
	BotToken              string
	AppToken              string
	BotTokenSource        SlackTokenSource
	AppTokenSource        SlackTokenSource
	Config                types.SlackAccountConfig
	GroupPolicy           types.GroupPolicy
	TextChunkLimit        *int
	MediaMaxMB            *int
	ReactionNotifications types.SlackReactionNotificationMode
	ReactionAllowlist     []interface{}
	ReplyToMode           types.ReplyToMode
	ReplyToModeByChatType *types.SlackReplyToModeByChatType
	Actions               *types.SlackActionConfig
	SlashCommand          *types.SlackSlashCommandConfig
	DM                    *types.SlackDmConfig
	Channels              map[string]*types.SlackChannelConfig
}

// NormalizeAccountID 规范化账户 ID
func NormalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAccountID
	}
	return strings.ToLower(trimmed)
}

// listConfiguredAccountIds 获取配置中定义的账户 ID 列表
func listConfiguredAccountIds(cfg *types.OpenAcosmiConfig) []string {
	if cfg.Channels == nil || cfg.Channels.Slack == nil {
		return nil
	}
	accounts := cfg.Channels.Slack.Accounts
	if len(accounts) == 0 {
		return nil
	}
	var ids []string
	for id := range accounts {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// ListSlackAccountIds 列出所有账户 ID（无配置时返回 [default]）
func ListSlackAccountIds(cfg *types.OpenAcosmiConfig) []string {
	ids := listConfiguredAccountIds(cfg)
	if len(ids) == 0 {
		return []string{defaultAccountID}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultSlackAccountId 解析默认账户 ID
func ResolveDefaultSlackAccountId(cfg *types.OpenAcosmiConfig) string {
	ids := ListSlackAccountIds(cfg)
	for _, id := range ids {
		if id == defaultAccountID {
			return defaultAccountID
		}
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return defaultAccountID
}

// resolveAccountConfig 获取指定账户的配置
func resolveAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) *types.SlackAccountConfig {
	if cfg.Channels == nil || cfg.Channels.Slack == nil {
		return nil
	}
	accounts := cfg.Channels.Slack.Accounts
	if len(accounts) == 0 {
		return nil
	}
	return accounts[accountID]
}

// mergeSlackAccountConfig 合并根级 + 账户级 Slack 配置
// TS 原版：将 accounts 字段排除后的根级配置作为 base，与账户级配置合并
func mergeSlackAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) types.SlackAccountConfig {
	var base types.SlackAccountConfig
	if cfg.Channels != nil && cfg.Channels.Slack != nil {
		base = cfg.Channels.Slack.SlackAccountConfig
	}

	acct := resolveAccountConfig(cfg, accountID)
	if acct == nil {
		return base
	}

	// 账户级覆盖根级：非零值字段优先
	merged := base
	if acct.Name != "" {
		merged.Name = acct.Name
	}
	if acct.Mode != "" {
		merged.Mode = acct.Mode
	}
	if acct.SigningSecret != "" {
		merged.SigningSecret = acct.SigningSecret
	}
	if acct.WebhookPath != "" {
		merged.WebhookPath = acct.WebhookPath
	}
	if len(acct.Capabilities) > 0 {
		merged.Capabilities = acct.Capabilities
	}
	if acct.Markdown != nil {
		merged.Markdown = acct.Markdown
	}
	if acct.Commands != nil {
		merged.Commands = acct.Commands
	}
	if acct.ConfigWrites != nil {
		merged.ConfigWrites = acct.ConfigWrites
	}
	if acct.Enabled != nil {
		merged.Enabled = acct.Enabled
	}
	if acct.BotToken != "" {
		merged.BotToken = acct.BotToken
	}
	if acct.AppToken != "" {
		merged.AppToken = acct.AppToken
	}
	if acct.UserToken != "" {
		merged.UserToken = acct.UserToken
	}
	if acct.UserTokenReadOnly != nil {
		merged.UserTokenReadOnly = acct.UserTokenReadOnly
	}
	if acct.AllowBots != nil {
		merged.AllowBots = acct.AllowBots
	}
	if acct.RequireMention != nil {
		merged.RequireMention = acct.RequireMention
	}
	if acct.GroupPolicy != "" {
		merged.GroupPolicy = acct.GroupPolicy
	}
	if acct.HistoryLimit != nil {
		merged.HistoryLimit = acct.HistoryLimit
	}
	if acct.DmHistoryLimit != nil {
		merged.DmHistoryLimit = acct.DmHistoryLimit
	}
	if len(acct.Dms) > 0 {
		merged.Dms = acct.Dms
	}
	if acct.TextChunkLimit != nil {
		merged.TextChunkLimit = acct.TextChunkLimit
	}
	if acct.ChunkMode != "" {
		merged.ChunkMode = acct.ChunkMode
	}
	if acct.BlockStreaming != nil {
		merged.BlockStreaming = acct.BlockStreaming
	}
	if acct.BlockStreamingCoalesce != nil {
		merged.BlockStreamingCoalesce = acct.BlockStreamingCoalesce
	}
	if acct.MediaMaxMB != nil {
		merged.MediaMaxMB = acct.MediaMaxMB
	}
	if acct.ReactionNotifications != "" {
		merged.ReactionNotifications = acct.ReactionNotifications
	}
	if len(acct.ReactionAllowlist) > 0 {
		merged.ReactionAllowlist = acct.ReactionAllowlist
	}
	if acct.ReplyToMode != "" {
		merged.ReplyToMode = acct.ReplyToMode
	}
	if acct.ReplyToModeByChatType != nil {
		merged.ReplyToModeByChatType = acct.ReplyToModeByChatType
	}
	if acct.Thread != nil {
		merged.Thread = acct.Thread
	}
	if acct.Actions != nil {
		merged.Actions = acct.Actions
	}
	if acct.SlashCommand != nil {
		merged.SlashCommand = acct.SlashCommand
	}
	if acct.DM != nil {
		merged.DM = acct.DM
	}
	if len(acct.Channels) > 0 {
		merged.Channels = acct.Channels
	}
	if acct.Heartbeat != nil {
		merged.Heartbeat = acct.Heartbeat
	}
	if acct.ResponsePrefix != "" {
		merged.ResponsePrefix = acct.ResponsePrefix
	}

	return merged
}

// ResolveSlackAccount 解析完整的 Slack 账户配置
func ResolveSlackAccount(cfg *types.OpenAcosmiConfig, accountID string) ResolvedSlackAccount {
	id := NormalizeAccountID(accountID)

	// 根级 enabled
	baseEnabled := true
	if cfg.Channels != nil && cfg.Channels.Slack != nil && cfg.Channels.Slack.Enabled != nil {
		baseEnabled = *cfg.Channels.Slack.Enabled
	}

	merged := mergeSlackAccountConfig(cfg, id)
	accountEnabled := true
	if merged.Enabled != nil {
		accountEnabled = *merged.Enabled
	}
	enabled := baseEnabled && accountEnabled

	// Token 解析
	allowEnv := id == defaultAccountID
	var envBot, envApp string
	if allowEnv {
		envBot = ResolveSlackBotToken(os.Getenv("SLACK_BOT_TOKEN"))
		envApp = ResolveSlackAppToken(os.Getenv("SLACK_APP_TOKEN"))
	}
	configBot := ResolveSlackBotToken(merged.BotToken)
	configApp := ResolveSlackAppToken(merged.AppToken)

	botToken := configBot
	if botToken == "" {
		botToken = envBot
	}
	appToken := configApp
	if appToken == "" {
		appToken = envApp
	}

	var botTokenSource SlackTokenSource
	switch {
	case configBot != "":
		botTokenSource = TokenSourceConfig
	case envBot != "":
		botTokenSource = TokenSourceEnv
	default:
		botTokenSource = TokenSourceNone
	}

	var appTokenSource SlackTokenSource
	switch {
	case configApp != "":
		appTokenSource = TokenSourceConfig
	case envApp != "":
		appTokenSource = TokenSourceEnv
	default:
		appTokenSource = TokenSourceNone
	}

	var name string
	if n := strings.TrimSpace(merged.Name); n != "" {
		name = n
	}

	return ResolvedSlackAccount{
		AccountID:             id,
		Enabled:               enabled,
		Name:                  name,
		BotToken:              botToken,
		AppToken:              appToken,
		BotTokenSource:        botTokenSource,
		AppTokenSource:        appTokenSource,
		Config:                merged,
		GroupPolicy:           merged.GroupPolicy,
		TextChunkLimit:        merged.TextChunkLimit,
		MediaMaxMB:            merged.MediaMaxMB,
		ReactionNotifications: merged.ReactionNotifications,
		ReactionAllowlist:     merged.ReactionAllowlist,
		ReplyToMode:           merged.ReplyToMode,
		ReplyToModeByChatType: merged.ReplyToModeByChatType,
		Actions:               merged.Actions,
		SlashCommand:          merged.SlashCommand,
		DM:                    merged.DM,
		Channels:              merged.Channels,
	}
}

// ListEnabledSlackAccounts 列出所有启用的 Slack 账户
func ListEnabledSlackAccounts(cfg *types.OpenAcosmiConfig) []ResolvedSlackAccount {
	var accounts []ResolvedSlackAccount
	for _, id := range ListSlackAccountIds(cfg) {
		acct := ResolveSlackAccount(cfg, id)
		if acct.Enabled {
			accounts = append(accounts, acct)
		}
	}
	return accounts
}

// NormalizeChatType 规范化聊天类型（Slack 本地版本，避免跨包引用）
func normalizeChatType(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case "direct", "dm":
		return "direct"
	case "group":
		return "group"
	case "channel":
		return "channel"
	default:
		return ""
	}
}

// ResolveSlackReplyToMode 解析 Slack 回复线程模式（按聊天类型）
func ResolveSlackReplyToMode(account ResolvedSlackAccount, chatType string) ReplyToMode {
	normalized := normalizeChatType(chatType)

	if normalized != "" && account.ReplyToModeByChatType != nil {
		var mode types.ReplyToMode
		switch normalized {
		case "direct":
			mode = account.ReplyToModeByChatType.Direct
		case "group":
			mode = account.ReplyToModeByChatType.Group
		case "channel":
			mode = account.ReplyToModeByChatType.Channel
		}
		if mode != "" {
			return ReplyToMode(mode)
		}
	}

	if normalized == "direct" && account.DM != nil && account.DM.ReplyToMode != "" {
		return ReplyToMode(account.DM.ReplyToMode)
	}

	if account.ReplyToMode != "" {
		return ReplyToMode(account.ReplyToMode)
	}
	return ReplyToModeOff
}
