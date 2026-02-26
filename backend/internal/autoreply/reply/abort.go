package reply

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// TS 对照: auto-reply/reply/abort.ts (~200L) — 完整版

// AbortTriggers 中止触发关键词（裸词，对齐 TS abort.ts ABORT_TRIGGERS）。
var AbortTriggers = []string{
	"stop",
	"esc",
	"abort",
	"wait",
	"exit",
	"interrupt",
}

// AbortSlashCommands slash 命令格式中止触发。
var AbortSlashCommands = []string{
	"/abort",
	"/stop",
	"/cancel",
}

// IsAbortTrigger 判断文本是否为中止触发。
// 同时匹配裸词（"stop"）和 slash 命令（"/stop"）。
// TS 对照: abort.ts isAbortTrigger
func IsAbortTrigger(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return false
	}
	for _, trigger := range AbortTriggers {
		if trimmed == trigger {
			return true
		}
	}
	for _, cmd := range AbortSlashCommands {
		if trimmed == cmd || strings.HasPrefix(trimmed, cmd+" ") {
			return true
		}
	}
	return false
}

// AbortableContext 可中止的上下文（Go 使用 context.WithCancel）。
type AbortableContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// NewAbortableContext 创建可中止的上下文。
func NewAbortableContext(parent context.Context) *AbortableContext {
	ctx, cancel := context.WithCancel(parent)
	return &AbortableContext{Ctx: ctx, Cancel: cancel}
}

// ---------- ABORT_MEMORY ----------

// abortMemory 模块级中止记忆。
// TS 对照: abort.ts ABORT_MEMORY = new Map<string, boolean>()
// Go 使用 sync.Map 替代，key 为 sessionKey，value 为 bool。
var abortMemory sync.Map

// GetAbortMemory 查询指定会话是否处于中止状态。
func GetAbortMemory(sessionKey string) bool {
	val, ok := abortMemory.Load(sessionKey)
	if !ok {
		return false
	}
	return val.(bool)
}

// SetAbortMemory 设置会话中止状态。
func SetAbortMemory(sessionKey, value string) {
	if value == "" {
		abortMemory.Delete(sessionKey)
		return
	}
	abortMemory.Store(sessionKey, true)
}

// ClearAbortMemory 清除会话中止记忆。
func ClearAbortMemory(sessionKey string) {
	abortMemory.Delete(sessionKey)
}

// ResetAbortMemoryForTesting 清空所有中止记忆（仅测试用）。
func ResetAbortMemoryForTesting() {
	abortMemory.Range(func(key, _ any) bool {
		abortMemory.Delete(key)
		return true
	})
}

// ---------- DI 接口 ----------

// AbortDeps 中止操作外部依赖。
type AbortDeps interface {
	// AbortEmbeddedPiRun 中止当前嵌入式 PI 运行。
	AbortEmbeddedPiRun(sessionKey string) bool
	// ListSubagentRuns 列出请求者关联的 subagent 运行 ID。
	ListSubagentRuns(requesterKey string) []string
	// StopSubagentRun 停止单个 subagent 运行。
	StopSubagentRun(runID string) error
	// ClearSessionQueues 清空会话消息队列。
	ClearSessionQueues(sessionKey string)
	// ResolveCommandAuthorization 检查用户是否有权执行中止。
	ResolveCommandAuthorization(userID, command string) bool
}

// ---------- 中止回复格式化 ----------

// FormatAbortReplyText 格式化中止回复文本。
// TS 对照: abort.ts formatAbortReplyText
func FormatAbortReplyText(stoppedCount int) string {
	if stoppedCount <= 0 {
		return "🛑 Stopped."
	}
	return fmt.Sprintf("🛑 Stopped. (%d subagent(s) also stopped)", stoppedCount)
}

// ---------- Subagent 停止 ----------

// StopSubagentsForRequester 停止请求者关联的所有 subagent。
// 返回成功停止的数量。
// TS 对照: abort.ts stopSubagentsForRequester
func StopSubagentsForRequester(deps AbortDeps, requesterKey string) int {
	if deps == nil || requesterKey == "" {
		return 0
	}
	runs := deps.ListSubagentRuns(requesterKey)
	stopped := 0
	for _, runID := range runs {
		if err := deps.StopSubagentRun(runID); err == nil {
			stopped++
		}
	}
	return stopped
}

// ---------- 快速中止 ----------

// FastAbortResult 快速中止结果。
type FastAbortResult struct {
	Aborted      bool
	ReplyText    string
	StoppedCount int
}

// TryFastAbortFromMessage 尝试从消息快速中止。
// TS 对照: abort.ts tryFastAbortFromMessage
func TryFastAbortFromMessage(
	deps AbortDeps,
	sessionKey string,
	userID string,
	messageText string,
) *FastAbortResult {
	if !IsAbortTrigger(messageText) {
		return &FastAbortResult{Aborted: false}
	}

	// 检查授权
	if deps != nil && userID != "" {
		if !deps.ResolveCommandAuthorization(userID, "abort") {
			return &FastAbortResult{Aborted: false}
		}
	}

	// 标记中止记忆
	SetAbortMemory(sessionKey, "abort")

	// 中止当前 PI 运行
	if deps != nil {
		deps.AbortEmbeddedPiRun(sessionKey)
	}

	// 停止 subagents
	stoppedCount := 0
	if deps != nil {
		stoppedCount = StopSubagentsForRequester(deps, sessionKey)
	}

	// 清空队列
	if deps != nil {
		deps.ClearSessionQueues(sessionKey)
	}

	return &FastAbortResult{
		Aborted:      true,
		ReplyText:    FormatAbortReplyText(stoppedCount),
		StoppedCount: stoppedCount,
	}
}
