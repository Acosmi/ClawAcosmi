// authprofile/repair.go — Profile ID 修复
// 对应 TS 文件: src/agents/auth-profiles/repair.ts
package authprofile

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// getProfileSuffix 获取 Profile ID 的后缀部分（冒号后）。
func getProfileSuffix(profileId string) string {
	idx := strings.Index(profileId, ":")
	if idx < 0 {
		return ""
	}
	return profileId[idx+1:]
}

// isEmailLike 检查字符串是否看起来像邮箱地址。
func isEmailLike(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "@") && strings.Contains(trimmed, ".")
}

// SuggestOAuthProfileIdForLegacyDefault 为遗留的 ":default" Profile 建议新的 OAuth Profile ID。
// 对应 TS: suggestOAuthProfileIdForLegacyDefault()
func SuggestOAuthProfileIdForLegacyDefault(
	cfg *OpenClawConfig,
	store *types.AuthProfileStore,
	provider string,
	legacyProfileId string,
) string {
	providerKey := common.NormalizeProviderId(provider)
	legacySuffix := getProfileSuffix(legacyProfileId)
	if legacySuffix != "default" {
		return ""
	}

	// 检查遗留配置
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		if legacyCfg, ok := cfg.Auth.Profiles[legacyProfileId]; ok {
			if common.NormalizeProviderId(legacyCfg.Provider) == providerKey && legacyCfg.Mode != "oauth" {
				return ""
			}
		}
	}

	// 获取所有 OAuth Profile
	allProfiles := ListProfilesForProvider(store, providerKey)
	var oauthProfiles []string
	for _, id := range allProfiles {
		cred := store.Profiles[id]
		credType, _ := cred["type"].(string)
		if credType == "oauth" {
			oauthProfiles = append(oauthProfiles, id)
		}
	}
	if len(oauthProfiles) == 0 {
		return ""
	}

	// 尝试通过邮箱匹配
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		if legacyCfg, ok := cfg.Auth.Profiles[legacyProfileId]; ok {
			configuredEmail := strings.TrimSpace(legacyCfg.Email)
			if configuredEmail != "" {
				for _, id := range oauthProfiles {
					cred := store.Profiles[id]
					credType, _ := cred["type"].(string)
					if credType != "oauth" {
						continue
					}
					email, _ := cred["email"].(string)
					email = strings.TrimSpace(email)
					if email == configuredEmail || id == fmt.Sprintf("%s:%s", providerKey, configuredEmail) {
						return id
					}
				}
			}
		}
	}

	// 尝试 lastGood
	if store.LastGood != nil {
		lastGood := store.LastGood[providerKey]
		if lastGood == "" {
			lastGood = store.LastGood[provider]
		}
		if lastGood != "" {
			for _, id := range oauthProfiles {
				if id == lastGood {
					return lastGood
				}
			}
		}
	}

	// 排除遗留 Profile 后若只剩一个
	var nonLegacy []string
	for _, id := range oauthProfiles {
		if id != legacyProfileId {
			nonLegacy = append(nonLegacy, id)
		}
	}
	if len(nonLegacy) == 1 {
		return nonLegacy[0]
	}

	// 排除遗留 Profile 后，找邮箱格式的
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

// AuthProfileIdRepairResult Profile ID 修复结果（扩展版本，含 config）。
type AuthProfileIdRepairResult struct {
	Config        *OpenClawConfig `json:"-"`
	Changes       []string        `json:"changes"`
	Migrated      bool            `json:"migrated"`
	FromProfileID string          `json:"fromProfileId,omitempty"`
	ToProfileID   string          `json:"toProfileId,omitempty"`
}

// RepairOAuthProfileIdMismatch 修复 OAuth Profile ID 不匹配问题。
// 对应 TS: repairOAuthProfileIdMismatch()
func RepairOAuthProfileIdMismatch(
	cfg *OpenClawConfig,
	store *types.AuthProfileStore,
	provider string,
	legacyProfileIdOverride string,
) AuthProfileIdRepairResult {
	legacyProfileId := legacyProfileIdOverride
	if legacyProfileId == "" {
		legacyProfileId = common.NormalizeProviderId(provider) + ":default"
	}

	noChange := AuthProfileIdRepairResult{Config: cfg, Changes: nil, Migrated: false}

	if cfg == nil || cfg.Auth == nil || cfg.Auth.Profiles == nil {
		return noChange
	}
	legacyCfg, ok := cfg.Auth.Profiles[legacyProfileId]
	if !ok {
		return noChange
	}
	if legacyCfg.Mode != "oauth" {
		return noChange
	}
	if common.NormalizeProviderId(legacyCfg.Provider) != common.NormalizeProviderId(provider) {
		return noChange
	}

	toProfileId := SuggestOAuthProfileIdForLegacyDefault(cfg, store, provider, legacyProfileId)
	if toProfileId == "" || toProfileId == legacyProfileId {
		return noChange
	}

	// 构建新配置
	toCred := store.Profiles[toProfileId]
	var toEmail string
	if toCred != nil {
		credType, _ := toCred["type"].(string)
		if credType == "oauth" {
			email, _ := toCred["email"].(string)
			toEmail = strings.TrimSpace(email)
		}
	}

	nextProfiles := make(map[string]*AuthProfileConfig)
	for k, v := range cfg.Auth.Profiles {
		nextProfiles[k] = v
	}
	delete(nextProfiles, legacyProfileId)
	newProfileCfg := &AuthProfileConfig{
		Provider: legacyCfg.Provider,
		Mode:     legacyCfg.Mode,
		Email:    legacyCfg.Email,
	}
	if toEmail != "" {
		newProfileCfg.Email = toEmail
	}
	nextProfiles[toProfileId] = newProfileCfg

	// 更新 order
	providerKey := common.NormalizeProviderId(provider)
	nextOrder := cfg.Auth.Order
	if nextOrder != nil {
		resolvedKey := common.FindNormalizedProviderKey(
			func() map[string]interface{} {
				m := make(map[string]interface{})
				for k, v := range nextOrder {
					m[k] = v
				}
				return m
			}(),
			providerKey,
		)
		if resolvedKey != "" {
			existingOrder := nextOrder[resolvedKey]
			if len(existingOrder) > 0 {
				replaced := make([]string, 0, len(existingOrder))
				for _, id := range existingOrder {
					if id == legacyProfileId {
						replaced = append(replaced, toProfileId)
					} else if strings.TrimSpace(id) != "" {
						replaced = append(replaced, id)
					}
				}
				deduped := DedupeProfileIds(replaced)
				newOrder := make(map[string][]string)
				for k, v := range nextOrder {
					newOrder[k] = v
				}
				newOrder[resolvedKey] = deduped
				nextOrder = newOrder
			}
		}
	}

	nextCfg := &OpenClawConfig{
		Auth: &AuthConfig{
			Profiles: nextProfiles,
			Order:    nextOrder,
		},
		Secrets: cfg.Secrets,
		Models:  cfg.Models,
	}
	if cfg.Auth.Cooldowns != nil {
		nextCfg.Auth.Cooldowns = cfg.Auth.Cooldowns
	}

	changes := []string{
		fmt.Sprintf("Auth: migrate %s → %s (OAuth profile id)", legacyProfileId, toProfileId),
	}

	return AuthProfileIdRepairResult{
		Config:        nextCfg,
		Changes:       changes,
		Migrated:      true,
		FromProfileID: legacyProfileId,
		ToProfileID:   toProfileId,
	}
}
