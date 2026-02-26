package channels

// 媒体限制 — 继承自 src/channels/plugins/media-limits.ts (26 行)

// DefaultMediaMaxBytes 默认媒体上传限制（8 MB）
const DefaultMediaMaxBytes = 8 * 1024 * 1024

// ResolveChannelMediaMaxBytes 解析频道媒体上传限制
// channelLimit：频道级别限制（0 表示未配置）
// agentLimit：Agent 级别限制（0 表示未配置）
func ResolveChannelMediaMaxBytes(channelLimit, agentLimit int64) int64 {
	if channelLimit > 0 {
		return channelLimit
	}
	if agentLimit > 0 {
		return agentLimit
	}
	return DefaultMediaMaxBytes
}
