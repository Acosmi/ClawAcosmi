package outbound

import (
	"context"
	"fmt"
)

// ============================================================================
// 顶层发送服务 — 面向外部调用的简洁接口
// 对应 TS: infra/outbound/outbound-send-service.ts + message.ts → sendMessage()
// ============================================================================

// OutboundMessageSender 顶层出站消息发送器接口。
// 对外暴露简洁的 SendMessage / SendPoll 方法，屏蔽内部 plugin/core 分发细节。
type OutboundMessageSender interface {
	// SendMessage 发送文本（含可选媒体）消息。
	SendMessage(ctx context.Context, p OutboundMessageParams) (*TopLevelMessageSendResult, error)
	// SendPoll 发送投票消息。
	SendPoll(ctx context.Context, p OutboundPollParams) (*TopLevelMessagePollResult, error)
}

// OutboundMessageParams 顶层发送消息参数。
// TS 参考: outbound/message.ts → MessageSendParams
type OutboundMessageParams struct {
	To             string
	Content        string
	Channel        string
	MediaURL       string
	MediaURLs      []string
	GifPlayback    bool
	AccountID      string
	DryRun         bool
	BestEffort     bool
	IdempotencyKey string
	Gateway        *OutboundGatewayContext
	Mirror         *MirrorConfig
}

// OutboundPollParams 顶层发送投票参数。
// TS 参考: outbound/message.ts → MessagePollParams
type OutboundPollParams struct {
	To             string
	Question       string
	Options        []string
	MaxSelections  int
	DurationHours  int
	Channel        string
	DryRun         bool
	IdempotencyKey string
	Gateway        *OutboundGatewayContext
}

// ---------- 默认实现 ----------

// DefaultOutboundMessageSender 默认顶层发送器，委托到 SendService。
type DefaultOutboundMessageSender struct {
	svc *SendService
}

// NewDefaultOutboundMessageSender 创建默认顶层发送器。
func NewDefaultOutboundMessageSender(opts SendServiceOpts) *DefaultOutboundMessageSender {
	return &DefaultOutboundMessageSender{
		svc: NewSendService(opts),
	}
}

// SendMessage 实现 OutboundMessageSender.SendMessage。
// 将 OutboundMessageParams 转换为 SendActionParams 并委托到 SendService。
func (s *DefaultOutboundMessageSender) SendMessage(ctx context.Context, p OutboundMessageParams) (*TopLevelMessageSendResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("outbound: send aborted: %w", err)
	}

	via := ViaDirect
	if p.Gateway != nil && p.Gateway.URL != "" {
		via = ViaGateway
	}

	// 收集媒体 URL
	mediaURLs := p.MediaURLs
	if len(mediaURLs) == 0 && p.MediaURL != "" {
		mediaURLs = []string{p.MediaURL}
	}
	var primaryMediaURL *string
	if len(mediaURLs) > 0 {
		u := mediaURLs[0]
		primaryMediaURL = &u
	} else if p.MediaURL != "" {
		primaryMediaURL = &p.MediaURL
	}

	if p.DryRun {
		return &TopLevelMessageSendResult{
			Channel:   p.Channel,
			To:        p.To,
			Via:       via,
			MediaURL:  primaryMediaURL,
			MediaURLs: mediaURLs,
			DryRun:    true,
		}, nil
	}

	sendCtx := OutboundSendContext{
		Channel:   p.Channel,
		AccountID: p.AccountID,
		Gateway:   p.Gateway,
		DryRun:    p.DryRun,
		Mirror:    p.Mirror,
	}

	result, err := s.svc.ExecuteSendAction(ctx, SendActionParams{
		Ctx:         sendCtx,
		To:          p.To,
		Message:     p.Content,
		MediaURL:    p.MediaURL,
		MediaURLs:   p.MediaURLs,
		GifPlayback: p.GifPlayback,
		BestEffort:  p.BestEffort,
	})
	if err != nil {
		return nil, err
	}

	return &TopLevelMessageSendResult{
		Channel:   p.Channel,
		To:        p.To,
		Via:       via,
		MediaURL:  primaryMediaURL,
		MediaURLs: mediaURLs,
		Result:    result.Payload,
	}, nil
}

// SendPoll 实现 OutboundMessageSender.SendPoll。
func (s *DefaultOutboundMessageSender) SendPoll(ctx context.Context, p OutboundPollParams) (*TopLevelMessagePollResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("outbound: send poll aborted: %w", err)
	}

	var durationHours *int
	if p.DurationHours > 0 {
		dh := p.DurationHours
		durationHours = &dh
	}

	if p.DryRun {
		return &TopLevelMessagePollResult{
			Channel:       p.Channel,
			To:            p.To,
			Question:      p.Question,
			Options:       p.Options,
			MaxSelections: p.MaxSelections,
			DurationHours: durationHours,
			Via:           ViaGateway,
			DryRun:        true,
		}, nil
	}

	sendCtx := OutboundSendContext{
		Channel: p.Channel,
		Gateway: p.Gateway,
		DryRun:  p.DryRun,
	}

	result, err := s.svc.ExecutePollAction(ctx, PollActionParams{
		Ctx:           sendCtx,
		To:            p.To,
		Question:      p.Question,
		Options:       p.Options,
		MaxSelections: p.MaxSelections,
		DurationHours: p.DurationHours,
	})
	if err != nil {
		return nil, err
	}

	return &TopLevelMessagePollResult{
		Channel:       p.Channel,
		To:            p.To,
		Question:      p.Question,
		Options:       p.Options,
		MaxSelections: p.MaxSelections,
		DurationHours: durationHours,
		Via:           ViaGateway,
		Result:        result.Payload,
	}, nil
}
