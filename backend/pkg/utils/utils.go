// Package utils 提供跨模块通用工具函数。
//
// 对应原版 src/utils.ts 和 src/utils/ 目录中的通用函数。
// 领域特定工具函数（WhatsApp JID、交付上下文等）在后续 Phase 中实现。
//
// TS 依赖: 无外部依赖 (仅 Node.js 标准库)
// Go 替代: 标准库
package utils

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ─── 标识符生成 ───

// GenerateID 生成指定长度的随机十六进制 ID。
func GenerateID(length int) string {
	bytes := make([]byte, (length+1)/2)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)[:length]
}

// ─── 数值工具 ───

// ClampNumber 将数值限制在 [min, max] 范围内。
// 对应原版 clampNumber。
func ClampNumber(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

// ClampInt 将整数限制在 [min, max] 范围内。
// 对应原版 clampInt。
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ─── 字符串工具 ───

// Truncate 截断字符串到指定长度，超出部分用 "..." 替代。
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Contains 检查字符串是否包含子串（简单包装，用于一致性）。
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// ─── 电话号码处理 ───

var e164StripRe = regexp.MustCompile(`[^\d+]`)

// NormalizeE164 将电话号码规范化为 E.164 格式。
// 对应原版 normalizeE164。
func NormalizeE164(number string) string {
	cleaned := e164StripRe.ReplaceAllString(number, "")
	if cleaned == "" {
		return number
	}
	// 确保以 + 开头
	if !strings.HasPrefix(cleaned, "+") {
		cleaned = "+" + cleaned
	}
	return cleaned
}

// ─── 端口工具 ───

// IsPortAvailable 检查 TCP 端口是否可用。
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// ─── 文件/路径工具 ───

// EnsureDir 确保目录存在，不存在则递归创建。
// 对应原版 ensureDir。
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// NormalizePath 规范化文件路径（展开 ~、解析相对路径）。
// 对应原版 normalizePath。
func NormalizePath(p string) string {
	if p == "" {
		return p
	}
	// 展开 ~
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	// 解析绝对路径
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// ShortenHomePath 将绝对路径中的 HOME 目录替换为 ~。
// 对应原版 shortenHomePath。
func ShortenHomePath(input string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return input
	}
	if input == home {
		return "~"
	}
	if strings.HasPrefix(input, home+"/") {
		return "~" + input[len(home):]
	}
	return input
}

// ShortenHomeInString 将字符串中所有包含 HOME 路径的部分替换为 ~。
// 对应原版 shortenHomeInString。
func ShortenHomeInString(input string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return input
	}
	return strings.ReplaceAll(input, home, "~")
}

// ─── 布尔值解析 (utils/boolean.ts) ───

var (
	truthyValues = map[string]bool{"true": true, "1": true, "yes": true, "on": true}
	falsyValues  = map[string]bool{"false": true, "0": true, "no": true, "off": true}
)

// ParseBooleanValue 将字符串解析为布尔值。
// 支持 "true/1/yes/on" (true) 和 "false/0/no/off" (false)。
// 无法解析时返回 (false, false)。
// 对应原版 parseBooleanValue。
func ParseBooleanValue(value string) (result bool, ok bool) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return false, false
	}
	if truthyValues[normalized] {
		return true, true
	}
	if falsyValues[normalized] {
		return false, true
	}
	return false, false
}

// IsTruthy 检查字符串是否为真值。
func IsTruthy(value string) bool {
	result, ok := ParseBooleanValue(value)
	return ok && result
}

// ─── Shell 参数解析 (utils/shell-argv.ts) ───

// SplitShellArgs 将 shell 命令字符串拆分为参数列表。
// 支持单引号、双引号和反斜杠转义。
// 解析失败（未闭合引号）返回 nil。
// 对应原版 splitShellArgs。
func SplitShellArgs(raw string) []string {
	var tokens []string
	var buf strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	pushToken := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for _, ch := range raw {
		if escaped {
			buf.WriteRune(ch)
			escaped = false
			continue
		}
		if !inSingle && !inDouble && ch == '\\' {
			escaped = true
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			} else {
				buf.WriteRune(ch)
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			} else {
				buf.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if unicode.IsSpace(ch) {
			pushToken()
			continue
		}
		buf.WriteRune(ch)
	}

	if escaped || inSingle || inDouble {
		return nil // 解析失败
	}
	pushToken()
	return tokens
}

// ─── 随机数工具 ───

// RandomInt 返回 [0, max) 范围内的加密安全随机整数。
func RandomInt(max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0
	}
	return n.Int64()
}
