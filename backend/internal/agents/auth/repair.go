package auth

// repair.go — OAuth profile ID 修复
// 对应 TS src/agents/auth-profiles/repair.ts (170L)
//
// 修复 legacy ":default" profile → 真实 OAuth profile 的迁移。

import (
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// AuthProfileIdRepairResult 修复结果。
type AuthProfileIdRepairResult struct {
	Config        *types.OpenAcosmiConfig
	Changes       []string
	Migrated      bool
	FromProfileId string
	ToProfileId   string
}

// getProfileSuffix 提取 profile ID 中 ":" 之后的部分。
func getProfileSuffix(profileId string) string {
	idx := strings.Index(profileId, ":")
	if idx < 0 {
		return ""
	}
	return profileId[idx+1:]
}

// isEmailLike 检查字符串是否像 email。
func isEmailLike(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "@") && strings.Contains(trimmed, ".")
}

// SuggestOAuthProfileIdForLegacyDefault 为 legacy ":default" profile 建议替代 OAuth profile ID。
// 对应 TS: suggestOAuthProfileIdForLegacyDefault
func SuggestOAuthProfileIdForLegacyDefault(cfg *types.OpenAcosmiConfig, store *AuthProfileStore, provider string, legacyProfileId string) string {
	providerKey := NormalizeProviderId(provider)
	legacySuffix := getProfileSuffix(legacyProfileId)
	if legacySuffix != "default" {
		return ""
	}

	// 检查 legacy 配置是否指定了非 OAuth 模式
	var legacyCfg *types.AuthProfileConfig
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		legacyCfg = cfg.Auth.Profiles[legacyProfileId]
	}
	if legacyCfg != nil {
		if NormalizeProviderId(legacyCfg.Provider) == providerKey && legacyCfg.Mode != "oauth" {
			return ""
		}
	}

	// 获取所有 OAuth profiles
	allProfiles := ListProfilesForProvider(store, providerKey)
	var oauthProfiles []string
	for _, id := range allProfiles {
		if p := store.Profiles[id]; p != nil && p.Type == CredentialOAuth {
			oauthProfiles = append(oauthProfiles, id)
		}
	}
	if len(oauthProfiles) == 0 {
		return ""
	}

	// 按 email 匹配
	if legacyCfg != nil && legacyCfg.Email != "" {
		configuredEmail := strings.TrimSpace(legacyCfg.Email)
		for _, id := range oauthProfiles {
			cred := store.Profiles[id]
			if cred == nil || cred.Type != CredentialOAuth {
				continue
			}
			email := strings.TrimSpace(cred.Email)
			if email == configuredEmail || id == providerKey+":"+configuredEmail {
				return id
			}
		}
	}

	// 按 lastGood 匹配
	if store.LastGood != nil {
		lastGood := store.LastGood[providerKey]
		if lastGood == "" {
			lastGood = store.LastGood[provider]
		}
		if lastGood != "" && contains(oauthProfiles, lastGood) {
			return lastGood
		}
	}

	// 非 legacy 候选
	var nonLegacy []string
	for _, id := range oauthProfiles {
		if id != legacyProfileId {
			nonLegacy = append(nonLegacy, id)
		}
	}
	if len(nonLegacy) == 1 {
		return nonLegacy[0]
	}

	// email-like 候选
	var emailLike []string
	for _, id := range nonLegacy {
		if isEmailLike(getProfileSuffix(id)) {
			emailLike = append(emailLike, id)
		}
	}
	if len(emailLike) == 1 {
		return emailLike[0]
	}

	return ""
}

// RepairOAuthProfileIdMismatch 修复 OAuth profile ID 不匹配。
// 对应 TS: repairOAuthProfileIdMismatch
func RepairOAuthProfileIdMismatch(cfg *types.OpenAcosmiConfig, store *AuthProfileStore, provider string, legacyProfileId string) *AuthProfileIdRepairResult {
	if legacyProfileId == "" {
		legacyProfileId = NormalizeProviderId(provider) + ":default"
	}

	// 检查 legacy 配置
	noChange := &AuthProfileIdRepairResult{Config: cfg, Migrated: false}

	if cfg == nil || cfg.Auth == nil || cfg.Auth.Profiles == nil {
		return noChange
	}
	legacyCfg, ok := cfg.Auth.Profiles[legacyProfileId]
	if !ok || legacyCfg == nil {
		return noChange
	}
	if legacyCfg.Mode != "oauth" {
		return noChange
	}
	if NormalizeProviderId(legacyCfg.Provider) != NormalizeProviderId(provider) {
		return noChange
	}

	// 寻找替代 profile
	toProfileId := SuggestOAuthProfileIdForLegacyDefault(cfg, store, provider, legacyProfileId)
	if toProfileId == "" || toProfileId == legacyProfileId {
		return noChange
	}

	// 构建新配置
	nextProfiles := make(map[string]*types.AuthProfileConfig)
	for k, v := range cfg.Auth.Profiles {
		if k != legacyProfileId {
			nextProfiles[k] = v
		}
	}

	// 从凭据获取 email
	toCred := store.Profiles[toProfileId]
	toEmail := ""
	if toCred != nil && toCred.Type == CredentialOAuth {
		toEmail = strings.TrimSpace(toCred.Email)
	}

	newProfileCfg := &types.AuthProfileConfig{
		Provider: legacyCfg.Provider,
		Mode:     legacyCfg.Mode,
		Email:    legacyCfg.Email,
	}
	if toEmail != "" {
		newProfileCfg.Email = toEmail
	}
	nextProfiles[toProfileId] = newProfileCfg

	// 更新 order
	nextCfg := *cfg
	nextCfg.Auth = &types.AuthConfig{
		Profiles: nextProfiles,
	}
	if cfg.Auth.Order != nil {
		nextOrder := make(map[string][]string)
		providerKey := NormalizeProviderId(provider)
		for key, value := range cfg.Auth.Order {
			if NormalizeProviderId(key) == providerKey {
				var replaced []string
				for _, id := range value {
					if id == legacyProfileId {
						replaced = append(replaced, toProfileId)
					} else {
						replaced = append(replaced, id)
					}
				}
				nextOrder[key] = dedupeStrings(replaced)
			} else {
				nextOrder[key] = value
			}
		}
		nextCfg.Auth.Order = nextOrder
	}

	return &AuthProfileIdRepairResult{
		Config:        &nextCfg,
		Changes:       []string{"Auth: migrate " + legacyProfileId + " → " + toProfileId + " (OAuth profile id)"},
		Migrated:      true,
		FromProfileId: legacyProfileId,
		ToProfileId:   toProfileId,
	}
}
