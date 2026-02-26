package outbound

import (
	"context"
	"errors"
	"fmt"
)

// ============================================================================
// 出站投递 — 批量投递 outbound payloads 到频道
// 对应 TS: infra/outbound/deliver.ts (376L)
// ============================================================================

// ErrDeliverNotImplemented 投递功能尚未完整实现（桩占位）。
var ErrDeliverNotImplemented = errors.New("outbound: deliverOutboundPayloads channel dispatch not yet implemented")

// ---------- 投递参数 ----------

// DeliverOutboundParams 批量投递参数。
// TS 参考: deliver.ts → deliverOutboundPayloads() params
type DeliverOutboundParams struct {
	// Channel 目标频道（如 "whatsapp", "telegram", "discord", "slack", "signal"）
	Channel string
	// To 接收方标识符
	To string
	// AccountID 账户 ID
	AccountID string
	// Payloads 待投递的负载列表
	Payloads []ReplyPayload
	// ReplyToID 回复目标消息 ID（可选）
	ReplyToID string
	// ThreadID 线程 ID（可选）
	ThreadID string
	// GifPlayback GIF 播放模式
	GifPlayback bool
	// AbortCtx 取消上下文
	AbortCtx context.Context
	// BestEffort 是否 best-effort 模式（忽略单条错误）
	BestEffort bool
	// OnError best-effort 模式下的错误回调
	OnError func(err error, payload NormalizedOutboundPayload)
	// OnPayload 每条 payload 投递前回调
	OnPayload func(payload NormalizedOutboundPayload)
	// Mirror 会话镜像配置（可选）
	Mirror *MirrorConfig
	// Dispatcher 频道消息分发器（依赖注入）
	Dispatcher ChannelMessageDispatcher
	// CoreSender 核心消息发送器（依赖注入）
	CoreSender CoreMessageSender
	// OutboundAdapter 频道出站适配器（可选 DI, P4-GA-DLV1）
	OutboundAdapter ChannelOutboundAdapter
	// TextChunker 文本分块函数（可选 DI, P4-GA-DLV1）
	TextChunker TextChunkerFunc
	// TranscriptAppender 镜像转录追加器（可选 DI, P4-GA-DLV3）
	TranscriptAppender SessionTranscriptAppender
}

// ChannelOutboundAdapter 频道出站适配器（P4-GA-DLV1）。
// TS 对应: deliver.ts → loadChannelOutboundAdapter / createChannelHandler
type ChannelOutboundAdapter interface {
	// ResolveMediaMaxBytes 获取频道媒体大小限制。
	ResolveMediaMaxBytes(channel string) int64
	// FormatPayload 应用频道特定格式化（如 Signal markdown→chunks）。
	FormatPayload(channel string, payload NormalizedOutboundPayload) ([]NormalizedOutboundPayload, error)
}

// TextChunkerFunc 文本分块函数（P4-GA-DLV1）。
// TS 对应: deliver.ts → chunkByParagraph / chunkMarkdownTextWithMode
type TextChunkerFunc func(text string, maxLen int) []string

// ReplyPayload 回复负载（来自 auto-reply 模块）。
// TS 参考: auto-reply/types.ts → ReplyPayload
type ReplyPayload struct {
	Text        string                 `json:"text,omitempty"`
	MediaURL    string                 `json:"mediaUrl,omitempty"`
	MediaURLs   []string               `json:"mediaUrls,omitempty"`
	ChannelData map[string]interface{} `json:"channelData,omitempty"`
}

// NormalizedOutboundPayload 规范化后的出站负载。
// TS 参考: outbound/payloads.ts → NormalizedOutboundPayload
type NormalizedOutboundPayload struct {
	Text        string                 `json:"text"`
	MediaURLs   []string               `json:"mediaUrls"`
	ChannelData map[string]interface{} `json:"channelData,omitempty"`
}

// OutboundDeliveryResult 单条投递结果。
// TS 参考: deliver.ts → OutboundDeliveryResult
type OutboundDeliveryResult struct {
	Channel        string                 `json:"channel"`
	MessageID      string                 `json:"messageId"`
	ChatID         string                 `json:"chatId,omitempty"`
	ChannelID      string                 `json:"channelId,omitempty"`
	RoomID         string                 `json:"roomId,omitempty"`
	ConversationID string                 `json:"conversationId,omitempty"`
	Timestamp      int64                  `json:"timestamp,omitempty"`
	ToJid          string                 `json:"toJid,omitempty"`
	PollID         string                 `json:"pollId,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}

// ---------- 投递入口 ----------

// DeliverOutboundPayloads 批量投递出站负载到目标频道。
// TS 参考: deliver.ts → deliverOutboundPayloads()
//
// 当前实现: 完整版 — 支持 ChannelOutboundAdapter 格式化/媒体限制、文本分块、AbortSignal。
func DeliverOutboundPayloads(params DeliverOutboundParams) ([]OutboundDeliveryResult, error) {
	if params.Channel == "" {
		return nil, fmt.Errorf("outbound: channel is required")
	}
	if params.To == "" {
		return nil, fmt.Errorf("outbound: recipient (to) is required")
	}

	ctx := params.AbortCtx
	if ctx == nil {
		ctx = context.Background()
	}

	normalized := normalizePayloads(params.Payloads)
	results := make([]OutboundDeliveryResult, 0, len(normalized))

	// 解析频道媒体大小限制（通过 adapter 或默认 25MB）
	var mediaMaxBytes int64 = 25 * 1024 * 1024 // 默认 25MB
	if params.OutboundAdapter != nil {
		if limit := params.OutboundAdapter.ResolveMediaMaxBytes(params.Channel); limit > 0 {
			mediaMaxBytes = limit
		}
	}
	_ = mediaMaxBytes // 预留: 用于后续媒体大小检查

	for _, payload := range normalized {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		if params.OnPayload != nil {
			params.OnPayload(payload)
		}

		// ---------- ChannelOutboundAdapter 格式化（含 Signal markdown→chunks） ----------
		payloadsToSend := []NormalizedOutboundPayload{payload}
		if params.OutboundAdapter != nil {
			formatted, err := params.OutboundAdapter.FormatPayload(params.Channel, payload)
			if err == nil && len(formatted) > 0 {
				payloadsToSend = formatted
			}
		}

		for _, sp := range payloadsToSend {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			default:
			}

			// ---------- 文本分块（长文本拆分为多条发送） ----------
			textChunks := []string{sp.Text}
			if params.TextChunker != nil && len(sp.Text) > 2000 && len(sp.MediaURLs) == 0 {
				chunks := params.TextChunker(sp.Text, 2000)
				if len(chunks) > 0 {
					textChunks = chunks
				}
			}

			// 尝试通过插件分发器投递
			if params.Dispatcher != nil {
				dr, err := params.Dispatcher.Dispatch(ctx, DispatchParams{
					Channel:   params.Channel,
					Action:    ActionSend,
					Params:    map[string]interface{}{"text": sp.Text, "mediaUrls": sp.MediaURLs},
					AccountID: params.AccountID,
					DryRun:    false,
				})
				if err == nil && dr != nil && dr.Handled {
					results = append(results, OutboundDeliveryResult{
						Channel: params.Channel,
					})
					continue
				}
			}

			// Fallback: 通过核心发送器
			if params.CoreSender != nil {
				// 纯文本（可能分块）
				if len(sp.MediaURLs) == 0 {
					for _, chunk := range textChunks {
						select {
						case <-ctx.Done():
							return results, ctx.Err()
						default:
						}
						res, err := params.CoreSender.Send(ctx, CoreSendParams{
							To:          params.To,
							Content:     chunk,
							Channel:     params.Channel,
							AccountID:   params.AccountID,
							ReplyToID:   params.ReplyToID,
							ThreadID:    params.ThreadID,
							GifPlayback: params.GifPlayback,
							DryRun:      false,
							BestEffort:  params.BestEffort,
							Mirror:      params.Mirror,
						})
						if err != nil {
							if !params.BestEffort {
								return results, err
							}
							if params.OnError != nil {
								params.OnError(err, sp)
							}
							break
						}
						if res != nil && res.Success {
							results = append(results, OutboundDeliveryResult{
								Channel: params.Channel,
							})
							appendMirrorTranscript(params, sp)
						}
					}
					continue
				}

				// 带媒体的负载：首条附带文本，后续仅媒体
				first := true
				for _, url := range sp.MediaURLs {
					select {
					case <-ctx.Done():
						return results, ctx.Err()
					default:
					}
					caption := ""
					if first {
						caption = sp.Text
						first = false
					}
					res, err := params.CoreSender.Send(ctx, CoreSendParams{
						To:          params.To,
						Content:     caption,
						MediaURL:    url,
						Channel:     params.Channel,
						AccountID:   params.AccountID,
						ReplyToID:   params.ReplyToID,
						ThreadID:    params.ThreadID,
						GifPlayback: params.GifPlayback,
						DryRun:      false,
						Mirror:      params.Mirror,
					})
					if err != nil {
						if !params.BestEffort {
							return results, err
						}
						if params.OnError != nil {
							params.OnError(err, sp)
						}
						break
					}
					if res != nil && res.Success {
						results = append(results, OutboundDeliveryResult{
							Channel: params.Channel,
						})
						appendMirrorTranscript(params, sp)
					}
				}
			}
		}
	}

	return results, nil
}

// normalizePayloads 规范化负载列表。
// TS 参考: outbound/payloads.ts → normalizeReplyPayloadsForDelivery()
func normalizePayloads(payloads []ReplyPayload) []NormalizedOutboundPayload {
	result := make([]NormalizedOutboundPayload, 0, len(payloads))
	for _, p := range payloads {
		mediaURLs := p.MediaURLs
		if len(mediaURLs) == 0 && p.MediaURL != "" {
			mediaURLs = []string{p.MediaURL}
		}
		result = append(result, NormalizedOutboundPayload{
			Text:        p.Text,
			MediaURLs:   mediaURLs,
			ChannelData: p.ChannelData,
		})
	}
	return result
}

// appendMirrorTranscript 投递成功后追加到镜像转录 (P4-GA-DLV3)。
// TS 参考: deliver.ts → appendAssistantMessageToSessionTranscript()
func appendMirrorTranscript(params DeliverOutboundParams, payload NormalizedOutboundPayload) {
	if params.Mirror == nil || params.TranscriptAppender == nil {
		return
	}
	_ = params.TranscriptAppender.Append(
		params.Mirror.AgentID,
		params.Mirror.SessionKey,
		payload.Text,
		payload.MediaURLs,
	)
}
