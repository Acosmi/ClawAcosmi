package channels

import (
	"fmt"
	"sort"
	"strings"
)

// 频道行为中枢 — 继承自 src/channels/dock.ts (457 行)
// 每个频道的能力、出站限制、流式默认值、提及正则等轻量级行为描述

// ChannelDock 频道行为坞 — 共享代码路径使用的轻量级频道描述
type ChannelDock struct {
	ID            ChannelID
	Capabilities  DockCapabilities
	Commands      *DockCommands
	Outbound      *DockOutbound
	Streaming     *DockStreaming
	Mentions      *DockMentions
	Threading     *DockThreading
	QueueDefaults *DockQueueDefaults
	Elevated      *DockElevated
}

// DockCapabilities 能力描述
type DockCapabilities struct {
	ChatTypes      []string
	Polls          bool
	Reactions      bool
	Media          bool
	NativeCommands bool
	Threads        bool
	BlockStreaming bool
	Edit           bool
	Unsend         bool
	Reply          bool
	Effects        bool
}

// DockCommands 命令适配
type DockCommands struct {
	EnforceOwnerForCommands bool
	SkipWhenConfigEmpty     bool
}

// DockOutbound 出站限制
type DockOutbound struct {
	TextChunkLimit int
}

// DockStreaming 流式默认值
type DockStreaming struct {
	MinChars int
	IdleMs   int
}

// DockMentions 提及清理模式
type DockMentions struct {
	StripPatterns []string
}

// DockThreading 线程行为
type DockThreading struct {
	DefaultReplyToMode string // "off" | "first" | "all"
	AllowTagsWhenOff   bool
}

// DockQueueDefaults 频道默认队列配置（对齐 TS ChannelPlugin.defaults.queue）
type DockQueueDefaults struct {
	DebounceMs *int
}

// DockElevatedFallbackParams AllowFromFallback 回调参数。
type DockElevatedFallbackParams struct {
	CfgRaw    interface{} // *types.OpenAcosmiConfig（避免直接导入 types）
	AccountId string
}

// DockElevated 提权相关配置（对齐 TS dock.elevated）
type DockElevated struct {
	// AllowFromFallbackFn 频道级别的 fallback 允许列表回调。
	// 当 config 中未为该 provider 配置 allowFrom 时，
	// 通过此回调从频道插件获取默认允许列表。
	AllowFromFallbackFn func(params DockElevatedFallbackParams) []interface{}
}

// coreDocks 核心频道行为配置 — 与 TS DOCKS 记录对齐
var coreDocks = map[ChannelID]*ChannelDock{
	ChannelTelegram: {
		ID: ChannelTelegram,
		Capabilities: DockCapabilities{
			ChatTypes:      []string{"direct", "group", "channel", "thread"},
			NativeCommands: true, BlockStreaming: true,
		},
		Outbound:  &DockOutbound{TextChunkLimit: 4000},
		Threading: &DockThreading{DefaultReplyToMode: "first"},
	},
	ChannelWhatsApp: {
		ID: ChannelWhatsApp,
		Capabilities: DockCapabilities{
			ChatTypes: []string{"direct", "group"},
			Polls:     true, Reactions: true, Media: true,
		},
		Commands: &DockCommands{EnforceOwnerForCommands: true, SkipWhenConfigEmpty: true},
		Outbound: &DockOutbound{TextChunkLimit: 4000},
		Mentions: &DockMentions{StripPatterns: []string{}}, // 动态生成
	},
	ChannelDiscord: {
		ID: ChannelDiscord,
		Capabilities: DockCapabilities{
			ChatTypes: []string{"direct", "channel", "thread"},
			Polls:     true, Reactions: true, Media: true,
			NativeCommands: true, Threads: true,
		},
		Outbound:  &DockOutbound{TextChunkLimit: 2000},
		Streaming: &DockStreaming{MinChars: 1500, IdleMs: 1000},
		Mentions:  &DockMentions{StripPatterns: []string{`<@!?\d+>`}},
		Threading: &DockThreading{DefaultReplyToMode: "off"},
	},
	ChannelGoogleChat: {
		ID: ChannelGoogleChat,
		Capabilities: DockCapabilities{
			ChatTypes: []string{"direct", "group", "thread"},
			Reactions: true, Media: true, Threads: true, BlockStreaming: true,
		},
		Outbound:  &DockOutbound{TextChunkLimit: 4000},
		Threading: &DockThreading{DefaultReplyToMode: "off"},
	},
	ChannelSlack: {
		ID: ChannelSlack,
		Capabilities: DockCapabilities{
			ChatTypes: []string{"direct", "channel", "thread"},
			Reactions: true, Media: true,
			NativeCommands: true, Threads: true,
		},
		Outbound:  &DockOutbound{TextChunkLimit: 4000},
		Streaming: &DockStreaming{MinChars: 1500, IdleMs: 1000},
		Mentions:  &DockMentions{StripPatterns: []string{`<@[^>]+>`}},
		Threading: &DockThreading{DefaultReplyToMode: "off", AllowTagsWhenOff: true},
	},
	ChannelSignal: {
		ID: ChannelSignal,
		Capabilities: DockCapabilities{
			ChatTypes: []string{"direct", "group"},
			Reactions: true, Media: true,
		},
		Outbound:  &DockOutbound{TextChunkLimit: 4000},
		Streaming: &DockStreaming{MinChars: 1500, IdleMs: 1000},
	},
	ChannelIMessage: {
		ID: ChannelIMessage,
		Capabilities: DockCapabilities{
			ChatTypes: []string{"direct", "group"},
			Reactions: true, Media: true,
		},
		Outbound: &DockOutbound{TextChunkLimit: 4000},
	},
}

// PluginChannelDockProvider 插件频道 dock 提供器（DI 注入）。
// 由 gateway 启动时设置，返回插件注册表中的所有插件频道 dock。
var PluginChannelDockProvider func() []*ChannelDock

// GetChannelDock 获取频道行为坞（核心频道直接返回，插件频道通过注册表）
func GetChannelDock(id ChannelID) *ChannelDock {
	if d, ok := coreDocks[id]; ok {
		return d
	}
	if PluginChannelDockProvider != nil {
		for _, d := range PluginChannelDockProvider() {
			if d.ID == id {
				return d
			}
		}
	}
	return nil
}

// ListChannelDocks 列出所有频道 dock（核心频道按排序，插件频道追加后排序）
func ListChannelDocks() []*ChannelDock {
	type dockEntry struct {
		id    ChannelID
		dock  *ChannelDock
		order int
	}
	var entries []dockEntry
	for i, id := range chatChannelOrder {
		if d, ok := coreDocks[id]; ok {
			entries = append(entries, dockEntry{id: id, dock: d, order: i})
		}
	}
	// 插件频道通过 PluginChannelDockProvider 动态追加
	if PluginChannelDockProvider != nil {
		for _, d := range PluginChannelDockProvider() {
			entries = append(entries, dockEntry{id: d.ID, dock: d, order: len(chatChannelOrder) + len(entries)})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].order != entries[j].order {
			return entries[i].order < entries[j].order
		}
		return string(entries[i].id) < string(entries[j].id)
	})
	result := make([]*ChannelDock, len(entries))
	for i, e := range entries {
		result[i] = e.dock
	}
	return result
}

// GetTextChunkLimit 获取频道的文本分块限制
func GetTextChunkLimit(id ChannelID) int {
	d := GetChannelDock(id)
	if d != nil && d.Outbound != nil && d.Outbound.TextChunkLimit > 0 {
		return d.Outbound.TextChunkLimit
	}
	return 4000 // 默认限制
}

// GetMentionStripPatterns 获取频道的 @提及清理正则
func GetMentionStripPatterns(id ChannelID) []string {
	d := GetChannelDock(id)
	if d != nil && d.Mentions != nil {
		return d.Mentions.StripPatterns
	}
	return nil
}

// GetStreamingDefaults 获取频道的流式默认值
func GetStreamingDefaults(id ChannelID) (minChars, idleMs int) {
	d := GetChannelDock(id)
	if d != nil && d.Streaming != nil {
		return d.Streaming.MinChars, d.Streaming.IdleMs
	}
	return 0, 0
}

// GetDefaultReplyToMode 获取频道默认回复模式
func GetDefaultReplyToMode(id ChannelID) string {
	d := GetChannelDock(id)
	if d != nil && d.Threading != nil && d.Threading.DefaultReplyToMode != "" {
		return d.Threading.DefaultReplyToMode
	}
	return "off"
}

// GetPluginDebounce 获取频道的插件级 debounce 默认值。
// 对齐 TS resolvePluginDebounce: getChannelPlugin(key).defaults.queue.debounceMs
func GetPluginDebounce(id ChannelID) *int {
	d := GetChannelDock(id)
	if d != nil && d.QueueDefaults != nil && d.QueueDefaults.DebounceMs != nil {
		v := *d.QueueDefaults.DebounceMs
		if v < 0 {
			v = 0
		}
		return &v
	}
	return nil
}

// GetBlockStreamingCoalesceDefaults 获取频道的 block-streaming coalesce 默认值。
// 对齐 TS dock.streaming.blockStreamingCoalesceDefaults
func GetBlockStreamingCoalesceDefaults(id ChannelID) (minChars, idleMs int) {
	// 复用 GetStreamingDefaults —— Go dock.Streaming 已存储 coalesce defaults
	return GetStreamingDefaults(id)
}

// ListNativeCommandChannels 列出所有支持 native commands 的频道 ID。
// 对齐 TS listChannelDocks().filter(d => d.capabilities.nativeCommands)
func ListNativeCommandChannels() []ChannelID {
	var ids []ChannelID
	for _, d := range ListChannelDocks() {
		if d.Capabilities.NativeCommands {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// FormatAllowFromLower 通用 allowFrom 格式化（小写 + trim）
func FormatAllowFromLower(entries []string) []string {
	var result []string
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			result = append(result, strings.ToLower(trimmed))
		}
	}
	return result
}

// FormatTelegramAllowFrom Telegram 专属 allowFrom 格式化
func FormatTelegramAllowFrom(entries []string) []string {
	var result []string
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		s := trimmed
		for _, prefix := range []string{"telegram:", "tg:", "Telegram:", "TG:"} {
			if strings.HasPrefix(s, prefix) {
				s = s[len(prefix):]
				break
			}
		}
		result = append(result, strings.ToLower(s))
	}
	return result
}

// FormatGoogleChatAllowFrom Google Chat 专属 allowFrom 格式化
func FormatGoogleChatAllowFrom(entries []string) []string {
	var result []string
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		s := trimmed
		for _, prefix := range []string{
			"googlechat:", "google-chat:", "gchat:",
			"user:", "users/",
			"GoogleChat:", "Google-Chat:", "GChat:",
			"User:", "Users/",
		} {
			if strings.HasPrefix(s, prefix) || strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
				s = s[len(prefix):]
				break
			}
		}
		result = append(result, strings.ToLower(s))
	}
	return result
}

// BuildWhatsAppMentionPatterns 构建 WhatsApp @提及清理模式（动态）
func BuildWhatsAppMentionPatterns(selfE164 string) []string {
	e164 := strings.TrimPrefix(strings.TrimSpace(selfE164), "whatsapp:")
	if e164 == "" {
		return nil
	}
	escaped := escapeRegExp(e164)
	return []string{escaped, fmt.Sprintf("@%s", escaped)}
}

func escapeRegExp(s string) string {
	special := `.*+?^${}()|[]\`
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(special, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}
