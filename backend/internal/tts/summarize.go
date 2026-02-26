// summarize.go — TTS 长文本 LLM 智能摘要。
//
// TS 对照: tts/tts.ts L909-987 (summarizeText)
//
// 当文本超过 maxLength 且摘要功能启用时，通过 LLM 生成浓缩文案后再合成语音。
// 采用函数注入模式（SummarizeFunc），由上层 gateway 初始化时注册 LLM 调用能力，
// 避免 tts 包直接依赖 llmclient/autoreply。
package tts

import (
	"context"
	"fmt"
	"log"
	"time"
)

// SummarizeRequest LLM 摘要请求参数。
type SummarizeRequest struct {
	Text         string // 需要摘要的原文
	TargetLength int    // 目标摘要长度（字符数）
	TimeoutMs    int    // 超时毫秒
}

// SummarizeResult LLM 摘要结果。
type SummarizeResult struct {
	Summary      string // 摘要文本
	LatencyMs    int64  // 延迟毫秒
	InputLength  int    // 原文长度
	OutputLength int    // 摘要长度
}

// SummarizerFunc 摘要函数类型。
// 由上层（如 gateway）注入具体 LLM 调用实现。
// prompt 已由 tts 包构建，调用方只需转发到 LLM 并返回纯文本结果。
type SummarizerFunc func(ctx context.Context, prompt string, maxTokens int) (string, error)

// summarizer 全局摘要函数句柄（可注入）。
var summarizer SummarizerFunc

// RegisterSummarizer 注册 LLM 摘要函数。
// 由 gateway 或测试在初始化时调用。
func RegisterSummarizer(fn SummarizerFunc) {
	summarizer = fn
}

// SummarizeTextPrompt 构建摘要 prompt（与 TS 一致）。
// TS 对照: tts.ts L937-948
func SummarizeTextPrompt(text string, targetLength int) string {
	return fmt.Sprintf(
		"You are an assistant that summarizes texts concisely while keeping the most important information. "+
			"Summarize the text to approximately %d characters. Maintain the original tone and style. "+
			"Reply only with the summary, without additional explanations.\n\n"+
			"<text_to_summarize>\n%s\n</text_to_summarize>",
		targetLength, text,
	)
}

// SummarizeTextForTts 对长文本执行 LLM 摘要。
// 如果 SummarizerFunc 未注册，返回错误，调用方应 fallback 到硬截断。
// TS 对照: tts.ts L909-987
func SummarizeTextForTts(req SummarizeRequest) (*SummarizeResult, error) {
	if summarizer == nil {
		return nil, fmt.Errorf("summarizer not registered")
	}
	if req.TargetLength < 100 || req.TargetLength > 10000 {
		return nil, fmt.Errorf("invalid targetLength: %d", req.TargetLength)
	}

	start := time.Now()
	prompt := SummarizeTextPrompt(req.Text, req.TargetLength)
	maxTokens := req.TargetLength / 2
	if maxTokens < 50 {
		maxTokens = 50
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 30000 // 默认 30s
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	summary, err := summarizer(ctx, prompt, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("summarization failed: %w", err)
	}
	if summary == "" {
		return nil, fmt.Errorf("no summary returned")
	}

	return &SummarizeResult{
		Summary:      summary,
		LatencyMs:    time.Since(start).Milliseconds(),
		InputLength:  len([]rune(req.Text)),
		OutputLength: len([]rune(summary)),
	}, nil
}

// maybeSummarizeText 内部辅助：根据偏好设置决定摘要或截断。
// 返回处理后文本和是否经过摘要。
// TS 对照: tts.ts L1493-1523
func maybeSummarizeText(text string, maxLength int, prefsPath string, config ResolvedTtsConfig) (string, bool) {
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text, false
	}

	if !IsSummarizationEnabled(prefsPath) {
		// 摘要未启用，硬截断
		log.Printf("[tts] truncating long text (%d > %d), summarization disabled", len(runes), maxLength)
		return string(runes[:maxLength-3]) + "...", false
	}

	// 尝试 LLM 摘要
	result, err := SummarizeTextForTts(SummarizeRequest{
		Text:         text,
		TargetLength: maxLength,
		TimeoutMs:    config.TimeoutMs,
	})
	if err != nil {
		// 摘要失败，fallback 截断
		log.Printf("[tts] summarization failed, truncating instead: %v", err)
		return string(runes[:maxLength-3]) + "...", false
	}

	summaryText := result.Summary
	summaryRunes := []rune(summaryText)
	// 如果摘要仍然超过硬限制，截断
	if len(summaryRunes) > config.MaxTextLength {
		log.Printf("[tts] summary exceeded hard limit (%d > %d); truncating", len(summaryRunes), config.MaxTextLength)
		summaryText = string(summaryRunes[:config.MaxTextLength-3]) + "..."
	}

	return summaryText, true
}
