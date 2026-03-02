package auth

// session_override.go — 会话级别 auth profile override
// 对应 TS src/agents/auth-profiles/session-override.ts (152L)
//
// 管理会话级别的 auth profile 覆盖（round-robin + cooldown 联动）。

import "github.com/openacosmi/claw-acismi/pkg/types"

// SessionAuthEntry 会话认证条目（由调用方提供）。
type SessionAuthEntry struct {
	AuthProfileOverride                string `json:"authProfileOverride,omitempty"`
	AuthProfileOverrideSource          string `json:"authProfileOverrideSource,omitempty"` // "auto" | "user"
	AuthProfileOverrideCompactionCount *int   `json:"authProfileOverrideCompactionCount,omitempty"`
	CompactionCount                    int    `json:"compactionCount,omitempty"`
}

// SessionOverrideResult 解析 session auth profile override 的结果。
type SessionOverrideResult struct {
	ProfileId string
	Changed   bool
	Entry     *SessionAuthEntry
}

// IsProfileForProvider 检查指定 profile 是否属于给定 provider。
// 对应 TS: isProfileForProvider
func IsProfileForProvider(store *AuthProfileStore, provider string, profileId string) bool {
	if store == nil {
		return false
	}
	entry := store.Profiles[profileId]
	if entry == nil || entry.Provider == "" {
		return false
	}
	return NormalizeProviderId(entry.Provider) == NormalizeProviderId(provider)
}

// ClearSessionAuthProfileOverride 清除会话 auth profile override。
// 对应 TS: clearSessionAuthProfileOverride
func ClearSessionAuthProfileOverride(entry *SessionAuthEntry) {
	if entry == nil {
		return
	}
	entry.AuthProfileOverride = ""
	entry.AuthProfileOverrideSource = ""
	entry.AuthProfileOverrideCompactionCount = nil
}

// ResolveSessionAuthProfileOverride 解析会话 auth profile override。
// 对应 TS: resolveSessionAuthProfileOverride
func ResolveSessionAuthProfileOverride(
	cfg *types.OpenAcosmiConfig,
	store *AuthProfileStore,
	provider string,
	entry *SessionAuthEntry,
	isNewSession bool,
) *SessionOverrideResult {
	if entry == nil || store == nil {
		result := &SessionOverrideResult{}
		if entry != nil {
			result.ProfileId = entry.AuthProfileOverride
		}
		return result
	}

	// 获取排序
	order := ResolveAuthProfileOrder(cfg, store, provider, "")
	current := entry.AuthProfileOverride

	// 验证 current 是否有效
	if current != "" && store.Profiles[current] == nil {
		ClearSessionAuthProfileOverride(entry)
		current = ""
	}
	if current != "" && !IsProfileForProvider(store, provider, current) {
		ClearSessionAuthProfileOverride(entry)
		current = ""
	}
	if current != "" && len(order) > 0 && !contains(order, current) {
		ClearSessionAuthProfileOverride(entry)
		current = ""
	}

	if len(order) == 0 {
		return &SessionOverrideResult{}
	}

	// pick helpers
	pickFirstAvailable := func() string {
		for _, profileId := range order {
			if !IsProfileInCooldown(store, profileId) {
				return profileId
			}
		}
		if len(order) > 0 {
			return order[0]
		}
		return ""
	}
	pickNextAvailable := func(active string) string {
		startIndex := -1
		for i, id := range order {
			if id == active {
				startIndex = i
				break
			}
		}
		if startIndex < 0 {
			return pickFirstAvailable()
		}
		for offset := 1; offset <= len(order); offset++ {
			candidate := order[(startIndex+offset)%len(order)]
			if !IsProfileInCooldown(store, candidate) {
				return candidate
			}
		}
		return order[startIndex]
	}

	compactionCount := entry.CompactionCount
	storedCompaction := compactionCount
	if entry.AuthProfileOverrideCompactionCount != nil {
		storedCompaction = *entry.AuthProfileOverrideCompactionCount
	}

	source := entry.AuthProfileOverrideSource
	if source == "" {
		if entry.AuthProfileOverrideCompactionCount != nil {
			source = "auto"
		} else if current != "" {
			source = "user"
		}
	}

	// user 设置且非新会话 — 保持
	if source == "user" && current != "" && !isNewSession {
		return &SessionOverrideResult{ProfileId: current}
	}

	// 选择 next
	next := current
	if isNewSession {
		if current != "" {
			next = pickNextAvailable(current)
		} else {
			next = pickFirstAvailable()
		}
	} else if current != "" && compactionCount > storedCompaction {
		next = pickNextAvailable(current)
	} else if current == "" || IsProfileInCooldown(store, current) {
		next = pickFirstAvailable()
	}

	if next == "" {
		return &SessionOverrideResult{ProfileId: current}
	}

	// 持久化变更
	changed := next != entry.AuthProfileOverride ||
		entry.AuthProfileOverrideSource != "auto" ||
		(entry.AuthProfileOverrideCompactionCount == nil || *entry.AuthProfileOverrideCompactionCount != compactionCount)

	if changed {
		entry.AuthProfileOverride = next
		entry.AuthProfileOverrideSource = "auto"
		entry.AuthProfileOverrideCompactionCount = &compactionCount
	}

	return &SessionOverrideResult{
		ProfileId: next,
		Changed:   changed,
		Entry:     entry,
	}
}
