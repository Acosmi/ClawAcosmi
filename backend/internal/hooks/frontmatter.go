package hooks

import (
	"encoding/json"
	"strings"
)

// ============================================================================
// Frontmatter 解析 — 从 HOOK.md 提取 frontmatter 和元数据
// 对应 TS: frontmatter.ts
// ============================================================================

// ManifestKey 清单主键名
const ManifestKey = "openacosmi"

// LegacyManifestKeys 旧版键名
var LegacyManifestKeys = []string{"acosmi", "acosmi-ai"}

// ParseFrontmatter 解析 HOOK.md 的 YAML-like frontmatter
// 对应 TS: frontmatter.ts parseFrontmatter → parseFrontmatterBlock
//
// 格式：
//
//	---
//	key: value
//	---
func ParseFrontmatter(content string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return result
	}

	// Find opening ---
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			start = i
			break
		}
		// If we hit non-empty, non-comment content before ---, no frontmatter
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return result
		}
	}
	if start == -1 {
		return result
	}

	// Find closing ---
	end := -1
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return result
	}

	// Parse key: value pairs
	for i := start + 1; i < end; i++ {
		line := lines[i]
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if key != "" {
			result[key] = value
		}
	}

	return result
}

// ResolveOpenAcosmiMetadata 从 frontmatter 的 metadata 字段提取 OpenAcosmi 钩子元数据
// 对应 TS: frontmatter.ts resolveOpenAcosmiMetadata
func ResolveOpenAcosmiMetadata(frontmatter map[string]string) *HookMetadata {
	raw, ok := frontmatter["metadata"]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}

	// 解析 JSON（TS 用 JSON5，Go 用标准 JSON）
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}

	// 查找 manifest key
	var metadataRaw map[string]interface{}
	candidates := append([]string{ManifestKey}, LegacyManifestKeys...)
	for _, key := range candidates {
		if candidate, ok := parsed[key].(map[string]interface{}); ok {
			metadataRaw = candidate
			break
		}
	}
	if metadataRaw == nil {
		return nil
	}

	metadata := &HookMetadata{
		Events: normalizeStringListFromAny(metadataRaw["events"]),
	}

	if a, ok := metadataRaw["always"].(bool); ok {
		metadata.Always = &a
	}
	if s, ok := metadataRaw["emoji"].(string); ok {
		metadata.Emoji = s
	}
	if s, ok := metadataRaw["homepage"].(string); ok {
		metadata.Homepage = s
	}
	if s, ok := metadataRaw["hookKey"].(string); ok {
		metadata.HookKey = s
	}
	if s, ok := metadataRaw["export"].(string); ok {
		metadata.Export = s
	}

	osList := normalizeStringListFromAny(metadataRaw["os"])
	if len(osList) > 0 {
		metadata.OS = osList
	}

	if reqRaw, ok := metadataRaw["requires"].(map[string]interface{}); ok {
		metadata.Requires = &HookRequirements{
			Bins:    normalizeStringListFromAny(reqRaw["bins"]),
			AnyBins: normalizeStringListFromAny(reqRaw["anyBins"]),
			Env:     normalizeStringListFromAny(reqRaw["env"]),
			Config:  normalizeStringListFromAny(reqRaw["config"]),
		}
	}

	if installRaw, ok := metadataRaw["install"].([]interface{}); ok {
		for _, entry := range installRaw {
			if spec := parseInstallSpec(entry); spec != nil {
				metadata.Install = append(metadata.Install, *spec)
			}
		}
	}

	return metadata
}

// ResolveHookInvocationPolicy 解析调用策略
// 对应 TS: frontmatter.ts resolveHookInvocationPolicy
func ResolveHookInvocationPolicy(frontmatter map[string]string) HookInvocationPolicy {
	enabled := true
	if raw, ok := frontmatter["enabled"]; ok {
		enabled = parseBooleanValue(raw, true)
	}
	return HookInvocationPolicy{Enabled: enabled}
}

// ResolveHookKey 解析钩子 key
// 对应 TS: frontmatter.ts resolveHookKey
func ResolveHookKey(hookName string, entry *HookEntry) string {
	if entry != nil && entry.Metadata != nil && entry.Metadata.HookKey != "" {
		return entry.Metadata.HookKey
	}
	return hookName
}

// --- helpers ---

func normalizeStringListFromAny(input interface{}) []string {
	if input == nil {
		return nil
	}
	switch v := input.(type) {
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case string:
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}
	return nil
}

func parseInstallSpec(input interface{}) *HookInstallSpec {
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil
	}
	kindStr := ""
	if s, ok := raw["kind"].(string); ok {
		kindStr = strings.TrimSpace(strings.ToLower(s))
	} else if s, ok := raw["type"].(string); ok {
		kindStr = strings.TrimSpace(strings.ToLower(s))
	}
	kind := HookInstallKind(kindStr)
	if kind != HookInstallBundled && kind != HookInstallNPM && kind != HookInstallGit {
		return nil
	}

	spec := &HookInstallSpec{Kind: kind}
	if s, ok := raw["id"].(string); ok {
		spec.ID = s
	}
	if s, ok := raw["label"].(string); ok {
		spec.Label = s
	}
	if s, ok := raw["package"].(string); ok {
		spec.Package = s
	}
	if s, ok := raw["repository"].(string); ok {
		spec.Repository = s
	}
	bins := normalizeStringListFromAny(raw["bins"])
	if len(bins) > 0 {
		spec.Bins = bins
	}
	return spec
}

func parseBooleanValue(s string, fallback bool) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch lower {
	case "true", "yes", "1", "on":
		return true
	case "false", "no", "0", "off":
		return false
	}
	return fallback
}
