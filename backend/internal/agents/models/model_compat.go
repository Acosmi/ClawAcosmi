package models

// ---------- 模型兼容性 ----------

// TS 参考: src/agents/model-compat.ts (25 行)

// ModelCompat 模型兼容性标记。
type ModelCompat struct {
	SupportsDeveloperRole *bool `json:"supportsDeveloperRole,omitempty"`
}

// NormalizeModelCompat 修正模型兼容性。
// Zai 供应商不支持 developer role。
func NormalizeModelCompat(provider, baseUrl string, compat *ModelCompat) *ModelCompat {
	isZai := provider == "zai" || containsIgnoreCase(baseUrl, "api.z.ai")
	if !isZai {
		return compat
	}
	// Zai 需要禁用 developer role
	if compat != nil && compat.SupportsDeveloperRole != nil && !*compat.SupportsDeveloperRole {
		return compat // 已经禁用
	}
	f := false
	if compat == nil {
		return &ModelCompat{SupportsDeveloperRole: &f}
	}
	return &ModelCompat{SupportsDeveloperRole: &f}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			match := true
			for j := 0; j < len(substr); j++ {
				c1, c2 := s[i+j], substr[j]
				if c1 != c2 && c1^0x20 != c2 && c1 != c2^0x20 {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		return false
	}()
}
