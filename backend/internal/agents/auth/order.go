package auth

// order.go — profile 排序 / 选择
// 对应 TS src/agents/auth-profiles/order.ts (211L)
//
// 实现基于 cooldown、round-robin、类型偏好的 profile 排序。
// resolveAuthProfileOrder 是入口；orderProfilesByMode 是内部排序。

import (
	"sort"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ResolveProfileUnusableUntil 计算 profile 不可用截止时间。
// 对应 TS: resolveProfileUnusableUntil
func ResolveProfileUnusableUntil(stats *ProfileUsageStats) *int64 {
	if stats == nil {
		return nil
	}
	var max int64
	if stats.CooldownUntil != nil && *stats.CooldownUntil > 0 {
		max = *stats.CooldownUntil
	}
	if stats.DisabledUntil != nil && *stats.DisabledUntil > 0 && *stats.DisabledUntil > max {
		max = *stats.DisabledUntil
	}
	if max == 0 {
		return nil
	}
	return &max
}

// ResolveAuthProfileOrder 解析 profile 排序。
// 对应 TS: resolveAuthProfileOrder
func ResolveAuthProfileOrder(cfg *types.OpenAcosmiConfig, store *AuthProfileStore, provider string, preferredProfile string) []string {
	if store == nil {
		return nil
	}
	providerKey := NormalizeProviderId(provider)
	now := time.Now().UnixMilli()

	// 获取存储的排序
	storedOrder := resolveOrderForProvider(store.Order, providerKey)

	// 获取配置的排序
	var configuredOrder []string
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Order != nil {
		configuredOrder = resolveOrderForProvider(cfg.Auth.Order, providerKey)
	}

	explicitOrder := storedOrder
	if explicitOrder == nil {
		explicitOrder = configuredOrder
	}

	// 获取配置中声明的 profiles
	var explicitProfiles []string
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		for profileId, profile := range cfg.Auth.Profiles {
			if NormalizeProviderId(profile.Provider) == providerKey {
				explicitProfiles = append(explicitProfiles, profileId)
			}
		}
	}

	// 确定基础排序
	var baseOrder []string
	if explicitOrder != nil {
		baseOrder = explicitOrder
	} else if len(explicitProfiles) > 0 {
		baseOrder = explicitProfiles
	} else {
		baseOrder = ListProfilesForProvider(store, providerKey)
	}
	if len(baseOrder) == 0 {
		return nil
	}

	// 过滤有效 profile
	filtered := filterValidProfiles(baseOrder, store, providerKey, cfg, now)
	deduped := dedupeStrings(filtered)

	if len(deduped) == 0 {
		return nil
	}

	// 如果有显式排序，应用 cooldown 排序
	if explicitOrder != nil && len(explicitOrder) > 0 {
		available, cooldownSorted := partitionByCooldown(deduped, store, now)
		ordered := append(available, cooldownSorted...)

		if preferredProfile != "" && contains(ordered, preferredProfile) {
			return prependPreferred(ordered, preferredProfile)
		}
		return ordered
	}

	// 否则使用 round-robin 模式
	sorted := orderProfilesByMode(deduped, store)

	if preferredProfile != "" && contains(sorted, preferredProfile) {
		return prependPreferred(sorted, preferredProfile)
	}
	return sorted
}

// orderProfilesByMode 按类型偏好 + round-robin 排序。
// 对应 TS: orderProfilesByMode
func orderProfilesByMode(order []string, store *AuthProfileStore) []string {
	now := time.Now().UnixMilli()

	// 分离可用 vs cooldown
	var available, inCooldown []string
	for _, profileId := range order {
		if IsProfileInCooldown(store, profileId) {
			inCooldown = append(inCooldown, profileId)
		} else {
			available = append(available, profileId)
		}
	}

	// 按 typeScore + lastUsed 排序
	type scored struct {
		profileId string
		typeScore int
		lastUsed  int64
	}
	var items []scored
	for _, profileId := range available {
		profile := store.Profiles[profileId]
		ts := int(2) // default: api_key
		if profile != nil {
			switch profile.Type {
			case CredentialOAuth:
				ts = 0
			case CredentialToken:
				ts = 1
			case CredentialAPIKey:
				ts = 2
			}
		}
		var lastUsed int64
		if stats := store.UsageStats[profileId]; stats != nil && stats.LastUsed != nil {
			lastUsed = *stats.LastUsed
		}
		items = append(items, scored{profileId, ts, lastUsed})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].typeScore != items[j].typeScore {
			return items[i].typeScore < items[j].typeScore
		}
		return items[i].lastUsed < items[j].lastUsed
	})

	var sorted []string
	for _, item := range items {
		sorted = append(sorted, item.profileId)
	}

	// cooldown 按过期时间排序后追加
	type cooldownEntry struct {
		profileId     string
		cooldownUntil int64
	}
	var cdEntries []cooldownEntry
	for _, profileId := range inCooldown {
		cu := now
		if stats := store.UsageStats[profileId]; stats != nil {
			if v := ResolveProfileUnusableUntil(stats); v != nil {
				cu = *v
			}
		}
		cdEntries = append(cdEntries, cooldownEntry{profileId, cu})
	}
	sort.Slice(cdEntries, func(i, j int) bool {
		return cdEntries[i].cooldownUntil < cdEntries[j].cooldownUntil
	})
	for _, entry := range cdEntries {
		sorted = append(sorted, entry.profileId)
	}

	return sorted
}

// ---------- helpers ----------

func resolveOrderForProvider(order map[string][]string, providerKey string) []string {
	if order == nil {
		return nil
	}
	for key, value := range order {
		if NormalizeProviderId(key) == providerKey {
			return value
		}
	}
	return nil
}

func filterValidProfiles(baseOrder []string, store *AuthProfileStore,
	providerKey string, cfg *types.OpenAcosmiConfig, now int64) []string {

	var result []string
	for _, profileId := range baseOrder {
		cred := store.Profiles[profileId]
		if cred == nil {
			continue
		}
		if NormalizeProviderId(cred.Provider) != providerKey {
			continue
		}

		// 检查配置一致性
		if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
			if profileCfg, ok := cfg.Auth.Profiles[profileId]; ok {
				if NormalizeProviderId(profileCfg.Provider) != providerKey {
					continue
				}
			}
		}

		// 检查凭据有效性
		switch cred.Type {
		case CredentialAPIKey:
			if cred.Key == "" {
				continue
			}
		case CredentialToken:
			if cred.Token == "" {
				continue
			}
			if cred.Expires != nil && *cred.Expires > 0 && now >= *cred.Expires {
				continue
			}
		case CredentialOAuth:
			if cred.Token == "" && cred.Key == "" {
				continue
			}
		default:
			continue
		}

		result = append(result, profileId)
	}
	return result
}

func partitionByCooldown(profiles []string, store *AuthProfileStore, now int64) (available []string, cooldownSorted []string) {
	type cdEntry struct {
		profileId     string
		cooldownUntil int64
	}
	var cdEntries []cdEntry
	for _, profileId := range profiles {
		cu := ResolveProfileUnusableUntil(store.UsageStats[profileId])
		if cu != nil && *cu > 0 && now < *cu {
			cdEntries = append(cdEntries, cdEntry{profileId, *cu})
		} else {
			available = append(available, profileId)
		}
	}
	sort.Slice(cdEntries, func(i, j int) bool {
		return cdEntries[i].cooldownUntil < cdEntries[j].cooldownUntil
	})
	for _, entry := range cdEntries {
		cooldownSorted = append(cooldownSorted, entry.profileId)
	}
	return
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func prependPreferred(order []string, preferred string) []string {
	result := []string{preferred}
	for _, s := range order {
		if s != preferred {
			result = append(result, s)
		}
	}
	return result
}
