package reply

import (
	"context"
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/route-reply.ts (163L)

// RouteReplyParams 回复路由参数。
type RouteReplyParams struct {
	Payload     autoreply.ReplyPayload
	Channel     string
	To          string
	SessionKey  string
	AccountID   string
	ThreadID    string
	AbortSignal context.Context
	Mirror      *bool // nil = 默认 true（当 SessionKey 非空时）
}

// RouteReplyResult 回复路由结果。
type RouteReplyResult struct {
	OK        bool
	MessageID string
	Error     string
}

// ChannelNormalizer 通道标识标准化接口（DI 接口）。
type ChannelNormalizer interface {
	NormalizeMessageChannel(channel string) string
	NormalizeChannelID(channel string) string
	IsInternalChannel(channel string) bool
}

// OutboundDeliverer 出站投递接口（DI 接口）。
type OutboundDeliverer interface {
	DeliverOutboundPayloads(ctx context.Context, params OutboundDeliverParams) ([]OutboundResult, error)
}

// OutboundDeliverParams 出站投递参数。
type OutboundDeliverParams struct {
	Channel   string
	To        string
	AccountID string
	Payloads  []autoreply.ReplyPayload
	ReplyToID string
	ThreadID  string
	Mirror    *OutboundMirror
}

// OutboundMirror 镜像参数。
type OutboundMirror struct {
	SessionKey string
	AgentID    string
	Text       string
	MediaURLs  []string
}

// OutboundResult 出站结果。
type OutboundResult struct {
	MessageID string
}

// ReplyRouter 回复路由器。
type ReplyRouter struct {
	channelNorm ChannelNormalizer
	deliverer   OutboundDeliverer
}

// NewReplyRouter 创建回复路由器。
func NewReplyRouter(channelNorm ChannelNormalizer, deliverer OutboundDeliverer) *ReplyRouter {
	return &ReplyRouter{
		channelNorm: channelNorm,
		deliverer:   deliverer,
	}
}

// RouteReply 路由回复到指定通道。
// TS 对照: route-reply.ts L57-147
func (r *ReplyRouter) RouteReply(params RouteReplyParams) RouteReplyResult {
	// 标准化回复载荷
	payload := NormalizeReplyPayload(params.Payload, nil)
	if payload == nil {
		return RouteReplyResult{OK: true}
	}

	text := ""
	if payload.Text != "" {
		text = payload.Text
	}
	var mediaURLs []string
	if len(payload.MediaURLs) > 0 {
		for _, u := range payload.MediaURLs {
			if u != "" {
				mediaURLs = append(mediaURLs, u)
			}
		}
	} else if payload.MediaURL != "" {
		mediaURLs = []string{payload.MediaURL}
	}

	// 跳过空回复
	if trimWhitespace(text) == "" && len(mediaURLs) == 0 {
		return RouteReplyResult{OK: true}
	}

	// 内部通道不可路由
	if r.channelNorm.IsInternalChannel(params.Channel) {
		return RouteReplyResult{
			OK:    false,
			Error: "Webchat routing not supported for queued replies",
		}
	}

	channelID := r.channelNorm.NormalizeChannelID(params.Channel)
	if channelID == "" {
		return RouteReplyResult{
			OK:    false,
			Error: fmt.Sprintf("Unknown channel: %s", params.Channel),
		}
	}

	// 检查取消
	if params.AbortSignal != nil {
		select {
		case <-params.AbortSignal.Done():
			return RouteReplyResult{OK: false, Error: "Reply routing aborted"}
		default:
		}
	}

	ctx := params.AbortSignal
	if ctx == nil {
		ctx = context.Background()
	}

	deliverParams := OutboundDeliverParams{
		Channel:   channelID,
		To:        params.To,
		AccountID: params.AccountID,
		Payloads:  []autoreply.ReplyPayload{*payload},
		ThreadID:  params.ThreadID,
	}

	// 设置镜像
	shouldMirror := params.Mirror == nil || *params.Mirror
	if shouldMirror && params.SessionKey != "" {
		deliverParams.Mirror = &OutboundMirror{
			SessionKey: params.SessionKey,
			Text:       text,
			MediaURLs:  mediaURLs,
		}
	}

	results, err := r.deliverer.DeliverOutboundPayloads(ctx, deliverParams)
	if err != nil {
		return RouteReplyResult{
			OK:    false,
			Error: fmt.Sprintf("Failed to route reply to %s: %s", params.Channel, err.Error()),
		}
	}

	var lastMsgID string
	if len(results) > 0 {
		lastMsgID = results[len(results)-1].MessageID
	}
	return RouteReplyResult{OK: true, MessageID: lastMsgID}
}

// IsRoutableChannel 检查通道是否支持路由。
// TS 对照: route-reply.ts L155-162
func (r *ReplyRouter) IsRoutableChannel(channel string) bool {
	if channel == "" {
		return false
	}
	if r.channelNorm.IsInternalChannel(channel) {
		return false
	}
	return r.channelNorm.NormalizeChannelID(channel) != ""
}

// ---------- 默认 stub 实现 ----------

// StubChannelNormalizer 通道标准化 stub。
type StubChannelNormalizer struct{}

func (s StubChannelNormalizer) NormalizeMessageChannel(channel string) string { return channel }
func (s StubChannelNormalizer) NormalizeChannelID(channel string) string      { return channel }
func (s StubChannelNormalizer) IsInternalChannel(_ string) bool               { return false }

// StubOutboundDeliverer 出站投递 stub。
type StubOutboundDeliverer struct{}

func (s StubOutboundDeliverer) DeliverOutboundPayloads(_ context.Context, _ OutboundDeliverParams) ([]OutboundResult, error) {
	return nil, nil
}

// trimWhitespace 去除前后空白。
func trimWhitespace(s string) string {
	result := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			result = append(result, b)
		}
	}
	return string(result)
}
