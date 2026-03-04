// authchoice/model_check.go — 模型配置检查
// 对应 TS 文件: src/commands/auth-choice.model-check.ts
package authchoice

import (
	"fmt"
	"strings"
)

// WarnIfModelConfigLooksOff 检查模型配置是否看起来有问题。
// 检查项：模型是否在目录中、是否有对应的认证配置、是否有 Codex 提示。
// 对应 TS: warnIfModelConfigLooksOff()
func WarnIfModelConfigLooksOff(
	config OpenClawConfig,
	prompter WizardPrompter,
	agentID string,
	agentDir string,
) error {
	ref := ResolveDefaultModelForAgent(config, agentID)
	var warnings []string

	// 检查模型是否在目录中
	catalog := LoadModelCatalog(config)
	if len(catalog) > 0 {
		known := false
		for _, entry := range catalog {
			if entry.Provider == ref.Provider && entry.ID == ref.Model {
				known = true
				break
			}
		}
		if !known {
			warnings = append(warnings, fmt.Sprintf(
				"Model not found: %s/%s. Update agents.defaults.model or run /models list.",
				ref.Provider, ref.Model,
			))
		}
	}

	// 检查是否有认证配置
	store := EnsureAuthProfileStore(agentDir)
	hasProfile := len(ListProfilesForProvider(store, ref.Provider)) > 0
	envKey := ResolveEnvApiKey(ref.Provider)
	customKey := GetCustomProviderApiKey(config, ref.Provider)
	if !hasProfile && envKey == nil && customKey == "" {
		warnings = append(warnings, fmt.Sprintf(
			"No auth configured for provider %q. The agent may fail until credentials are added.",
			ref.Provider,
		))
	}

	// 检查 OpenAI Codex 提示
	if ref.Provider == "openai" {
		hasCodex := len(ListProfilesForProvider(store, "openai-codex")) > 0
		if hasCodex {
			warnings = append(warnings, fmt.Sprintf(
				"Detected OpenAI Codex OAuth. Consider setting agents.defaults.model to %s.",
				OpenaiCodexDefaultModel,
			))
		}
	}

	if len(warnings) > 0 {
		return prompter.Note(strings.Join(warnings, "\n"), "Model check")
	}
	return nil
}
