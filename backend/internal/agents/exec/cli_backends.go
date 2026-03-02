package exec

// ============================================================================
// CLI 后端配置解析
// 对应 TS: agents/cli-backends.ts (158L)
// ============================================================================

import (
	"strings"

	"github.com/openacosmi/claw-acismi/internal/agents/models"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ResolvedCliBackend 已解析的 CLI 后端配置。
type ResolvedCliBackend struct {
	ID     string
	Config *types.CliBackendConfig
}

// -------------------------------------------------------------------
// 默认 claude-cli 后端
// -------------------------------------------------------------------

var defaultClaudeBackend = types.CliBackendConfig{
	Command: "claude",
	Args:    []string{"-p", "--output-format", "json", "--dangerously-skip-permissions"},
	ResumeArgs: []string{
		"-p", "--output-format", "json", "--dangerously-skip-permissions",
		"--resume", "{sessionId}",
	},
	Output:           "json",
	Input:            "arg",
	ModelArg:         "--model",
	SessionArg:       "--session-id",
	SessionMode:      "always",
	SessionIDFields:  []string{"session_id", "sessionId", "conversation_id", "conversationId"},
	SystemPromptArg:  "--append-system-prompt",
	SystemPromptMode: "append",
	SystemPromptWhen: "first",
	ClearEnv:         []string{"ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY_OLD"},
	ModelAliases: map[string]string{
		"opus": "opus", "opus-4.6": "opus", "opus-4.5": "opus", "opus-4": "opus",
		"claude-opus-4-6": "opus", "claude-opus-4-5": "opus", "claude-opus-4": "opus",
		"sonnet": "sonnet", "sonnet-4.5": "sonnet", "sonnet-4.1": "sonnet", "sonnet-4.0": "sonnet",
		"claude-sonnet-4-5": "sonnet", "claude-sonnet-4-1": "sonnet", "claude-sonnet-4-0": "sonnet",
		"haiku": "haiku", "haiku-3.5": "haiku", "claude-haiku-3-5": "haiku",
	},
}

// -------------------------------------------------------------------
// 默认 codex-cli 后端
// -------------------------------------------------------------------

var defaultCodexBackend = types.CliBackendConfig{
	Command: "codex",
	Args: []string{
		"exec", "--json", "--color", "never",
		"--sandbox", "read-only", "--skip-git-repo-check",
	},
	ResumeArgs: []string{
		"exec", "resume", "{sessionId}",
		"--color", "never", "--sandbox", "read-only", "--skip-git-repo-check",
	},
	Output:          "jsonl",
	ResumeOutput:    "text",
	Input:           "arg",
	ModelArg:        "--model",
	SessionIDFields: []string{"thread_id"},
	SessionMode:     "existing",
	ImageArg:        "--image",
	ImageMode:       "repeat",
}

func normalizeBackendKey(key string) string {
	return models.NormalizeProviderId(key)
}

// pickBackendConfig 从用户配置中匹配键。
func pickBackendConfig(configured map[string]*types.CliBackendConfig, normalizedID string) *types.CliBackendConfig {
	for key, entry := range configured {
		if normalizeBackendKey(key) == normalizedID {
			return entry
		}
	}
	return nil
}

// mergeBackendConfig 将 override 合并到 base 上。
func mergeBackendConfig(base *types.CliBackendConfig, override *types.CliBackendConfig) *types.CliBackendConfig {
	if override == nil {
		return cloneBackendConfig(base)
	}
	merged := cloneBackendConfig(base)
	if override.Command != "" {
		merged.Command = override.Command
	}
	if override.Args != nil {
		merged.Args = override.Args
	}
	if override.Output != "" {
		merged.Output = override.Output
	}
	if override.ResumeOutput != "" {
		merged.ResumeOutput = override.ResumeOutput
	}
	if override.Input != "" {
		merged.Input = override.Input
	}
	if override.ModelArg != "" {
		merged.ModelArg = override.ModelArg
	}
	if override.SessionArg != "" {
		merged.SessionArg = override.SessionArg
	}
	if override.SessionArgs != nil {
		merged.SessionArgs = override.SessionArgs
	}
	if override.ResumeArgs != nil {
		merged.ResumeArgs = override.ResumeArgs
	}
	if override.SessionMode != "" {
		merged.SessionMode = override.SessionMode
	}
	if override.SessionIDFields != nil {
		merged.SessionIDFields = override.SessionIDFields
	}
	if override.SystemPromptArg != "" {
		merged.SystemPromptArg = override.SystemPromptArg
	}
	if override.SystemPromptMode != "" {
		merged.SystemPromptMode = override.SystemPromptMode
	}
	if override.SystemPromptWhen != "" {
		merged.SystemPromptWhen = override.SystemPromptWhen
	}
	if override.ImageArg != "" {
		merged.ImageArg = override.ImageArg
	}
	if override.ImageMode != "" {
		merged.ImageMode = override.ImageMode
	}
	if override.Serialize != nil {
		merged.Serialize = override.Serialize
	}
	// 合并 env
	for k, v := range override.Env {
		if merged.Env == nil {
			merged.Env = map[string]string{}
		}
		merged.Env[k] = v
	}
	// 合并 modelAliases
	for k, v := range override.ModelAliases {
		if merged.ModelAliases == nil {
			merged.ModelAliases = map[string]string{}
		}
		merged.ModelAliases[k] = v
	}
	// 合并 clearEnv（去重）
	if len(override.ClearEnv) > 0 {
		seen := map[string]bool{}
		for _, v := range merged.ClearEnv {
			seen[v] = true
		}
		for _, v := range override.ClearEnv {
			if !seen[v] {
				merged.ClearEnv = append(merged.ClearEnv, v)
				seen[v] = true
			}
		}
	}
	return merged
}

// cloneBackendConfig 浅克隆。
func cloneBackendConfig(src *types.CliBackendConfig) *types.CliBackendConfig {
	c := *src
	if src.Args != nil {
		c.Args = append([]string(nil), src.Args...)
	}
	if src.ClearEnv != nil {
		c.ClearEnv = append([]string(nil), src.ClearEnv...)
	}
	if src.Env != nil {
		c.Env = make(map[string]string, len(src.Env))
		for k, v := range src.Env {
			c.Env[k] = v
		}
	}
	if src.ModelAliases != nil {
		c.ModelAliases = make(map[string]string, len(src.ModelAliases))
		for k, v := range src.ModelAliases {
			c.ModelAliases[k] = v
		}
	}
	return &c
}

// ResolveCliBackendConfig 解析 CLI backend 配置。
// TS 对应: cli-backends.ts → resolveCliBackendConfig()
func ResolveCliBackendConfig(provider string, cfg *types.OpenAcosmiConfig) *ResolvedCliBackend {
	normalized := normalizeBackendKey(provider)
	var configured map[string]*types.CliBackendConfig
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil {
		configured = cfg.Agents.Defaults.CliBackends
	}
	override := pickBackendConfig(configured, normalized)

	switch normalized {
	case "claude-cli":
		merged := mergeBackendConfig(&defaultClaudeBackend, override)
		cmd := strings.TrimSpace(merged.Command)
		if cmd == "" {
			return nil
		}
		merged.Command = cmd
		return &ResolvedCliBackend{ID: normalized, Config: merged}
	case "codex-cli":
		merged := mergeBackendConfig(&defaultCodexBackend, override)
		cmd := strings.TrimSpace(merged.Command)
		if cmd == "" {
			return nil
		}
		merged.Command = cmd
		return &ResolvedCliBackend{ID: normalized, Config: merged}
	default:
		if override == nil {
			return nil
		}
		cmd := strings.TrimSpace(override.Command)
		if cmd == "" {
			return nil
		}
		return &ResolvedCliBackend{ID: normalized, Config: override}
	}
}
