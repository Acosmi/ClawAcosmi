package channels

import "strings"

// 媒体限制常量（全局聊天 + 渠道发送链路统一基线）。
const (
	// DefaultMediaMaxBytes 默认媒体上传限制（8 MB，兼容历史逻辑）。
	DefaultMediaMaxBytes = 8 * 1024 * 1024

	// ChatAttachmentImageMaxBytes 聊天图片附件上限（10 MB）。
	// 对齐飞书图片上传 API 限制。
	ChatAttachmentImageMaxBytes = 10 * 1024 * 1024

	// ChatAttachmentAudioMaxBytes 聊天音频附件上限（25 MB）。
	// 对齐 STT 转写接口限制。
	ChatAttachmentAudioMaxBytes = 25 * 1024 * 1024

	// ChatAttachmentFileMaxBytes 聊天文档/视频等附件上限（30 MB）。
	// 对齐飞书文件上传 API 限制。
	ChatAttachmentFileMaxBytes = 30 * 1024 * 1024

	// FeishuImageMaxBytes 飞书图片上传上限（10 MB）。
	FeishuImageMaxBytes = ChatAttachmentImageMaxBytes

	// FeishuFileMaxBytes 飞书文件上传上限（30 MB）。
	FeishuFileMaxBytes = ChatAttachmentFileMaxBytes

	// FeishuResourceDownloadMaxBytes 飞书资源下载上限（30 MB）。
	// 统一为可回传窗口，避免 50MB 可下载但 30MB 无法上传回传的不一致。
	FeishuResourceDownloadMaxBytes = FeishuFileMaxBytes
)

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

// MaxChatAttachmentBytesForType 返回 chat 附件类型的限制（按 type 字段）。
// type: image | audio | document | video
func MaxChatAttachmentBytesForType(attType string) int {
	switch strings.ToLower(strings.TrimSpace(attType)) {
	case "image":
		return ChatAttachmentImageMaxBytes
	case "audio":
		return ChatAttachmentAudioMaxBytes
	default:
		return ChatAttachmentFileMaxBytes
	}
}

// MaxOutboundMediaBytesForMime 返回渠道二进制发送限制（按 MIME）。
// 图片 10MB，其他媒体默认 30MB。
func MaxOutboundMediaBytesForMime(mimeType string) int {
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mime, "image/") {
		return FeishuImageMaxBytes
	}
	return FeishuFileMaxBytes
}
