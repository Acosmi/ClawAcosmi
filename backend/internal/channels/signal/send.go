package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/media"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// Signal 消息发送 — 继承自 src/signal/send.ts (282L)

// SignalSendResult 发送结果
type SignalSendResult struct {
	MessageID string `json:"messageId"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// SignalSendOpts 发送选项
type SignalSendOpts struct {
	BaseURL        string
	Account        string
	AccountID      string
	MaxBytes       int
	TextChunkLimit int
	SkipFormatting bool
	MediaURL       string // 出站附件 URL（自动下载并发送）
	TableMode      types.MarkdownTableMode
}

// formatTextStyles 将 TextStyles 转换为 signal-cli 要求的字符串数组格式
// 每个元素为 "start:length:STYLE"，如 "0:5:BOLD"
func formatTextStyles(styles []SignalTextStyleRange) []string {
	result := make([]string, 0, len(styles))
	for _, s := range styles {
		result = append(result, fmt.Sprintf("%d:%d:%s", s.Start, s.Length, s.Style))
	}
	return result
}

// SignalTargetKind 发送目标类型
type SignalTargetKind string

const (
	TargetRecipient SignalTargetKind = "recipient" // signal:+1234567890
	TargetGroup     SignalTargetKind = "group"     // group:abcdef
	TargetUsername  SignalTargetKind = "username"  // username:alice
)

// SignalTarget 解析后的发送目标
type SignalTarget struct {
	Kind  SignalTargetKind
	Value string
}

// ParseTarget 解析发送目标字符串
func ParseTarget(raw string) *SignalTarget {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	// 去掉 "signal:" 前缀
	stripped := trimmed
	lower := strings.ToLower(stripped)
	if strings.HasPrefix(lower, "signal:") {
		stripped = strings.TrimSpace(stripped[len("signal:"):])
		lower = strings.ToLower(stripped)
	}

	switch {
	case strings.HasPrefix(lower, "group:"):
		id := strings.TrimSpace(stripped[len("group:"):])
		if id == "" {
			return nil
		}
		return &SignalTarget{Kind: TargetGroup, Value: id}

	case strings.HasPrefix(lower, "username:"):
		username := strings.TrimSpace(stripped[len("username:"):])
		if username == "" {
			return nil
		}
		return &SignalTarget{Kind: TargetUsername, Value: username}

	case strings.HasPrefix(lower, "u:"):
		username := strings.TrimSpace(stripped[len("u:"):])
		if username == "" {
			return nil
		}
		return &SignalTarget{Kind: TargetUsername, Value: username}
	}

	// 默认视为 recipient（电话号码或 UUID）
	return &SignalTarget{Kind: TargetRecipient, Value: stripped}
}

// buildTargetParams 构建发送目标的 RPC 参数
func buildTargetParams(target *SignalTarget) map[string]interface{} {
	switch target.Kind {
	case TargetGroup:
		return map[string]interface{}{
			"groupId": target.Value,
		}
	case TargetUsername:
		// TS 端使用 { username: [target.username] }（数组格式）
		return map[string]interface{}{
			"username": []string{target.Value},
		}
	case TargetRecipient:
		// TS 端使用 { recipient: [target.recipient] }（数组格式）
		return map[string]interface{}{
			"recipient": []string{target.Value},
		}
	}
	return nil
}

// ResolveMaxBytes 解析最大字节数（对齐 TS: opts → account → global → 8MB 级联）
func ResolveMaxBytes(cfg *types.OpenAcosmiConfig, optsMaxBytes int, accountID string) int {
	if optsMaxBytes > 0 {
		return optsMaxBytes
	}
	account := ResolveSignalAccount(cfg, accountID)
	if account.Config.MediaMaxMB != nil {
		return *account.Config.MediaMaxMB * 1024 * 1024
	}
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil && cfg.Agents.Defaults.MediaMaxMB != nil {
		return *cfg.Agents.Defaults.MediaMaxMB * 1024 * 1024
	}
	return 8 * 1024 * 1024
}

// resolveOutboundAttachment 下载出站附件 URL 并存储为本地文件。
// 对齐 TS send.ts resolveAttachment()
func resolveOutboundAttachment(ctx context.Context, mediaURL string, maxBytes int) (string, string, error) {
	if mediaURL == "" {
		return "", "", nil
	}
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create media request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("media download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return "", "", fmt.Errorf("read media body: %w", err)
	}
	if len(data) > maxBytes {
		return "", "", fmt.Errorf("media exceeds %dMB limit", maxBytes/(1024*1024))
	}
	contentType := resp.Header.Get("Content-Type")
	saved, err := media.SaveMediaBuffer(data, contentType, "outbound", int64(maxBytes), "")
	if err != nil {
		return "", "", fmt.Errorf("save media: %w", err)
	}
	return saved.Path, saved.ContentType, nil
}

// SendMessageSignal 发送 Signal 消息
// 支持纯文本、Markdown 格式化、附件、分块发送
// 返回 (SignalSendResult, error) 对齐 TS send.ts
func SendMessageSignal(ctx context.Context, to string, text string, opts SignalSendOpts) (*SignalSendResult, error) {
	target := ParseTarget(to)
	if target == nil {
		return nil, fmt.Errorf("invalid signal target: %q", to)
	}

	message := text
	var attachments []string

	// 对齐 TS: 出站附件下载
	if opts.MediaURL != "" {
		maxBytes := opts.MaxBytes
		if maxBytes <= 0 {
			maxBytes = 8 * 1024 * 1024
		}
		path, ct, err := resolveOutboundAttachment(ctx, opts.MediaURL, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("resolve attachment: %w", err)
		}
		if path != "" {
			attachments = append(attachments, path)
			// 对齐 TS: 无文本 + 附件时生成占位符
			if message == "" {
				kind := media.MediaKindFromMime(ct)
				if kind != "" {
					message = fmt.Sprintf("<media:%s>", kind)
				} else {
					message = "<media:attachment>"
				}
			}
		}
	}

	if message == "" && len(attachments) == 0 {
		return nil, nil
	}

	// Markdown → Signal 样式转换（对齐 TS: 含 tableMode）
	formatOpts := SignalFormatOpts{TableMode: opts.TableMode}
	var chunks []SignalFormattedText
	if opts.SkipFormatting {
		chunks = []SignalFormattedText{{Text: message}}
	} else if opts.TextChunkLimit > 0 {
		chunks = MarkdownToSignalTextChunks(message, opts.TextChunkLimit)
	} else {
		chunks = []SignalFormattedText{MarkdownToSignalText(message, formatOpts)}
	}

	var lastResult *SignalSendResult
	for i, chunk := range chunks {
		params := buildTargetParams(target)
		params["message"] = chunk.Text
		if len(chunk.TextStyles) > 0 {
			params["text-style"] = formatTextStyles(chunk.TextStyles)
		}
		// 仅第一块附带附件
		if i == 0 && len(attachments) > 0 {
			params["attachments"] = attachments
		}

		raw, err := SignalRpcRequest(ctx, opts.BaseURL, "send", params, opts.Account)
		if err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}
		var resp struct {
			Timestamp int64 `json:"timestamp"`
		}
		if raw != nil {
			_ = json.Unmarshal(raw, &resp)
		}
		lastResult = &SignalSendResult{
			MessageID: fmt.Sprintf("%d", resp.Timestamp),
			Timestamp: resp.Timestamp,
		}
	}
	return lastResult, nil
}

// SendMessageWithAttachment 发送带附件的 Signal 消息
func SendMessageWithAttachment(ctx context.Context, to string, text string, attachmentPath string, opts SignalSendOpts) error {
	target := ParseTarget(to)
	if target == nil {
		return fmt.Errorf("invalid signal target: %q", to)
	}

	params := buildTargetParams(target)
	if text != "" {
		formatted := MarkdownToSignalText(text)
		params["message"] = formatted.Text
		if len(formatted.TextStyles) > 0 {
			params["text-style"] = formatTextStyles(formatted.TextStyles)
		}
	}
	if attachmentPath != "" {
		params["attachments"] = []string{attachmentPath}
	}

	_, err := SignalRpcRequest(ctx, opts.BaseURL, "send", params, opts.Account)
	if err != nil {
		return fmt.Errorf("send attachment: %w", err)
	}
	return nil
}

// SendTypingSignal 发送打字状态
func SendTypingSignal(ctx context.Context, to string, opts SignalSendOpts) error {
	target := ParseTarget(to)
	if target == nil {
		return fmt.Errorf("invalid signal target: %q", to)
	}

	params := buildTargetParams(target)
	_, err := SignalRpcRequest(ctx, opts.BaseURL, "sendTyping", params, opts.Account)
	if err != nil {
		return fmt.Errorf("send typing: %w", err)
	}
	return nil
}

// SendReadReceiptSignal 发送已读回执
func SendReadReceiptSignal(ctx context.Context, to string, timestamp int64, opts SignalSendOpts) error {
	target := ParseTarget(to)
	if target == nil {
		return fmt.Errorf("invalid signal target: %q", to)
	}
	// TS 端: params 使用 targetTimestamp + type，仅允许 recipient 类型
	if target.Kind != TargetRecipient {
		return fmt.Errorf("read receipt requires recipient target, got %s", target.Kind)
	}
	params := buildTargetParams(target)
	params["targetTimestamp"] = timestamp
	params["type"] = "read"

	_, err := SignalRpcRequest(ctx, opts.BaseURL, "sendReceipt", params, opts.Account)
	if err != nil {
		return fmt.Errorf("send read receipt: %w", err)
	}
	return nil
}

// FetchAttachmentSignal 获取附件内容
func FetchAttachmentSignal(ctx context.Context, baseURL string, account string, attachmentID string) (json.RawMessage, error) {
	params := map[string]interface{}{
		"id": attachmentID,
	}
	return SignalRpcRequest(ctx, baseURL, "getAttachment", params, account)
}

// SignalVersion 获取 signal-cli 版本
func SignalVersion(ctx context.Context, baseURL string, account string) (string, error) {
	result, err := SignalRpcRequest(ctx, baseURL, "version", nil, account)
	if err != nil {
		return "", err
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(result, &v); err != nil {
		return "", fmt.Errorf("unmarshal version: %w", err)
	}
	return v.Version, nil
}
