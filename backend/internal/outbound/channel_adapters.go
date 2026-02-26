package outbound

// ============================================================================
// 通道消息适配器
// 对应 TS: infra/outbound/channel-adapters.ts
// ============================================================================

// ChannelMessageAdapter 频道消息适配器，描述频道的富消息能力。
// TS 参考: channel-adapters.ts → ChannelMessageAdapter
type ChannelMessageAdapter struct {
	// SupportsEmbeds 是否支持嵌入内容（如 Discord embeds）。
	SupportsEmbeds bool
	// BuildCrossContextEmbeds 构建跨上下文嵌入内容的函数（可选）。
	// originLabel 为来源标签字符串。
	BuildCrossContextEmbeds func(originLabel string) []interface{}
}

// defaultChannelMessageAdapter 默认（不支持 embeds）适配器。
var defaultChannelMessageAdapter = &ChannelMessageAdapter{
	SupportsEmbeds: false,
}

// discordChannelMessageAdapter Discord 频道适配器（支持 embeds）。
var discordChannelMessageAdapter = &ChannelMessageAdapter{
	SupportsEmbeds: true,
	BuildCrossContextEmbeds: func(originLabel string) []interface{} {
		return []interface{}{
			map[string]interface{}{
				"description": "From " + originLabel,
			},
		}
	},
}

// channelAdapterRegistry 频道适配器注册表。
var channelAdapterRegistry = map[string]*ChannelMessageAdapter{
	"discord": discordChannelMessageAdapter,
}

// RegisterChannelMessageAdapter 注册自定义频道消息适配器。
// 可用于插件扩展以覆盖默认行为。
func RegisterChannelMessageAdapter(channel string, adapter *ChannelMessageAdapter) {
	if adapter != nil {
		channelAdapterRegistry[channel] = adapter
	}
}

// GetChannelMessageAdapter 获取频道对应的消息适配器。
// 未注册的频道返回默认适配器（不支持 embeds）。
// TS 参考: channel-adapters.ts → getChannelMessageAdapter()
func GetChannelMessageAdapter(channel string) *ChannelMessageAdapter {
	if adapter, ok := channelAdapterRegistry[channel]; ok {
		return adapter
	}
	return defaultChannelMessageAdapter
}
