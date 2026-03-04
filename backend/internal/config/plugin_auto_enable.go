package config

// 插件自动启用模块 — 对应 src/config/plugin-auto-enable.ts (456行)
//
// 根据配置中已存在的频道凭据和 Provider 配置，自动发现并启用对应插件。
// 例如：发现 channels.telegram.botToken 已配置 → 自动启用 telegram 插件。

import (
	"os"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// PluginEnableChange 记录一个插件自动启用变更
type PluginEnableChange struct {
	PluginID string
	Reason   string
}

// PluginAutoEnableResult 自动启用结果
type PluginAutoEnableResult struct {
	Config  *types.OpenAcosmiConfig
	Changes []string
}

// channelPluginIDs 已知频道插件 ID 列表
// 对应 TS plugin-auto-enable.ts:L24-L29 (CHANNEL_PLUGIN_IDS)
// Go 版硬编码，待 Phase 5 频道注册表实现后对接
var channelPluginIDs = []string{
	"telegram", "discord", "slack", "whatsapp", "signal", "imessage",
	"googlechat", "msteams",
}

// providerPluginMappings Provider → Plugin 映射
// 对应 TS plugin-auto-enable.ts:L31-L37 (PROVIDER_PLUGIN_IDS)
var providerPluginMappings = []struct {
	PluginID   string
	ProviderID string
}{
	{"google-antigravity-auth", "google-antigravity"},
	{"google-gemini-cli-auth", "google-gemini-cli"},
	{"qwen-portal-auth", "qwen-portal"},
	{"copilot-proxy", "copilot-proxy"},
	{"minimax-portal-auth", "minimax-portal"},
}

// EnvLookup 环境变量查询函数签名
type EnvLookup func(key string) string

// pluginEnvLookup 默认使用 os.Getenv
func pluginEnvLookup(key string) string {
	return os.Getenv(key)
}

// ---------- 工具函数 ----------

func hasNonEmptyString(value interface{}) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) != ""
}

func isMapValue(value interface{}) bool {
	if value == nil {
		return false
	}
	_, ok := value.(map[string]interface{})
	return ok
}

func mapHasKeys(value interface{}) bool {
	m, ok := value.(map[string]interface{})
	return ok && len(m) > 0
}

func accountsHaveKeys(value interface{}, keys []string) bool {
	accounts, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	for _, account := range accounts {
		acctMap, ok := account.(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range keys {
			if hasNonEmptyString(acctMap[key]) {
				return true
			}
		}
	}
	return false
}

// ---------- 频道检测函数 ----------

// resolveChannelConfigMap 从 Channels 中获取指定频道的 map 表示
// 因为 Go 是强类型 struct，需要按频道 ID switch
func resolveChannelConfigMap(cfg *types.OpenAcosmiConfig, channelID string) map[string]interface{} {
	if cfg.Channels == nil {
		return nil
	}
	// 直接检查对应频道是否有配置即可（非 nil ≈ 有配置）
	// 返回 nil 表示无配置
	switch channelID {
	case "telegram":
		if cfg.Channels.Telegram != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "discord":
		if cfg.Channels.Discord != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "slack":
		if cfg.Channels.Slack != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "whatsapp":
		if cfg.Channels.WhatsApp != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "signal":
		if cfg.Channels.Signal != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "imessage":
		if cfg.Channels.IMessage != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "googlechat":
		if cfg.Channels.GoogleChat != nil {
			return map[string]interface{}{"_configured": true}
		}
	case "msteams":
		if cfg.Channels.MSTeams != nil {
			return map[string]interface{}{"_configured": true}
		}
	}
	return nil
}

func isTelegramConfigured(cfg *types.OpenAcosmiConfig, env EnvLookup) bool {
	if strings.TrimSpace(env("TELEGRAM_BOT_TOKEN")) != "" {
		return true
	}
	if cfg.Channels == nil || cfg.Channels.Telegram == nil {
		return false
	}
	tg := cfg.Channels.Telegram
	if tg.BotToken != "" || tg.TokenFile != "" {
		return true
	}
	// 检查 accounts 子对象 — 对应 TS L88
	for _, acct := range tg.Accounts {
		if acct != nil && (acct.BotToken != "" || acct.TokenFile != "") {
			return true
		}
	}
	// Telegram config 存在即视为 configured（对应 TS recordHasKeys）
	return true
}

func isDiscordConfigured(cfg *types.OpenAcosmiConfig, env EnvLookup) bool {
	if strings.TrimSpace(env("DISCORD_BOT_TOKEN")) != "" {
		return true
	}
	if cfg.Channels == nil || cfg.Channels.Discord == nil {
		return false
	}
	dc := cfg.Channels.Discord
	if dc.Token != nil && *dc.Token != "" {
		return true
	}
	// 检查 accounts 子对象 — 对应 TS L105
	for _, acct := range dc.Accounts {
		if acct != nil && acct.Token != nil && *acct.Token != "" {
			return true
		}
	}
	return true // config exists
}

func isSlackConfigured(cfg *types.OpenAcosmiConfig, env EnvLookup) bool {
	if strings.TrimSpace(env("SLACK_BOT_TOKEN")) != "" ||
		strings.TrimSpace(env("SLACK_APP_TOKEN")) != "" ||
		strings.TrimSpace(env("SLACK_USER_TOKEN")) != "" {
		return true
	}
	if cfg.Channels == nil || cfg.Channels.Slack == nil {
		return false
	}
	return true
}

func isSignalConfigured(cfg *types.OpenAcosmiConfig) bool {
	if cfg.Channels == nil || cfg.Channels.Signal == nil {
		return false
	}
	return true
}

func isIMessageConfigured(cfg *types.OpenAcosmiConfig) bool {
	if cfg.Channels == nil || cfg.Channels.IMessage == nil {
		return false
	}
	return true
}

func isWhatsAppConfigured(cfg *types.OpenAcosmiConfig) bool {
	if cfg.Channels == nil || cfg.Channels.WhatsApp == nil {
		return false
	}
	return true
}

func isGenericChannelConfigured(cfg *types.OpenAcosmiConfig, channelID string) bool {
	return resolveChannelConfigMap(cfg, channelID) != nil
}

// IsChannelConfigured 判断指定频道是否已配置
// 对应 TS plugin-auto-enable.ts:L183-L204
func IsChannelConfigured(cfg *types.OpenAcosmiConfig, channelID string, env EnvLookup) bool {
	if env == nil {
		env = pluginEnvLookup
	}
	switch channelID {
	case "telegram":
		return isTelegramConfigured(cfg, env)
	case "discord":
		return isDiscordConfigured(cfg, env)
	case "slack":
		return isSlackConfigured(cfg, env)
	case "whatsapp":
		return isWhatsAppConfigured(cfg)
	case "signal":
		return isSignalConfigured(cfg)
	case "imessage":
		return isIMessageConfigured(cfg)
	default:
		return isGenericChannelConfigured(cfg, channelID)
	}
}

// ---------- Provider 检测 ----------

// normalizeProviderID 规范化 provider ID
func normalizeProviderID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// collectModelRefs 收集配置中所有模型引用
// 对应 TS plugin-auto-enable.ts:L206-L249
func collectModelRefs(cfg *types.OpenAcosmiConfig) []string {
	var refs []string
	pushRef := func(v string) {
		if s := strings.TrimSpace(v); s != "" {
			refs = append(refs, s)
		}
	}

	if cfg.Agents == nil {
		return refs
	}

	// 从 defaults 收集
	if d := cfg.Agents.Defaults; d != nil {
		if d.Model != nil {
			pushRef(d.Model.Primary)
			if d.Model.Fallbacks != nil {
				for _, f := range *d.Model.Fallbacks {
					pushRef(f)
				}
			}
		}
		for key := range d.Models {
			pushRef(key)
		}
	}

	// 从 agents list 收集 — agents list 是附加在 agent 中的
	// Go 没有 agents.list 概念（在 AgentsConfig 中没有 List 字段）
	// 跳过

	return refs
}

// extractProviderFromModelRef 从模型引用中提取 provider
func extractProviderFromModelRef(ref string) string {
	trimmed := strings.TrimSpace(ref)
	idx := strings.Index(trimmed, "/")
	if idx <= 0 {
		return ""
	}
	return normalizeProviderID(trimmed[:idx])
}

// isProviderConfigured 判断指定 provider 是否已配置
// 对应 TS plugin-auto-enable.ts:L260-L294
func isProviderConfigured(cfg *types.OpenAcosmiConfig, providerID string) bool {
	normalized := normalizeProviderID(providerID)

	// 检查 auth profiles
	if cfg.Auth != nil && cfg.Auth.Profiles != nil {
		for _, profile := range cfg.Auth.Profiles {
			if profile == nil {
				continue
			}
			if normalizeProviderID(profile.Provider) == normalized {
				return true
			}
		}
	}

	// 检查 models.providers
	if cfg.Models != nil && cfg.Models.Providers != nil {
		for key := range cfg.Models.Providers {
			if normalizeProviderID(key) == normalized {
				return true
			}
		}
	}

	// 检查模型引用
	for _, ref := range collectModelRefs(cfg) {
		provider := extractProviderFromModelRef(ref)
		if provider == normalized {
			return true
		}
	}

	return false
}

// ---------- 插件状态检查 ----------

func isPluginExplicitlyDisabled(cfg *types.OpenAcosmiConfig, pluginID string) bool {
	if cfg.Plugins == nil || cfg.Plugins.Entries == nil {
		return false
	}
	entry := cfg.Plugins.Entries[pluginID]
	if entry == nil || entry.Enabled == nil {
		return false
	}
	return !*entry.Enabled
}

func isPluginDenied(cfg *types.OpenAcosmiConfig, pluginID string) bool {
	if cfg.Plugins == nil {
		return false
	}
	for _, denied := range cfg.Plugins.Deny {
		if denied == pluginID {
			return true
		}
	}
	return false
}

// ---------- 主函数 ----------

// resolveConfiguredPlugins 发现所有已配置的频道和 Provider 插件
func resolveConfiguredPlugins(cfg *types.OpenAcosmiConfig, env EnvLookup) []PluginEnableChange {
	var changes []PluginEnableChange

	// 收集所有频道 ID（硬编码列表）
	// B3 注: TS 版从 cfg.channels 对象的 keys 动态发现自定义频道。
	// Go 版使用硬编码列表覆盖所有已知频道类型。
	// Phase 5 Extra 字段 (types_channels.go) 已实现，自定义频道通过插件注册表发现。
	channelIDs := make(map[string]bool)
	for _, id := range channelPluginIDs {
		channelIDs[id] = true
	}

	for channelID := range channelIDs {
		if IsChannelConfigured(cfg, channelID, env) {
			changes = append(changes, PluginEnableChange{
				PluginID: channelID,
				Reason:   channelID + " configured",
			})
		}
	}

	// 检查 Provider 插件
	for _, mapping := range providerPluginMappings {
		if isProviderConfigured(cfg, mapping.ProviderID) {
			changes = append(changes, PluginEnableChange{
				PluginID: mapping.PluginID,
				Reason:   mapping.ProviderID + " auth configured",
			})
		}
	}

	return changes
}

// ensureAllowlisted 确保插件在 allow 列表中
func ensureAllowlisted(cfg *types.OpenAcosmiConfig, pluginID string) {
	if cfg.Plugins == nil {
		return
	}
	allow := cfg.Plugins.Allow
	if len(allow) == 0 {
		return // 没有 allow list → 不需要添加
	}
	for _, id := range allow {
		if id == pluginID {
			return // 已在列表中
		}
	}
	cfg.Plugins.Allow = append(cfg.Plugins.Allow, pluginID)
}

// registerPluginEntry 注册插件条目（enabled = false，即"已发现但未启用"）
func registerPluginEntry(cfg *types.OpenAcosmiConfig, pluginID string) {
	if cfg.Plugins == nil {
		cfg.Plugins = &types.PluginsConfig{}
	}
	if cfg.Plugins.Entries == nil {
		cfg.Plugins.Entries = make(map[string]*types.PluginEntryConfig)
	}
	enabled := false
	existing := cfg.Plugins.Entries[pluginID]
	if existing != nil {
		existing.Enabled = &enabled
	} else {
		cfg.Plugins.Entries[pluginID] = &types.PluginEntryConfig{
			Enabled: &enabled,
		}
	}
}

// ---------- B1 修复: 插件偏好冲突检测 ----------

// channelPreferOver 硬编码的频道偏好映射
// 对应 TS plugin-auto-enable.ts:L343-L350 中的 getChatChannelMeta().preferOver
// 注: TS 核心频道同样无 preferOver 值，当前空 map 行为正确。
// 未来如需频道优先级覆盖，可在此 map 中添加条目。
var channelPreferOver = map[string][]string{}

// shouldSkipPreferredPluginAutoEnable 检查是否应跳过插件启用（因更高优先级插件存在）
// 对应 TS plugin-auto-enable.ts:L352-L373
func shouldSkipPreferredPluginAutoEnable(
	cfg *types.OpenAcosmiConfig,
	entry PluginEnableChange,
	configured []PluginEnableChange,
) bool {
	for _, other := range configured {
		if other.PluginID == entry.PluginID {
			continue
		}
		if isPluginDenied(cfg, other.PluginID) {
			continue
		}
		if isPluginExplicitlyDisabled(cfg, other.PluginID) {
			continue
		}
		preferOver := channelPreferOver[other.PluginID]
		for _, preferred := range preferOver {
			if preferred == entry.PluginID {
				return true
			}
		}
	}
	return false
}

// ApplyPluginAutoEnable 自动发现并启用已配置的插件
// 对应 TS plugin-auto-enable.ts:L416-L455
func ApplyPluginAutoEnable(cfg *types.OpenAcosmiConfig, env EnvLookup) *PluginAutoEnableResult {
	if env == nil {
		env = pluginEnvLookup
	}

	configured := resolveConfiguredPlugins(cfg, env)
	if len(configured) == 0 {
		return &PluginAutoEnableResult{Config: cfg, Changes: nil}
	}

	// 如果插件系统全局禁用则跳过
	if cfg.Plugins != nil && cfg.Plugins.Enabled != nil && !*cfg.Plugins.Enabled {
		return &PluginAutoEnableResult{Config: cfg, Changes: nil}
	}

	var changes []string
	for _, entry := range configured {
		if isPluginDenied(cfg, entry.PluginID) {
			continue
		}
		if isPluginExplicitlyDisabled(cfg, entry.PluginID) {
			continue
		}
		// B1 修复: 检查偏好冲突
		if shouldSkipPreferredPluginAutoEnable(cfg, entry, configured) {
			continue
		}

		// 检查是否已启用
		alreadyEnabled := false
		if cfg.Plugins != nil && cfg.Plugins.Entries != nil {
			if e := cfg.Plugins.Entries[entry.PluginID]; e != nil && e.Enabled != nil && *e.Enabled {
				alreadyEnabled = true
			}
		}

		// 检查是否缺少 allowlist
		allowMissing := false
		if cfg.Plugins != nil && len(cfg.Plugins.Allow) > 0 {
			found := false
			for _, id := range cfg.Plugins.Allow {
				if id == entry.PluginID {
					found = true
					break
				}
			}
			allowMissing = !found
		}

		if alreadyEnabled && !allowMissing {
			continue
		}

		registerPluginEntry(cfg, entry.PluginID)
		ensureAllowlisted(cfg, entry.PluginID)
		changes = append(changes, entry.Reason+", not enabled yet.")
	}

	return &PluginAutoEnableResult{Config: cfg, Changes: changes}
}
