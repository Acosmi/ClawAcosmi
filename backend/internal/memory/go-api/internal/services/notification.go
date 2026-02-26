// Package services — 通知服务接口与实现。
// 提供可插拔的通知策略（日志/Webhook/邮件），消除 billing.go 中 TODO-02。
package services

import (
	"context"
	"log/slog"
	"sync"
)

// NotifyEvent 通知事件类型。
type NotifyEvent string

const (
	EventBalanceLow  NotifyEvent = "balance_low"
	EventBalanceZero NotifyEvent = "balance_zero"
)

// Notifier 定义通知发送接口，支持多种后端实现。
type Notifier interface {
	// Send 发送通知给指定用户。
	Send(ctx context.Context, userID string, event NotifyEvent, payload map[string]string) error
}

// LogNotifier 日志通知实现（开发/默认环境）。
type LogNotifier struct{}

// Send 将通知输出到结构化日志。
func (n *LogNotifier) Send(ctx context.Context, userID string, event NotifyEvent, payload map[string]string) error {
	slog.WarnContext(ctx, "Notification triggered",
		"user_id", userID,
		"event", string(event),
		"payload", payload,
	)
	return nil
}

// --- Singleton ---

var (
	notifierOnce sync.Once
	notifier     Notifier
)

// GetNotifier 返回全局 Notifier 实例（默认 LogNotifier）。
func GetNotifier() Notifier {
	notifierOnce.Do(func() {
		notifier = &LogNotifier{}
		slog.Info("Notification service initialized", "backend", "log")
	})
	return notifier
}

// SetNotifier 替换全局 Notifier（用于测试或切换 Webhook 实现）。
func SetNotifier(n Notifier) {
	notifier = n
}
