package log

import (
	"regexp"
	"strings"
)

// 对应原版 src/logging/redact.ts。
// 提供日志中敏感信息（API Key、Token、Password、PEM 密钥等）的自动脱敏。

// RedactMode 脱敏模式
type RedactMode string

const (
	// RedactOff 关闭脱敏
	RedactOff RedactMode = "off"
	// RedactTools 仅对工具输出脱敏（默认）
	RedactTools RedactMode = "tools"
)

const (
	redactMinLength = 18
	redactKeepStart = 6
	redactKeepEnd   = 4
)

// DefaultRedactPatterns 默认脱敏正则模式列表。
// 覆盖 ENV 赋值、JSON 字段、CLI 参数、Auth Header、PEM 块、常见 token 前缀。
var DefaultRedactPatterns = []string{
	// ENV-style: KEY=value
	`\b[A-Z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD|PASSWD)\b\s*[=:]\s*(['"]?)([^\s'"\\]+)\1`,
	// JSON fields
	`"(?:apiKey|token|secret|password|passwd|accessToken|refreshToken)"\s*:\s*"([^"]+)"`,
	// CLI flags
	`--(?:api[-_]?key|token|secret|password|passwd)\s+(['"]?)([^\s"']+)\1`,
	// Authorization headers
	`Authorization\s*[:=]\s*Bearer\s+([A-Za-z0-9._\-+=]+)`,
	`\bBearer\s+([A-Za-z0-9._\-+=]{18,})\b`,
	// PEM blocks
	`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`,
	// Common token prefixes
	`\b(sk-[A-Za-z0-9_-]{8,})\b`,
	`\b(ghp_[A-Za-z0-9]{20,})\b`,
	`\b(github_pat_[A-Za-z0-9_]{20,})\b`,
	`\b(xox[baprs]-[A-Za-z0-9-]{10,})\b`,
	`\b(xapp-[A-Za-z0-9-]{10,})\b`,
	`\b(gsk_[A-Za-z0-9_-]{10,})\b`,
	`\b(AIza[0-9A-Za-z\-_]{20,})\b`,
	`\b(pplx-[A-Za-z0-9_-]{10,})\b`,
	`\b(npm_[A-Za-z0-9]{10,})\b`,
	`\b(\d{6,}:[A-Za-z0-9_-]{20,})\b`,
}

// compiledDefaultPatterns 预编译的默认正则列表（包级缓存）
var compiledDefaultPatterns []*regexp.Regexp

func init() {
	compiledDefaultPatterns = compilePatterns(DefaultRedactPatterns)
}

// compilePatterns 将模式字符串列表编译为正则表达式。
// 无效模式会被静默跳过。
func compilePatterns(patterns []string) []*regexp.Regexp {
	var result []*regexp.Regexp
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			continue // 静默忽略无效模式
		}
		result = append(result, re)
	}
	return result
}

// maskToken 部分遮蔽 token。
// 短于 redactMinLength 的返回 "***"，
// 否则保留前 redactKeepStart 和后 redactKeepEnd 个字符。
func maskToken(token string) string {
	if len(token) < redactMinLength {
		return "***"
	}
	start := token[:redactKeepStart]
	end := token[len(token)-redactKeepEnd:]
	return start + "…" + end
}

// redactPEM 脱敏 PEM 密钥块，保留首尾行。
func redactPEM(block string) string {
	lines := strings.Split(block, "\n")
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) < 2 {
		return "***"
	}
	return nonEmpty[0] + "\n…redacted…\n" + nonEmpty[len(nonEmpty)-1]
}

// RedactSensitiveText 对文本中的敏感信息进行脱敏。
// patterns 为空则使用默认模式。
func RedactSensitiveText(text string, patterns []*regexp.Regexp) string {
	if text == "" {
		return text
	}
	if len(patterns) == 0 {
		patterns = compiledDefaultPatterns
	}
	result := text
	for _, re := range patterns {
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			// PEM 块特殊处理
			if strings.Contains(match, "PRIVATE KEY-----") {
				return redactPEM(match)
			}
			// 提取捕获组中最长的有效 token
			subs := re.FindStringSubmatch(match)
			token := match
			for i := len(subs) - 1; i >= 1; i-- {
				if subs[i] != "" {
					token = subs[i]
					break
				}
			}
			masked := maskToken(token)
			if token == match {
				return masked
			}
			return strings.Replace(match, token, masked, 1)
		})
	}
	return result
}

// RedactToolDetail 对工具输出进行脱敏（仅在 tools 模式下）。
func RedactToolDetail(detail string, mode RedactMode) string {
	if mode != RedactTools {
		return detail
	}
	return RedactSensitiveText(detail, nil)
}
