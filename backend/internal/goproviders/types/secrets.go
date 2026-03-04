// types/secrets.go — 密钥引用系统
// 对应 TS 文件: src/config/types.secrets.ts
// 包含 SecretRef 类型系统和所有相关工具函数。
package types

import (
	"fmt"
	"regexp"
	"strings"
)

// SecretRefSource 密钥引用来源类型。
// 对应 TS: SecretRefSource = "env" | "file" | "exec"
type SecretRefSource string

const (
	SecretRefSourceEnv  SecretRefSource = "env"
	SecretRefSourceFile SecretRefSource = "file"
	SecretRefSourceExec SecretRefSource = "exec"
)

// SecretRef 密钥的稳定标识符，指向配置的来源。
// 示例:
//   - env 来源: provider "default", id "OPENAI_API_KEY"
//   - file 来源: provider "mounted-json", id "/providers/openai/apiKey"
//   - exec 来源: provider "vault", id "openai/api-key"
type SecretRef struct {
	Source   SecretRefSource `json:"source"`
	Provider string          `json:"provider"`
	ID       string          `json:"id"`
}

// SecretInput 密钥输入，可以是明文字符串或 SecretRef 引用。
// 在 Go 中使用 interface{} 表示联合类型。
type SecretInput interface{}

// DefaultSecretProviderAlias 默认密钥提供者别名。
const DefaultSecretProviderAlias = "default"

// envSecretTemplateRE 环境变量模板正则表达式。
var envSecretTemplateRE = regexp.MustCompile(`^\$\{([A-Z][A-Z0-9_]{0,127})\}$`)

// SecretDefaults 密钥默认提供者配置。
type SecretDefaults struct {
	Env  string
	File string
	Exec string
}

// IsSecretRef 检查给定值是否是一个合法的 SecretRef。
// 对应 TS: isSecretRef()
func IsSecretRef(ref *SecretRef) bool {
	if ref == nil {
		return false
	}
	return (ref.Source == SecretRefSourceEnv || ref.Source == SecretRefSourceFile || ref.Source == SecretRefSourceExec) &&
		len(strings.TrimSpace(ref.Provider)) > 0 &&
		len(strings.TrimSpace(ref.ID)) > 0
}

// ParseEnvTemplateSecretRef 解析环境变量模板字符串为 SecretRef。
// 例如 "${OPENAI_API_KEY}" → SecretRef{Source: "env", Provider: "default", ID: "OPENAI_API_KEY"}
// 对应 TS: parseEnvTemplateSecretRef()
func ParseEnvTemplateSecretRef(value string, provider string) *SecretRef {
	trimmed := strings.TrimSpace(value)
	matches := envSecretTemplateRE.FindStringSubmatch(trimmed)
	if matches == nil {
		return nil
	}
	p := strings.TrimSpace(provider)
	if p == "" {
		p = DefaultSecretProviderAlias
	}
	return &SecretRef{
		Source:   SecretRefSourceEnv,
		Provider: p,
		ID:       matches[1],
	}
}

// CoerceSecretRef 尝试将输入值强制转换为 SecretRef。
// 支持完整 SecretRef、无 Provider 的遗留格式、和环境变量模板字符串。
// 对应 TS: coerceSecretRef()
func CoerceSecretRef(ref *SecretRef, value string, defaults *SecretDefaults) *SecretRef {
	// 1. 已经是合法的 SecretRef
	if ref != nil && IsSecretRef(ref) {
		return ref
	}
	// 2. 遗留格式：有 source 和 id 但无 provider
	if ref != nil && isLegacySecretRefWithoutProvider(ref) {
		provider := DefaultSecretProviderAlias
		if defaults != nil {
			switch ref.Source {
			case SecretRefSourceEnv:
				if defaults.Env != "" {
					provider = defaults.Env
				}
			case SecretRefSourceFile:
				if defaults.File != "" {
					provider = defaults.File
				}
			case SecretRefSourceExec:
				if defaults.Exec != "" {
					provider = defaults.Exec
				}
			}
		}
		return &SecretRef{
			Source:   ref.Source,
			Provider: provider,
			ID:       ref.ID,
		}
	}
	// 3. 尝试环境变量模板解析
	envProvider := DefaultSecretProviderAlias
	if defaults != nil && defaults.Env != "" {
		envProvider = defaults.Env
	}
	envTemplate := ParseEnvTemplateSecretRef(value, envProvider)
	if envTemplate != nil {
		return envTemplate
	}
	return nil
}

// isLegacySecretRefWithoutProvider 检查是否是缺少 provider 字段的遗留 SecretRef。
func isLegacySecretRefWithoutProvider(ref *SecretRef) bool {
	if ref == nil {
		return false
	}
	return (ref.Source == SecretRefSourceEnv || ref.Source == SecretRefSourceFile || ref.Source == SecretRefSourceExec) &&
		len(strings.TrimSpace(ref.ID)) > 0 &&
		ref.Provider == ""
}

// NormalizeSecretInputString 规范化明文字符串输入。
// 返回去除首尾空白后的非空字符串，否则返回空字符串。
// 对应 TS: normalizeSecretInputString()
func NormalizeSecretInputString(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 0 {
		return trimmed
	}
	return ""
}

// HasConfiguredSecretInput 检查是否存在已配置的密钥输入（明文或引用）。
// 对应 TS: hasConfiguredSecretInput()
func HasConfiguredSecretInput(value string, ref *SecretRef, defaults *SecretDefaults) bool {
	if NormalizeSecretInputString(value) != "" {
		return true
	}
	return CoerceSecretRef(ref, value, defaults) != nil
}

// formatSecretRefLabel 格式化 SecretRef 的可读标签。
func formatSecretRefLabel(ref *SecretRef) string {
	return fmt.Sprintf("%s:%s:%s", ref.Source, ref.Provider, ref.ID)
}

// ResolveSecretInputRefResult 密钥输入引用解析结果。
type ResolveSecretInputRefResult struct {
	// ExplicitRef 通过独立 refValue 字段显式指定的引用
	ExplicitRef *SecretRef
	// InlineRef 从 value 本身解析出的内联引用
	InlineRef *SecretRef
	// Ref 最终生效的引用（ExplicitRef 优先）
	Ref *SecretRef
}

// ResolveSecretInputRef 解析密钥输入引用，区分显式引用和内联引用。
// 对应 TS: resolveSecretInputRef()
func ResolveSecretInputRef(value string, refValue *SecretRef, defaults *SecretDefaults) ResolveSecretInputRefResult {
	explicitRef := CoerceSecretRef(refValue, "", defaults)
	var inlineRef *SecretRef
	if explicitRef == nil {
		inlineRef = CoerceSecretRef(nil, value, defaults)
	}
	ref := explicitRef
	if ref == nil {
		ref = inlineRef
	}
	return ResolveSecretInputRefResult{
		ExplicitRef: explicitRef,
		InlineRef:   inlineRef,
		Ref:         ref,
	}
}

// AssertSecretInputResolved 断言密钥输入已解析（无未解析的 SecretRef）。
// 如果存在未解析的引用，返回错误。
// 对应 TS: assertSecretInputResolved()
func AssertSecretInputResolved(value string, refValue *SecretRef, defaults *SecretDefaults, path string) error {
	result := ResolveSecretInputRef(value, refValue, defaults)
	if result.Ref == nil {
		return nil
	}
	return fmt.Errorf("%s: 未解析的 SecretRef \"%s\"。请先通过活动网关运行时快照解析此命令后再读取", path, formatSecretRefLabel(result.Ref))
}

// NormalizeResolvedSecretInputString 规范化已解析的密钥输入字符串。
// 如果明文值存在则返回，否则断言引用已解析。
// 对应 TS: normalizeResolvedSecretInputString()
func NormalizeResolvedSecretInputString(value string, refValue *SecretRef, defaults *SecretDefaults, path string) (string, error) {
	normalized := NormalizeSecretInputString(value)
	if normalized != "" {
		return normalized, nil
	}
	if err := AssertSecretInputResolved(value, refValue, defaults, path); err != nil {
		return "", err
	}
	return "", nil
}
