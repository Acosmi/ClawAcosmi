package runner

// command_rule_engine.go — P3 Allow/Ask/Deny 命令规则匹配引擎
// 行业对照: ABAC/PBAC 策略引擎 (Cerbos, OPA)
//
// 对每条 bash 命令应用 CommandRule 规则集，返回匹配结果。
// 匹配优先级: deny > ask > allow。
// 模式匹配支持: glob（path.Match）、前缀、子串。

import (
	"path"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/infra"
)

// RuleMatchResult 规则匹配结果。
type RuleMatchResult struct {
	Matched bool                    // 是否命中规则
	Action  infra.CommandRuleAction // 匹配的动作
	Rule    *infra.CommandRule      // 匹配的规则（nil 表示无匹配）
	Reason  string                  // 人类可读描述
}

// EvaluateCommand 对命令应用规则集，返回匹配结果。
//
// 规则选取逻辑:
//  1. 收集所有匹配的规则
//  2. 按 action 权重排序: deny(0) > ask(1) > allow(2)
//  3. 同一 action 内按 priority 排序（值越小优先级越高）
//  4. 返回最高优先级的匹配规则
//
// 无匹配规则时返回 Matched=false。
func EvaluateCommand(command string, rules []infra.CommandRule) RuleMatchResult {
	if len(rules) == 0 || command == "" {
		return RuleMatchResult{Matched: false}
	}

	// Shell wrapper 预检: 提取 bash -c / sh -c / eval 内的实际命令并递归评估
	if innerCmd := extractShellWrapperCommand(command); innerCmd != "" {
		innerResult := EvaluateCommand(innerCmd, rules)
		if innerResult.Matched {
			return innerResult
		}
	}

	var bestMatch *infra.CommandRule
	bestWeight := 999 // 初始化为最低优先级

	for i := range rules {
		rule := &rules[i]
		if !matchPattern(command, rule.Pattern) {
			continue
		}
		weight := actionWeight(rule.Action)
		// deny(0) < ask(1) < allow(2) — 数值越小越优先
		if weight < bestWeight || (weight == bestWeight && (bestMatch == nil || rule.Priority < bestMatch.Priority)) {
			bestMatch = rule
			bestWeight = weight
		}
	}

	if bestMatch == nil {
		return RuleMatchResult{Matched: false}
	}

	reason := bestMatch.Description
	if reason == "" {
		reason = "Matched rule: " + bestMatch.Pattern
	}

	return RuleMatchResult{
		Matched: true,
		Action:  bestMatch.Action,
		Rule:    bestMatch,
		Reason:  reason,
	}
}

// matchPattern 检查命令是否匹配规则模式。
//
// 匹配策略（按优先级）:
//  1. 精确匹配
//  2. glob 模式匹配（使用 path.Match，如 "rm -rf *"）
//  3. 前缀匹配（模式末尾有空格或 *，如 "npm *" 匹配 "npm install"）
//  4. 子串匹配（模式被 * 包围，如 "*sudo*" 匹配 "echo | sudo tee"）
//
// 所有匹配均为大小写不敏感。
func matchPattern(command, pattern string) bool {
	if pattern == "" {
		return false
	}

	cmdLower := strings.ToLower(strings.TrimSpace(command))
	patLower := strings.ToLower(strings.TrimSpace(pattern))

	// 精确匹配
	if cmdLower == patLower {
		return true
	}

	// 多段通配符匹配: "*segment1*segment2*"
	// 将 pattern 按 * 分割，检查各段是否按顺序出现在命令中
	if strings.Contains(patLower, "*") {
		if matchMultiGlob(cmdLower, patLower) {
			return true
		}
	}

	// glob 模式匹配 (path.Match)
	if matched, err := path.Match(patLower, cmdLower); err == nil && matched {
		return true
	}

	// 前缀匹配: "npm " 匹配 "npm install express"
	if strings.HasSuffix(patLower, " ") {
		prefix := strings.TrimRight(patLower, " ")
		if strings.HasPrefix(cmdLower, prefix) {
			return true
		}
	}

	// 以模式开头匹配（如 "rm -rf /" 匹配 "rm -rf /tmp/foo"）
	// Word boundary: 确保模式后紧跟空格或结尾，防止 "shutdown" 误杀 "shutdownGuard"
	if strings.HasPrefix(cmdLower, patLower) {
		if len(cmdLower) == len(patLower) || cmdLower[len(patLower)] == ' ' {
			return true
		}
	}

	return false
}

// matchMultiGlob 多段通配符匹配。
// 将 pattern 按 "*" 分割为多个段，检查各段是否按顺序出现在命令中。
// 例如: pattern "*dd *of=/dev/*" 匹配 "dd if=/dev/zero of=/dev/sda"
//
// 规则:
//   - 如果 pattern 不以 "*" 开头，命令必须以第一段为前缀
//   - 如果 pattern 不以 "*" 结尾，命令必须以最后一段为后缀
//   - 中间段必须按顺序出现
func matchMultiGlob(cmd, pattern string) bool {
	segments := strings.Split(pattern, "*")

	// 过滤空段
	nonEmpty := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg != "" {
			nonEmpty = append(nonEmpty, seg)
		}
	}
	if len(nonEmpty) == 0 {
		// pattern 全是 *
		return true
	}

	startsWithStar := strings.HasPrefix(pattern, "*")
	endsWithStar := strings.HasSuffix(pattern, "*")

	pos := 0

	for i, seg := range nonEmpty {
		if i == 0 && !startsWithStar {
			// 第一段必须是前缀
			if !strings.HasPrefix(cmd, seg) {
				return false
			}
			pos = len(seg)
			continue
		}

		idx := strings.Index(cmd[pos:], seg)
		if idx < 0 {
			return false
		}
		pos += idx + len(seg)
	}

	// 如果 pattern 不以 * 结尾，命令必须以最后一段为后缀
	if !endsWithStar {
		lastSeg := nonEmpty[len(nonEmpty)-1]
		if !strings.HasSuffix(cmd, lastSeg) {
			return false
		}
	}

	return true
}

// actionWeight 返回动作的权重，用于优先级排序。
// deny=0（最高）, ask=1, allow=2（最低）。
func actionWeight(action infra.CommandRuleAction) int {
	switch action {
	case infra.RuleActionDeny:
		return 0
	case infra.RuleActionAsk:
		return 1
	case infra.RuleActionAllow:
		return 2
	default:
		return 3
	}
}

// extractShellWrapperCommand 从 shell wrapper 命令中提取内部实际命令。
// 处理 "bash -c 'rm -rf /'"、"sh -c 'shutdown'"、"eval 'rm *'" 等绕过模式。
// 返回内部命令字符串，无匹配返回空字符串。
func extractShellWrapperCommand(command string) string {
	cmdLower := strings.ToLower(strings.TrimSpace(command))

	// bash -c / sh -c / zsh -c 模式
	for _, shell := range []string{"bash -c ", "sh -c ", "zsh -c ", "/bin/bash -c ", "/bin/sh -c "} {
		if strings.HasPrefix(cmdLower, shell) {
			inner := strings.TrimSpace(command[len(shell):])
			return unquote(inner)
		}
	}

	// eval 模式
	if strings.HasPrefix(cmdLower, "eval ") {
		inner := strings.TrimSpace(command[5:])
		return unquote(inner)
	}

	return ""
}

// unquote 去除字符串首尾引号（单引号或双引号）。
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}
