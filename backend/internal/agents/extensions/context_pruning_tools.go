package extensions

// context_pruning_tools.go — 工具可剪枝性判定（glob 模式匹配）
// 对应 TS: agents/pi-extensions/context-pruning/tools.ts (70L)
//
// 提供基于 allow/deny glob 的工具名称匹配。

import (
	"regexp"
	"strings"
)

// compiledPattern 编译后的模式。
type compiledPattern struct {
	kind  string // "all" | "exact" | "regex"
	value string
	re    *regexp.Regexp
}

// normalizePatterns 规范化模式列表。
func normalizePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	var result []string
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// compilePattern 编译单个模式。
func compilePattern(pattern string) compiledPattern {
	if pattern == "*" {
		return compiledPattern{kind: "all"}
	}
	if !strings.Contains(pattern, "*") {
		return compiledPattern{kind: "exact", value: pattern}
	}

	// 转换 glob * 为正则 .*
	escaped := regexp.QuoteMeta(pattern)
	// QuoteMeta 会转义 *，恢复 \* 为 .*
	reStr := strings.ReplaceAll(escaped, `\*`, ".*")
	re, err := regexp.Compile("^" + reStr + "$")
	if err != nil {
		return compiledPattern{kind: "exact", value: pattern}
	}
	return compiledPattern{kind: "regex", re: re}
}

// compilePatterns 编译模式列表。
func compilePatterns(patterns []string) []compiledPattern {
	normalized := normalizePatterns(patterns)
	result := make([]compiledPattern, 0, len(normalized))
	for _, p := range normalized {
		result = append(result, compilePattern(p))
	}
	return result
}

// matchesAny 检查工具名是否匹配任意模式。
func matchesAny(toolName string, patterns []compiledPattern) bool {
	for _, p := range patterns {
		switch p.kind {
		case "all":
			return true
		case "exact":
			if toolName == p.value {
				return true
			}
		case "regex":
			if p.re != nil && p.re.MatchString(toolName) {
				return true
			}
		}
	}
	return false
}

// MakeToolPrunablePredicate 创建工具可剪枝性判定函数。
// 对应 TS: makeToolPrunablePredicate
func MakeToolPrunablePredicate(match ContextPruningToolMatch) func(string) bool {
	deny := compilePatterns(match.Deny)
	allow := compilePatterns(match.Allow)

	return func(toolName string) bool {
		normalized := strings.ToLower(strings.TrimSpace(toolName))
		if matchesAny(normalized, deny) {
			return false
		}
		if len(allow) == 0 {
			return true
		}
		return matchesAny(normalized, allow)
	}
}
