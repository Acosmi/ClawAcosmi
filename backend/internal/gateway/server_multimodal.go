package gateway

// server_multimodal.go — 多模态消息预处理（Phase B）
// 纯新增文件：在渠道消息路由到 Agent 管线之前，
// 对图片/音频/文件附件进行下载和预处理。
// 不修改任何已有 DispatchFunc 逻辑。

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/internal/channels/feishu"
	"github.com/openacosmi/claw-acismi/internal/media"
)

// MultimodalPreprocessor 多模态消息预处理器。
// 在消息路由到 Agent 之前，下载附件并生成增强文本。
type MultimodalPreprocessor struct {
	// STTProvider 语音转文本 Provider（可选）
	STTProvider media.STTProvider
	// DocConverter 文档转换 Provider（可选）
	DocConverter media.DocConverter
	// ImageDescriber 图片理解 Fallback Provider（可选，Phase E）
	// 当主模型不支持多模态时，调用此 Provider 生成图片文字描述
	ImageDescriber media.ImageDescriber
}

// PreprocessResult 预处理结果
type PreprocessResult struct {
	// Text 增强后的文本（原文本 + 附件描述）
	Text string
	// ImageBase64 第一张图片的 base64 编码（用于前端显示）
	ImageBase64 string
	// ImageMimeType 图片 MIME 类型
	ImageMimeType string
}

// ProcessFeishuMessage 预处理飞书多模态消息。
// 下载附件并生成增强文本（STT 转录、文档转换、图片 base64）。
// client 参数为飞书客户端实例，用于下载资源。
// 返回的 PreprocessResult.Text 可直接传给 DispatchFunc。
func (p *MultimodalPreprocessor) ProcessFeishuMessage(
	ctx context.Context,
	client *feishu.FeishuClient,
	msg *channels.ChannelMessage,
) *PreprocessResult {
	if msg == nil {
		return &PreprocessResult{}
	}

	result := &PreprocessResult{
		Text: msg.Text,
	}

	// 无附件 → 直接返回纯文本
	if len(msg.Attachments) == 0 {
		return result
	}

	if client == nil {
		slog.Warn("multimodal: feishu client is nil, skipping attachments")
		return result
	}

	var textParts []string
	if msg.Text != "" {
		textParts = append(textParts, msg.Text)
	}

	// M-02: 限制附件数量，防止海量附件阻塞 dispatch
	const maxAttachments = 10
	attachments := msg.Attachments
	if len(attachments) > maxAttachments {
		slog.Warn("multimodal: attachment count exceeds limit, truncating",
			"total", len(attachments), "limit", maxAttachments)
		attachments = attachments[:maxAttachments]
	}

	// 并发处理附件（索引数组保持顺序）
	type attachResult struct {
		text          string
		imageBase64   string
		imageMimeType string
	}
	results := make([]attachResult, len(attachments))
	var wg sync.WaitGroup
	for i, att := range attachments {
		wg.Add(1)
		go func(idx int, a channels.ChannelAttachment) {
			defer wg.Done()
			switch a.Category {
			case "image":
				data, err := client.DownloadImage(ctx, msg.MessageID, a.FileKey)
				if err != nil {
					slog.Error("multimodal: failed to download image",
						"message_id", msg.MessageID,
						"file_key", a.FileKey,
						"error", err,
					)
					results[idx].text = "[图片下载失败]"
					return
				}
				mediaType := a.MimeType
				if mediaType == "" {
					mediaType = detectImageMediaType(data)
				}
				// Phase E: 若配置了 ImageDescriber，调用视觉 API 生成文字描述
				if p.ImageDescriber != nil {
					desc, descErr := p.ImageDescriber.Describe(ctx, data, mediaType)
					if descErr != nil {
						slog.Warn("multimodal: image describe failed, fallback to metadata",
							"provider", p.ImageDescriber.Name(),
							"error", descErr,
						)
						results[idx].text = fmt.Sprintf("[图片: %s, 大小: %s]", mediaType, humanReadableSize(int64(len(data))))
					} else {
						results[idx].text = fmt.Sprintf("[图片描述]: %s", desc)
					}
				} else {
					results[idx].text = fmt.Sprintf("[图片: %s, 大小: %s]", mediaType, humanReadableSize(int64(len(data))))
				}
				// 保留 base64 数据，供前端直接显示
				results[idx].imageBase64 = base64.StdEncoding.EncodeToString(data)
				results[idx].imageMimeType = mediaType

			case "audio":
				data, err := client.DownloadFile(ctx, msg.MessageID, a.FileKey)
				if err != nil {
					slog.Error("multimodal: failed to download audio",
						"message_id", msg.MessageID,
						"file_key", a.FileKey,
						"error", err,
					)
					results[idx].text = "[语音下载失败]"
					return
				}
				if p.STTProvider != nil {
					mimeType := a.MimeType
					if mimeType == "" {
						mimeType = "audio/opus"
					}
					transcript, sttErr := p.STTProvider.Transcribe(ctx, data, mimeType)
					if sttErr != nil {
						slog.Error("multimodal: STT transcription failed",
							"file_key", a.FileKey, "error", sttErr)
						results[idx].text = "[语音转录失败]"
					} else {
						results[idx].text = fmt.Sprintf("[语音转录]: %s", transcript)
					}
				} else {
					results[idx].text = "[语音消息: STT 未配置]"
				}

			case "document":
				name := a.FileName
				if name == "" {
					name = "未命名文件"
				}
				data, err := client.DownloadFile(ctx, msg.MessageID, a.FileKey)
				if err != nil {
					slog.Error("multimodal: failed to download document",
						"message_id", msg.MessageID,
						"file_key", a.FileKey,
						"error", err,
					)
					results[idx].text = fmt.Sprintf("[文件: %s, 下载失败]", name)
					return
				}
				if p.DocConverter != nil && media.IsSupportedFormat(name) {
					markdown, convErr := p.DocConverter.Convert(ctx, data, a.MimeType, name)
					if convErr != nil {
						slog.Error("multimodal: document conversion failed",
							"file", name, "error", convErr)
						results[idx].text = fmt.Sprintf("[文件: %s, 转换失败]", name)
					} else {
						results[idx].text = fmt.Sprintf("[文件: %s]\n%s", name, markdown)
					}
				} else {
					results[idx].text = fmt.Sprintf("[文件: %s, 大小: %s]", name, humanReadableSize(a.FileSize))
				}

			case "video":
				results[idx].text = "[视频消息: 暂不支持]"

			default:
				results[idx].text = fmt.Sprintf("[附件: %s, 类型: %s]", a.FileName, a.Category)
			}
		}(i, att)
	}
	wg.Wait()
	for _, r := range results {
		if r.text != "" {
			textParts = append(textParts, r.text)
		}
		// 取第一张图片的 base64 数据
		if result.ImageBase64 == "" && r.imageBase64 != "" {
			result.ImageBase64 = r.imageBase64
			result.ImageMimeType = r.imageMimeType
		}
	}

	result.Text = strings.Join(textParts, "\n")
	return result
}

// detectImageMediaType 从图片数据的 magic bytes 检测 MIME 类型
func detectImageMediaType(data []byte) string {
	if len(data) < 4 {
		return "image/png"
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// GIF: 47 49 46
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}
	// WebP: 52 49 46 46 ... 57 45 42 50
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}
	// BMP: 42 4D
	if data[0] == 0x42 && data[1] == 0x4D {
		return "image/bmp"
	}
	return "image/png" // 默认
}

// humanReadableSize 将字节数转为人类可读格式
func humanReadableSize(size int64) string {
	if size <= 0 {
		return "未知大小"
	}
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
