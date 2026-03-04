// authprofile/external_cli_sync.go — 外部 CLI 同步
// 对应 TS 文件: src/agents/auth-profiles/external-cli-sync.ts
package authprofile

import (
	"log"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// shallowEqualOAuthCredentials 浅比较两个 OAuth 凭证是否相同。
func shallowEqualOAuthCredentials(a, b map[string]interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	aType, _ := a["type"].(string)
	if aType != "oauth" {
		return false
	}
	keys := []string{"provider", "access", "refresh", "email", "enterpriseUrl", "projectId", "accountId"}
	for _, k := range keys {
		aVal, _ := a[k].(string)
		bVal, _ := b[k].(string)
		if aVal != bVal {
			return false
		}
	}
	// 比较 expires
	aExp := GetFloat64FromMap(a, "expires")
	bExp := GetFloat64FromMap(b, "expires")
	return aExp == bExp
}

// isExternalProfileFresh 检查外部 Profile 是否仍然有效。
func isExternalProfileFresh(cred map[string]interface{}, now int64) bool {
	if cred == nil {
		return false
	}
	credType, _ := cred["type"].(string)
	if credType != "oauth" && credType != "token" {
		return false
	}
	provider, _ := cred["provider"].(string)
	if provider != "qwen-portal" && provider != "minimax-portal" {
		return false
	}
	expires := GetFloat64FromMap(cred, "expires")
	if expires == 0 {
		return true
	}
	return int64(expires) > now+int64(common.ExternalCLINearExpiryMs)
}

// GetFloat64FromMap 从 map 中安全获取 float64 值。
func GetFloat64FromMap(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch fv := v.(type) {
		case float64:
			return fv
		case int64:
			return float64(fv)
		case int:
			return float64(fv)
		}
	}
	return 0
}

// ExternalCliCredentialReader 外部 CLI 凭证读取函数类型。
type ExternalCliCredentialReader func() map[string]interface{}

// SyncExternalCliCredentialsForProvider 同步指定 Provider 的外部 CLI 凭证。
func SyncExternalCliCredentialsForProvider(
	store *types.AuthProfileStore,
	profileId string,
	provider string,
	readCredentials ExternalCliCredentialReader,
	now int64,
) bool {
	existing := store.Profiles[profileId]
	shouldSync := existing == nil
	if !shouldSync {
		existingProvider, _ := existing["provider"].(string)
		shouldSync = existingProvider != provider || !isExternalProfileFresh(existing, now)
	}

	if !shouldSync {
		return false
	}

	creds := readCredentials()
	if creds == nil {
		return false
	}

	existingOAuth := existing
	if existingOAuth != nil {
		eType, _ := existingOAuth["type"].(string)
		if eType != "oauth" {
			existingOAuth = nil
		}
	}

	shouldUpdate := existingOAuth == nil
	if !shouldUpdate {
		eProvider, _ := existingOAuth["provider"].(string)
		eExpires := GetFloat64FromMap(existingOAuth, "expires")
		cExpires := GetFloat64FromMap(creds, "expires")
		shouldUpdate = eProvider != provider || eExpires <= float64(now) || cExpires > eExpires
	}

	if shouldUpdate && !shallowEqualOAuthCredentials(existingOAuth, creds) {
		if store.Profiles == nil {
			store.Profiles = make(map[string]map[string]interface{})
		}
		store.Profiles[profileId] = creds
		cExpires := GetFloat64FromMap(creds, "expires")
		log.Printf("[auth-profiles] 已同步 %s 外部 CLI 凭证，profileId=%s，过期时间=%s",
			provider, profileId, time.UnixMilli(int64(cExpires)).Format(time.RFC3339))
		return true
	}

	return false
}

// SyncExternalCliCredentials 同步所有外部 CLI 凭证到存储中。
// 当前支持 Qwen Code CLI 和 MiniMax CLI。
// 对应 TS: syncExternalCliCredentials()
//
// 注意：实际的 CLI 凭证读取函数（readQwenCliCredentialsCached 等）
// 依赖外部 CLI 工具的文件系统，在 Go 版本中通过注入 reader 函数实现。
// 默认实现返回 nil（不读取），需要由上层调用方注入实际的读取逻辑。
func SyncExternalCliCredentials(store *types.AuthProfileStore, readers map[string]ExternalCliCredentialReader) bool {
	mutated := false
	now := time.Now().UnixMilli()

	// 同步 Qwen Code CLI
	if reader, ok := readers[common.QwenCLIProfileID]; ok {
		if SyncExternalCliCredentialsForProvider(
			store, common.QwenCLIProfileID, "qwen-portal", reader, now,
		) {
			mutated = true
		}
	}

	// 同步 MiniMax Portal CLI
	if reader, ok := readers[common.MinimaxCLIProfileID]; ok {
		if SyncExternalCliCredentialsForProvider(
			store, common.MinimaxCLIProfileID, "minimax-portal", reader, now,
		) {
			mutated = true
		}
	}

	return mutated
}
