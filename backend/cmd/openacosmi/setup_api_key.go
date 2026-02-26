package main

// setup_api_key.go — API Key 管理工具
// 对应 TS src/commands/auth-choice.api-key.ts (49L) + model-auth.ts 部分

import (
	"os"
	"regexp"
	"strings"
)

// ---------- API Key 规范化 ----------

// shellAssignmentRe 匹配 shell 赋值格式: export KEY="value" 或 KEY=value
var shellAssignmentRe = regexp.MustCompile(`^(?:export\s+)?[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$`)

// NormalizeApiKeyInput 规范化 API key 输入。
// 处理 shell 赋值格式、去引号、去分号。
// 对应 TS: normalizeApiKeyInput
func NormalizeApiKeyInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// 处理 shell 赋值: export KEY="value" 或 KEY=value
	valuePart := trimmed
	if matches := shellAssignmentRe.FindStringSubmatch(trimmed); len(matches) > 1 {
		valuePart = strings.TrimSpace(matches[1])
	}

	// 去除引号
	unquoted := valuePart
	if len(valuePart) >= 2 {
		first, last := valuePart[0], valuePart[len(valuePart)-1]
		if (first == '"' && last == '"') ||
			(first == '\'' && last == '\'') ||
			(first == '`' && last == '`') {
			unquoted = valuePart[1 : len(valuePart)-1]
		}
	}

	// 去分号
	withoutSemicolon := unquoted
	if strings.HasSuffix(unquoted, ";") {
		withoutSemicolon = unquoted[:len(unquoted)-1]
	}

	return strings.TrimSpace(withoutSemicolon)
}

// ValidateApiKeyInput 验证 API key 输入非空。
func ValidateApiKeyInput(value string) string {
	if NormalizeApiKeyInput(value) == "" {
		return "Required"
	}
	return ""
}

// ---------- API Key 预览 ----------

const (
	defaultKeyPreviewHead = 4
	defaultKeyPreviewTail = 4
)

// FormatApiKeyPreview 格式化 API key 预览（head…tail）。
// 对应 TS: formatApiKeyPreview
func FormatApiKeyPreview(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "…"
	}

	head := defaultKeyPreviewHead
	tail := defaultKeyPreviewTail

	if len(trimmed) <= head+tail {
		shortHead := min(2, len(trimmed))
		shortTail := min(2, len(trimmed)-shortHead)
		if shortTail <= 0 {
			return trimmed[:shortHead] + "…"
		}
		return trimmed[:shortHead] + "…" + trimmed[len(trimmed)-shortTail:]
	}

	return trimmed[:head] + "…" + trimmed[len(trimmed)-tail:]
}

// ---------- 环境变量 API Key 解析 ----------

// providerEnvVarMap 提供商 → 环境变量名映射（对应 TS resolveEnvApiKey）。
var providerEnvVarMap = map[string][]string{
	"anthropic":             {"ANTHROPIC_API_KEY"},
	"openai":                {"OPENAI_API_KEY"},
	"google":                {"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	"openrouter":            {"OPENROUTER_API_KEY"},
	"vercel-ai-gateway":     {"AI_GATEWAY_API_KEY"},
	"cloudflare-ai-gateway": {"CLOUDFLARE_AI_GATEWAY_API_KEY"},
	"moonshot":              {"MOONSHOT_API_KEY"},
	"kimi-coding":           {"KIMI_API_KEY"},
	"zai":                   {"ZAI_API_KEY"},
	"xiaomi":                {"XIAOMI_API_KEY"},
	"synthetic":             {"SYNTHETIC_API_KEY"},
	"venice":                {"VENICE_API_KEY"},
	"openacosmi":            {"OPENACOSMI_API_KEY"},
	"xai":                   {"XAI_API_KEY"},
	"qianfan":               {"QIANFAN_API_KEY"},
}

// EnvApiKeyResult 环境变量 API key 查找结果。
type EnvApiKeyResult struct {
	ApiKey string
	Source string // 环境变量名
}

// ResolveEnvApiKey 从环境变量中查找已配置的 API key。
func ResolveEnvApiKey(provider string) *EnvApiKeyResult {
	envVars, ok := providerEnvVarMap[provider]
	if !ok {
		return nil
	}
	for _, envName := range envVars {
		if val := strings.TrimSpace(os.Getenv(envName)); val != "" {
			return &EnvApiKeyResult{ApiKey: val, Source: envName}
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
