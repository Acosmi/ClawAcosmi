package dingtalk

// client.go — 钉钉 Stream SDK 客户端封装
// 使用官方 dingtalk-stream-sdk-go 建立长连接

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	dingtalkstream "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingtalkclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

// DingTalkStreamClient 钉钉 Stream 模式客户端
type DingTalkStreamClient struct {
	AppKey    string
	AppSecret string
	RobotCode string

	mu        sync.Mutex
	client    *dingtalkclient.StreamClient
	onMessage func(*dingtalkstream.BotCallbackDataModel)
	cancel    context.CancelFunc
}

// NewDingTalkStreamClient 创建钉钉 Stream 客户端
func NewDingTalkStreamClient(acct *ResolvedDingTalkAccount) *DingTalkStreamClient {
	return &DingTalkStreamClient{
		AppKey:    acct.Config.AppKey,
		AppSecret: acct.Config.AppSecret,
		RobotCode: acct.Config.RobotCode,
	}
}

// SetMessageHandler 设置消息回调
func (c *DingTalkStreamClient) SetMessageHandler(handler func(*dingtalkstream.BotCallbackDataModel)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = handler
}

// Start 启动 Stream 长连接
// 使用 recover 防护已知的 SDK panic bug
func (c *DingTalkStreamClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	client := dingtalkclient.NewStreamClient(
		dingtalkclient.WithAppCredential(dingtalkclient.NewAppCredentialConfig(c.AppKey, c.AppSecret)),
	)

	// 注册机器人消息回调
	client.RegisterChatBotCallbackRouter(func(ctx context.Context, data *dingtalkstream.BotCallbackDataModel) ([]byte, error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("dingtalk stream callback panic recovered",
					"recover", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
			}
		}()

		if c.onMessage != nil {
			c.onMessage(data)
		}
		return []byte(""), nil
	})

	c.client = client

	// 在独立 goroutine 中启动 stream（带 recover 防护 SDK 内部 panic）
	childCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("dingtalk stream client panic recovered",
					"recover", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
			}
		}()

		if err := client.Start(childCtx); err != nil {
			slog.Error("dingtalk stream client stopped", "error", err)
		}
	}()

	slog.Info("dingtalk stream client started", "app_key", c.AppKey)
	return nil
}

// Stop 停止 Stream 连接
func (c *DingTalkStreamClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	slog.Info("dingtalk stream client stopped")
}
