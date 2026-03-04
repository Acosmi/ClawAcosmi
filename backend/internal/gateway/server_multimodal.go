package gateway

// server_multimodal.go — 多模态消息预处理（Phase B）
// 纯新增文件：在渠道消息路由到 Agent 管线之前，
// 对图片/音频/文件附件进行下载和预处理。
// 不修改任何已有 DispatchFunc 逻辑。

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
	"github.com/Acosmi/ClawAcosmi/internal/channels/feishu"
	"github.com/Acosmi/ClawAcosmi/internal/media"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
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

// MultimodalPreprocessorResolver 运行态预处理器解析器。
// 通过短 TTL 缓存避免每条消息都重建 provider，同时保证配置修改后能快速生效。
type MultimodalPreprocessorResolver struct {
	loader interface {
		LoadConfig() (*types.OpenAcosmiConfig, error)
	}
	fallbackCfg *types.OpenAcosmiConfig
	ttl         time.Duration

	mu        sync.Mutex
	cached    *MultimodalPreprocessor
	expiresAt time.Time
}

// NewMultimodalPreprocessorResolver 创建运行态预处理器解析器。
func NewMultimodalPreprocessorResolver(
	loader interface {
		LoadConfig() (*types.OpenAcosmiConfig, error)
	},
	fallbackCfg *types.OpenAcosmiConfig,
	ttl time.Duration,
) *MultimodalPreprocessorResolver {
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	return &MultimodalPreprocessorResolver{
		loader:      loader,
		fallbackCfg: fallbackCfg,
		ttl:         ttl,
	}
}

// Get 获取当前可用的预处理器实例（带 TTL 缓存）。
func (r *MultimodalPreprocessorResolver) Get() *MultimodalPreprocessor {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cached != nil && now.Before(r.expiresAt) {
		return r.cached
	}

	cfg := r.fallbackCfg
	if r.loader != nil {
		if freshCfg, err := r.loader.LoadConfig(); err != nil {
			slog.Warn("multimodal: reload config failed, using cached/fallback", "error", err)
		} else if freshCfg != nil {
			cfg = freshCfg
		}
	}

	r.cached = NewMultimodalPreprocessorFromConfig(cfg)
	r.expiresAt = now.Add(r.ttl)
	return r.cached
}

// NewMultimodalPreprocessorFromConfig 根据配置构建预处理器。
func NewMultimodalPreprocessorFromConfig(cfg *types.OpenAcosmiConfig) *MultimodalPreprocessor {
	p := &MultimodalPreprocessor{}
	if cfg == nil {
		return p
	}

	if cfg.STT != nil && cfg.STT.Provider != "" {
		if prov, err := media.NewSTTProvider(cfg.STT); err == nil {
			p.STTProvider = prov
		} else {
			slog.Warn("multimodal: STT provider init failed (non-fatal)", "error", err)
		}
	}
	if cfg.DocConv != nil && cfg.DocConv.Provider != "" {
		if conv, err := media.NewDocConverter(cfg.DocConv); err == nil {
			p.DocConverter = conv
		} else {
			slog.Warn("multimodal: DocConv provider init failed (non-fatal)", "error", err)
		}
	}
	if cfg.ImageUnderstanding != nil && cfg.ImageUnderstanding.Provider != "" {
		if desc, err := media.NewImageDescriber(cfg.ImageUnderstanding); err == nil {
			p.ImageDescriber = desc
		} else {
			slog.Warn("multimodal: ImageDescriber provider init failed (non-fatal)", "error", err)
		}
	}

	return p
}

// PreprocessImage 预处理后的图片数据。
type PreprocessImage struct {
	Base64   string
	MimeType string
}

// PreprocessResult 预处理结果
type PreprocessResult struct {
	// Text 增强后的文本（原文本 + 附件描述）
	Text string
	// Images 所有图片 base64 数据（新增，多图支持）。
	Images []PreprocessImage
	// ImageBase64 第一张图片的 base64 编码（用于前端显示）
	// 兼容字段：后续可迁移到 Images。
	ImageBase64 string
	// ImageMimeType 图片 MIME 类型
	// 兼容字段：后续可迁移到 Images。
	ImageMimeType string
}

// feishuResourceDownloader 抽象飞书资源下载能力，便于单测覆盖多媒体预处理分支。
type feishuResourceDownloader interface {
	DownloadImage(ctx context.Context, messageID, imageKey string) ([]byte, error)
	DownloadFile(ctx context.Context, messageID, fileKey string) ([]byte, error)
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
	return p.processFeishuMessageWithDownloader(ctx, client, msg)
}

func (p *MultimodalPreprocessor) processFeishuMessageWithDownloader(
	ctx context.Context,
	client feishuResourceDownloader,
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
		if r.imageBase64 != "" {
			result.Images = append(result.Images, PreprocessImage{
				Base64:   r.imageBase64,
				MimeType: r.imageMimeType,
			})
		}
		// 兼容：保留第一张图片到旧字段
		if result.ImageBase64 == "" && r.imageBase64 != "" {
			result.ImageBase64 = r.imageBase64
			result.ImageMimeType = r.imageMimeType
		}
	}

	result.Text = strings.Join(textParts, "\n")
	return result
}

// ProcessGenericChannelMessage 预处理通用渠道消息（钉钉/企微等）。
// 优先使用附件内联数据（Data/DataURL），其次尝试安全下载外部 URL。
// 结果统一为增强文本，可直接交给 DispatchFunc。
func (p *MultimodalPreprocessor) ProcessGenericChannelMessage(
	ctx context.Context,
	msg *channels.ChannelMessage,
) *PreprocessResult {
	if msg == nil {
		return &PreprocessResult{}
	}

	result := &PreprocessResult{
		Text: strings.TrimSpace(msg.Text),
	}
	if len(msg.Attachments) == 0 {
		return result
	}

	var textParts []string
	if result.Text != "" {
		textParts = append(textParts, result.Text)
	}

	const maxAttachments = 10
	attachments := msg.Attachments
	if len(attachments) > maxAttachments {
		slog.Warn("multimodal: attachment count exceeds limit, truncating",
			"total", len(attachments), "limit", maxAttachments)
		attachments = attachments[:maxAttachments]
	}

	for _, att := range attachments {
		data, mimeType, loadErr := loadChannelAttachmentData(ctx, att)
		category := strings.ToLower(strings.TrimSpace(att.Category))
		switch category {
		case "image":
			if loadErr != nil || len(data) == 0 {
				textParts = append(textParts, "[图片消息: 当前渠道未提供可读取内容]")
				continue
			}
			if mimeType == "" {
				mimeType = detectImageMediaType(data)
			}
			if p.ImageDescriber != nil {
				desc, descErr := p.ImageDescriber.Describe(ctx, data, mimeType)
				if descErr != nil {
					slog.Warn("multimodal: generic image describe failed, fallback metadata",
						"provider", p.ImageDescriber.Name(), "error", descErr)
					textParts = append(textParts, fmt.Sprintf("[图片: %s, 大小: %s]", mimeType, humanReadableSize(int64(len(data)))))
				} else {
					textParts = append(textParts, fmt.Sprintf("[图片描述]: %s", desc))
				}
			} else {
				textParts = append(textParts, fmt.Sprintf("[图片: %s, 大小: %s]", mimeType, humanReadableSize(int64(len(data)))))
			}

			imgB64 := base64.StdEncoding.EncodeToString(data)
			result.Images = append(result.Images, PreprocessImage{
				Base64:   imgB64,
				MimeType: mimeType,
			})
			if result.ImageBase64 == "" {
				result.ImageBase64 = imgB64
				result.ImageMimeType = mimeType
			}

		case "audio":
			if loadErr != nil || len(data) == 0 {
				textParts = append(textParts, "[语音消息: 当前渠道未提供可读取内容]")
				continue
			}
			if p.STTProvider == nil {
				textParts = append(textParts, "[语音消息: STT 未配置]")
				continue
			}
			if mimeType == "" {
				mimeType = "audio/opus"
			}
			transcript, sttErr := p.STTProvider.Transcribe(ctx, data, mimeType)
			if sttErr != nil {
				slog.Error("multimodal: generic STT failed", "error", sttErr)
				textParts = append(textParts, "[语音转录失败]")
			} else {
				textParts = append(textParts, fmt.Sprintf("[语音转录]: %s", transcript))
			}

		case "document":
			name := strings.TrimSpace(att.FileName)
			if name == "" {
				name = "未命名文件"
			}
			if loadErr != nil || len(data) == 0 {
				textParts = append(textParts, fmt.Sprintf("[文件: %s, 当前渠道未提供可读取内容]", name))
				continue
			}
			if p.DocConverter != nil && media.IsSupportedFormat(name) {
				md, convErr := p.DocConverter.Convert(ctx, data, mimeType, name)
				if convErr != nil {
					slog.Error("multimodal: generic DocConv failed", "file", name, "error", convErr)
					textParts = append(textParts, fmt.Sprintf("[文件: %s, 转换失败]", name))
				} else {
					textParts = append(textParts, fmt.Sprintf("[文件: %s]\n%s", name, md))
				}
			} else {
				textParts = append(textParts, fmt.Sprintf("[文件: %s, 大小: %s]", name, humanReadableSize(int64(len(data)))))
			}

		case "video":
			textParts = append(textParts, "[视频消息: 暂不支持]")

		default:
			if name := strings.TrimSpace(att.FileName); name != "" {
				textParts = append(textParts, fmt.Sprintf("[附件: %s, 类型: %s]", name, category))
			} else {
				textParts = append(textParts, fmt.Sprintf("[附件消息: 类型=%s]", category))
			}
		}
	}

	result.Text = strings.TrimSpace(strings.Join(textParts, "\n"))
	return result
}

func loadChannelAttachmentData(ctx context.Context, att channels.ChannelAttachment) ([]byte, string, error) {
	if len(att.Data) > 0 {
		return att.Data, strings.TrimSpace(att.MimeType), nil
	}

	mimeType := strings.TrimSpace(att.MimeType)
	source := strings.TrimSpace(att.DataURL)
	if source == "" {
		source = strings.TrimSpace(att.FileKey)
	}
	if source == "" {
		return nil, mimeType, fmt.Errorf("attachment payload missing")
	}

	if strings.HasPrefix(strings.ToLower(source), "data:") {
		data, dataMime, err := decodeDataURLPayload(source)
		if err != nil {
			return nil, mimeType, err
		}
		if mimeType == "" {
			mimeType = dataMime
		}
		return data, mimeType, nil
	}

	if isRemoteHTTPURL(source) {
		data, remoteMime, err := fetchRemoteAttachmentData(ctx, source, channels.ChatAttachmentFileMaxBytes)
		if err != nil {
			return nil, mimeType, err
		}
		if mimeType == "" {
			mimeType = remoteMime
		}
		if mimeType == "" && len(data) > 0 {
			mimeType = strings.ToLower(http.DetectContentType(data))
		}
		return data, mimeType, nil
	}

	return nil, mimeType, fmt.Errorf("attachment payload unavailable")
}

func decodeDataURLPayload(raw string) ([]byte, string, error) {
	commaIdx := strings.Index(raw, ",")
	if commaIdx < 0 {
		return nil, "", fmt.Errorf("invalid data URL")
	}
	meta := raw[len("data:"):commaIdx]
	payload := raw[commaIdx+1:]

	mimeType := "application/octet-stream"
	parts := strings.Split(meta, ";")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		mimeType = strings.TrimSpace(parts[0])
	}
	isBase64 := false
	for _, p := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(p), "base64") {
			isBase64 = true
			break
		}
	}
	if !isBase64 {
		decoded, err := url.QueryUnescape(payload)
		if err != nil {
			return nil, "", err
		}
		return []byte(decoded), mimeType, nil
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", err
	}
	return data, mimeType, nil
}

func isRemoteHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func fetchRemoteAttachmentData(ctx context.Context, rawURL string, maxBytes int) ([]byte, string, error) {
	if err := validateRemoteAttachmentURL(rawURL); err != nil {
		return nil, "", fmt.Errorf("remote attachment rejected: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return validateRemoteAttachmentURL(req.URL.String())
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("remote fetch status=%d body=%s", resp.StatusCode, string(body))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxBytes {
		return nil, "", fmt.Errorf("remote attachment too large (max %d bytes)", maxBytes)
	}
	return data, strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type"))), nil
}

func validateRemoteAttachmentURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("resolve host %q failed: %w", host, err)
	}
	for _, ipText := range ips {
		ip := net.ParseIP(ipText)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("URL resolves to private/loopback address %s", ipText)
		}
	}
	return nil
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
