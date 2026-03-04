// common/display.go — Auth Profile 显示辅助
// 对应 TS 文件: src/agents/auth-profiles/display.ts
package common

import (
	"fmt"
	"strings"
)

// AuthProfileDisplayLabelParams 获取 Profile 显示标签的参数。
type AuthProfileDisplayLabelParams struct {
	// AuthProfiles 配置中的 Profile 信息（provider + email 等）
	AuthProfiles map[string]AuthProfileDisplayConfig
	// StoreProfiles 存储中的 Profile 凭证（含 email 字段）
	StoreProfiles map[string]AuthProfileDisplayCredential
	// ProfileID 目标 Profile ID
	ProfileID string
}

// AuthProfileDisplayConfig 配置级 Profile 信息（简化，仅取 email 字段）。
type AuthProfileDisplayConfig struct {
	Email string
}

// AuthProfileDisplayCredential 存储级 Profile 信息（简化，仅取 email 字段）。
type AuthProfileDisplayCredential struct {
	Email string
}

// ResolveAuthProfileDisplayLabel 解析 Profile 的用户友好显示标签。
// 如果 Profile 关联了邮箱，返回 "profileId (email)" 格式；否则仅返回 profileId。
// 对应 TS: resolveAuthProfileDisplayLabel()
func ResolveAuthProfileDisplayLabel(params AuthProfileDisplayLabelParams) string {
	// 优先使用配置中的 email
	var email string
	if cfg, ok := params.AuthProfiles[params.ProfileID]; ok {
		email = strings.TrimSpace(cfg.Email)
	}
	// 回退到存储中的 email
	if email == "" {
		if cred, ok := params.StoreProfiles[params.ProfileID]; ok {
			email = strings.TrimSpace(cred.Email)
		}
	}
	if email != "" {
		return fmt.Sprintf("%s (%s)", params.ProfileID, email)
	}
	return params.ProfileID
}
