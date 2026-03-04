// authprofile/session_override.go — 会话级 Profile 覆盖
// 对应 TS 文件: src/agents/auth-profiles/session-override.ts
package authprofile

import (
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// isProfileForProvider 检查 Profile 是否属于指定 Provider。
func isProfileForProvider(provider, profileId string, store *types.AuthProfileStore) bool {
	entry, exists := store.Profiles[profileId]
	if !exists {
		return false
	}
	entryProvider, _ := entry["provider"].(string)
	if entryProvider == "" {
		return false
	}
	return common.NormalizeProviderId(entryProvider) == common.NormalizeProviderId(provider)
}

// ClearSessionAuthProfileOverride 清除会话的 Profile 覆盖。
// 对应 TS: clearSessionAuthProfileOverride()
func ClearSessionAuthProfileOverride(sessionEntry *SessionEntry) {
	sessionEntry.AuthProfileOverride = ""
	sessionEntry.AuthProfileOverrideSource = ""
	sessionEntry.AuthProfileOverrideCompactionCount = nil
	sessionEntry.UpdatedAt = time.Now().UnixMilli()
}

// ResolveSessionAuthProfileOverride 解析会话级 Profile 覆盖。
// 对应 TS: resolveSessionAuthProfileOverride()
func ResolveSessionAuthProfileOverride(
	cfg *OpenClawConfig,
	provider string,
	agentDir string,
	sessionEntry *SessionEntry,
	isNewSession bool,
	cliReaders map[string]ExternalCliCredentialReader,
) string {
	if sessionEntry == nil {
		return ""
	}

	store := EnsureAuthProfileStore(agentDir, cliReaders)
	order := ResolveAuthProfileOrder(cfg, store, provider, "")
	current := sessionEntry.AuthProfileOverride

	// 验证 current 是否仍然有效
	if current != "" && store.Profiles[current] == nil {
		ClearSessionAuthProfileOverride(sessionEntry)
		current = ""
	}
	if current != "" && !isProfileForProvider(provider, current, store) {
		ClearSessionAuthProfileOverride(sessionEntry)
		current = ""
	}
	if current != "" && len(order) > 0 {
		found := false
		for _, id := range order {
			if id == current {
				found = true
				break
			}
		}
		if !found {
			ClearSessionAuthProfileOverride(sessionEntry)
			current = ""
		}
	}

	if len(order) == 0 {
		return ""
	}

	// 选择函数
	pickFirstAvailable := func() string {
		for _, id := range order {
			if !IsProfileInCooldown(store, id) {
				return id
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

	// 确定 compaction 计数
	compactionCount := 0
	if sessionEntry.CompactionCount != nil {
		compactionCount = *sessionEntry.CompactionCount
	}
	storedCompaction := compactionCount
	if sessionEntry.AuthProfileOverrideCompactionCount != nil {
		storedCompaction = *sessionEntry.AuthProfileOverrideCompactionCount
	}

	// 确定来源
	source := sessionEntry.AuthProfileOverrideSource
	if source == "" {
		if sessionEntry.AuthProfileOverrideCompactionCount != nil {
			source = "auto"
		} else if current != "" {
			source = "user"
		}
	}

	// 用户手动设置的 Profile，非新会话时不变
	if source == "user" && current != "" && !isNewSession {
		return current
	}

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
		return current
	}

	// 持久化更新
	shouldPersist := next != sessionEntry.AuthProfileOverride ||
		sessionEntry.AuthProfileOverrideSource != "auto" ||
		(sessionEntry.AuthProfileOverrideCompactionCount == nil || *sessionEntry.AuthProfileOverrideCompactionCount != compactionCount)
	if shouldPersist {
		sessionEntry.AuthProfileOverride = next
		sessionEntry.AuthProfileOverrideSource = "auto"
		sessionEntry.AuthProfileOverrideCompactionCount = &compactionCount
		sessionEntry.UpdatedAt = time.Now().UnixMilli()
	}

	return next
}
