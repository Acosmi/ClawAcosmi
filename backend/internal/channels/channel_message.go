package channels

// channel_message.go — 统一渠道多模态消息类型
// 纯加法：不影响已有 DispatchFunc(text string) 路径

// ChannelMessage 统一渠道入站消息（支持多模态）。
// DispatchMultimodalFunc 使用此类型传递完整消息信息。
type ChannelMessage struct {
	// Text 消息文本内容（与 DispatchFunc 兼容）
	Text string

	// MessageID 平台消息 ID（用于资源下载）
	MessageID string

	// MessageType 原始消息类型（平台相关值，如 text/image/audio/file/post）
	MessageType string

	// Attachments 消息中的多媒体附件
	Attachments []ChannelAttachment

	// RawContent 原始消息 JSON（调试/审计用）
	RawContent string
}

// ChannelAttachment 渠道消息附件。
type ChannelAttachment struct {
	// Category 归一化类别：image / audio / document / video
	Category string

	// FileKey 平台资源标识符（飞书 file_key / image_key，钉钉 downloadCode）
	FileKey string

	// FileName 原始文件名（如有）
	FileName string

	// FileSize 文件大小（字节，如有）
	FileSize int64

	// MimeType MIME 类型（如有）
	MimeType string

	// Data 下载后的二进制数据（可选，Phase B 填充）
	Data []byte

	// DataURL base64 data URL（可选，小文件用）
	DataURL string
}

// DispatchMultimodalFunc 多模态消息分发回调类型。
// 与 DispatchFunc 并存：优先使用此回调，未设置则回退 DispatchFunc(text)。
type DispatchMultimodalFunc func(
	channel, accountID, chatID, userID string,
	msg *ChannelMessage,
) string
