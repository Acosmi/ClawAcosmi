package runner

// command_rule_presets.go — P3 预设安全规则集
// 行业对照: ABAC/PBAC 策略引擎预设策略
//
// 内置危险命令拦截规则，防止智能体执行高风险操作。
// 这些规则 IsPreset=true，用户不可删除。

import "github.com/Acosmi/ClawAcosmi/internal/infra"

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

	// ---------- deny: 家目录删除 ----------
	{
		ID:          "preset-deny-rm-home",
		Pattern:     "rm -rf ~*",
		Action:      infra.RuleActionDeny,
		Description: "Block: delete home directory",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-rm-home-var",
		Pattern:     "rm -rf $HOME*",
		Action:      infra.RuleActionDeny,
		Description: "Block: delete home directory via $HOME",
		IsPreset:    true,
		Priority:    0,
	},

	// ---------- deny: 系统管理命令 ----------
	{
		ID:          "preset-deny-iptables-flush",
		Pattern:     "iptables -F*",
		Action:      infra.RuleActionDeny,
		Description: "Block: flush firewall rules",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-crontab-remove",
		Pattern:     "crontab -r*",
		Action:      infra.RuleActionDeny,
		Description: "Block: remove all cron jobs",
		IsPreset:    true,
		Priority:    0,
	},

	// ---------- deny: 危险搜索模式（防止 Token 浪费 + 安全风险）----------
	{
		ID:          "preset-deny-find-home",
		Pattern:     "find ~ *",
		Action:      infra.RuleActionDeny,
		Description: "Block: unconstrained home directory search (use known paths from history)",
		IsPreset:    true,
		Priority:    0,
	},
	{
		ID:          "preset-deny-find-root",
		Pattern:     "find / *",
		Action:      infra.RuleActionDeny,
		Description: "Block: unconstrained root filesystem search",
		IsPreset:    true,
		Priority:    0,
	},

	// ---------- ask: 需要确认的命令 ----------
	{
		ID:          "preset-ask-rm",
		Pattern:     "rm *",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: file deletion",
		IsPreset:    true,
		Priority:    5,
	},
	{
		ID:          "preset-ask-rmdir",
		Pattern:     "rmdir *",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: directory deletion",
		IsPreset:    true,
		Priority:    5,
	},
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
	{
		ID:          "preset-ask-killall",
		Pattern:     "killall *",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: kill all processes by name",
		IsPreset:    true,
		Priority:    10,
	},
	{
		ID:          "preset-ask-pkill",
		Pattern:     "pkill *",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: kill processes by pattern",
		IsPreset:    true,
		Priority:    10,
	},
	{
		ID:          "preset-ask-truncate",
		Pattern:     "truncate *",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: truncate file to specified size",
		IsPreset:    true,
		Priority:    10,
	},

	// ---------- ask: 服务管理命令（D1-F1: 防止目标偏移） ----------
	{
		ID:          "preset-ask-gateway-start",
		Pattern:     "*openacosmi*gateway*start*",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: start gateway service",
		IsPreset:    true,
		Priority:    10,
	},
	{
		ID:          "preset-ask-gateway-stop",
		Pattern:     "*openacosmi*gateway*stop*",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: stop gateway service",
		IsPreset:    true,
		Priority:    10,
	},
	{
		ID:          "preset-ask-gateway-restart",
		Pattern:     "*openacosmi*gateway*restart*",
		Action:      infra.RuleActionAsk,
		Description: "Confirm: restart gateway service",
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
