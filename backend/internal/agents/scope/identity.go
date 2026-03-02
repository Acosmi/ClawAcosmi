package scope

import (
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- Agent 身份 ----------

// TS 参考: src/agents/identity.ts (146 行)

const DefaultAckReaction = "👀"

// ResolveAgentIdentity 获取 Agent 的身份配置。
func ResolveAgentIdentity(cfg *types.OpenAcosmiConfig, agentId string) *types.IdentityConfig {
	ac := ResolveAgentConfig(cfg, agentId)
	if ac == nil {
		return nil
	}
	return ac.Identity
}

// ResolveAckReaction 解析确认反应 emoji。
func ResolveAckReaction(cfg *types.OpenAcosmiConfig, agentId string) string {
	if cfg != nil && cfg.Messages != nil {
		if configured := cfg.Messages.AckReaction; configured != "" {
			return strings.TrimSpace(configured)
		}
	}
	identity := ResolveAgentIdentity(cfg, agentId)
	if identity != nil {
		emoji := strings.TrimSpace(identity.Emoji)
		if emoji != "" {
			return emoji
		}
	}
	return DefaultAckReaction
}

// ResolveIdentityNamePrefix 获取身份名称前缀 "[Name]"。
func ResolveIdentityNamePrefix(cfg *types.OpenAcosmiConfig, agentId string) string {
	identity := ResolveAgentIdentity(cfg, agentId)
	if identity == nil {
		return ""
	}
	name := strings.TrimSpace(identity.Name)
	if name == "" {
		return ""
	}
	return "[" + name + "]"
}

// ResolveIdentityName 获取身份名称（不含括号）。
func ResolveIdentityName(cfg *types.OpenAcosmiConfig, agentId string) string {
	identity := ResolveAgentIdentity(cfg, agentId)
	if identity == nil {
		return ""
	}
	return strings.TrimSpace(identity.Name)
}

// ResolveMessagePrefix 解析消息前缀。
// 优先级: configured > cfg.messages.messagePrefix > identity name prefix > fallback
func ResolveMessagePrefix(cfg *types.OpenAcosmiConfig, agentId string, configured string, hasAllowFrom bool, fallback string) string {
	if configured != "" {
		return configured
	}
	if cfg != nil && cfg.Messages != nil && cfg.Messages.MessagePrefix != "" {
		return cfg.Messages.MessagePrefix
	}
	if hasAllowFrom {
		return ""
	}
	prefix := ResolveIdentityNamePrefix(cfg, agentId)
	if prefix != "" {
		return prefix
	}
	if fallback != "" {
		return fallback
	}
	return "[openacosmi]"
}

// ResolveResponsePrefix 解析回复前缀。
// TS 参考: identity.ts L68-107
// 4 层优先级: channel-account → channel → global → empty
func ResolveResponsePrefix(cfg *types.OpenAcosmiConfig, agentId string, channel string, accountId string) string {
	if cfg == nil {
		return ""
	}

	// L1: Channel-account level
	if channel != "" && accountId != "" {
		accountPrefix := getChannelAccountResponsePrefix(cfg, channel, accountId)
		if accountPrefix != "" {
			if accountPrefix == "auto" {
				return ResolveIdentityNamePrefix(cfg, agentId)
			}
			return accountPrefix
		}
	}

	// L2: Channel level
	if channel != "" {
		channelPrefix := getChannelResponsePrefix(cfg, channel)
		if channelPrefix != "" {
			if channelPrefix == "auto" {
				return ResolveIdentityNamePrefix(cfg, agentId)
			}
			return channelPrefix
		}
	}

	// L4: Global level
	if cfg.Messages != nil && cfg.Messages.ResponsePrefix != "" {
		if cfg.Messages.ResponsePrefix == "auto" {
			return ResolveIdentityNamePrefix(cfg, agentId)
		}
		return cfg.Messages.ResponsePrefix
	}

	return ""
}

// getChannelResponsePrefix 获取频道级 responsePrefix。
func getChannelResponsePrefix(cfg *types.OpenAcosmiConfig, channel string) string {
	if cfg.Channels == nil {
		return ""
	}
	switch strings.ToLower(channel) {
	case "whatsapp":
		if cfg.Channels.WhatsApp != nil {
			return cfg.Channels.WhatsApp.ResponsePrefix
		}
	case "telegram":
		if cfg.Channels.Telegram != nil {
			return cfg.Channels.Telegram.ResponsePrefix
		}
	case "discord":
		if cfg.Channels.Discord != nil {
			return cfg.Channels.Discord.DiscordAccountConfigResponsePrefix()
		}
	case "slack":
		if cfg.Channels.Slack != nil {
			return cfg.Channels.Slack.ResponsePrefix
		}
	case "googlechat":
		if cfg.Channels.GoogleChat != nil {
			return cfg.Channels.GoogleChat.ResponsePrefix
		}
	case "signal":
		if cfg.Channels.Signal != nil {
			return cfg.Channels.Signal.ResponsePrefix
		}
	case "imessage":
		if cfg.Channels.IMessage != nil {
			return cfg.Channels.IMessage.ResponsePrefix
		}
	case "msteams":
		if cfg.Channels.MSTeams != nil {
			return cfg.Channels.MSTeams.ResponsePrefix
		}
	}
	return ""
}

// getChannelAccountResponsePrefix 获取频道-账户级 responsePrefix。
func getChannelAccountResponsePrefix(cfg *types.OpenAcosmiConfig, channel, accountId string) string {
	if cfg.Channels == nil {
		return ""
	}
	switch strings.ToLower(channel) {
	case "whatsapp":
		if cfg.Channels.WhatsApp != nil && cfg.Channels.WhatsApp.Accounts != nil {
			if acc, ok := cfg.Channels.WhatsApp.Accounts[accountId]; ok && acc != nil {
				return acc.ResponsePrefix
			}
		}
	case "telegram":
		if cfg.Channels.Telegram != nil && cfg.Channels.Telegram.Accounts != nil {
			if acc, ok := cfg.Channels.Telegram.Accounts[accountId]; ok && acc != nil {
				return acc.ResponsePrefix
			}
		}
	case "discord":
		if cfg.Channels.Discord != nil && cfg.Channels.Discord.Accounts != nil {
			if acc, ok := cfg.Channels.Discord.Accounts[accountId]; ok && acc != nil {
				return acc.DiscordAccountConfigResponsePrefix()
			}
		}
	case "slack":
		if cfg.Channels.Slack != nil && cfg.Channels.Slack.Accounts != nil {
			if acc, ok := cfg.Channels.Slack.Accounts[accountId]; ok && acc != nil {
				return acc.ResponsePrefix
			}
		}
	}
	return ""
}

// ResolveEffectiveMessagesConfig 一次性解析消息前缀和回复前缀。
func ResolveEffectiveMessagesConfig(cfg *types.OpenAcosmiConfig, agentId string, hasAllowFrom bool, fallbackMessagePrefix string, channel string, accountId string) (messagePrefix string, responsePrefix string) {
	messagePrefix = ResolveMessagePrefix(cfg, agentId, "", hasAllowFrom, fallbackMessagePrefix)
	responsePrefix = ResolveResponsePrefix(cfg, agentId, channel, accountId)
	return
}

// ResolveHumanDelayConfig 解析仿人延迟配置。
func ResolveHumanDelayConfig(cfg *types.OpenAcosmiConfig, agentId string) *types.HumanDelayConfig {
	var defaults *types.HumanDelayConfig
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil {
		defaults = cfg.Agents.Defaults.HumanDelay
	}
	overrides := func() *types.HumanDelayConfig {
		ac := ResolveAgentConfig(cfg, agentId)
		if ac == nil {
			return nil
		}
		return ac.HumanDelay
	}()

	if defaults == nil && overrides == nil {
		return nil
	}

	result := &types.HumanDelayConfig{}
	if defaults != nil {
		result.Mode = defaults.Mode
		result.MinMs = defaults.MinMs
		result.MaxMs = defaults.MaxMs
	}
	if overrides != nil {
		if overrides.Mode != "" {
			result.Mode = overrides.Mode
		}
		if overrides.MinMs != 0 {
			result.MinMs = overrides.MinMs
		}
		if overrides.MaxMs != 0 {
			result.MaxMs = overrides.MaxMs
		}
	}
	return result
}
