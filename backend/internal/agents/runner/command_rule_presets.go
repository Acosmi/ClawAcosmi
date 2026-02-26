package runner

// command_rule_presets.go — P3 预设安全规则集
// 行业对照: ABAC/PBAC 策略引擎预设策略
//
// 内置危险命令拦截规则，防止智能体执行高风险操作。
// 这些规则 IsPreset=true，用户不可删除。

import "github.com/anthropic/open-acosmi/internal/infra"

// PresetCommandRules 内置安全规则集（不可变）。
// 按 deny → ask → allow 分组，每组内按优先级排序。
var PresetCommandRules = []infra.CommandRule{
	// ---------- deny: 危险命令（绝对禁止）----------
	{
		ID:          "preset-deny-rm-root",
		Pattern:     "rm -rf /",
		Action:      infra.RuleActionDeny,
		Description: "Block: delete root filesystem",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-rm-root-star",
		Pattern:     "rm -rf /*",
		Action:      infra.RuleActionDeny,
		Description: "Block: delete root filesystem contents",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-chmod-777",
		Pattern:     "chmod 777 *",
		Action:      infra.RuleActionDeny,
		Description: "Block: overly permissive file permissions",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-chmod-r-777",
		Pattern:     "chmod -R 777 *",
		Action:      infra.RuleActionDeny,
		Description: "Block: recursive overly permissive permissions",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-mkfs",
		Pattern:     "mkfs*",
		Action:      infra.RuleActionDeny,
		Description: "Block: format disk",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-dd-dev",
		Pattern:     "*dd *of=/dev/*",
		Action:      infra.RuleActionDeny,
		Description: "Block: overwrite device",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-forkbomb",
		Pattern:     "*(){*|*&*};*",
		Action:      infra.RuleActionDeny,
		Description: "Block: fork bomb",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-shutdown",
		Pattern:     "shutdown*",
		Action:      infra.RuleActionDeny,
		Description: "Block: system shutdown",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-reboot",
		Pattern:     "reboot*",
		Action:      infra.RuleActionDeny,
		Description: "Block: system reboot",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-halt",
		Pattern:     "halt*",
		Action:      infra.RuleActionDeny,
		Description: "Block: system halt",
		IsPreset:    true,
		Priority:    0,
	},

	// ---------- ask: 需要确认的命令 ----------
	{
		ID:          "preset-ask-curl-pipe",
		Pattern:     "*curl*|*sh*",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: pipe remote script to shell",
		IsPreset:    true,
		Priority:    10,
	},
	{
		ID:          "preset-ask-wget-pipe",
		Pattern:     "*wget*|*sh*",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: pipe remote download to shell",
		IsPreset:    true,
		Priority:    10,
	},
	{
		ID:          "preset-ask-sudo",
		Pattern:     "sudo *",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: privilege escalation via sudo",
		IsPreset:    true,
		Priority:    10,
	},

	// ---------- allow: 安全开发命令 ----------
	{
		ID:          "preset-allow-npm-install",
		Pattern:     "npm install*",
		Action:      infra.RuleActionAllow,
		Description: "Allow: npm package installation",
		IsPreset:    true,
		Priority:    20,
	},
	{
		ID:          "preset-allow-pip-install",
		Pattern:     "pip install*",
		Action:      infra.RuleActionAllow,
		Description: "Allow: Python package installation",
		IsPreset:    true,
		Priority:    20,
	},
	{
		ID:          "preset-allow-go-build",
		Pattern:     "go build*",
		Action:      infra.RuleActionAllow,
		Description: "Allow: Go build",
		IsPreset:    true,
		Priority:    20,
	},
	{
		ID:          "preset-allow-go-test",
		Pattern:     "go test*",
		Action:      infra.RuleActionAllow,
		Description: "Allow: Go test",
		IsPreset:    true,
		Priority:    20,
	},
	{
		ID:          "preset-allow-go-vet",
		Pattern:     "go vet*",
		Action:      infra.RuleActionAllow,
		Description: "Allow: Go vet",
		IsPreset:    true,
		Priority:    20,
	},
	{
		ID:          "preset-allow-pnpm",
		Pattern:     "pnpm *",
		Action:      infra.RuleActionAllow,
		Description: "Allow: pnpm commands",
		IsPreset:    true,
		Priority:    20,
	},
}

// MergeRulesWithPresets 将用户自定义规则与预设规则合并。
// 预设规则始终在前（优先级更高）。
func MergeRulesWithPresets(userRules []infra.CommandRule) []infra.CommandRule {
	merged := make([]infra.CommandRule, 0, len(PresetCommandRules)+len(userRules))
	merged = append(merged, PresetCommandRules...)
	merged = append(merged, userRules...)
	return merged
}
