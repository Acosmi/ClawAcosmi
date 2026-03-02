package auth

// display.go — profile 显示标签
// 对应 TS src/agents/auth-profiles/display.ts (18L)

import "github.com/openacosmi/claw-acismi/pkg/types"

// ResolveAuthProfileDisplayLabel 解析 profile 的显示标签。
// 对应 TS: resolveAuthProfileDisplayLabel
func ResolveAuthProfileDisplayLabel(cfg *types.OpenAcosmiConfig, store *AuthProfileStore, profileId string) string {
	if store == nil {
		return profileId
	}

	// 优先使用配置中的 email
	var configEmail string
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		if profileCfg, ok := cfg.Auth.Profiles[profileId]; ok {
			configEmail = profileCfg.Email
		}
	}

	// 其次使用凭据中的 email
	email := configEmail
	if email == "" {
		if profile := store.Profiles[profileId]; profile != nil {
			email = profile.Email
		}
	}

	if email != "" {
		return profileId + " (" + email + ")"
	}
	return profileId
}
