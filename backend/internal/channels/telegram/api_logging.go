package telegram

import (
	"fmt"
	"log/slog"
)

// Telegram API 日志 — 继承自 src/telegram/api-logging.ts (46L)

// WithTelegramAPIErrorLogging 包装 API 调用，在错误时自动记录日志。
// shouldLog 为 nil 时记录所有错误；非 nil 时仅在 shouldLog(err)==true 时记录。
// 对齐 TS: api-logging.ts shouldLog 回调参数。
func WithTelegramAPIErrorLogging[T any](operation string, fn func() (T, error), shouldLog func(error) bool) (T, error) {
	result, err := fn()
	if err != nil && (shouldLog == nil || shouldLog(err)) {
		slog.Error(fmt.Sprintf("telegram %s failed: %v", operation, err))
	}
	return result, err
}

// WithTelegramAPIErrorLoggingVoid 不返回结果的 API 日志包装。
func WithTelegramAPIErrorLoggingVoid(operation string, fn func() error, shouldLog func(error) bool) error {
	err := fn()
	if err != nil && (shouldLog == nil || shouldLog(err)) {
		slog.Error(fmt.Sprintf("telegram %s failed: %v", operation, err))
	}
	return err
}
