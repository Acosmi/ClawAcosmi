package telegram

import (
	"math"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Telegram 草稿流分块 — 继承自 src/telegram/draft-chunking.ts (42L)

const (
	defaultDraftStreamMin = 200
	defaultDraftStreamMax = 800
)

// DraftStreamChunking 草稿流分块参数
type DraftStreamChunking struct {
	MinChars        int
	MaxChars        int
	BreakPreference string // "paragraph", "newline", "sentence"
}

// draftChunkCfg is an alias for the shared config type.
func resolveDraftChunkConfig(cfg *types.OpenAcosmiConfig, normalized string) *types.BlockStreamingChunkConfig {
	if cfg == nil || cfg.Channels == nil || cfg.Channels.Telegram == nil {
		return nil
	}
	tg := cfg.Channels.Telegram
	if acct, ok := tg.Accounts[normalized]; ok && acct != nil && acct.DraftChunk != nil {
		return acct.DraftChunk
	}
	if tg.DraftChunk != nil {
		return tg.DraftChunk
	}
	return nil
}

// ResolveTelegramDraftStreamingChunking 解析草稿流分块配置。
// 对齐 TS draft-chunking.ts L17-20: 使用 GetChannelDock + resolveTextChunkLimit 动态解析 textLimit。
func ResolveTelegramDraftStreamingChunking(cfg *types.OpenAcosmiConfig, accountID string) DraftStreamChunking {
	// 动态解析 textLimit（TS: resolveTextChunkLimit(cfg, "telegram", accountId, { fallbackLimit })）
	// DY-020: 使用 autoreply.ResolveTextChunkLimit 实现完整 config 级联:
	//   账户级 → 通道级 → dock 级 → 默认值(4000)
	providerChunkLimit := channels.GetTextChunkLimit(channels.ChannelTelegram)
	pcc := buildTelegramProviderChunkConfig(cfg)
	textLimit := autoreply.ResolveTextChunkLimit(pcc, accountID, providerChunkLimit)
	normalized := NormalizeAccountID(accountID)
	dc := resolveDraftChunkConfig(cfg, normalized)

	maxRequested := defaultDraftStreamMax
	if dc != nil && dc.MaxChars > 0 {
		maxRequested = int(math.Floor(float64(dc.MaxChars)))
	}
	if maxRequested < 1 {
		maxRequested = 1
	}
	maxChars := maxRequested
	if maxChars > textLimit {
		maxChars = textLimit
	}

	minRequested := defaultDraftStreamMin
	if dc != nil && dc.MinChars > 0 {
		minRequested = int(math.Floor(float64(dc.MinChars)))
	}
	if minRequested < 1 {
		minRequested = 1
	}
	minChars := minRequested
	if minChars > maxChars {
		minChars = maxChars
	}

	breakPref := "paragraph"
	if dc != nil {
		bp := strings.TrimSpace(strings.ToLower(dc.BreakPreference))
		if bp == "newline" || bp == "sentence" {
			breakPref = bp
		}
	}

	return DraftStreamChunking{MinChars: minChars, MaxChars: maxChars, BreakPreference: breakPref}
}

// buildTelegramProviderChunkConfig 从 OpenAcosmiConfig 构建 Telegram 频道的 ProviderChunkConfig。
// DY-020: 用于 autoreply.ResolveTextChunkLimit 实现完整 config 级联。
func buildTelegramProviderChunkConfig(cfg *types.OpenAcosmiConfig) *autoreply.ProviderChunkConfig {
	if cfg == nil || cfg.Channels == nil || cfg.Channels.Telegram == nil {
		return nil
	}
	tg := cfg.Channels.Telegram
	pcc := &autoreply.ProviderChunkConfig{}

	// 通道级配置
	if tg.TextChunkLimit != nil {
		pcc.TextChunkLimit = *tg.TextChunkLimit
	}
	if tg.ChunkMode != "" {
		pcc.ChunkMode = autoreply.ChunkMode(tg.ChunkMode)
	}

	// 账号级配置
	if len(tg.Accounts) > 0 {
		pcc.Accounts = make(map[string]autoreply.AccountChunkConfig, len(tg.Accounts))
		for id, acct := range tg.Accounts {
			if acct != nil {
				acc := autoreply.AccountChunkConfig{}
				if acct.TextChunkLimit != nil {
					acc.TextChunkLimit = *acct.TextChunkLimit
				}
				if acct.ChunkMode != "" {
					acc.ChunkMode = autoreply.ChunkMode(acct.ChunkMode)
				}
				pcc.Accounts[id] = acc
			}
		}
	}

	return pcc
}
