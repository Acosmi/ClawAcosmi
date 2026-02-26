package outbound

import (
	"context"
	"fmt"
)

// ---------- 出站发送上下文 ----------

// OutboundGatewayContext 出站网关上下文。
type OutboundGatewayContext struct {
	URL               string `json:"url,omitempty"`
	Token             string `json:"token,omitempty"`
	TimeoutMs         int    `json:"timeoutMs,omitempty"`
	ClientName        string `json:"clientName"`
	ClientDisplayName string `json:"clientDisplayName,omitempty"`
	Mode              string `json:"mode"`
}

// OutboundSendContext 出站发送完整上下文。
type OutboundSendContext struct {
	Channel     string
	Params      map[string]interface{}
	AccountID   string
	Gateway     *OutboundGatewayContext
	ToolContext *ToolContext
	DryRun      bool
	Mirror      *MirrorConfig
}

// MirrorConfig 会话镜像配置。
type MirrorConfig struct {
	SessionKey string   `json:"sessionKey"`
	AgentID    string   `json:"agentId,omitempty"`
	Text       string   `json:"text,omitempty"`
	MediaURLs  []string `json:"mediaUrls,omitempty"`
}

// ---------- 发送结果 ----------

// MessageSendResult 消息发送结果。
type MessageSendResult struct {
	Success bool        `json:"success"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// MessagePollResult 投票发送结果。
type MessagePollResult struct {
	Success bool        `json:"success"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ---------- 依赖接口 ----------

// ChannelMessageDispatcher 频道消息分发器（依赖注入）。
type ChannelMessageDispatcher interface {
	// Dispatch 尝试通过频道插件处理消息。
	// 返回 nil 表示插件未处理，应 fallback 到核心发送。
	Dispatch(ctx context.Context, params DispatchParams) (*DispatchResult, error)
}

// DispatchParams 分发参数。
type DispatchParams struct {
	Channel     string
	Action      ChannelMessageActionName
	Params      map[string]interface{}
	AccountID   string
	Gateway     *OutboundGatewayContext
	ToolContext *ToolContext
	DryRun      bool
}

// DispatchResult 分发结果。
type DispatchResult struct {
	Handled bool
	Payload interface{}
}

// CoreMessageSender 核心消息发送器（依赖注入）。
type CoreMessageSender interface {
	Send(ctx context.Context, params CoreSendParams) (*MessageSendResult, error)
	SendPoll(ctx context.Context, params CorePollParams) (*MessagePollResult, error)
}

// CoreSendParams 核心发送参数。
type CoreSendParams struct {
	To          string
	Content     string
	MediaURL    string
	MediaURLs   []string
	Channel     string
	AccountID   string
	ReplyToID   string // 回复目标消息 ID
	ThreadID    string // 线程 ID
	GifPlayback bool
	DryRun      bool
	BestEffort  bool
	Gateway     *OutboundGatewayContext
	Mirror      *MirrorConfig
}

// CorePollParams 核心投票参数。
type CorePollParams struct {
	To            string
	Question      string
	Options       []string
	MaxSelections int
	DurationHours int
	Channel       string
	DryRun        bool
	Gateway       *OutboundGatewayContext
}

// SessionTranscriptAppender 会话记录追加器（依赖注入）。
type SessionTranscriptAppender interface {
	Append(agentID, sessionKey, text string, mediaURLs []string) error
}

// ---------- 出站发送服务 ----------

// SendService 出站消息发送服务。
type SendService struct {
	dispatcher ChannelMessageDispatcher
	sender     CoreMessageSender
	transcript SessionTranscriptAppender
}

// NewSendService 创建出站发送服务。
func NewSendService(opts SendServiceOpts) *SendService {
	return &SendService{
		dispatcher: opts.Dispatcher,
		sender:     opts.Sender,
		transcript: opts.Transcript,
	}
}

// SendServiceOpts 发送服务配置选项。
type SendServiceOpts struct {
	Dispatcher ChannelMessageDispatcher
	Sender     CoreMessageSender
	Transcript SessionTranscriptAppender
}

// SendActionResult 发送动作结果。
type SendActionResult struct {
	HandledBy  string             `json:"handledBy"` // "plugin" | "core"
	Payload    interface{}        `json:"payload,omitempty"`
	SendResult *MessageSendResult `json:"sendResult,omitempty"`
}

// PollActionResult 投票动作结果。
type PollActionResult struct {
	HandledBy  string             `json:"handledBy"`
	Payload    interface{}        `json:"payload,omitempty"`
	PollResult *MessagePollResult `json:"pollResult,omitempty"`
}

// ExecuteSendAction 执行发送动作。
// 先尝试 channel plugin 处理，未处理则 fallback 到核心发送。
func (s *SendService) ExecuteSendAction(ctx context.Context, params SendActionParams) (*SendActionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("message send aborted: %w", err)
	}

	// 尝试 plugin 处理
	if !params.Ctx.DryRun && s.dispatcher != nil {
		dispatched, err := s.dispatcher.Dispatch(ctx, DispatchParams{
			Channel:     params.Ctx.Channel,
			Action:      ActionSend,
			Params:      params.Ctx.Params,
			AccountID:   params.Ctx.AccountID,
			Gateway:     params.Ctx.Gateway,
			ToolContext: params.Ctx.ToolContext,
			DryRun:      params.Ctx.DryRun,
		})
		if err != nil {
			return nil, err
		}
		if dispatched != nil && dispatched.Handled {
			// 镜像到会话记录
			if params.Ctx.Mirror != nil && s.transcript != nil {
				mirrorText := params.Ctx.Mirror.Text
				if mirrorText == "" {
					mirrorText = params.Message
				}
				mirrorMedia := params.Ctx.Mirror.MediaURLs
				if mirrorMedia == nil {
					mirrorMedia = params.MediaURLs
					if mirrorMedia == nil && params.MediaURL != "" {
						mirrorMedia = []string{params.MediaURL}
					}
				}
				_ = s.transcript.Append(
					params.Ctx.Mirror.AgentID,
					params.Ctx.Mirror.SessionKey,
					mirrorText,
					mirrorMedia,
				)
			}
			return &SendActionResult{
				HandledBy: "plugin",
				Payload:   dispatched.Payload,
			}, nil
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("message send aborted: %w", err)
	}

	// Fallback 到核心发送
	if s.sender == nil {
		return nil, fmt.Errorf("no core sender configured")
	}

	result, err := s.sender.Send(ctx, CoreSendParams{
		To:          params.To,
		Content:     params.Message,
		MediaURL:    params.MediaURL,
		MediaURLs:   params.MediaURLs,
		Channel:     params.Ctx.Channel,
		AccountID:   params.Ctx.AccountID,
		GifPlayback: params.GifPlayback,
		DryRun:      params.Ctx.DryRun,
		BestEffort:  params.BestEffort,
		Gateway:     params.Ctx.Gateway,
		Mirror:      params.Ctx.Mirror,
	})
	if err != nil {
		return nil, err
	}

	return &SendActionResult{
		HandledBy:  "core",
		Payload:    result,
		SendResult: result,
	}, nil
}

// SendActionParams 发送动作参数。
type SendActionParams struct {
	Ctx         OutboundSendContext
	To          string
	Message     string
	MediaURL    string
	MediaURLs   []string
	GifPlayback bool
	BestEffort  bool
}

// ExecutePollAction 执行投票动作。
func (s *SendService) ExecutePollAction(ctx context.Context, params PollActionParams) (*PollActionResult, error) {
	// 尝试 plugin 处理
	if !params.Ctx.DryRun && s.dispatcher != nil {
		dispatched, err := s.dispatcher.Dispatch(ctx, DispatchParams{
			Channel:     params.Ctx.Channel,
			Action:      ActionPoll,
			Params:      params.Ctx.Params,
			AccountID:   params.Ctx.AccountID,
			Gateway:     params.Ctx.Gateway,
			ToolContext: params.Ctx.ToolContext,
			DryRun:      params.Ctx.DryRun,
		})
		if err != nil {
			return nil, err
		}
		if dispatched != nil && dispatched.Handled {
			return &PollActionResult{
				HandledBy: "plugin",
				Payload:   dispatched.Payload,
			}, nil
		}
	}

	// Fallback 到核心发送
	if s.sender == nil {
		return nil, fmt.Errorf("no core sender configured")
	}

	result, err := s.sender.SendPoll(ctx, CorePollParams{
		To:            params.To,
		Question:      params.Question,
		Options:       params.Options,
		MaxSelections: params.MaxSelections,
		DurationHours: params.DurationHours,
		Channel:       params.Ctx.Channel,
		DryRun:        params.Ctx.DryRun,
		Gateway:       params.Ctx.Gateway,
	})
	if err != nil {
		return nil, err
	}

	return &PollActionResult{
		HandledBy:  "core",
		Payload:    result,
		PollResult: result,
	}, nil
}

// PollActionParams 投票动作参数。
type PollActionParams struct {
	Ctx           OutboundSendContext
	To            string
	Question      string
	Options       []string
	MaxSelections int
	DurationHours int
}
