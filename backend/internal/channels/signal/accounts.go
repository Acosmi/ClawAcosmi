package signal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// Signal 账户解析 — 继承自 src/signal/accounts.ts (92L)

// defaultAccountID 默认账户 ID（与 channels.DefaultAccountID 一致，避免循环导入）
const defaultAccountID = "default"

// ResolvedSignalAccount 解析后的 Signal 账户配置
type ResolvedSignalAccount struct {
	AccountID  string
	Enabled    bool
	Name       string
	BaseURL    string
	Configured bool
	Config     types.SignalAccountConfig
}

// listConfiguredAccountIds 获取配置中定义的账户 ID 列表
func listConfiguredAccountIds(cfg *types.OpenAcosmiConfig) []string {
	if cfg.Channels == nil || cfg.Channels.Signal == nil {
		return nil
	}
	accounts := cfg.Channels.Signal.Accounts
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

// ListSignalAccountIds 列出所有账户 ID（无配置时返回 [default]）
func ListSignalAccountIds(cfg *types.OpenAcosmiConfig) []string {
	ids := listConfiguredAccountIds(cfg)
	if len(ids) == 0 {
		return []string{defaultAccountID}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultSignalAccountId 解析默认账户 ID
func ResolveDefaultSignalAccountId(cfg *types.OpenAcosmiConfig) string {
	ids := ListSignalAccountIds(cfg)
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

// NormalizeAccountID 规范化账户 ID
func NormalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAccountID
	}
	return strings.ToLower(trimmed)
}

// resolveAccountConfig 获取指定账户的配置
func resolveAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) *types.SignalAccountConfig {
	if cfg.Channels == nil || cfg.Channels.Signal == nil {
		return nil
	}
	accounts := cfg.Channels.Signal.Accounts
	if len(accounts) == 0 {
		return nil
	}
	return accounts[accountID]
}

// mergeSignalAccountConfig 合并根级 + 账户级 Signal 配置
// TS 原版：将 accounts 字段排除后的根级配置作为 base，与账户级配置合并
func mergeSignalAccountConfig(cfg *types.OpenAcosmiConfig, accountID string) types.SignalAccountConfig {
	var base types.SignalAccountConfig
	if cfg.Channels != nil && cfg.Channels.Signal != nil {
		base = cfg.Channels.Signal.SignalAccountConfig
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
	if acct.Account != "" {
		merged.Account = acct.Account
	}
	if acct.HttpURL != "" {
		merged.HttpURL = acct.HttpURL
	}
	if acct.HttpHost != "" {
		merged.HttpHost = acct.HttpHost
	}
	if acct.HttpPort != nil {
		merged.HttpPort = acct.HttpPort
	}
	if acct.CliPath != "" {
		merged.CliPath = acct.CliPath
	}
	if acct.AutoStart != nil {
		merged.AutoStart = acct.AutoStart
	}
	if acct.StartupTimeoutMs != nil {
		merged.StartupTimeoutMs = acct.StartupTimeoutMs
	}
	if acct.ReceiveMode != "" {
		merged.ReceiveMode = acct.ReceiveMode
	}
	if acct.IgnoreAttachments != nil {
		merged.IgnoreAttachments = acct.IgnoreAttachments
	}
	if acct.IgnoreStories != nil {
		merged.IgnoreStories = acct.IgnoreStories
	}
	if acct.SendReadReceipts != nil {
		merged.SendReadReceipts = acct.SendReadReceipts
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
	if acct.Actions != nil {
		merged.Actions = acct.Actions
	}
	if acct.ReactionLevel != "" {
		merged.ReactionLevel = acct.ReactionLevel
	}
	if acct.Heartbeat != nil {
		merged.Heartbeat = acct.Heartbeat
	}
	if acct.ResponsePrefix != "" {
		merged.ResponsePrefix = acct.ResponsePrefix
	}

	return merged
}

// ResolveSignalAccount 解析完整的 Signal 账户配置
func ResolveSignalAccount(cfg *types.OpenAcosmiConfig, accountID string) ResolvedSignalAccount {
	id := NormalizeAccountID(accountID)

	// 根级 enabled
	baseEnabled := true
	if cfg.Channels != nil && cfg.Channels.Signal != nil && cfg.Channels.Signal.Enabled != nil {
		baseEnabled = *cfg.Channels.Signal.Enabled
	}

	merged := mergeSignalAccountConfig(cfg, id)
	accountEnabled := true
	if merged.Enabled != nil {
		accountEnabled = *merged.Enabled
	}
	enabled := baseEnabled && accountEnabled

	// 解析 baseURL
	host := strings.TrimSpace(merged.HttpHost)
	if host == "" {
		host = "127.0.0.1"
	}
	port := 8080
	if merged.HttpPort != nil {
		port = *merged.HttpPort
	}
	baseURL := strings.TrimSpace(merged.HttpURL)
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%d", host, port)
	}

	// 判断是否已配置
	configured := strings.TrimSpace(merged.Account) != "" ||
		strings.TrimSpace(merged.HttpURL) != "" ||
		strings.TrimSpace(merged.CliPath) != "" ||
		strings.TrimSpace(merged.HttpHost) != "" ||
		merged.HttpPort != nil ||
		merged.AutoStart != nil

	name := ""
	if n := strings.TrimSpace(merged.Name); n != "" {
		name = n
	}

	return ResolvedSignalAccount{
		AccountID:  id,
		Enabled:    enabled,
		Name:       name,
		BaseURL:    baseURL,
		Configured: configured,
		Config:     merged,
	}
}

// ListEnabledSignalAccounts 列出所有启用的 Signal 账户
func ListEnabledSignalAccounts(cfg *types.OpenAcosmiConfig) []ResolvedSignalAccount {
	var accounts []ResolvedSignalAccount
	for _, id := range ListSignalAccountIds(cfg) {
		acct := ResolveSignalAccount(cfg, id)
		if acct.Enabled {
			accounts = append(accounts, acct)
		}
	}
	return accounts
}
