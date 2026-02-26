package reply

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/agent-runner-utils.ts (137L)
// TS 对照: auto-reply/reply/agent-runner-helpers.ts (94L)

// ---------- Bun fetch socket error ----------

var bunFetchSocketErrorRe = regexp.MustCompile(`(?i)socket connection was closed unexpectedly`)

// IsBunFetchSocketError 检查是否为 Bun fetch socket 错误。
func IsBunFetchSocketError(message string) bool {
	return message != "" && bunFetchSocketErrorRe.MatchString(message)
}

// FormatBunFetchSocketError 格式化 socket 错误为用户可读消息。
func FormatBunFetchSocketError(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = "Unknown error"
	}
	return fmt.Sprintf(
		"⚠️ LLM connection failed. This could be due to server issues, network problems, or context length exceeded (e.g., with local LLMs like LM Studio). Original error:\n```\n%s\n```",
		trimmed,
	)
}

// ---------- Usage formatting ----------

// NormalizedUsage 标准化用量数据。
// TS 对照: agents/usage.ts
type NormalizedUsage struct {
	Input      *int
	Output     *int
	CacheRead  *int
	CacheWrite *int
}

// UsageCostConfig 用量成本配置。
type UsageCostConfig struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// UsageLineParams 用量行格式化参数。
type UsageLineParams struct {
	Usage      *NormalizedUsage
	ShowCost   bool
	CostConfig *UsageCostConfig
}

// FormatResponseUsageLine 格式化响应用量行。
// TS 对照: agent-runner-utils.ts L74-110
func FormatResponseUsageLine(params UsageLineParams) string {
	usage := params.Usage
	if usage == nil {
		return ""
	}
	if usage.Input == nil && usage.Output == nil {
		return ""
	}

	inputLabel := "?"
	if usage.Input != nil {
		inputLabel = formatTokenCount(*usage.Input)
	}
	outputLabel := "?"
	if usage.Output != nil {
		outputLabel = formatTokenCount(*usage.Output)
	}

	costLabel := ""
	if params.ShowCost && usage.Input != nil && usage.Output != nil {
		cost := estimateUsageCost(usage, params.CostConfig)
		costLabel = formatUsd(cost)
	}

	suffix := ""
	if costLabel != "" {
		suffix = " · est " + costLabel
	}
	return fmt.Sprintf("Usage: %s in / %s out%s", inputLabel, outputLabel, suffix)
}

// formatTokenCount 格式化 token 数量。
func formatTokenCount(count int) string {
	if count >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	}
	if count >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(count)/1_000)
	}
	return fmt.Sprintf("%d", count)
}

// estimateUsageCost 估算用量成本。
func estimateUsageCost(usage *NormalizedUsage, costCfg *UsageCostConfig) float64 {
	if costCfg == nil || usage == nil {
		return 0
	}
	var cost float64
	if usage.Input != nil {
		cost += float64(*usage.Input) * costCfg.Input / 1_000_000
	}
	if usage.Output != nil {
		cost += float64(*usage.Output) * costCfg.Output / 1_000_000
	}
	if usage.CacheRead != nil {
		cost += float64(*usage.CacheRead) * costCfg.CacheRead / 1_000_000
	}
	if usage.CacheWrite != nil {
		cost += float64(*usage.CacheWrite) * costCfg.CacheWrite / 1_000_000
	}
	return cost
}

// formatUsd 格式化 USD 金额。
func formatUsd(amount float64) string {
	if amount == 0 {
		return "$0.00"
	}
	if amount < 0.01 {
		return fmt.Sprintf("$%.4f", amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}

// ---------- Payload helpers ----------

// AppendUsageLine 向最后一个含文本的 payload 追加用量行。
// TS 对照: agent-runner-utils.ts L112-133
func AppendUsageLine(payloads []autoreply.ReplyPayload, line string) []autoreply.ReplyPayload {
	index := -1
	for i := len(payloads) - 1; i >= 0; i-- {
		if payloads[i].Text != "" {
			index = i
			break
		}
	}
	if index == -1 {
		return append(payloads, autoreply.ReplyPayload{Text: line})
	}
	updated := make([]autoreply.ReplyPayload, len(payloads))
	copy(updated, payloads)
	existing := updated[index].Text
	sep := ""
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}
	updated[index].Text = existing + sep + line
	return updated
}

// ResolveEnforceFinalTag 解析是否强制最终标签。
// TS 对照: agent-runner-utils.ts L135-136
func ResolveEnforceFinalTag(enforceFinal bool, provider string) bool {
	return enforceFinal || isReasoningTagProvider(provider)
}

// isReasoningTagProvider 检查是否为推理标签 provider。
func isReasoningTagProvider(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == "deepseek" || p == "deepseek-reasoner"
}

// IsAudioPayload 检查 payload 是否包含音频媒体。
// TS 对照: agent-runner-helpers.ts L8-12
func IsAudioPayload(payload autoreply.ReplyPayload) bool {
	urls := payload.MediaURLs
	if len(urls) == 0 && payload.MediaURL != "" {
		urls = []string{payload.MediaURL}
	}
	for _, u := range urls {
		if isAudioFileName(u) {
			return true
		}
	}
	return false
}

// isAudioFileName 检查文件名是否为音频。
func isAudioFileName(name string) bool {
	lower := strings.ToLower(name)
	audioExts := []string{".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".opus", ".wma", ".webm"}
	for _, ext := range audioExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// SignalTypingIfNeeded 如果 payload 含可渲染内容则触发 typing。
// TS 对照: agent-runner-helpers.ts L73-93
func SignalTypingIfNeeded(payloads []autoreply.ReplyPayload, signaler *TypingSignaler) error {
	for _, p := range payloads {
		if strings.TrimSpace(p.Text) != "" || p.MediaURL != "" || len(p.MediaURLs) > 0 {
			return signaler.SignalRunStart()
		}
	}
	return nil
}

// ---------- Verbose level helpers ----------

// ShouldEmitToolResult 基于 verbose level 判断是否发送工具结果。
// TS 对照: agent-runner-helpers.ts L14-37
func ShouldEmitToolResult(verboseLevel autoreply.VerboseLevel) bool {
	return verboseLevel != "" && verboseLevel != autoreply.VerboseOff
}

// ShouldEmitToolOutput 基于 verbose level 判断是否发送工具输出。
// TS 对照: agent-runner-helpers.ts L39-61
func ShouldEmitToolOutput(verboseLevel autoreply.VerboseLevel) bool {
	return verboseLevel == autoreply.VerboseFull
}
