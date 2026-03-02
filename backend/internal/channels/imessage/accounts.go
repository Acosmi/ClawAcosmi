//go:build darwin

package imessage

import (
	"sort"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// iMessage 账户解析 — 继承自 src/imessage/accounts.ts (91L)

// defaultAccountID 默认账户 ID（与 channels.DefaultAccountID 一致，避免循环导入）
const defaultAccountID = "default"

// ResolvedIMessageAccount 解析后的 iMessage 账户配置
type ResolvedIMessageAccount struct {
	AccountID  string
	Enabled    bool
	Name       string
	Config     types.IMessageAccountConfig
	Configured bool
}

// NormalizeAccountID 规范化账户 ID
func NormalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAccountID
	}
	return strings.ToLower(trimmed)
}

// listConfiguredAccountIds 获取配置中定义的 iMessage 账户 ID 列表
func listConfiguredAccountIds(cfg *types.OpenAcosmiConfig) []string {
	if cfg.Channels == nil || cfg.Channels.IMessage == nil {
		return nil
	}
	accounts := cfg.Channels.IMessage.Accounts
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

// ListIMessageAccountIds 列出所有 iMessage 账户 ID（无配置时返回 [default]）
func ListIMessageAccountIds(cfg *types.OpenAcosmiConfig) []string {
	ids := listConfiguredAccountIds(cfg)
	if len(ids) == 0 {
		return []string{defaultAccountID}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultIMessageAccountId 解析默认 iMessage 账户 ID
func ResolveDefaultIMessageAccountId(cfg *types.OpenAcosmiConfig) string {
	ids := ListIMessageAccountIds(cfg)
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
func resolveAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) *types.IMessageAccountConfig {
	if cfg.Channels == nil || cfg.Channels.IMessage == nil {
		return nil
	}
	accounts := cfg.Channels.IMessage.Accounts
	if len(accounts) == 0 {
		return nil
	}
	return accounts[accountID]
}

// mergeIMessageAccountConfig 合并根级 + 账户级 iMessage 配置
// TS 原版：将 accounts 字段排除后的根级配置作为 base，与账户级配置合并
func mergeIMessageAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) types.IMessageAccountConfig {
	var base types.IMessageAccountConfig
	if cfg.Channels != nil && cfg.Channels.IMessage != nil {
		base = cfg.Channels.IMessage.IMessageAccountConfig
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
	if len(acct.Capabilities) > 0 {
		merged.Capabilities = acct.Capabilities
	}
	if acct.Markdown != nil {
		merged.Markdown = acct.Markdown
	}
	if acct.ConfigWrites != nil {
		merged.ConfigWrites = acct.ConfigWrites
	}
	if acct.Enabled != nil {
		merged.Enabled = acct.Enabled
	}
	if acct.CliPath != "" {
		merged.CliPath = acct.CliPath
	}
	if acct.DbPath != "" {
		merged.DbPath = acct.DbPath
	}
	if acct.RemoteHost != "" {
		merged.RemoteHost = acct.RemoteHost
	}
	if acct.Service != "" {
		merged.Service = acct.Service
	}
	if acct.Region != "" {
		merged.Region = acct.Region
	}
	if acct.DmPolicy != "" {
		merged.DmPolicy = acct.DmPolicy
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
	if acct.IncludeAttachments != nil {
		merged.IncludeAttachments = acct.IncludeAttachments
	}
	if acct.MediaMaxMB != nil {
		merged.MediaMaxMB = acct.MediaMaxMB
	}
	if acct.ProbeTimeoutMs != nil {
		merged.ProbeTimeoutMs = acct.ProbeTimeoutMs
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
	if len(acct.Groups) > 0 {
		merged.Groups = acct.Groups
	}
	if acct.Heartbeat != nil {
		merged.Heartbeat = acct.Heartbeat
	}
	if acct.ResponsePrefix != "" {
		merged.ResponsePrefix = acct.ResponsePrefix
	}

	return merged
}

// ResolveIMessageAccount 解析完整的 iMessage 账户配置
func ResolveIMessageAccount(cfg *types.OpenAcosmiConfig, accountID string) ResolvedIMessageAccount {
	id := NormalizeAccountID(accountID)

	// 根级 enabled
	baseEnabled := true
	if cfg.Channels != nil && cfg.Channels.IMessage != nil && cfg.Channels.IMessage.Enabled != nil {
		baseEnabled = *cfg.Channels.IMessage.Enabled
	}

	merged := mergeIMessageAccountConfig(cfg, id)
	accountEnabled := true
	if merged.Enabled != nil {
		accountEnabled = *merged.Enabled
	}

	// 判断是否已配置
	configured := strings.TrimSpace(merged.CliPath) != "" ||
		strings.TrimSpace(merged.DbPath) != "" ||
		merged.Service != "" ||
		strings.TrimSpace(merged.Region) != "" ||
		len(merged.AllowFrom) > 0 ||
		len(merged.GroupAllowFrom) > 0 ||
		merged.DmPolicy != "" ||
		merged.GroupPolicy != "" ||
		merged.IncludeAttachments != nil ||
		merged.MediaMaxMB != nil ||
		merged.TextChunkLimit != nil ||
		len(merged.Groups) > 0

	name := ""
	if n := strings.TrimSpace(merged.Name); n != "" {
		name = n
	}

	return ResolvedIMessageAccount{
		AccountID:  id,
		Enabled:    baseEnabled && accountEnabled,
		Name:       name,
		Config:     merged,
		Configured: configured,
	}
}

// ListEnabledIMessageAccounts 列出所有启用的 iMessage 账户
func ListEnabledIMessageAccounts(cfg *types.OpenAcosmiConfig) []ResolvedIMessageAccount {
	var accounts []ResolvedIMessageAccount
	for _, id := range ListIMessageAccountIds(cfg) {
		acct := ResolveIMessageAccount(cfg, id)
		if acct.Enabled {
			accounts = append(accounts, acct)
		}
	}
	return accounts
}
