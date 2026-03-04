// authchoice/apply_copilot_proxy.go — Copilot Proxy 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.copilot-proxy.ts
// Copilot Proxy 直接委托给 plugin provider 通用流程。
package authchoice

// ApplyAuthChoiceCopilotProxy Copilot Proxy 认证 apply。
// 委托给 ApplyAuthChoicePluginProvider 通用流程。
// 对应 TS: applyAuthChoiceCopilotProxy()
func ApplyAuthChoiceCopilotProxy(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	return ApplyAuthChoicePluginProvider(params, PluginProviderAuthChoiceOptions{
		AuthChoice: "copilot-proxy",
		PluginID:   "copilot-proxy",
		ProviderID: "copilot-proxy",
		MethodID:   "local",
		Label:      "Copilot Proxy",
	})
}
