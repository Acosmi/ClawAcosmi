package gateway

// server_methods_security.go — security.* 方法处理器
// 提供安全级别的聚合查询 API，供前端安全设置页面使用。
//
// security.get 返回当前全局安全级别（从 exec-approvals.json 的 defaults.security 读取）。
// 写入操作复用 exec.approvals.set API。

import (
	"github.com/anthropic/open-acosmi/internal/infra"
)

// SecurityHandlers 返回 security.* 方法处理器映射。
func SecurityHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"security.get": handleSecurityGet,
	}
}

// ---------- security.get ----------
// 返回当前安全级别信息，供前端安全设置页面使用。

func handleSecurityGet(ctx *MethodHandlerContext) {
	if _, err := infra.EnsureExecApprovals(); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to ensure exec-approvals: "+err.Error()))
		return
	}

	snapshot := infra.ReadExecApprovalsSnapshot()
	file := snapshot.File

	// 解析当前安全级别
	currentLevel := string(infra.ExecSecurityDeny)
	if file != nil && file.Defaults != nil && file.Defaults.Security != "" {
		currentLevel = string(file.Defaults.Security)
	}

	// 判断是否为永久授权（full 模式）
	isPermanentFull := currentLevel == string(infra.ExecSecurityFull)

	// 构建安全级别描述
	levels := []map[string]interface{}{
		{
			"id":            string(infra.ExecSecurityDeny),
			"label":         "L0 — Read Only",
			"labelZh":       "L0 — 只读",
			"description":   "Agent can only read files and analyze code. No write or execute permissions.",
			"descriptionZh": "智能体只能读取文件和分析代码。没有写入或执行权限。",
			"risk":          "low",
			"active":        currentLevel == string(infra.ExecSecurityDeny),
		},
		{
			"id":            string(infra.ExecSecurityAllowlist),
			"label":         "L1 — Allowlist",
			"labelZh":       "L1 — 允许列表",
			"description":   "Agent can execute pre-approved commands from the allowlist. Write access controlled by rules.",
			"descriptionZh": "智能体可以执行允许列表中预批准的命令。写入权限由规则控制。",
			"risk":          "medium",
			"active":        currentLevel == string(infra.ExecSecurityAllowlist),
		},
		{
			"id":            string(infra.ExecSecurityFull),
			"label":         "L2 — Full Access",
			"labelZh":       "L2 — 完全访问",
			"description":   "Agent has full write and execute permissions. Use with caution — permanent authorization requires confirmation.",
			"descriptionZh": "智能体拥有完全的写入和执行权限。请谨慎使用 — 永久授权需要确认。",
			"risk":          "high",
			"active":        currentLevel == string(infra.ExecSecurityFull),
		},
	}

	ctx.Respond(true, map[string]interface{}{
		"currentLevel":    currentLevel,
		"isPermanentFull": isPermanentFull,
		"levels":          levels,
		"hash":            snapshot.Hash,
	}, nil)
}
