package discord

import (
	"os"
	"sort"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Discord 账户解析 — 继承自 src/discord/accounts.ts (83L)

// ResolvedDiscordAccount 解析后的 Discord 账户配置
type ResolvedDiscordAccount struct {
	AccountID   string
	Enabled     bool
	Name        string
	Token       string
	TokenSource DiscordTokenSource
	Config      types.DiscordAccountConfig
}

// listConfiguredAccountIds 获取配置中定义的账户 ID 列表
func listConfiguredAccountIds(cfg *types.OpenAcosmiConfig) []string {
	if cfg == nil || cfg.Channels == nil || cfg.Channels.Discord == nil {
		return nil
	}
	accounts := cfg.Channels.Discord.Accounts
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

// ListDiscordAccountIds 列出所有账户 ID（无配置时返回 [default]）
func ListDiscordAccountIds(cfg *types.OpenAcosmiConfig) []string {
	ids := listConfiguredAccountIds(cfg)
	if len(ids) == 0 {
		return []string{defaultAccountID}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultDiscordAccountId 解析默认账户 ID
func ResolveDefaultDiscordAccountId(cfg *types.OpenAcosmiConfig) string {
	ids := ListDiscordAccountIds(cfg)
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
func resolveAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) *types.DiscordAccountConfig {
	if cfg == nil || cfg.Channels == nil || cfg.Channels.Discord == nil {
		return nil
	}
	accounts := cfg.Channels.Discord.Accounts
	if len(accounts) == 0 {
		return nil
	}
	return accounts[accountID]
}

// mergeDiscordAccountConfig 合并根级 + 账户级 Discord 配置
// TS 原版：将 accounts 字段排除后的根级配置作为 base，与账户级配置合并
func mergeDiscordAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) types.DiscordAccountConfig {
	var base types.DiscordAccountConfig
	if cfg != nil && cfg.Channels != nil && cfg.Channels.Discord != nil {
		base = cfg.Channels.Discord.DiscordAccountConfig
	}

	acct := resolveAccountConfig(cfg, accountID)
	if acct == nil {
		return base
	}

	// DY-001: 账户级覆盖根级 — 对齐 TS `{ ...base, ...account }` 展开合并。
	// 指针字段(*string/*GroupPolicy/*ReplyToMode)使用 != nil 判断，
	// 这样账户级设为空字符串也能正确覆盖根级的非空值。
	merged := base
	if acct.Name != nil {
		merged.Name = acct.Name
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
	if acct.Token != nil {
		merged.Token = acct.Token
	}
	if acct.AllowBots != nil {
		merged.AllowBots = acct.AllowBots
	}
	if acct.GroupPolicy != nil {
		merged.GroupPolicy = acct.GroupPolicy
	}
	if acct.TextChunkLimit != nil {
		merged.TextChunkLimit = acct.TextChunkLimit
	}
	if acct.ChunkMode != nil {
		merged.ChunkMode = acct.ChunkMode
	}
	if acct.BlockStreaming != nil {
		merged.BlockStreaming = acct.BlockStreaming
	}
	if acct.BlockStreamingCoalesce != nil {
		merged.BlockStreamingCoalesce = acct.BlockStreamingCoalesce
	}
	if acct.MaxLinesPerMessage != nil {
		merged.MaxLinesPerMessage = acct.MaxLinesPerMessage
	}
	if acct.MediaMaxMB != nil {
		merged.MediaMaxMB = acct.MediaMaxMB
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
	if acct.Retry != nil {
		merged.Retry = acct.Retry
	}
	if acct.Actions != nil {
		merged.Actions = acct.Actions
	}
	if acct.ReplyToMode != nil {
		merged.ReplyToMode = acct.ReplyToMode
	}
	if acct.DM != nil {
		merged.DM = acct.DM
	}
	if len(acct.Guilds) > 0 {
		merged.Guilds = acct.Guilds
	}
	if acct.Heartbeat != nil {
		merged.Heartbeat = acct.Heartbeat
	}
	if acct.ExecApprovals != nil {
		merged.ExecApprovals = acct.ExecApprovals
	}
	if acct.Intents != nil {
		merged.Intents = acct.Intents
	}
	if acct.Pluralkit != nil {
		merged.Pluralkit = acct.Pluralkit
	}
	if acct.ResponsePrefix != nil {
		merged.ResponsePrefix = acct.ResponsePrefix
	}

	return merged
}

// ResolveDiscordAccount 解析完整的 Discord 账户配置
func ResolveDiscordAccount(cfg *types.OpenAcosmiConfig, accountID string) ResolvedDiscordAccount {
	id := NormalizeAccountID(accountID)

	// 根级 enabled
	baseEnabled := true
	if cfg != nil && cfg.Channels != nil && cfg.Channels.Discord != nil && cfg.Channels.Discord.Enabled != nil {
		baseEnabled = *cfg.Channels.Discord.Enabled
	}

	merged := mergeDiscordAccountConfig(cfg, id)
	accountEnabled := true
	if merged.Enabled != nil {
		accountEnabled = *merged.Enabled
	}
	enabled := baseEnabled && accountEnabled

	// Token 解析
	tokenResolution := ResolveDiscordToken(cfg, WithAccountID(id), WithEnvToken(os.Getenv("DISCORD_BOT_TOKEN")))

	var name string
	if n := strings.TrimSpace(merged.DiscordAccountConfigName()); n != "" {
		name = n
	}

	return ResolvedDiscordAccount{
		AccountID:   id,
		Enabled:     enabled,
		Name:        name,
		Token:       tokenResolution.Token,
		TokenSource: tokenResolution.Source,
		Config:      merged,
	}
}

// ListEnabledDiscordAccounts 列出所有启用的 Discord 账户
func ListEnabledDiscordAccounts(cfg *types.OpenAcosmiConfig) []ResolvedDiscordAccount {
	var accounts []ResolvedDiscordAccount
	for _, id := range ListDiscordAccountIds(cfg) {
		acct := ResolveDiscordAccount(cfg, id)
		if acct.Enabled {
			accounts = append(accounts, acct)
		}
	}
	return accounts
}
