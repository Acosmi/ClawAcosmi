package channels

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/i18n"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Discord 引导适配器 — 继承自 src/channels/plugins/onboarding/discord.ts (495L)

// DiscordDmPolicyInfo Discord DM 策略元数据
var DiscordDmPolicyInfo = struct {
	Label        string
	PolicyKey    string
	AllowFromKey string
}{
	Label:        "Discord",
	PolicyKey:    "channels.discord.dm.policy",
	AllowFromKey: "channels.discord.dm.allowFrom",
}

// ParseDiscordAllowFromInput 解析 Discord allowFrom 输入
func ParseDiscordAllowFromInput(raw string) []string {
	var result []string
	for _, part := range regexp.MustCompile(`[\n,;]+`).Split(raw, -1) {
		t := strings.TrimSpace(part)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

// ParseDiscordAllowFromID 从用户输入中提取 Discord user ID
func ParseDiscordAllowFromID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	mention := regexp.MustCompile(`^<@!?(\d+)>$`).FindStringSubmatch(trimmed)
	if len(mention) > 1 {
		return mention[1]
	}
	prefixed := regexp.MustCompile(`(?i)^(user:|discord:)`).ReplaceAllString(trimmed, "")
	if regexp.MustCompile(`^\d+$`).MatchString(prefixed) {
		return prefixed
	}
	return ""
}

// DiscordGuildChannelEntry guild+channel 允许列表条目
type DiscordGuildChannelEntry struct {
	GuildKey   string `json:"guildKey"`
	ChannelKey string `json:"channelKey,omitempty"`
}

// ParseDiscordGuildChannelEntries 从 guild 配置恢复 entry 列表
func ParseDiscordGuildChannelEntries(guilds map[string]interface{}) []string {
	var entries []string
	for guildKey, value := range guilds {
		gm, ok := value.(map[string]interface{})
		if !ok {
			entries = append(entries, guildKey)
			continue
		}
		channels, ok := gm["channels"].(map[string]interface{})
		if !ok || len(channels) == 0 {
			entries = append(entries, guildKey)
			continue
		}
		for chKey := range channels {
			entries = append(entries, fmt.Sprintf("%s/%s", guildKey, chKey))
		}
	}
	return entries
}

// BuildDiscordOnboardingStatus 构建 Discord 引导状态
func BuildDiscordOnboardingStatus(configured bool) OnboardingStatus {
	statusStr := "needs token"
	if configured {
		statusStr = "configured"
	}
	var hint string
	score := 1
	if configured {
		hint = "configured"
		score = 2
	} else {
		hint = "needs token"
	}
	return OnboardingStatus{
		Channel:    ChannelDiscord,
		Configured: configured,
		StatusLines: []string{
			fmt.Sprintf("Discord: %s", statusStr),
		},
		SelectionHint:   hint,
		QuickstartScore: &score,
	}
}

// ---------- 交互向导 ----------

// ConfigureDiscordParams Discord 配置参数。
type ConfigureDiscordParams struct {
	Cfg       *types.OpenAcosmiConfig
	Prompter  Prompter
	AccountID string
}

// ConfigureDiscordResult Discord 配置结果。
type ConfigureDiscordResult struct {
	Cfg       *types.OpenAcosmiConfig
	AccountID string
}

// NoteDiscordTokenHelp 展示 Discord bot token 帮助文本。
// 对应 TS noteDiscordTokenHelp (discord.ts L44-55)。
func NoteDiscordTokenHelp(prompter Prompter) {
	prompter.Note(strings.Join([]string{
		"1) Discord Developer Portal → Applications → New Application",
		"2) Bot → Add Bot → Reset Token → copy token",
		"3) OAuth2 → URL Generator → scope 'bot' → invite to your server",
		"Tip: enable Message Content Intent if you need message text.",
		"Docs: https://docs.openacosmi.dev/discord",
	}, "\n"), i18n.Tp("onboard.ch.discord.title"))
}

// ConfigureDiscord 交互式 Discord 频道配置向导。
// 对应 TS discordOnboardingAdapter.configure (discord.ts L287-484)。
func ConfigureDiscord(params ConfigureDiscordParams) (*ConfigureDiscordResult, error) {
	cfg := params.Cfg
	if cfg == nil {
		cfg = &types.OpenAcosmiConfig{}
	}
	prompter := params.Prompter
	accountID := params.AccountID
	if accountID == "" {
		accountID = DefaultAccountID
	}

	// 确保 channels.discord 结构存在
	if cfg.Channels == nil {
		cfg.Channels = &types.ChannelsConfig{}
	}
	if cfg.Channels.Discord == nil {
		cfg.Channels.Discord = &types.DiscordConfig{}
	}

	enabledTrue := true
	cfg.Channels.Discord.Enabled = &enabledTrue

	// 检测已有 token
	hasConfigToken := cfg.Channels.Discord.Token != nil && *cfg.Channels.Discord.Token != ""
	envToken := strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN"))
	canUseEnv := accountID == DefaultAccountID && envToken != ""

	var token string

	if !hasConfigToken {
		NoteDiscordTokenHelp(prompter)
	}

	if canUseEnv && !hasConfigToken {
		keepEnv, err := prompter.Confirm(i18n.Tp("onboard.ch.discord.env_found"), true)
		if err != nil {
			return nil, err
		}
		if !keepEnv {
			t, err := prompter.TextInput(i18n.Tp("onboard.ch.discord.token"), "", "", func(v string) string {
				if strings.TrimSpace(v) == "" {
					return "Required"
				}
				return ""
			})
			if err != nil {
				return nil, err
			}
			token = strings.TrimSpace(t)
		}
	} else if hasConfigToken {
		keep, err := prompter.Confirm(i18n.Tp("onboard.ch.discord.keep"), true)
		if err != nil {
			return nil, err
		}
		if !keep {
			t, err := prompter.TextInput(i18n.Tp("onboard.ch.discord.token"), "", "", func(v string) string {
				if strings.TrimSpace(v) == "" {
					return "Required"
				}
				return ""
			})
			if err != nil {
				return nil, err
			}
			token = strings.TrimSpace(t)
		}
	} else {
		t, err := prompter.TextInput(i18n.Tp("onboard.ch.discord.token"), "", "", func(v string) string {
			if strings.TrimSpace(v) == "" {
				return "Required"
			}
			return ""
		})
		if err != nil {
			return nil, err
		}
		token = strings.TrimSpace(t)
	}

	// 写入 token
	if token != "" {
		if accountID == DefaultAccountID {
			cfg.Channels.Discord.Token = &token
		} else {
			if cfg.Channels.Discord.Accounts == nil {
				cfg.Channels.Discord.Accounts = make(map[string]*types.DiscordAccountConfig)
			}
			acct := cfg.Channels.Discord.Accounts[accountID]
			if acct == nil {
				acct = &types.DiscordAccountConfig{}
				acctEnabled := true
				acct.Enabled = &acctEnabled
			}
			acct.Token = &token
			cfg.Channels.Discord.Accounts[accountID] = acct
		}
	}

	// Guild/channel access 配置
	hasGuilds := cfg.Channels.Discord.Guilds != nil && len(cfg.Channels.Discord.Guilds) > 0
	currentPolicy := cfg.Channels.Discord.GroupPolicy
	defaultPolicy := types.GroupPolicy("allowlist")
	if currentPolicy == nil {
		currentPolicy = &defaultPolicy
	}

	accessConfig, err := PromptChannelAccessConfig(
		prompter, "Discord channels",
		ChannelAccessPolicy(*currentPolicy), nil,
		"My Server/#general, guildId/channelId",
		hasGuilds,
	)
	if err != nil {
		return nil, err
	}
	if accessConfig != nil {
		if accessConfig.Policy != AccessPolicyAllowlist {
			cfg = SetDiscordGroupPolicy(cfg, accountID, string(accessConfig.Policy))
		} else {
			cfg = SetDiscordGroupPolicy(cfg, accountID, "allowlist")
			var guildEntries []DiscordGuildChannelEntry
			for _, entry := range accessConfig.Entries {
				parts := strings.SplitN(entry, "/", 2)
				ge := DiscordGuildChannelEntry{GuildKey: parts[0]}
				if len(parts) > 1 {
					ge.ChannelKey = parts[1]
				}
				guildEntries = append(guildEntries, ge)
			}
			cfg = SetDiscordGuildChannelAllowlist(cfg, accountID, guildEntries)
		}
	}

	return &ConfigureDiscordResult{Cfg: cfg, AccountID: accountID}, nil
}

// ---------- 配置写入辅助 ----------

// SetDiscordDmPolicy 设置 Discord DM 策略。
func SetDiscordDmPolicy(cfg *types.OpenAcosmiConfig, policy types.DmPolicy) *types.OpenAcosmiConfig {
	ensureDiscordConfig(cfg)
	if cfg.Channels.Discord.DM == nil {
		cfg.Channels.Discord.DM = &types.DiscordDmConfig{}
	}
	if cfg.Channels.Discord.DM.Enabled == nil {
		e := true
		cfg.Channels.Discord.DM.Enabled = &e
	}
	cfg.Channels.Discord.DM.Policy = policy
	if policy == "open" {
		cfg.Channels.Discord.DM.AllowFrom = addWildcardInterface(cfg.Channels.Discord.DM.AllowFrom)
	}
	return cfg
}

// SetDiscordGroupPolicy 设置 Discord 群组策略。
func SetDiscordGroupPolicy(cfg *types.OpenAcosmiConfig, accountID string, policy string) *types.OpenAcosmiConfig {
	ensureDiscordConfig(cfg)
	gp := types.GroupPolicy(policy)
	if accountID == DefaultAccountID {
		cfg.Channels.Discord.GroupPolicy = &gp
	} else {
		if cfg.Channels.Discord.Accounts == nil {
			cfg.Channels.Discord.Accounts = make(map[string]*types.DiscordAccountConfig)
		}
		acct := cfg.Channels.Discord.Accounts[accountID]
		if acct == nil {
			acct = &types.DiscordAccountConfig{}
			e := true
			acct.Enabled = &e
		}
		acctGP := types.GroupPolicy(policy)
		acct.GroupPolicy = &acctGP
		cfg.Channels.Discord.Accounts[accountID] = acct
	}
	return cfg
}

// SetDiscordGuildChannelAllowlist 设置 Discord guild/channel 允许列表。
func SetDiscordGuildChannelAllowlist(cfg *types.OpenAcosmiConfig, accountID string, entries []DiscordGuildChannelEntry) *types.OpenAcosmiConfig {
	ensureDiscordConfig(cfg)
	guilds := make(map[string]*types.DiscordGuildEntry)
	for _, entry := range entries {
		guildKey := entry.GuildKey
		if guildKey == "" {
			guildKey = "*"
		}
		existing := guilds[guildKey]
		if existing == nil {
			existing = &types.DiscordGuildEntry{}
		}
		if entry.ChannelKey != "" {
			if existing.Channels == nil {
				existing.Channels = make(map[string]*types.DiscordGuildChannelConfig)
			}
			allow := true
			existing.Channels[entry.ChannelKey] = &types.DiscordGuildChannelConfig{Allow: &allow}
		}
		guilds[guildKey] = existing
	}
	cfg.Channels.Discord.Guilds = guilds
	return cfg
}

// SetDiscordAllowFrom 设置 Discord DM allowFrom 列表。
func SetDiscordAllowFrom(cfg *types.OpenAcosmiConfig, allowFrom []string) *types.OpenAcosmiConfig {
	ensureDiscordConfig(cfg)
	if cfg.Channels.Discord.DM == nil {
		cfg.Channels.Discord.DM = &types.DiscordDmConfig{}
	}
	e := true
	cfg.Channels.Discord.DM.Enabled = &e
	ifaces := make([]interface{}, len(allowFrom))
	for i, v := range allowFrom {
		ifaces[i] = v
	}
	cfg.Channels.Discord.DM.AllowFrom = ifaces
	return cfg
}

// DisableDiscord 禁用 Discord 频道。
func DisableDiscord(cfg *types.OpenAcosmiConfig) *types.OpenAcosmiConfig {
	if cfg.Channels != nil && cfg.Channels.Discord != nil {
		e := false
		cfg.Channels.Discord.Enabled = &e
	}
	return cfg
}

// ---------- 内部辅助 ----------

func ensureDiscordConfig(cfg *types.OpenAcosmiConfig) {
	if cfg.Channels == nil {
		cfg.Channels = &types.ChannelsConfig{}
	}
	if cfg.Channels.Discord == nil {
		cfg.Channels.Discord = &types.DiscordConfig{}
	}
	e := true
	cfg.Channels.Discord.Enabled = &e
}

func addWildcardInterface(existing []interface{}) []interface{} {
	for _, v := range existing {
		if s, ok := v.(string); ok && s == "*" {
			return existing
		}
	}
	return append(existing, "*")
}
