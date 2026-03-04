// authprofile/doctor.go — 认证诊断提示
// 对应 TS 文件: src/agents/auth-profiles/doctor.ts
package authprofile

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// FormatAuthDoctorHint 生成认证诊断提示信息。
// 仅针对 anthropic 提供者生成提示。
// 对应 TS: formatAuthDoctorHint()
func FormatAuthDoctorHint(cfg *OpenClawConfig, store *types.AuthProfileStore, provider, profileId string) string {
	providerKey := common.NormalizeProviderId(provider)
	if providerKey != "anthropic" {
		return ""
	}

	legacyProfileId := profileId
	if legacyProfileId == "" {
		legacyProfileId = "anthropic:default"
	}

	suggested := SuggestOAuthProfileIdForLegacyDefault(cfg, store, providerKey, legacyProfileId)
	if suggested == "" || suggested == legacyProfileId {
		return ""
	}

	oauthProfiles := ListProfilesForProvider(store, providerKey)
	var oauthProfileIds []string
	for _, id := range oauthProfiles {
		cred := store.Profiles[id]
		credType, _ := cred["type"].(string)
		if credType == "oauth" {
			oauthProfileIds = append(oauthProfileIds, id)
		}
	}
	storeOauthProfiles := "(none)"
	if len(oauthProfileIds) > 0 {
		storeOauthProfiles = strings.Join(oauthProfileIds, ", ")
	}

	var cfgMode, cfgProvider string
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		if pc, ok := cfg.Auth.Profiles[legacyProfileId]; ok {
			cfgMode = pc.Mode
			cfgProvider = pc.Provider
		}
	}

	configExtra := ""
	if cfgProvider != "" || cfgMode != "" {
		prov := cfgProvider
		if prov == "" {
			prov = "?"
		}
		mode := cfgMode
		if mode == "" {
			mode = "?"
		}
		configExtra = fmt.Sprintf(" (provider=%s, mode=%s)", prov, mode)
	}

	lines := []string{
		"Doctor hint (for GitHub issue):",
		fmt.Sprintf("- provider: %s", providerKey),
		fmt.Sprintf("- config: %s%s", legacyProfileId, configExtra),
		fmt.Sprintf("- auth store oauth profiles: %s", storeOauthProfiles),
		fmt.Sprintf("- suggested profile: %s", suggested),
		`Fix: run "openclaw doctor --yes"`,
	}
	return strings.Join(lines, "\n")
}
