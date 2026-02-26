package understanding

// TS 对照: media-understanding/resolve.ts (188L)
// 模型/配置/作用域解析函数。

// ---------- 超时解析 ----------

// ResolveTimeoutMs 解析超时时间。
// TS 对照: resolve.ts L10-22
func ResolveTimeoutMs(configured, perCapability, global int) int {
	if configured > 0 {
		return configured
	}
	if perCapability > 0 {
		return perCapability
	}
	if global > 0 {
		return global
	}
	return DefaultTimeoutMs
}

// ResolvePrompt 解析 prompt。
// TS 对照: resolve.ts L24-35
func ResolvePrompt(configured, perCapability, global string) string {
	if configured != "" {
		return configured
	}
	if perCapability != "" {
		return perCapability
	}
	if global != "" {
		return global
	}
	return DefaultPrompt
}

// ResolveMaxChars 解析最大字符数。
// TS 对照: resolve.ts L37-48
func ResolveMaxChars(configured, perCapability, global int) int {
	if configured > 0 {
		return configured
	}
	if perCapability > 0 {
		return perCapability
	}
	if global > 0 {
		return global
	}
	return DefaultMaxChars
}

// ResolveMaxBytes 解析最大字节数。
// TS 对照: resolve.ts L50-61
func ResolveMaxBytes(configured, perCapability, global int) int {
	if configured > 0 {
		return configured
	}
	if perCapability > 0 {
		return perCapability
	}
	if global > 0 {
		return global
	}
	return DefaultMaxBytes
}

// ---------- 模型解析 ----------

// ModelEntry 模型配置条目。
// TS 对照: resolve.ts L70-75
type ModelEntry struct {
	ProviderID string
	Model      string
}

// ResolveModelEntries 解析给定能力种类的模型条目列表。
// 优先级：用户配置 > Provider 能力默认值。
// TS 对照: resolve.ts L80-130
func ResolveModelEntries(kind Kind, configured []ModelEntry, registry *Registry) []ModelEntry {
	if len(configured) > 0 {
		return configured
	}

	// 使用默认模型映射
	defaults := defaultModelsForKind(kind)
	var entries []ModelEntry
	for providerID, model := range defaults {
		if registry != nil && registry.Get(providerID) != nil {
			entries = append(entries, ModelEntry{
				ProviderID: providerID,
				Model:      model,
			})
		}
	}
	return entries
}

// defaultModelsForKind 获取指定能力种类的默认模型映射。
func defaultModelsForKind(kind Kind) map[string]string {
	switch kind {
	case KindAudioTranscription:
		return DefaultAudioModels
	case KindVideoDescription:
		return DefaultVideoModels
	case KindImageDescription:
		return DefaultImageModels
	default:
		return nil
	}
}

// ---------- 并发解析 ----------

// ResolveConcurrency 解析并发数。
// TS 对照: resolve.ts L140-150
func ResolveConcurrency(configured, global int) int {
	if configured > 0 {
		return configured
	}
	if global > 0 {
		return global
	}
	return 3 // 默认并发数
}

// ---------- 作用域解析 ----------

// ResolveScopeDecision 解析作用域决策（便捷函数）。
// TS 对照: resolve.ts L160-188
func ResolveScopeDecision(kind Kind, channel, chatType string, scopeRules ScopeParams) ScopeDecision {
	scopeRules.Kind = kind
	scopeRules.Channel = channel
	scopeRules.ChatType = chatType
	return ResolveMediaUnderstandingScope(scopeRules)
}
