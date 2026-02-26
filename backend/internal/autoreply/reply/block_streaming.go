package reply

import (
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- Block Streaming Config 解析 ----------
// 对齐 TS auto-reply/reply/block-streaming.ts
// 从 config + provider/account 级覆盖解析 block streaming chunking/coalescing 配置。

// 默认值（对齐 TS 源）
const (
	DefaultBlockStreamMin    = 800
	DefaultBlockStreamMax    = 1200
	DefaultBlockStreamIdleMs = 1000
	DefaultBlockStreamJoiner = " "
)

// BlockStreamingCoalesceDefaultsProvider 获取频道的 block-streaming coalesce 默认值（DI 注入）。
// 由 gateway 启动时注入 channels.GetBlockStreamingCoalesceDefaults。
// 返回 (minChars, idleMs)；(0,0) 表示无频道级默认值。
var BlockStreamingCoalesceDefaultsProvider func(channelKey string) (minChars, idleMs int)

// BlockStreamingChunking 块流式分块配置。
type BlockStreamingChunking struct {
	MinChars         int
	MaxChars         int
	BreakPreference  string // "sentence", "paragraph", "newline"
	FlushOnParagraph bool
}

// ResolveBlockStreamingChunking 解析块流式分块配置。
// 优先级：account > provider > global > defaults。
func ResolveBlockStreamingChunking(coalesce *types.BlockStreamingCoalesceConfig) BlockStreamingChunking {
	result := BlockStreamingChunking{
		MinChars:        DefaultBlockStreamMin,
		MaxChars:        DefaultBlockStreamMax,
		BreakPreference: "sentence",
	}

	if coalesce == nil {
		return result
	}

	if coalesce.MinChars > 0 {
		result.MinChars = coalesce.MinChars
	}
	if coalesce.MaxChars > 0 {
		result.MaxChars = coalesce.MaxChars
	}
	if coalesce.IdleMs > 0 {
		// IdleMs 在 chunking 中不直接使用，但存储以备用
	}

	return result
}

// ResolveBlockStreamingCoalescing 从 chunking 配置生成 coalescing 配置。
func ResolveBlockStreamingCoalescing(chunking BlockStreamingChunking, coalesce *types.BlockStreamingCoalesceConfig) BlockStreamingCoalescing {
	idleMs := DefaultBlockStreamIdleMs
	if coalesce != nil && coalesce.IdleMs > 0 {
		idleMs = coalesce.IdleMs
	}

	joiner := DefaultBlockStreamJoiner
	switch chunking.BreakPreference {
	case "paragraph":
		joiner = "\n\n"
	case "newline":
		joiner = "\n"
	default:
		joiner = " "
	}

	return BlockStreamingCoalescing{
		MinChars:       chunking.MinChars,
		MaxChars:       chunking.MaxChars,
		IdleMs:         idleMs,
		Joiner:         joiner,
		FlushOnEnqueue: false,
	}
}

// ResolveBlockStreamingChunkingWithDock 带频道 dock 联动的块流式分块配置解析。
// 优先级: config coalesce > dock defaults > global defaults。
// 对齐 TS dock.streaming.blockStreamingCoalesceDefaults 的合并逻辑。
func ResolveBlockStreamingChunkingWithDock(coalesce *types.BlockStreamingCoalesceConfig, channelKey string) BlockStreamingChunking {
	// 1. 从 dock 获取频道级默认值
	var dockMinChars, dockIdleMs int
	if channelKey != "" && BlockStreamingCoalesceDefaultsProvider != nil {
		dockMinChars, dockIdleMs = BlockStreamingCoalesceDefaultsProvider(channelKey)
	}

	// 2. 构建合并后的 coalesce: dock defaults 作为 base, config 覆盖其上
	merged := &types.BlockStreamingCoalesceConfig{}
	if dockMinChars > 0 {
		merged.MinChars = dockMinChars
	}
	if dockIdleMs > 0 {
		merged.IdleMs = dockIdleMs
	}
	// config 级覆盖
	if coalesce != nil {
		if coalesce.MinChars > 0 {
			merged.MinChars = coalesce.MinChars
		}
		if coalesce.MaxChars > 0 {
			merged.MaxChars = coalesce.MaxChars
		}
		if coalesce.IdleMs > 0 {
			merged.IdleMs = coalesce.IdleMs
		}
	}

	return ResolveBlockStreamingChunking(merged)
}
