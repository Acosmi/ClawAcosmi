package hooks

// LLM-based slug generator for session memory filenames
// TS 参考: src/hooks/llm-slug-generator.ts

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// LLMClient 轻量 LLM 调用接口（便于测试替换）
type LLMClient interface {
	// Complete 发送 prompt，返回文本响应
	Complete(ctx context.Context, prompt string, timeoutMs int) (string, error)
}

// slugCleanRe 仅保留小写字母、数字、连字符
var slugCleanRe = regexp.MustCompile(`[^a-z0-9-]`)

// slugCollapseRe 合并连续连字符
var slugCollapseRe = regexp.MustCompile(`-+`)

// slugTrimRe 去除首尾连字符
var slugTrimRe = regexp.MustCompile(`^-|-$`)

// cleanSlug 将 LLM 返回文本清洗为合法 slug：
// 小写 → 非法字符换成 "-" → 合并多个 "-" → 去首尾 "-" → 截取 30 字符
func cleanSlug(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = slugCleanRe.ReplaceAllString(s, "-")
	s = slugCollapseRe.ReplaceAllString(s, "-")
	s = slugTrimRe.ReplaceAllString(s, "")
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}

// GenerateSessionSlug 调用 LLM 为会话生成简短标题（slug）。
//
// messages 是会话历史摘要（拼接文本，建议不超过 2000 字符）。
// 返回：简短英文 slug，如 "debug-go-build-error"；失败时返回空字符串和错误。
//
// TS 对照: src/hooks/llm-slug-generator.ts generateSlugViaLLM
func GenerateSessionSlug(ctx context.Context, messages []string, llmClient LLMClient) (string, error) {
	if llmClient == nil {
		return "", fmt.Errorf("llmClient is nil")
	}

	// 拼接会话内容，限制长度与 TS 版一致（2000 字符）
	sessionContent := strings.Join(messages, "\n")
	if len(sessionContent) > 2000 {
		sessionContent = sessionContent[:2000]
	}
	if strings.TrimSpace(sessionContent) == "" {
		return "", fmt.Errorf("empty session content")
	}

	prompt := `Based on this conversation, generate a short 1-2 word filename slug (lowercase, hyphen-separated, no file extension).

Conversation summary:
` + sessionContent + `

Reply with ONLY the slug, nothing else. Examples: "vendor-pitch", "api-design", "bug-fix"`

	// 15 秒超时（与 TS 版一致）
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	text, err := llmClient.Complete(callCtx, prompt, 15_000)
	if err != nil {
		return "", fmt.Errorf("llm complete: %w", err)
	}

	slug := cleanSlug(text)
	if slug == "" {
		return "", fmt.Errorf("llm returned empty slug")
	}
	return slug, nil
}
