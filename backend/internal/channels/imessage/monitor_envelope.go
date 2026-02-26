//go:build darwin

package imessage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 信封格式化 — 对标 TS auto-reply/envelope.ts + monitor-provider.ts

// IMessageReplyContext 引用消息上下文
type IMessageReplyContext struct {
	ID     string
	Body   string
	Sender string
}

// DescribeReplyContext 从入站消息中提取引用消息上下文。
// TS 对照: monitor-provider.ts describeReplyContext()
func DescribeReplyContext(msg *IMessagePayload) *IMessageReplyContext {
	if msg == nil {
		return nil
	}
	body := normalizeReplyField(msg.ReplyToText)
	if body == "" {
		return nil
	}
	id := normalizeReplyFieldRaw(msg.ReplyToID)
	sender := normalizeReplyField(msg.ReplyToSender)
	return &IMessageReplyContext{
		ID:     id,
		Body:   body,
		Sender: sender,
	}
}

// normalizeReplyField 规范化字符串指针字段
func normalizeReplyField(value *string) string {
	if value == nil {
		return ""
	}
	trimmed := strings.TrimSpace(*value)
	return trimmed
}

// normalizeReplyFieldRaw 规范化 json.RawMessage 字段（支持 string | number | null）
func normalizeReplyFieldRaw(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("%g", n)
	}
	return ""
}

// FormatInboundFromLabel 格式化入站消息的 from 标签。
// TS 对照: auto-reply/envelope.ts formatInboundFromLabel()
func FormatInboundFromLabel(params FormatFromLabelParams) string {
	if params.IsGroup {
		label := strings.TrimSpace(params.GroupLabel)
		if label == "" {
			label = params.GroupFallback
		}
		if label == "" {
			label = "Group"
		}
		groupID := strings.TrimSpace(params.GroupID)
		if groupID != "" && groupID != label && !strings.Contains(label, groupID) {
			label = label + " id:" + groupID
		}
		return label
	}
	label := strings.TrimSpace(params.DirectLabel)
	if label == "" {
		label = strings.TrimSpace(params.DirectID)
	}
	return label
}

// FormatFromLabelParams from 标签格式化参数
type FormatFromLabelParams struct {
	IsGroup       bool
	GroupLabel    string
	GroupID       string
	GroupFallback string
	DirectLabel   string
	DirectID      string
}

// FormatInboundEnvelope 格式化入站消息信封。
// TS 对照: auto-reply/envelope.ts formatInboundEnvelope()
func FormatInboundEnvelope(params FormatEnvelopeParams) string {
	var parts []string

	// 频道标记
	if params.Channel != "" {
		parts = append(parts, fmt.Sprintf("[%s]", params.Channel))
	}

	// 发送者
	if params.From != "" {
		parts = append(parts, params.From)
	}

	// 时间戳
	if params.Timestamp != nil {
		ts := time.UnixMilli(*params.Timestamp)
		parts = append(parts, ts.Format("15:04"))
	}

	// 聊天类型
	if params.ChatType != "" {
		parts = append(parts, fmt.Sprintf("(%s)", params.ChatType))
	}

	header := strings.Join(parts, " ")

	// 发送者标签（用于群组历史条目）
	if params.SenderLabel != "" {
		header = params.SenderLabel
		if params.Channel != "" {
			header = fmt.Sprintf("[%s] %s", params.Channel, header)
		}
		if params.Timestamp != nil {
			ts := time.UnixMilli(*params.Timestamp)
			header = header + " " + ts.Format("15:04")
		}
	}

	body := strings.TrimSpace(params.Body)
	if body == "" {
		return header
	}

	return header + ": " + body
}

// FormatEnvelopeParams 信封格式化参数
type FormatEnvelopeParams struct {
	Channel           string
	From              string
	Timestamp         *int64
	Body              string
	ChatType          string
	SenderLabel       string // 优先于 From（用于历史条目）
	PreviousTimestamp *int64
}

// FormatReplySuffix 格式化引用消息后缀
func FormatReplySuffix(replyCtx *IMessageReplyContext) string {
	if replyCtx == nil || replyCtx.Body == "" {
		return ""
	}
	sender := replyCtx.Sender
	if sender == "" {
		sender = "unknown sender"
	}
	idPart := ""
	if replyCtx.ID != "" {
		idPart = fmt.Sprintf(" id:%s", replyCtx.ID)
	}
	return fmt.Sprintf("\n\n[Replying to %s%s]\n%s\n[/Replying]", sender, idPart, replyCtx.Body)
}
