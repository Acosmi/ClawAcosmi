package auth

// profiles.go — 配置文件管理
// 对应 TS src/agents/auth-profiles/profiles.ts (93L)
//
// 提供 profile 的 CRUD 操作和 provider 级别的 profile 列表。

// SetAuthProfileOrder 设置指定 provider 的 profile 排序。
// 对应 TS: setAuthProfileOrder
func SetAuthProfileOrder(store *AuthStore, provider string, order []string) (*AuthProfileStore, error) {
	providerKey := NormalizeProviderId(provider)

	// 去重 + 清理
	deduped := dedupeStrings(order)

	return store.Update(func(s *AuthProfileStore) bool {
		if s.Order == nil {
			s.Order = make(map[string][]string)
		}
		if len(deduped) == 0 {
			if _, exists := s.Order[providerKey]; !exists {
				return false
			}
			delete(s.Order, providerKey)
			if len(s.Order) == 0 {
				s.Order = nil
			}
			return true
		}
		s.Order[providerKey] = deduped
		return true
	})
}

// UpsertAuthProfile 插入或更新认证 profile。
// 对应 TS: upsertAuthProfile
func UpsertAuthProfile(store *AuthStore, profileId string, credential *AuthProfileCredential) {
	if store == nil || profileId == "" || credential == nil {
		return
	}

	data, err := store.Load()
	if err != nil || data == nil {
		data = newEmptyStore()
	}
	data.Profiles[profileId] = credential
	store.Save(data)
}

// ListProfilesForProvider 列出指定 provider 的所有 profile ID。
// 对应 TS: listProfilesForProvider
func ListProfilesForProvider(store *AuthProfileStore, provider string) []string {
	if store == nil {
		return nil
	}
	providerKey := NormalizeProviderId(provider)
	var result []string
	for id, cred := range store.Profiles {
		if cred != nil && NormalizeProviderId(cred.Provider) == providerKey {
			result = append(result, id)
		}
	}
	return result
}

// MarkAuthProfileGood 标记 profile 为最近成功使用。
// 对应 TS: markAuthProfileGood
func MarkAuthProfileGood(store *AuthStore, provider string, profileId string) {
	if store == nil {
		return
	}

	store.Update(func(s *AuthProfileStore) bool {
		profile := s.Profiles[profileId]
		if profile == nil || profile.Provider != provider {
			return false
		}
		if s.LastGood == nil {
			s.LastGood = make(map[string]string)
		}
		s.LastGood[provider] = profileId
		return true
	})
}

// dedupeStrings 去重并过滤空字符串。
func dedupeStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		trimmed := s
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}
