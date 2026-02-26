package outbound

// ============================================================================
// 出站消息类型定义
// 对应 TS: infra/outbound/payloads.ts, format.ts, envelope.ts, targets.ts
// ============================================================================

// ---------- Payload JSON 表示（用于序列化/API 响应） ----------

// OutboundPayloadJson 出站 payload 的 JSON 序列化形式。
// TS 参考: outbound/payloads.ts → OutboundPayloadJson
type OutboundPayloadJson struct {
	Text        string                 `json:"text"`
	MediaURL    *string                `json:"mediaUrl"`
	MediaURLs   []string               `json:"mediaUrls,omitempty"`
	ChannelData map[string]interface{} `json:"channelData,omitempty"`
}

// ---------- 投递 JSON 结果（对外暴露） ----------

// OutboundDeliveryVia 投递方式。
type OutboundDeliveryVia string

const (
	ViaDirect  OutboundDeliveryVia = "direct"
	ViaGateway OutboundDeliveryVia = "gateway"
)

// OutboundDeliveryJson 完整投递结果的 JSON 表示。
// TS 参考: outbound/format.ts → OutboundDeliveryJson
type OutboundDeliveryJson struct {
	Channel        string                 `json:"channel"`
	Via            OutboundDeliveryVia    `json:"via"`
	To             string                 `json:"to"`
	MessageID      string                 `json:"messageId"`
	MediaURL       *string                `json:"mediaUrl"`
	ChatID         string                 `json:"chatId,omitempty"`
	ChannelID      string                 `json:"channelId,omitempty"`
	RoomID         string                 `json:"roomId,omitempty"`
	ConversationID string                 `json:"conversationId,omitempty"`
	Timestamp      int64                  `json:"timestamp,omitempty"`
	ToJid          string                 `json:"toJid,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}

// ---------- 结果信封（包装 payloads + delivery） ----------

// OutboundResultEnvelope 出站结果信封。
// TS 参考: outbound/envelope.ts → OutboundResultEnvelope
type OutboundResultEnvelope struct {
	Payloads []OutboundPayloadJson `json:"payloads,omitempty"`
	Meta     interface{}           `json:"meta,omitempty"`
	Delivery *OutboundDeliveryJson `json:"delivery,omitempty"`
}

// BuildOutboundResultEnvelope 构建出站结果信封。
// 当仅有 delivery 且 flattenDelivery 为 true 时直接返回 delivery（扁平化）。
// TS 参考: outbound/envelope.ts → buildOutboundResultEnvelope()
func BuildOutboundResultEnvelope(params BuildEnvelopeParams) interface{} {
	if params.FlattenDelivery && params.Delivery != nil && params.Meta == nil && params.Payloads == nil {
		return params.Delivery
	}
	env := &OutboundResultEnvelope{}
	if params.Payloads != nil {
		env.Payloads = params.Payloads
	}
	if params.Meta != nil {
		env.Meta = params.Meta
	}
	if params.Delivery != nil {
		env.Delivery = params.Delivery
	}
	return env
}

// BuildEnvelopeParams 构建信封的参数。
type BuildEnvelopeParams struct {
	Payloads        []OutboundPayloadJson
	Meta            interface{}
	Delivery        *OutboundDeliveryJson
	FlattenDelivery bool
}

// ---------- 投递通道类型 ----------

// OutboundChannel 出站频道（包含 "none" 占位）。
type OutboundChannel string

const (
	OutboundChannelNone OutboundChannel = "none"
)

// ---------- 出站目标 ----------

// OutboundTarget 完整的出站目标描述。
// TS 参考: outbound/targets.ts → OutboundTarget
type OutboundTarget struct {
	Channel       OutboundChannel `json:"channel"`
	To            string          `json:"to,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	AccountID     string          `json:"accountId,omitempty"`
	LastChannel   OutboundChannel `json:"lastChannel,omitempty"`
	LastAccountID string          `json:"lastAccountId,omitempty"`
}

// OutboundTargetResolution 目标解析结果（ok/error 二选一）。
// TS 参考: outbound/targets.ts → OutboundTargetResolution
type OutboundTargetResolution struct {
	OK    bool
	To    string
	Error error
}

// ---------- 会话投递目标 ----------

// ChannelOutboundTargetMode 频道出站目标模式。
type ChannelOutboundTargetMode string

const (
	TargetModeExplicit  ChannelOutboundTargetMode = "explicit"
	TargetModeImplicit  ChannelOutboundTargetMode = "implicit"
	TargetModeHeartbeat ChannelOutboundTargetMode = "heartbeat"
)

// SessionDeliveryTarget 会话投递目标。
// TS 参考: outbound/targets.ts → SessionDeliveryTarget
type SessionDeliveryTarget struct {
	Channel       OutboundChannel           `json:"channel,omitempty"`
	To            string                    `json:"to,omitempty"`
	AccountID     string                    `json:"accountId,omitempty"`
	ThreadID      string                    `json:"threadId,omitempty"`
	Mode          ChannelOutboundTargetMode `json:"mode"`
	LastChannel   OutboundChannel           `json:"lastChannel,omitempty"`
	LastTo        string                    `json:"lastTo,omitempty"`
	LastAccountID string                    `json:"lastAccountId,omitempty"`
	LastThreadID  string                    `json:"lastThreadId,omitempty"`
}

// ---------- 发送结果（外部格式，对应 message.ts 中的 MessageSendResult） ----------

// TopLevelMessageSendResult 顶层发送结果（CLI/Gateway 场景）。
// TS 参考: outbound/message.ts → MessageSendResult
type TopLevelMessageSendResult struct {
	Channel   string              `json:"channel"`
	To        string              `json:"to"`
	Via       OutboundDeliveryVia `json:"via"`
	MediaURL  *string             `json:"mediaUrl"`
	MediaURLs []string            `json:"mediaUrls,omitempty"`
	Result    interface{}         `json:"result,omitempty"`
	DryRun    bool                `json:"dryRun,omitempty"`
}

// TopLevelMessagePollResult 顶层投票结果（CLI/Gateway 场景）。
// TS 参考: outbound/message.ts → MessagePollResult
type TopLevelMessagePollResult struct {
	Channel       string              `json:"channel"`
	To            string              `json:"to"`
	Question      string              `json:"question"`
	Options       []string            `json:"options"`
	MaxSelections int                 `json:"maxSelections"`
	DurationHours *int                `json:"durationHours"`
	Via           OutboundDeliveryVia `json:"via"`
	Result        interface{}         `json:"result,omitempty"`
	DryRun        bool                `json:"dryRun,omitempty"`
}

// ---------- 格式化工具 ----------

// FormatOutboundDeliverySummary 格式化投递结果摘要文字。
// TS 参考: outbound/format.ts → formatOutboundDeliverySummary()
func FormatOutboundDeliverySummary(channel string, result *OutboundDeliveryResult) string {
	if result == nil {
		return "Sent via " + channel + ". Message ID: unknown"
	}
	base := "Sent via " + result.Channel + ". Message ID: " + result.MessageID
	switch {
	case result.ChatID != "":
		return base + " (chat " + result.ChatID + ")"
	case result.ChannelID != "":
		return base + " (channel " + result.ChannelID + ")"
	case result.RoomID != "":
		return base + " (room " + result.RoomID + ")"
	case result.ConversationID != "":
		return base + " (conversation " + result.ConversationID + ")"
	}
	return base
}

// BuildOutboundDeliveryJson 将 OutboundDeliveryResult 转换为 JSON 可序列化结构。
// TS 参考: outbound/format.ts → buildOutboundDeliveryJson()
func BuildOutboundDeliveryJson(channel, to string, via OutboundDeliveryVia, mediaURL *string, result *OutboundDeliveryResult) OutboundDeliveryJson {
	if via == "" {
		via = ViaDirect
	}
	j := OutboundDeliveryJson{
		Channel:  channel,
		Via:      via,
		To:       to,
		MediaURL: mediaURL,
	}
	if result != nil {
		if result.MessageID != "" {
			j.MessageID = result.MessageID
		} else {
			j.MessageID = "unknown"
		}
		j.ChatID = result.ChatID
		j.ChannelID = result.ChannelID
		j.RoomID = result.RoomID
		j.ConversationID = result.ConversationID
		if result.Timestamp != 0 {
			j.Timestamp = result.Timestamp
		}
		j.ToJid = result.ToJid
		j.Meta = result.Meta
	} else {
		j.MessageID = "unknown"
	}
	return j
}

// NormalizePayloadsToJson 将规范化 payload 列表转为 JSON 格式列表。
// TS 参考: outbound/payloads.ts → normalizeOutboundPayloadsForJson()
func NormalizePayloadsToJson(payloads []NormalizedOutboundPayload) []OutboundPayloadJson {
	result := make([]OutboundPayloadJson, 0, len(payloads))
	for _, p := range payloads {
		var mediaURL *string
		if len(p.MediaURLs) > 0 {
			u := p.MediaURLs[0]
			mediaURL = &u
		}
		result = append(result, OutboundPayloadJson{
			Text:        p.Text,
			MediaURL:    mediaURL,
			MediaURLs:   p.MediaURLs,
			ChannelData: p.ChannelData,
		})
	}
	return result
}

// FormatOutboundPayloadLog 格式化 payload 用于日志输出。
// TS 参考: outbound/payloads.ts → formatOutboundPayloadLog()
func FormatOutboundPayloadLog(payload NormalizedOutboundPayload) string {
	lines := make([]string, 0, 1+len(payload.MediaURLs))
	if payload.Text != "" {
		// trim trailing whitespace
		text := payload.Text
		for len(text) > 0 && (text[len(text)-1] == ' ' || text[len(text)-1] == '\t' || text[len(text)-1] == '\n' || text[len(text)-1] == '\r') {
			text = text[:len(text)-1]
		}
		lines = append(lines, text)
	}
	for _, url := range payload.MediaURLs {
		lines = append(lines, "MEDIA:"+url)
	}
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for _, l := range lines[1:] {
		result += "\n" + l
	}
	return result
}
