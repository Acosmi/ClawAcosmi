// 控制命令门控 — 对齐 TS channels/command-gating.ts (46L)
// 共享实现，供 telegram/imessage/discord 等频道复用。

package channels

// ControlCommandGateResult 控制命令门控结果
type ControlCommandGateResult struct {
	CommandAuthorized bool
	ShouldBlock       bool
}

// ControlCommandAuthorizer 单个授权者
type ControlCommandAuthorizer struct {
	Configured bool
	Allowed    bool
}

// ControlCommandGateParams 门控参数
type ControlCommandGateParams struct {
	UseAccessGroups   bool
	Authorizers       []ControlCommandAuthorizer
	AllowTextCommands bool
	HasControlCommand bool
}

// ResolveControlCommandGate 解析控制命令门控。
// TS 对照: channels/command-gating.ts resolveControlCommandGate()
func ResolveControlCommandGate(params ControlCommandGateParams) ControlCommandGateResult {
	if !params.HasControlCommand {
		return ControlCommandGateResult{
			CommandAuthorized: false,
			ShouldBlock:       false,
		}
	}

	// 如果不启用访问组，所有命令均授权
	if !params.UseAccessGroups {
		return ControlCommandGateResult{
			CommandAuthorized: true,
			ShouldBlock:       false,
		}
	}

	// 检查所有授权者：至少一个已配置且允许 → 授权
	authorized := false
	anyConfigured := false
	for _, auth := range params.Authorizers {
		if auth.Configured {
			anyConfigured = true
			if auth.Allowed {
				authorized = true
				break
			}
		}
	}

	// 无任何授权者配置 → 放行
	if !anyConfigured {
		authorized = true
	}

	shouldBlock := false
	if params.AllowTextCommands && params.HasControlCommand && !authorized {
		shouldBlock = true
	}

	return ControlCommandGateResult{
		CommandAuthorized: authorized,
		ShouldBlock:       shouldBlock,
	}
}
