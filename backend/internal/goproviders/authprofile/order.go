// authprofile/order.go — Profile 排序策略
// 对应 TS 文件: src/agents/auth-profiles/order.ts
package authprofile

import (
	"sort"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ResolveAuthProfileOrder 解析 Profile 的排序。
// 对应 TS: resolveAuthProfileOrder()
func ResolveAuthProfileOrder(cfg *OpenClawConfig, store *types.AuthProfileStore, provider string, preferredProfile string) []string {
	providerKey := common.NormalizeProviderId(provider)
	providerAuthKey := common.NormalizeProviderIdForAuth(provider)
	now := time.Now().UnixMilli()

	// 清除过期冷却
	ClearExpiredCooldowns(store, &now)

	storedOrder := common.FindNormalizedProviderValue(store.Order, providerKey)

	var configuredOrder []string
	if cfg != nil && cfg.Auth != nil {
		configuredOrder = common.FindNormalizedProviderValue(cfg.Auth.Order, providerKey)
	}

	explicitOrder := storedOrder
	if explicitOrder == nil {
		explicitOrder = configuredOrder
	}

	// 从配置中获取显式 Profile
	var explicitProfiles []string
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		for profileId, profile := range cfg.Auth.Profiles {
			if common.NormalizeProviderIdForAuth(profile.Provider) == providerAuthKey {
				explicitProfiles = append(explicitProfiles, profileId)
			}
		}
	}

	var baseOrder []string
	if explicitOrder != nil {
		baseOrder = explicitOrder
	} else if len(explicitProfiles) > 0 {
		baseOrder = explicitProfiles
	} else {
		baseOrder = ListProfilesForProvider(store, provider)
	}

	if len(baseOrder) == 0 {
		return nil
	}

	// Profile 有效性检查
	isValidProfile := func(profileId string) bool {
		cred := store.Profiles[profileId]
		if cred == nil {
			return false
		}
		credProvider, _ := cred["provider"].(string)
		if common.NormalizeProviderIdForAuth(credProvider) != providerAuthKey {
			return false
		}
		credType, _ := cred["type"].(string)

		// 检查配置兼容性
		if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
			if profileConfig, ok := cfg.Auth.Profiles[profileId]; ok {
				if common.NormalizeProviderIdForAuth(profileConfig.Provider) != providerAuthKey {
					return false
				}
				if profileConfig.Mode != credType {
					oauthCompat := profileConfig.Mode == "oauth" && credType == "token"
					if !oauthCompat {
						return false
					}
				}
			}
		}

		switch credType {
		case "api_key":
			key, _ := cred["key"].(string)
			return strings.TrimSpace(key) != ""
		case "token":
			token, _ := cred["token"].(string)
			if strings.TrimSpace(token) == "" {
				return false
			}
			expires := GetFloat64FromMap(cred, "expires")
			if expires > 0 && now >= int64(expires) {
				return false
			}
			return true
		case "oauth":
			access, _ := cred["access"].(string)
			refresh, _ := cred["refresh"].(string)
			return strings.TrimSpace(access) != "" || strings.TrimSpace(refresh) != ""
		}
		return false
	}

	var filtered []string
	for _, id := range baseOrder {
		if isValidProfile(id) {
			filtered = append(filtered, id)
		}
	}

	// 修复配置/存储 Profile 漂移
	allBaseProfilesMissing := true
	for _, id := range baseOrder {
		if store.Profiles[id] != nil {
			allBaseProfilesMissing = false
			break
		}
	}
	if len(filtered) == 0 && len(explicitProfiles) > 0 && allBaseProfilesMissing {
		storeProfiles := ListProfilesForProvider(store, provider)
		for _, id := range storeProfiles {
			if isValidProfile(id) {
				filtered = append(filtered, id)
			}
		}
	}

	deduped := DedupeProfileIds(filtered)

	// 显式顺序：尊重用户指定但仍应用冷却排序
	if explicitOrder != nil && len(explicitOrder) > 0 {
		var available []string
		type cooldownEntry struct {
			profileId     string
			cooldownUntil int64
		}
		var inCooldown []cooldownEntry

		for _, profileId := range deduped {
			if IsProfileInCooldown(store, profileId) {
				stats := store.UsageStats[profileId]
				cu := ResolveProfileUnusableUntil(&stats)
				until := now
				if cu != nil {
					until = *cu
				}
				inCooldown = append(inCooldown, cooldownEntry{profileId, until})
			} else {
				available = append(available, profileId)
			}
		}

		sort.Slice(inCooldown, func(i, j int) bool {
			return inCooldown[i].cooldownUntil < inCooldown[j].cooldownUntil
		})

		ordered := make([]string, 0, len(available)+len(inCooldown))
		ordered = append(ordered, available...)
		for _, e := range inCooldown {
			ordered = append(ordered, e.profileId)
		}

		if preferredProfile != "" {
			idx := -1
			for i, id := range ordered {
				if id == preferredProfile {
					idx = i
					break
				}
			}
			if idx >= 0 {
				result := make([]string, 0, len(ordered))
				result = append(result, preferredProfile)
				for i, id := range ordered {
					if i != idx {
						result = append(result, id)
					}
				}
				return result
			}
		}
		return ordered
	}

	// 轮询排序
	sorted := orderProfilesByMode(deduped, store)

	if preferredProfile != "" {
		idx := -1
		for i, id := range sorted {
			if id == preferredProfile {
				idx = i
				break
			}
		}
		if idx >= 0 {
			result := make([]string, 0, len(sorted))
			result = append(result, preferredProfile)
			for i, id := range sorted {
				if i != idx {
					result = append(result, id)
				}
			}
			return result
		}
	}

	return sorted
}

// orderProfilesByMode 按类型偏好 + lastUsed 排序。
func orderProfilesByMode(order []string, store *types.AuthProfileStore) []string {
	now := time.Now().UnixMilli()

	var available []string
	var inCooldown []string
	for _, profileId := range order {
		if IsProfileInCooldown(store, profileId) {
			inCooldown = append(inCooldown, profileId)
		} else {
			available = append(available, profileId)
		}
	}

	type scoredProfile struct {
		profileId string
		typeScore int
		lastUsed  int64
	}

	scored := make([]scoredProfile, len(available))
	for i, profileId := range available {
		cred := store.Profiles[profileId]
		credType, _ := cred["type"].(string)
		typeScore := 3
		switch credType {
		case "oauth":
			typeScore = 0
		case "token":
			typeScore = 1
		case "api_key":
			typeScore = 2
		}
		var lastUsed int64
		if store.UsageStats != nil {
			if stats, ok := store.UsageStats[profileId]; ok && stats.LastUsed != nil {
				lastUsed = *stats.LastUsed
			}
		}
		scored[i] = scoredProfile{profileId, typeScore, lastUsed}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].typeScore != scored[j].typeScore {
			return scored[i].typeScore < scored[j].typeScore
		}
		return scored[i].lastUsed < scored[j].lastUsed
	})

	sorted := make([]string, len(scored))
	for i, s := range scored {
		sorted[i] = s.profileId
	}

	// 冷却 Profile 按过期时间排序追加
	type cooldownProfile struct {
		profileId     string
		cooldownUntil int64
	}
	cdSorted := make([]cooldownProfile, len(inCooldown))
	for i, profileId := range inCooldown {
		until := now
		if store.UsageStats != nil {
			if stats, ok := store.UsageStats[profileId]; ok {
				if cu := ResolveProfileUnusableUntil(&stats); cu != nil {
					until = *cu
				}
			}
		}
		cdSorted[i] = cooldownProfile{profileId, until}
	}
	sort.Slice(cdSorted, func(i, j int) bool {
		return cdSorted[i].cooldownUntil < cdSorted[j].cooldownUntil
	})

	for _, cd := range cdSorted {
		sorted = append(sorted, cd.profileId)
	}

	return sorted
}
