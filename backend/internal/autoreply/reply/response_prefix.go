package reply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/reply/response-prefix-template.ts

// templateVarPattern 模板变量正则：{variableName} 或 {variable.name}。
// TS 对照: TEMPLATE_VAR_PATTERN
var templateVarPattern = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9.]*)\}`)

// dateSuffixPattern 日期后缀正则（YYYYMMDD 格式）。
var dateSuffixPattern = regexp.MustCompile(`-\d{8}$`)

// ApplyResponsePrefix 应用响应前缀到回复文本。
// TS 对照: response-prefix-template.ts
func ApplyResponsePrefix(text, prefix string, ctx *ResponsePrefixContext) string {
	if prefix == "" || text == "" {
		return text
	}
	expanded := applyResponsePrefixTemplate(prefix, ctx)
	if expanded == "" {
		return text
	}
	if strings.HasPrefix(text, expanded) {
		return text
	}
	return expanded + text
}

// ExtractShortModelName 从完整模型字符串提取短名。
// 剥离 provider 前缀（如 "openai-codex/" → "gpt-5.2"），
// 去除日期后缀（如 "-20260205"）和 "-latest" 后缀。
// TS 对照: response-prefix-template.ts extractShortModelName
func ExtractShortModelName(fullModel string) string {
	// 剥离 provider 前缀
	modelPart := fullModel
	if slash := strings.LastIndex(fullModel, "/"); slash >= 0 {
		modelPart = fullModel[slash+1:]
	}
	// 剥离日期后缀 (YYYYMMDD)
	modelPart = dateSuffixPattern.ReplaceAllString(modelPart, "")
	// 剥离 -latest 后缀
	modelPart = strings.TrimSuffix(modelPart, "-latest")
	return modelPart
}

// HasTemplateVariables 检查模板字符串是否含有模板变量。
// TS 对照: response-prefix-template.ts hasTemplateVariables
func HasTemplateVariables(template string) bool {
	if template == "" {
		return false
	}
	return templateVarPattern.MatchString(template)
}
