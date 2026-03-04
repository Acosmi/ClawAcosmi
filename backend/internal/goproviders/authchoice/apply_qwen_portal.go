// authchoice/apply_qwen_portal.go — 通义千问 Portal 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.qwen-portal.ts
// 通义千问 Portal 直接委托给 plugin provider 通用流程。
package authchoice

// ApplyAuthChoiceQwenPortal 通义千问 Portal 认证 apply。
// 委托给 ApplyAuthChoicePluginProvider 通用流程。
// 对应 TS: applyAuthChoiceQwenPortal()
func ApplyAuthChoiceQwenPortal(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	return ApplyAuthChoicePluginProvider(params, PluginProviderAuthChoiceOptions{
		AuthChoice: "qwen-portal",
		PluginID:   "qwen-portal-auth",
		ProviderID: "qwen-portal",
		MethodID:   "device",
		Label:      "Qwen",
	})
}
