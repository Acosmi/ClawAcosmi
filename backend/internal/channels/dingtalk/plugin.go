package dingtalk

// plugin.go — 钉钉频道插件（Stream SDK 模式）
// 实现 channels.Plugin 接口
// 使用 dingtalk-stream-sdk-go 建立长连接接收消息

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	dingtalkstream "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"

	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// DingTalkPlugin 钉钉频道插件
type DingTalkPlugin struct {
	config *types.OpenAcosmiConfig
	mu     sync.Mutex

	streams map[string]*DingTalkStreamClient
	senders map[string]*DingTalkSender

	// DispatchFunc 消息分发回调 — 由 gateway 注入，路由到 autoreply 管线
	DispatchFunc func(ctx context.Context, channel, accountID, chatID, userID, text string) string

	// DispatchMultimodalFunc 多模态消息分发回调（Phase A 新增）
	// 优先使用：如未设置则回退 DispatchFunc(text)
	DispatchMultimodalFunc channels.DispatchMultimodalFunc
}

// NewDingTalkPlugin 创建钉钉插件
func NewDingTalkPlugin(cfg *types.OpenAcosmiConfig) *DingTalkPlugin {
	return &DingTalkPlugin{
		config:  cfg,
		streams: make(map[string]*DingTalkStreamClient),
		senders: make(map[string]*DingTalkSender),
	}
}

// ID 返回频道标识
func (p *DingTalkPlugin) ID() channels.ChannelID {
	return channels.ChannelDingTalk
}

// UpdateConfig 实现 channels.ConfigUpdater 接口。
// 热重载时注入新配置，后续 Start() 将使用新凭证（AppKey/AppSecret）建立 Stream 连接。
func (p *DingTalkPlugin) UpdateConfig(cfg interface{}) {
	if c, ok := cfg.(*types.OpenAcosmiConfig); ok {
		p.mu.Lock()
		p.config = c
		p.mu.Unlock()
	}
}

// Start 启动钉钉频道
func (p *DingTalkPlugin) Start(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	acct := ResolveDingTalkAccount(p.config, accountID)
	if acct == nil {
		return fmt.Errorf("dingtalk account %q not found in config", accountID)
	}

	if err := ValidateDingTalkConfig(acct.Config); err != nil {
		return fmt.Errorf("dingtalk config validation: %w", err)
	}

	if !channels.IsAccountEnabled(acct.Config.Enabled) {
		slog.Info("dingtalk account disabled, skipping", "account", accountID)
		return nil
	}

	// Stream 客户端（接收消息 — 长连接）
	stream := NewDingTalkStreamClient(acct)
	capturedAccountID := accountID
	stream.SetMessageHandler(func(data *dingtalkstream.BotCallbackDataModel) {
		LogCallbackInfo(data)
		text := ExtractTextFromCallback(data)
		chatID := data.ConversationId
		userID := data.SenderId
		slog.Info("dingtalk message received (plugin)",
			"account", capturedAccountID,
			"chat_id", chatID,
			"user_id", userID,
			"text", text,
		)

		// 路由到 Agent 管线获取回复（优先多模态）
		var dispatchReply *channels.DispatchReply
		if p.DispatchMultimodalFunc != nil {
			// 构建 ChannelMessage（当前只有文本，Phase B 扩展）
			cm := &channels.ChannelMessage{
				Text:        text,
				MessageType: "text",
			}
			dispatchReply = p.DispatchMultimodalFunc(
				"dingtalk", capturedAccountID, chatID, userID, cm)
		} else if p.DispatchFunc != nil {
			replyText := p.DispatchFunc(context.Background(),
				"dingtalk", capturedAccountID, chatID, userID, text)
			if replyText != "" {
				dispatchReply = &channels.DispatchReply{Text: replyText}
			}
		} else {
			slog.Warn("dingtalk: DispatchFunc not set, message not routed to agent",
				"account", capturedAccountID)
		}

		if dispatchReply != nil && dispatchReply.Text != "" {
			// 发送回复（根据会话类型选择群消息或单聊消息）
			sender := p.GetSender(capturedAccountID)
			if sender != nil {
				if strings.HasPrefix(chatID, "cid") {
					if err := sender.SendGroupMessage(context.Background(), chatID, dispatchReply.Text); err != nil {
						slog.Error("dingtalk: failed to send group reply",
							"account", capturedAccountID, "chat_id", chatID, "error", err)
					}
				} else {
					if err := sender.SendOToMessage(context.Background(), []string{userID}, dispatchReply.Text); err != nil {
						slog.Error("dingtalk: failed to send reply",
							"account", capturedAccountID, "user_id", userID, "error", err)
					}
				}
			}
		}
	})

	if err := stream.Start(context.Background()); err != nil {
		return fmt.Errorf("start dingtalk stream: %w", err)
	}
	p.streams[accountID] = stream

	// 消息发送器（发送消息 — HTTP API）
	sender := NewDingTalkSender(acct.Config.AppKey, acct.Config.AppSecret, acct.Config.RobotCode)
	p.senders[accountID] = sender

	slog.Info("dingtalk channel started",
		"account", accountID,
		"app_key", acct.Config.AppKey,
		"robot_code", acct.Config.RobotCode,
	)
	return nil
}

// Stop 停止钉钉频道
func (p *DingTalkPlugin) Stop(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	if stream, ok := p.streams[accountID]; ok {
		stream.Stop()
		delete(p.streams, accountID)
	}
	delete(p.senders, accountID)

	slog.Info("dingtalk channel stopped", "account", accountID)
	return nil
}

// GetSender 获取消息发送器
func (p *DingTalkPlugin) GetSender(accountID string) *DingTalkSender {
	p.mu.Lock()
	defer p.mu.Unlock()
	if accountID == "" {
		accountID = channels.DefaultAccountID
	}
	return p.senders[accountID]
}

// SendMessage 实现 channels.MessageSender 接口。
// 根据 To 格式自动路由：包含 "cid" 前缀时使用群消息，否则使用单聊消息。
func (p *DingTalkPlugin) SendMessage(params channels.OutboundSendParams) (*channels.OutboundSendResult, error) {
	accountID := params.AccountID
	if accountID == "" {
		accountID = channels.DefaultAccountID
	}
	sender := p.GetSender(accountID)
	if sender == nil {
		return nil, fmt.Errorf("dingtalk sender not available for account %s", accountID)
	}

	ctx := params.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// 群消息 vs 单聊消息路由
	if strings.HasPrefix(params.To, "cid") {
		if err := sender.SendGroupMessage(ctx, params.To, params.Text); err != nil {
			return nil, err
		}
	} else {
		if err := sender.SendOToMessage(ctx, []string{params.To}, params.Text); err != nil {
			return nil, err
		}
	}

	return &channels.OutboundSendResult{
		Channel: string(channels.ChannelDingTalk),
		ChatID:  params.To,
	}, nil
}
