// authprofile/profiles.go — Profile 聚合操作
// 对应 TS 文件: src/agents/auth-profiles/profiles.ts
package authprofile

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// DedupeProfileIds 去重 Profile ID 列表（保持顺序）。
// 对应 TS: dedupeProfileIds()
func DedupeProfileIds(profileIds []string) []string {
	seen := make(map[string]bool, len(profileIds))
	result := make([]string, 0, len(profileIds))
	for _, id := range profileIds {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// SetAuthProfileOrder 设置指定 Provider 的 Profile 排序。
// 对应 TS: setAuthProfileOrder()
func SetAuthProfileOrder(store *types.AuthProfileStore, provider string, order []string) bool {
	providerKey := common.NormalizeProviderId(provider)
	sanitized := make([]string, 0, len(order))
	for _, entry := range order {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			sanitized = append(sanitized, trimmed)
		}
	}
	deduped := DedupeProfileIds(sanitized)

	if store.Order == nil {
		store.Order = make(map[string][]string)
	}

	if len(deduped) == 0 {
		if _, exists := store.Order[providerKey]; !exists {
			return false
		}
		delete(store.Order, providerKey)
		if len(store.Order) == 0 {
			store.Order = nil
		}
		return true
	}
	store.Order[providerKey] = deduped
	return true
}

// UpsertAuthProfile 插入或更新 Profile 凭证。
// 对应 TS: upsertAuthProfile()
func UpsertAuthProfile(store *types.AuthProfileStore, profileId string, credential map[string]interface{}) {
	if store.Profiles == nil {
		store.Profiles = make(map[string]map[string]interface{})
	}
	store.Profiles[profileId] = credential
}

// ListProfilesForProvider 列出指定 Provider 的所有 Profile ID。
// 对应 TS: listProfilesForProvider()
func ListProfilesForProvider(store *types.AuthProfileStore, provider string) []string {
	providerKey := common.NormalizeProviderIdForAuth(provider)
	result := make([]string, 0)
	for id, cred := range store.Profiles {
		credProvider, _ := cred["provider"].(string)
		if common.NormalizeProviderIdForAuth(credProvider) == providerKey {
			result = append(result, id)
		}
	}
	return result
}

// MarkAuthProfileGood 标记 Profile 为最近成功。
// 对应 TS: markAuthProfileGood()
func MarkAuthProfileGood(store *types.AuthProfileStore, provider, profileId string) bool {
	profile, exists := store.Profiles[profileId]
	if !exists {
		return false
	}
	credProvider, _ := profile["provider"].(string)
	if credProvider != provider {
		return false
	}
	if store.LastGood == nil {
		store.LastGood = make(map[string]string)
	}
	store.LastGood[provider] = profileId
	return true
}
