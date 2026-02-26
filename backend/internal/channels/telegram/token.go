package telegram

import (
	"os"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// Telegram Token 解析 — 继承自 src/telegram/token.ts (103L)

// TelegramTokenSource Token 来源
type TelegramTokenSource string

const (
	TokenSourceEnv       TelegramTokenSource = "env"
	TokenSourceTokenFile TelegramTokenSource = "tokenFile"
	TokenSourceConfig    TelegramTokenSource = "config"
	TokenSourceNone      TelegramTokenSource = "none"
)

// TelegramTokenResolution Token 解析结果
type TelegramTokenResolution struct {
	Token  string
	Source TelegramTokenSource
}

// ResolveTelegramToken 解析 Telegram Bot Token。
// 按优先级：账户 tokenFile → 账户 botToken → 根 tokenFile → 根 botToken → 环境变量。
// 非 default 账户不允许使用根级/环境变量 token。
func ResolveTelegramToken(cfg *types.OpenAcosmiConfig, accountID string) TelegramTokenResolution {
	id := NormalizeAccountID(accountID)
	var telegramCfg *types.TelegramConfig
	if cfg != nil && cfg.Channels != nil {
		telegramCfg = cfg.Channels.Telegram
	}

	// 解析账户级配置
	var accountCfg *types.TelegramAccountConfig
	if telegramCfg != nil && len(telegramCfg.Accounts) > 0 {
		// 直接匹配
		if acct, ok := telegramCfg.Accounts[id]; ok {
			accountCfg = acct
		} else {
			// 归一化匹配
			for key, acct := range telegramCfg.Accounts {
				if NormalizeAccountID(key) == id {
					accountCfg = acct
					break
				}
			}
		}
	}

	// 1. 账户级 tokenFile
	if accountCfg != nil {
		if tokenFile := strings.TrimSpace(accountCfg.TokenFile); tokenFile != "" {
			data, err := os.ReadFile(tokenFile)
			if err != nil {
				return TelegramTokenResolution{Token: "", Source: TokenSourceNone}
			}
			token := strings.TrimSpace(string(data))
			if token != "" {
				return TelegramTokenResolution{Token: token, Source: TokenSourceTokenFile}
			}
			return TelegramTokenResolution{Token: "", Source: TokenSourceNone}
		}
	}

	// 2. 账户级 botToken
	if accountCfg != nil {
		if token := strings.TrimSpace(accountCfg.BotToken); token != "" {
			return TelegramTokenResolution{Token: token, Source: TokenSourceConfig}
		}
	}

	// 以下仅限 default 账户
	allowEnv := id == defaultAccountID

	// 3. 根级 tokenFile
	if telegramCfg != nil && allowEnv {
		if tokenFile := strings.TrimSpace(telegramCfg.TokenFile); tokenFile != "" {
			data, err := os.ReadFile(tokenFile)
			if err != nil {
				return TelegramTokenResolution{Token: "", Source: TokenSourceNone}
			}
			token := strings.TrimSpace(string(data))
			if token != "" {
				return TelegramTokenResolution{Token: token, Source: TokenSourceTokenFile}
			}
		}
	}

	// 4. 根级 botToken
	if telegramCfg != nil && allowEnv {
		if token := strings.TrimSpace(telegramCfg.BotToken); token != "" {
			return TelegramTokenResolution{Token: token, Source: TokenSourceConfig}
		}
	}

	// 5. 环境变量
	if allowEnv {
		if token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")); token != "" {
			return TelegramTokenResolution{Token: token, Source: TokenSourceEnv}
		}
	}

	return TelegramTokenResolution{Token: "", Source: TokenSourceNone}
}
