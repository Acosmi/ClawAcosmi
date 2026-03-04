// authchoice/apply_plugin_provider.go — 插件 Provider 通用 apply
// 对应 TS 文件: src/commands/auth-choice.apply.plugin-provider.ts
// 提供插件 Provider 的通用认证 apply 流程。
// 当前为 stub 实现，因为插件系统需窗口 7/8 补全；
// 但保留了完整的类型定义和逻辑骨架。
package authchoice

import "fmt"

// PluginProviderAuthChoiceOptions 插件 Provider 认证选项。
// 对应 TS: PluginProviderAuthChoiceOptions
type PluginProviderAuthChoiceOptions struct {
	AuthChoice string
	PluginID   string
	ProviderID string
	MethodID   string
	Label      string
}

// PluginProviderAuthRunResult 插件 Provider 认证执行结果。
// 对应 TS: method.run() 的返回类型
type PluginProviderAuthRunResult struct {
	ConfigPatch  OpenClawConfig
	Profiles     []PluginProviderProfile
	DefaultModel string
	Notes        []string
}

// PluginProviderProfile 插件 Provider 认证 Profile。
type PluginProviderProfile struct {
	ProfileID  string
	Credential PluginProviderCredential
}

// PluginProviderCredential 插件 Provider 凭据。
type PluginProviderCredential struct {
	Type     string // "token" | "oauth" | "api_key"
	Provider string
	Email    string
}

// ApplyAuthChoicePluginProvider 插件 Provider 通用认证 apply。
// 处理插件启用、Provider 解析、认证方法执行、Profile 保存和模型配置。
// 对应 TS: applyAuthChoicePluginProvider()
func ApplyAuthChoicePluginProvider(params ApplyAuthChoiceParams, options PluginProviderAuthChoiceOptions) (*ApplyAuthChoiceResult, error) {
	if string(params.AuthChoice) != options.AuthChoice {
		return nil, nil
	}

	nextConfig := params.Config

	// 启用插件（stub：假设已启用）
	enabled, reason := EnablePluginInConfig(nextConfig, options.PluginID)
	if !enabled {
		msg := fmt.Sprintf("%s plugin is disabled (%s).", options.Label, reason)
		_ = params.Prompter.Note(msg, options.Label)
		return &ApplyAuthChoiceResult{Config: nextConfig}, nil
	}

	// 解析插件 Provider（stub）
	provider := ResolvePluginProvider(nextConfig, options.ProviderID)
	if provider == nil {
		msg := fmt.Sprintf("%s auth plugin is not available. Enable it and re-run the wizard.", options.Label)
		_ = params.Prompter.Note(msg, options.Label)
		return &ApplyAuthChoiceResult{Config: nextConfig}, nil
	}

	// 执行认证方法（stub）
	result, err := RunPluginAuthMethod(RunPluginAuthMethodParams{
		Config:     nextConfig,
		AgentDir:   params.AgentDir,
		Prompter:   params.Prompter,
		Runtime:    params.Runtime,
		IsRemote:   IsRemoteEnvironment(),
		ProviderID: options.ProviderID,
		MethodID:   options.MethodID,
		Label:      options.Label,
	})
	if err != nil {
		return nil, err
	}

	// 应用配置补丁
	if result.ConfigPatch != nil {
		nextConfig = MergeConfigPatch(nextConfig, result.ConfigPatch)
	}

	// 保存 Profile 并更新配置
	for _, profile := range result.Profiles {
		UpsertAuthProfile(profile.ProfileID, profile.Credential)

		mode := profile.Credential.Type
		if mode == "token" {
			mode = "token"
		}
		applyParams := ApplyProfileConfigParams{
			ProfileID: profile.ProfileID,
			Provider:  profile.Credential.Provider,
			Mode:      mode,
		}
		nextConfig = ApplyAuthProfileConfig(nextConfig, applyParams)
	}

	// 处理默认模型
	var agentModelOverride string
	if result.DefaultModel != "" {
		if params.SetDefaultModel {
			nextConfig = ApplyDefaultModel(nextConfig, result.DefaultModel)
			_ = params.Prompter.Note(
				fmt.Sprintf("Default model set to %s", result.DefaultModel),
				"Model configured",
			)
		} else if params.AgentID != "" {
			agentModelOverride = result.DefaultModel
			_ = params.Prompter.Note(
				fmt.Sprintf("Default model set to %s for agent \"%s\".", result.DefaultModel, params.AgentID),
				"Model configured",
			)
		}
	}

	// 显示 Provider 注释
	if len(result.Notes) > 0 {
		noteText := ""
		for i, n := range result.Notes {
			if i > 0 {
				noteText += "\n"
			}
			noteText += n
		}
		_ = params.Prompter.Note(noteText, "Provider notes")
	}

	return &ApplyAuthChoiceResult{Config: nextConfig, AgentModelOverride: agentModelOverride}, nil
}
