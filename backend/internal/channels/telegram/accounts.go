package telegram

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/routing"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Telegram 账户解析 — 继承自 src/telegram/accounts.ts (140L)

// defaultAccountID 默认账户 ID
const defaultAccountID = "default"

// ResolvedTelegramAccount 解析后的 Telegram 账户配置
type ResolvedTelegramAccount struct {
	AccountID   string
	Enabled     bool
	Name        string
	Token       string
	TokenSource TelegramTokenSource
	Config      types.TelegramAccountConfig
}

// NormalizeAccountID 规范化账户 ID（小写+去空格）
func NormalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAccountID
	}
	return strings.ToLower(trimmed)
}

// listConfiguredAccountIds 获取配置中定义的 Telegram 账户 ID 列表
func listConfiguredAccountIds(cfg *types.OpenAcosmiConfig) []string {
	if cfg.Channels == nil || cfg.Channels.Telegram == nil {
		return nil
	}
	accounts := cfg.Channels.Telegram.Accounts
	if len(accounts) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var ids []string
	for key := range accounts {
		if key == "" {
			continue
		}
		normalized := NormalizeAccountID(key)
		if !seen[normalized] {
			seen[normalized] = true
			ids = append(ids, normalized)
		}
	}
	return ids
}

// ListTelegramAccountIds 列出所有 Telegram 账户 ID。
// 合并配置中定义的账户和绑定的账户（对齐 TS listTelegramAccountIds）。
// 无账户时返回 ["default"]。
func ListTelegramAccountIds(cfg *types.OpenAcosmiConfig) []string {
	configIDs := listConfiguredAccountIds(cfg)
	boundIDs := routing.ListBoundAccountIds(cfg, "telegram")

	// 合并去重
	seen := make(map[string]bool)
	var ids []string
	for _, id := range configIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range boundIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	if isTruthyEnvValue(os.Getenv("OPENACOSMI_DEBUG_TELEGRAM_ACCOUNTS")) {
		fmt.Fprintf(os.Stderr, "[telegram:accounts] listTelegramAccountIds %v\n", ids)
	}

	if len(ids) == 0 {
		return []string{defaultAccountID}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultTelegramAccountId 解析默认 Telegram 账户 ID。
// 优先使用默认 agent 绑定的账户（对齐 TS resolveDefaultTelegramAccountId）。
func ResolveDefaultTelegramAccountId(cfg *types.OpenAcosmiConfig) string {
	// 优先：默认 agent 绑定的账户
	boundDefault := routing.ResolveDefaultAgentBoundAccountId(cfg, "telegram")
	if boundDefault != "" {
		return boundDefault
	}
	// 回退：检查 "default" 是否在列表中，否则取第一个
	ids := ListTelegramAccountIds(cfg)
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

// resolveAccountConfig 获取指定账户的原始配置
func resolveAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) *types.TelegramAccountConfig {
	if cfg.Channels == nil || cfg.Channels.Telegram == nil {
		return nil
	}
	accounts := cfg.Channels.Telegram.Accounts
	if len(accounts) == 0 {
		return nil
	}
	// 直接匹配
	if acct, ok := accounts[accountID]; ok {
		return acct
	}
	// 归一化匹配
	normalized := NormalizeAccountID(accountID)
	for key, acct := range accounts {
		if NormalizeAccountID(key) == normalized {
			return acct
		}
	}
	return nil
}

// mergeTelegramAccountConfig 合并根级 + 账户级 Telegram 配置。
// TS 原版：将 accounts 字段排除后的根级配置作为 base，与账户级配置合并。
func mergeTelegramAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) types.TelegramAccountConfig {
	var base types.TelegramAccountConfig
	if cfg.Channels != nil && cfg.Channels.Telegram != nil {
		base = cfg.Channels.Telegram.TelegramAccountConfig
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
	if acct.Capabilities != nil {
		merged.Capabilities = acct.Capabilities
	}
	if acct.Markdown != nil {
		merged.Markdown = acct.Markdown
	}
	if acct.Commands != nil {
		merged.Commands = acct.Commands
	}
	if len(acct.CustomCommands) > 0 {
		merged.CustomCommands = acct.CustomCommands
	}
	if acct.ConfigWrites != nil {
		merged.ConfigWrites = acct.ConfigWrites
	}
	if acct.DmPolicy != "" {
		merged.DmPolicy = acct.DmPolicy
	}
	if acct.Enabled != nil {
		merged.Enabled = acct.Enabled
	}
	if acct.BotToken != "" {
		merged.BotToken = acct.BotToken
	}
	if acct.TokenFile != "" {
		merged.TokenFile = acct.TokenFile
	}
	if acct.ReplyToMode != "" {
		merged.ReplyToMode = acct.ReplyToMode
	}
	if len(acct.Groups) > 0 {
		merged.Groups = acct.Groups
	}
	if len(acct.AllowFrom) > 0 {
		merged.AllowFrom = acct.AllowFrom
	}
	if len(acct.GroupAllowFrom) > 0 {
		merged.GroupAllowFrom = acct.GroupAllowFrom
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
	if acct.DraftChunk != nil {
		merged.DraftChunk = acct.DraftChunk
	}
	if acct.BlockStreamingCoalesce != nil {
		merged.BlockStreamingCoalesce = acct.BlockStreamingCoalesce
	}
	if acct.StreamMode != "" {
		merged.StreamMode = acct.StreamMode
	}
	if acct.MediaMaxMB != nil {
		merged.MediaMaxMB = acct.MediaMaxMB
	}
	if acct.TimeoutSeconds != nil {
		merged.TimeoutSeconds = acct.TimeoutSeconds
	}
	if acct.Retry != nil {
		merged.Retry = acct.Retry
	}
	if acct.Network != nil {
		merged.Network = acct.Network
	}
	if acct.Proxy != "" {
		merged.Proxy = acct.Proxy
	}
	if acct.WebhookURL != "" {
		merged.WebhookURL = acct.WebhookURL
	}
	if acct.WebhookSecret != "" {
		merged.WebhookSecret = acct.WebhookSecret
	}
	if acct.WebhookPath != "" {
		merged.WebhookPath = acct.WebhookPath
	}
	if acct.Actions != nil {
		merged.Actions = acct.Actions
	}
	if acct.ReactionNotifications != "" {
		merged.ReactionNotifications = acct.ReactionNotifications
	}
	if acct.ReactionLevel != "" {
		merged.ReactionLevel = acct.ReactionLevel
	}
	if acct.Heartbeat != nil {
		merged.Heartbeat = acct.Heartbeat
	}
	if acct.LinkPreview != nil {
		merged.LinkPreview = acct.LinkPreview
	}
	if acct.ResponsePrefix != "" {
		merged.ResponsePrefix = acct.ResponsePrefix
	}

	return merged
}

// ResolveTelegramAccount 解析完整的 Telegram 账户配置。
// 如未指定 accountId 且默认账户无 token，尝试回退到配置中有 token 的账户。
func ResolveTelegramAccount(cfg *types.OpenAcosmiConfig, accountID string) ResolvedTelegramAccount {
	hasExplicit := strings.TrimSpace(accountID) != ""

	// 根级 enabled
	baseEnabled := true
	if cfg.Channels != nil && cfg.Channels.Telegram != nil && cfg.Channels.Telegram.Enabled != nil {
		baseEnabled = *cfg.Channels.Telegram.Enabled
	}

	resolve := func(id string) ResolvedTelegramAccount {
		merged := mergeTelegramAccountConfig(cfg, id)
		accountEnabled := true
		if merged.Enabled != nil {
			accountEnabled = *merged.Enabled
		}
		enabled := baseEnabled && accountEnabled

		tokenRes := ResolveTelegramToken(cfg, id)

		var name string
		if n := strings.TrimSpace(merged.Name); n != "" {
			name = n
		}

		debugTelegramAccounts("resolve", id, enabled, tokenRes.Source)

		return ResolvedTelegramAccount{
			AccountID:   id,
			Enabled:     enabled,
			Name:        name,
			Token:       tokenRes.Token,
			TokenSource: tokenRes.Source,
			Config:      merged,
		}
	}

	normalized := NormalizeAccountID(accountID)
	primary := resolve(normalized)
	if hasExplicit {
		return primary
	}
	if primary.TokenSource != TokenSourceNone {
		return primary
	}

	// 未指定 accountId 时，优先选择有 token 的配置账户
	fallbackID := ResolveDefaultTelegramAccountId(cfg)
	if fallbackID == primary.AccountID {
		return primary
	}
	fallback := resolve(fallbackID)
	if fallback.TokenSource == TokenSourceNone {
		return primary
	}
	return fallback
}

// ListEnabledTelegramAccounts 列出所有启用的 Telegram 账户
func ListEnabledTelegramAccounts(cfg *types.OpenAcosmiConfig) []ResolvedTelegramAccount {
	var accounts []ResolvedTelegramAccount
	for _, id := range ListTelegramAccountIds(cfg) {
		acct := ResolveTelegramAccount(cfg, id)
		if acct.Enabled {
			accounts = append(accounts, acct)
		}
	}
	return accounts
}

// debugTelegramAccounts 调试输出（受环境变量控制）
func debugTelegramAccounts(action, accountID string, enabled bool, tokenSource TelegramTokenSource) {
	if isTruthyEnvValue(os.Getenv("OPENACOSMI_DEBUG_TELEGRAM_ACCOUNTS")) {
		fmt.Fprintf(os.Stderr, "[telegram:accounts] %s accountId=%s enabled=%v tokenSource=%s\n",
			action, accountID, enabled, tokenSource)
	}
}
