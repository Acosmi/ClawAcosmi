// tools/policy.go — 工具策略匹配引擎（高层）。
// TS 参考：src/agents/pi-tools.policy.ts (339L)
// 注意：底层策略函数（NormalizeToolName, ExpandToolGroups 等）已在 scope/tool_policy.go 中实现。
package tools

import (
	"regexp"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/scope"
)

// ---------- Pattern Matching ----------

// CompiledPattern 编译后的工具名匹配模式。
type CompiledPattern struct {
	raw     string
	isExact bool
	regex   *regexp.Regexp
}

// CompilePattern 编译通配符模式 → 正则表达式。
// 支持 *、**、group:xxx 展开。
// TS 参考: pi-tools.policy.ts compilePatterns
func CompilePattern(pattern string) *CompiledPattern {
	trimmed := strings.TrimSpace(pattern)

	// group: 开头 → 展开
	if strings.HasPrefix(trimmed, "group:") {
		expanded := scope.ExpandToolGroups([]string{trimmed})
		if len(expanded) == 1 && expanded[0] == trimmed {
			// 未知组 → 精确匹配
			return &CompiledPattern{raw: trimmed, isExact: true}
		}
		// 构建 or 正则
		escaped := make([]string, len(expanded))
		for i, name := range expanded {
			escaped[i] = regexp.QuoteMeta(name)
		}
		re := regexp.MustCompile("^(" + strings.Join(escaped, "|") + ")$")
		return &CompiledPattern{raw: trimmed, regex: re}
	}

	// 无通配符 → 精确匹配
	if !strings.Contains(trimmed, "*") {
		return &CompiledPattern{raw: trimmed, isExact: true}
	}

	// 通配符 → 正则
	regexStr := "^" + wildcardToRegex(trimmed) + "$"
	re, err := regexp.Compile(regexStr)
	if err != nil {
		return &CompiledPattern{raw: trimmed, isExact: true}
	}
	return &CompiledPattern{raw: trimmed, regex: re}
}

// Matches 检查工具名是否匹配此模式。
func (p *CompiledPattern) Matches(toolName string) bool {
	if p.isExact {
		return p.raw == toolName
	}
	if p.regex != nil {
		return p.regex.MatchString(toolName)
	}
	return false
}

// CompilePatterns 批量编译模式列表。
func CompilePatterns(patterns []string) []*CompiledPattern {
	result := make([]*CompiledPattern, len(patterns))
	for i, p := range patterns {
		result[i] = CompilePattern(p)
	}
	return result
}

// MatchesAny 检查工具名是否匹配任一模式。
func MatchesAny(toolName string, patterns []*CompiledPattern) bool {
	for _, p := range patterns {
		if p.Matches(toolName) {
			return true
		}
	}
	return false
}

// ---------- Tool Policy Matcher ----------

// ToolPolicyMatcher 策略匹配器。
type ToolPolicyMatcher struct {
	AllowPatterns []*CompiledPattern
	DenyPatterns  []*CompiledPattern
	AllowAll      bool // allow 为空或包含 "*"
}

// MakeToolPolicyMatcher 从策略创建匹配器。
// TS 参考: pi-tools.policy.ts makeToolPolicyMatcher
func MakeToolPolicyMatcher(policy *scope.ToolPolicy) *ToolPolicyMatcher {
	if policy == nil {
		return &ToolPolicyMatcher{AllowAll: true}
	}

	matcher := &ToolPolicyMatcher{}

	if len(policy.Allow) == 0 {
		matcher.AllowAll = true
	} else {
		for _, a := range policy.Allow {
			if a == "*" {
				matcher.AllowAll = true
				break
			}
		}
		if !matcher.AllowAll {
			expanded := scope.ExpandToolGroups(policy.Allow)
			matcher.AllowPatterns = CompilePatterns(expanded)
		}
	}

	if len(policy.Deny) > 0 {
		expanded := scope.ExpandToolGroups(policy.Deny)
		matcher.DenyPatterns = CompilePatterns(expanded)
	}

	return matcher
}

// IsAllowed 检查工具是否被策略允许。
func (m *ToolPolicyMatcher) IsAllowed(toolName string) bool {
	normalized := scope.NormalizeToolName(toolName)

	// deny 优先
	if len(m.DenyPatterns) > 0 && MatchesAny(normalized, m.DenyPatterns) {
		return false
	}

	// allow 检查
	if m.AllowAll {
		return true
	}
	return MatchesAny(normalized, m.AllowPatterns)
}

// ---------- 多策略求值 ----------

// PolicyLayer 策略层。
type PolicyLayer struct {
	Name    string // 用于调试
	Matcher *ToolPolicyMatcher
}

// IsToolAllowedByPolicies 检查工具是否通过所有策略层。
// TS 参考: pi-tools.policy.ts isToolAllowed
func IsToolAllowedByPolicies(toolName string, layers []PolicyLayer) bool {
	for _, layer := range layers {
		if !layer.Matcher.IsAllowed(toolName) {
			return false
		}
	}
	return true
}

// FilterToolsByPolicy 按策略过滤工具列表。
// TS 参考: pi-tools.policy.ts filterToolsByPolicy
func FilterToolsByPolicy(tools []*AgentTool, layers []PolicyLayer) []*AgentTool {
	if len(layers) == 0 {
		return tools
	}
	var result []*AgentTool
	for _, tool := range tools {
		if IsToolAllowedByPolicies(tool.Name, layers) {
			result = append(result, tool)
		}
	}
	return result
}

// ---------- 子代理策略 ----------

// SubagentPolicyDefaults 子代理工具的默认策略。
// TS 参考: pi-tools.policy.ts resolveSubagentToolPolicy
var SubagentPolicyDefaults = &scope.ToolPolicy{
	Deny: []string{"gateway", "cron*", "sessions_spawn"},
}

// ResolveSubagentToolPolicy 解析子代理工具策略。
func ResolveSubagentToolPolicy(subagentPolicy *scope.ToolPolicy) *scope.ToolPolicy {
	if subagentPolicy != nil {
		return subagentPolicy
	}
	return SubagentPolicyDefaults
}

// ---------- 群组策略 ----------

// ResolveGroupToolPolicy 从群组/频道配置解析工具策略。
// TS 参考: pi-tools.policy.ts resolveGroupToolPolicy
func ResolveGroupToolPolicy(groupPolicy *scope.ToolPolicy, isGroupChat bool) *scope.ToolPolicy {
	if !isGroupChat {
		return nil
	}
	if groupPolicy != nil {
		return groupPolicy
	}
	// 群组默认策略 — 限制高危操作
	return &scope.ToolPolicy{
		Deny: []string{"gateway", "cron*"},
	}
}

// ---------- 辅助 ----------

// wildcardToRegex 将通配符模式转为正则。
func wildcardToRegex(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}
