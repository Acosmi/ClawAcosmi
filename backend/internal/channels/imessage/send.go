//go:build darwin

package imessage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropic/open-acosmi/internal/media"
	"github.com/anthropic/open-acosmi/pkg/markdown"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// iMessage 消息发送 — 继承自 src/imessage/send.ts (141L)

// IMessageSendOpts 发送选项
type IMessageSendOpts struct {
	CliPath   string
	DbPath    string
	Service   IMessageService
	Region    string
	AccountID string
	MediaUrl  string
	MaxBytes  int
	TimeoutMs int
	ChatID    *int
	Client    *IMessageRpcClient
	LogError  func(string) // H10: 可选日志回调，媒体解析失败时记录警告
}

// IMessageSendResult 发送结果
type IMessageSendResult struct {
	MessageID string
}

// resolveMessageId 从 RPC 响应中提取消息 ID
func resolveMessageId(result json.RawMessage) string {
	if len(result) == 0 || string(result) == "null" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		return ""
	}

	// 按优先级尝试提取字符串或数字 ID
	for _, key := range []string{"messageId", "message_id", "id", "guid"} {
		raw, ok := m[key]
		if !ok || len(raw) == 0 {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			s = strings.TrimSpace(s)
			if s != "" {
				return s
			}
			continue
		}
		var n float64
		if err := json.Unmarshal(raw, &n); err == nil {
			return fmt.Sprintf("%g", n)
		}
	}
	return ""
}

// resolveAttachment 解析媒体附件（下载+保存）
// 使用 media.SaveMediaSource 从 URL 或本地路径保存媒体文件。
func resolveAttachment(mediaUrl string, maxBytes int) (string, string, error) {
	saved, err := media.SaveMediaSource(mediaUrl)
	if err != nil {
		return "", "", fmt.Errorf("resolve attachment: %w", err)
	}
	return saved.Path, saved.ContentType, nil
}

// SendMessageIMessage 发送 iMessage 消息
func SendMessageIMessage(ctx context.Context, to string, text string, opts IMessageSendOpts, cfg *types.OpenAcosmiConfig) (*IMessageSendResult, error) {
	account := ResolveIMessageAccount(cfg, opts.AccountID)
	cliPath := strings.TrimSpace(opts.CliPath)
	if cliPath == "" {
		cliPath = strings.TrimSpace(account.Config.CliPath)
	}
	if cliPath == "" {
		cliPath = "imsg"
	}
	dbPath := strings.TrimSpace(opts.DbPath)
	if dbPath == "" {
		dbPath = strings.TrimSpace(account.Config.DbPath)
	}

	// 解析目标
	var target *IMessageTarget
	var err error
	if opts.ChatID != nil {
		target, err = ParseIMessageTarget(FormatIMessageChatTarget(opts.ChatID))
	} else {
		target, err = ParseIMessageTarget(to)
	}
	if err != nil {
		return nil, fmt.Errorf("parse target: %w", err)
	}

	// 解析 service
	service := opts.Service
	if service == "" && target.Kind == TargetKindHandle {
		service = target.Service
	}
	if service == "" {
		service = IMessageService(account.Config.Service)
	}

	// 解析 region
	region := strings.TrimSpace(opts.Region)
	if region == "" {
		region = strings.TrimSpace(account.Config.Region)
	}
	if region == "" {
		region = "US"
	}

	// 解析 maxBytes
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		if account.Config.MediaMaxMB != nil {
			maxBytes = *account.Config.MediaMaxMB * 1024 * 1024
		} else {
			maxBytes = 16 * 1024 * 1024
		}
	}

	message := text
	var filePath string

	// 处理媒体附件
	if mediaUrl := strings.TrimSpace(opts.MediaUrl); mediaUrl != "" {
		path, contentType, err := resolveAttachment(mediaUrl, maxBytes)
		if err != nil {
			// H10: 媒体解析失败不阻塞文本发送，记录警告
			if opts.LogError != nil {
				opts.LogError(fmt.Sprintf("imessage: media resolution failed (url=%s): %s", mediaUrl, err))
			}
		} else {
			filePath = path
			if strings.TrimSpace(message) == "" {
				kind := mediaKindFromMime(contentType)
				if kind != "" {
					if kind == "image" {
						message = "<media:image>"
					} else {
						message = fmt.Sprintf("<media:%s>", kind)
					}
				}
			}
		}
	}

	if strings.TrimSpace(message) == "" && filePath == "" {
		return nil, fmt.Errorf("iMessage send requires text or media")
	}

	// 表格转换: 将 Markdown 表格转为纯文本格式
	// TS 对照: send.ts convertMarkdownTables(message, tableMode)
	tableMode := markdown.TableMode(resolveIMessageTableMode(cfg))
	message = markdown.ConvertMarkdownTables(message, tableMode)

	// 构建 RPC 参数
	params := map[string]interface{}{
		"text":    message,
		"service": string(service),
		"region":  region,
	}
	if service == "" {
		params["service"] = "auto"
	}
	if filePath != "" {
		params["file"] = filePath
	}

	switch target.Kind {
	case TargetKindChatID:
		params["chat_id"] = target.ChatID
	case TargetKindChatGUID:
		params["chat_guid"] = target.ChatGUID
	case TargetKindChatIdentifier:
		params["chat_identifier"] = target.ChatIdentifier
	default:
		params["to"] = target.To
	}

	// 使用现有 client 或创建新的
	client := opts.Client
	shouldClose := false
	if client == nil {
		var err error
		client, err = CreateIMessageRpcClient(ctx, IMessageRpcClientOptions{
			CliPath: cliPath,
			DbPath:  dbPath,
		})
		if err != nil {
			return nil, fmt.Errorf("create rpc client: %w", err)
		}
		shouldClose = true
	}
	if shouldClose {
		defer client.Stop()
	}

	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultProbeTimeoutMs
	}

	result, err := client.Request(ctx, "send", params, timeoutMs)
	if err != nil {
		return nil, fmt.Errorf("imsg send: %w", err)
	}

	resolvedID := resolveMessageId(result)
	if resolvedID == "" {
		// 检查是否有 ok 字段
		var okResp struct {
			OK string `json:"ok"`
		}
		if json.Unmarshal(result, &okResp) == nil && okResp.OK != "" {
			resolvedID = "ok"
		} else {
			resolvedID = "unknown"
		}
	}

	return &IMessageSendResult{MessageID: resolvedID}, nil
}

// mediaKindFromMime 从 MIME 类型推断媒体类别，委托 media.MediaKindFromMime。
func mediaKindFromMime(mime string) string {
	kind := media.MediaKindFromMime(mime)
	if kind == media.KindUnknown {
		return "file"
	}
	return string(kind)
}

// resolveIMessageTableMode 从 config 解析 iMessage 表格模式。
// TS 对照: send.ts resolveMarkdownTableMode({cfg, channel: "imessage"})
// iMessage 默认使用 bullets，因为原生不支持 Markdown 表格。
func resolveIMessageTableMode(cfg *types.OpenAcosmiConfig) types.MarkdownTableMode {
	if cfg != nil && cfg.Markdown != nil && cfg.Markdown.Tables != "" {
		return cfg.Markdown.Tables
	}
	return types.MarkdownTableBullets
}
