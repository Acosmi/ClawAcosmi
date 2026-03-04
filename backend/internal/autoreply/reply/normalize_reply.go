package reply

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/normalize-reply.ts

// NormalizeReplyOptions 回复规范化选项。
type NormalizeReplyOptions struct {
	ResponsePrefix        string
	ResponsePrefixContext *ResponsePrefixContext
	OnHeartbeatStrip      func()
	OnSkip                func(reason NormalizeReplySkipReason)
}

// NormalizeReplyPayload 规范化回复载荷。
// 执行心跳剥离、响应前缀注入、空回复过滤。
// TS 对照: normalize-reply.ts
func NormalizeReplyPayload(payload autoreply.ReplyPayload, opts *NormalizeReplyOptions) *autoreply.ReplyPayload {
	text := strings.TrimSpace(payload.Text)

	// 心跳令牌剥离
	stripResult := autoreply.StripHeartbeatToken(text, nil)
	if stripResult.DidStrip {
		if opts != nil && opts.OnHeartbeatStrip != nil {
			opts.OnHeartbeatStrip()
		}
		text = stripResult.Text
	}

	// 静默回复检测
	if autoreply.IsSilentReplyText(text) {
		if opts != nil && opts.OnSkip != nil {
			opts.OnSkip(SkipReasonEmpty)
		}
		return nil
	}

	// 空文本 + 无媒体 → 跳过
	if text == "" && payload.MediaURL == "" && len(payload.MediaURLs) == 0 {
		if opts != nil && opts.OnSkip != nil {
			opts.OnSkip(SkipReasonEmpty)
		}
		return nil
	}

	// 应用响应前缀
	if opts != nil && opts.ResponsePrefix != "" && text != "" {
		prefix := opts.ResponsePrefix
		if opts.ResponsePrefixContext != nil {
			prefix = applyResponsePrefixTemplate(prefix, opts.ResponsePrefixContext)
		}
		if prefix != "" && !strings.HasPrefix(text, prefix) {
			text = prefix + text
		}
	}

	result := payload
	result.Text = text
	return &result
}

// applyResponsePrefixTemplate 应用响应前缀模板。
// 支持双花括号 {{var}} 和 TS 兼容的单花括号 {var} 语法。
// TS 对照: response-prefix-template.ts resolveResponsePrefixTemplate
func applyResponsePrefixTemplate(prefix string, ctx *ResponsePrefixContext) string {
	if ctx == nil {
		return prefix
	}

	// 双花括号 {{var}} 替换（向后兼容）
	if strings.Contains(prefix, "{{") {
		replacements := map[string]string{
			"{{model}}":    ctx.Model,
			"{{provider}}": ctx.Provider,
			"{{date}}":     ctx.Date,
			"{{time}}":     ctx.Time,
			"{{weekday}}":  ctx.Weekday,
			"{{timezone}}": ctx.Timezone,
		}
		for k, v := range replacements {
			prefix = strings.ReplaceAll(prefix, k, v)
		}
	}

	// 单花括号 {var} 替换（TS 兼容，case-insensitive）
	// TS 对照: TEMPLATE_VAR_PATTERN + resolveResponsePrefixTemplate switch
	if !strings.Contains(prefix, "{") {
		return prefix
	}
	return templateVarPattern.ReplaceAllStringFunc(prefix, func(match string) string {
		// 提取变量名（去除花括号）
		varName := strings.ToLower(match[1 : len(match)-1])
		switch varName {
		case "model":
			if ctx.Model != "" {
				return ctx.Model
			}
		case "modelfull":
			if ctx.ModelFull != "" {
				return ctx.ModelFull
			}
		case "provider":
			if ctx.Provider != "" {
				return ctx.Provider
			}
		case "thinkinglevel", "think":
			if ctx.ThinkingLevel != "" {
				return ctx.ThinkingLevel
			}
		case "identity.name", "identityname":
			if ctx.IdentityName != "" {
				return ctx.IdentityName
			}
		case "date":
			if ctx.Date != "" {
				return ctx.Date
			}
		case "time":
			if ctx.Time != "" {
				return ctx.Time
			}
		case "weekday":
			if ctx.Weekday != "" {
				return ctx.Weekday
			}
		case "timezone":
			if ctx.Timezone != "" {
				return ctx.Timezone
			}
		}
		// 未识别或空值 → 保留原始文本
		return match
	})
}
