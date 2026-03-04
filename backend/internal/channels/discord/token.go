package discord

import (
	"os"
	"regexp"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Discord Token 解析 — 继承自 src/discord/token.ts (52L)

// DiscordTokenSource Token 来源
type DiscordTokenSource string

const (
	TokenSourceEnv    DiscordTokenSource = "env"
	TokenSourceConfig DiscordTokenSource = "config"
	TokenSourceNone   DiscordTokenSource = "none"
)

// DiscordTokenResolution 解析后的 Token 结果
type DiscordTokenResolution struct {
	Token  string
	Source DiscordTokenSource
}

// botPrefixRe 匹配 "Bot " 前缀（不区分大小写）
var botPrefixRe = regexp.MustCompile(`(?i)^Bot\s+`)

// NormalizeDiscordToken 规范化 Discord token 字符串：去除首尾空白并移除 "Bot " 前缀。
// 返回空字符串表示 token 无效。
func NormalizeDiscordToken(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return botPrefixRe.ReplaceAllString(trimmed, "")
}

// ResolveDiscordToken 解析 Discord Bot Token。
// 优先级：账户级 config > 根级 config > 环境变量 DISCORD_BOT_TOKEN
// 仅 default 账户允许从环境变量和根级 config 读取。
func ResolveDiscordToken(cfg *types.OpenAcosmiConfig, opts ...DiscordTokenResolveOpt) DiscordTokenResolution {
	var o discordTokenResolveOptions
	for _, fn := range opts {
		fn(&o)
	}

	accountID := NormalizeAccountID(o.accountID)
	isDefault := accountID == defaultAccountID

	// 1. 账户级 token
	var accountToken string
	if cfg != nil && cfg.Channels != nil && cfg.Channels.Discord != nil {
		accounts := cfg.Channels.Discord.Accounts
		if acct, ok := accounts[accountID]; ok && acct != nil {
			accountToken = NormalizeDiscordToken(acct.DiscordAccountConfigToken())
		}
	}
	if accountToken != "" {
		return DiscordTokenResolution{Token: accountToken, Source: TokenSourceConfig}
	}

	// 2. 根级 token（仅 default 账户）
	if isDefault && cfg != nil && cfg.Channels != nil && cfg.Channels.Discord != nil {
		configToken := NormalizeDiscordToken(cfg.Channels.Discord.DiscordAccountConfigToken())
		if configToken != "" {
			return DiscordTokenResolution{Token: configToken, Source: TokenSourceConfig}
		}
	}

	// 3. 环境变量（仅 default 账户）
	if isDefault {
		envRaw := o.envToken
		if envRaw == "" {
			envRaw = os.Getenv("DISCORD_BOT_TOKEN")
		}
		envToken := NormalizeDiscordToken(envRaw)
		if envToken != "" {
			return DiscordTokenResolution{Token: envToken, Source: TokenSourceEnv}
		}
	}

	return DiscordTokenResolution{Token: "", Source: TokenSourceNone}
}

// --- 选项模式 ---

type discordTokenResolveOptions struct {
	accountID string
	envToken  string
}

// DiscordTokenResolveOpt token 解析选项
type DiscordTokenResolveOpt func(*discordTokenResolveOptions)

// WithAccountID 指定账户 ID
func WithAccountID(id string) DiscordTokenResolveOpt {
	return func(o *discordTokenResolveOptions) {
		o.accountID = id
	}
}

// WithEnvToken 指定环境变量 token（测试用）
func WithEnvToken(token string) DiscordTokenResolveOpt {
	return func(o *discordTokenResolveOptions) {
		o.envToken = token
	}
}
