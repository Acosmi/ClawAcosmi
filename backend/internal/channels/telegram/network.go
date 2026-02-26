package telegram

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
)

// ---------- 允许的更新类型（allowed-updates.ts） ----------

// AllowedUpdates 返回 Telegram Bot API 长轮询允许的更新类型列表。
// 继承自 grammy API_CONSTANTS.DEFAULT_UPDATE_TYPES + message_reaction。
func AllowedUpdates() []string {
	return []string{
		"message",
		"edited_message",
		"channel_post",
		"edited_channel_post",
		"inline_query",
		"chosen_inline_result",
		"callback_query",
		"shipping_query",
		"pre_checkout_query",
		"poll",
		"poll_answer",
		"my_chat_member",
		"chat_member",
		"chat_join_request",
		"message_reaction",
	}
}

// ---------- 网络配置（network-config.ts） ----------

// 环境变量名称
const (
	TelegramDisableAutoSelectFamilyEnv = "OPENACOSMI_TELEGRAM_DISABLE_AUTO_SELECT_FAMILY"
	TelegramEnableAutoSelectFamilyEnv  = "OPENACOSMI_TELEGRAM_ENABLE_AUTO_SELECT_FAMILY"
)

// AutoSelectFamilyDecision 网络自动选择 IP 协议族的决策结果。
// Go 的 net.Dialer 默认使用 Happy Eyeballs (RFC 6555)，此配置主要
// 用于兼容 TS 端的 autoSelectFamily 行为。
type AutoSelectFamilyDecision struct {
	Value  *bool  // nil 表示未决定
	Source string // 决策来源
}

// TelegramNetworkConfig Telegram 网络配置。
type TelegramNetworkConfig struct {
	AutoSelectFamily *bool  `json:"autoSelectFamily,omitempty"`
	ProxyURL         string `json:"proxyUrl,omitempty"`
}

// isTruthyEnvValue 判断环境变量值是否为真值。
func isTruthyEnvValue(val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// ResolveAutoSelectFamilyDecision 解析网络自动选择 IP 协议族配置。
func ResolveAutoSelectFamilyDecision(network *TelegramNetworkConfig) AutoSelectFamilyDecision {
	if val := os.Getenv(TelegramEnableAutoSelectFamilyEnv); isTruthyEnvValue(val) {
		v := true
		return AutoSelectFamilyDecision{Value: &v, Source: "env:" + TelegramEnableAutoSelectFamilyEnv}
	}
	if val := os.Getenv(TelegramDisableAutoSelectFamilyEnv); isTruthyEnvValue(val) {
		v := false
		return AutoSelectFamilyDecision{Value: &v, Source: "env:" + TelegramDisableAutoSelectFamilyEnv}
	}
	if network != nil && network.AutoSelectFamily != nil {
		return AutoSelectFamilyDecision{Value: network.AutoSelectFamily, Source: "config"}
	}
	// Go 默认使用 Happy Eyeballs，无需特殊处理
	return AutoSelectFamilyDecision{}
}

// ---------- 网络错误（network-errors.ts） ----------

// recoverableSyscallErrnos maps syscall.Errno values to recoverability.
// This provides a direct, type-safe check for Go syscall errors without
// relying on string matching.
var recoverableSyscallErrnos = map[syscall.Errno]bool{
	syscall.ECONNRESET:   true,
	syscall.ECONNREFUSED: true,
	syscall.EPIPE:        true,
	syscall.ETIMEDOUT:    true,
	syscall.ENETUNREACH:  true,
	syscall.EHOSTUNREACH: true,
	syscall.ECONNABORTED: true,
}

// recoverableMessageSnippets 可恢复错误的消息片段。
var recoverableMessageSnippets = []string{
	"fetch failed",
	"network error",
	"network request",
	"client network socket disconnected",
	"socket hang up",
	"getaddrinfo",
	"timeout",
	"timed out",
	"connection refused",
	"connection reset",
	"no such host",
	"i/o timeout",
	"context deadline exceeded",
	"eof",
}

// TelegramNetworkErrorContext 网络错误发生的上下文。
type TelegramNetworkErrorContext string

const (
	NetworkCtxPolling TelegramNetworkErrorContext = "polling"
	NetworkCtxSend    TelegramNetworkErrorContext = "send"
	NetworkCtxWebhook TelegramNetworkErrorContext = "webhook"
	NetworkCtxUnknown TelegramNetworkErrorContext = "unknown"
)

// collectErrorCandidates performs a BFS traversal of an error tree,
// following both single-error chains (errors.Unwrap() returning error)
// and multi-error chains (Unwrap() returning []error, as used by
// errors.Join() and similar Go 1.20+ multi-errors).
//
// This mirrors the TS collectErrorCandidates which traverses .cause,
// .reason, .errors[], and .error (for Grammy HttpError).
func collectErrorCandidates(err error) []error {
	queue := []error{err}
	seen := make(map[error]bool)
	var candidates []error

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == nil || seen[current] {
			continue
		}
		seen[current] = true
		candidates = append(candidates, current)

		// Single unwrap: follows the standard errors.Unwrap() chain
		if unwrapped := errors.Unwrap(current); unwrapped != nil {
			queue = append(queue, unwrapped)
		}

		// Multi-unwrap (Go 1.20+): handles errors.Join() and any error
		// type implementing Unwrap() []error
		if multi, ok := current.(interface{ Unwrap() []error }); ok {
			queue = append(queue, multi.Unwrap()...)
		}
	}

	return candidates
}

// IsRecoverableTelegramNetworkError determines whether an error is a
// recoverable network error. It performs a BFS traversal of the full error
// tree (including multi-errors from errors.Join) and checks each candidate
// for recoverable conditions.
//
// Equivalent to TS isRecoverableTelegramNetworkError in network-errors.ts.
func IsRecoverableTelegramNetworkError(err error, ctx TelegramNetworkErrorContext) bool {
	if err == nil {
		return false
	}
	allowMessageMatch := ctx != NetworkCtxSend

	// context.Canceled is the Go equivalent of TS AbortError.
	// context.DeadlineExceeded is the Go equivalent of TS TimeoutError.
	// Use errors.Is for correct matching across wrapped error chains.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	for _, candidate := range collectErrorCandidates(err) {
		// Check for syscall.Errno directly (type-safe, no string matching)
		var errno syscall.Errno
		if errors.As(candidate, &errno) && recoverableSyscallErrnos[errno] {
			return true
		}

		// Check net.OpError (typically wraps a recoverable network error)
		var opErr *net.OpError
		if errors.As(candidate, &opErr) {
			return true
		}

		// Check DNS errors
		var dnsErr *net.DNSError
		if errors.As(candidate, &dnsErr) {
			return true
		}

		// Check timeout via net.Error interface
		var netErr net.Error
		if errors.As(candidate, &netErr) && netErr.Timeout() {
			return true
		}

		// Message-based matching (disabled for "send" context to avoid
		// false positives on API error messages)
		if allowMessageMatch {
			msg := strings.ToLower(candidate.Error())
			for _, snippet := range recoverableMessageSnippets {
				if strings.Contains(msg, snippet) {
					return true
				}
			}
		}
	}

	return false
}
