// authprofile/oauth.go — OAuth Profile 管理
// 对应 TS 文件: src/agents/auth-profiles/oauth.ts
package authprofile

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// OAuthProviderChecker OAuth 提供者检查接口。
// 由上层注入实际的 OAuth 提供者列表。
type OAuthProviderChecker interface {
	IsOAuthProvider(provider string) bool
}

// OAuthTokenRefresher OAuth 令牌刷新接口。
type OAuthTokenRefresher interface {
	RefreshOAuthToken(provider string, credentials types.OAuthCredentials) (*types.OAuthCredentials, error)
}

// bearerAuthModes Bearer 令牌模式集合。
var bearerAuthModes = map[string]bool{"oauth": true, "token": true}

// isCompatibleModeType 检查 mode 和 type 是否兼容。
func isCompatibleModeType(mode, credType string) bool {
	if mode == "" || credType == "" {
		return false
	}
	if mode == credType {
		return true
	}
	return bearerAuthModes[mode] && bearerAuthModes[credType]
}

// isProfileConfigCompatible 检查 Profile 配置是否兼容。
func isProfileConfigCompatible(cfg *OpenClawConfig, profileId, provider, mode string) bool {
	if cfg == nil || cfg.Auth == nil || cfg.Auth.Profiles == nil {
		return true
	}
	profileConfig, ok := cfg.Auth.Profiles[profileId]
	if !ok {
		return true
	}
	if profileConfig.Provider != provider {
		return false
	}
	if !isCompatibleModeType(profileConfig.Mode, mode) {
		return false
	}
	return true
}

// BuildOAuthApiKey 构建 OAuth API 密钥。
// Google Gemini CLI 需要 projectId 的 JSON 格式。
func BuildOAuthApiKey(provider string, access string, projectId string) string {
	if provider == "google-gemini-cli" && projectId != "" {
		data, _ := json.Marshal(map[string]string{
			"token":     access,
			"projectId": projectId,
		})
		return string(data)
	}
	return access
}

// ApiKeyProfileResult API 密钥解析结果。
type ApiKeyProfileResult struct {
	ApiKey   string
	Provider string
	Email    string
}

// isExpiredCredential 检查凭证是否已过期。
func isExpiredCredential(expires float64) bool {
	return expires > 0 && float64(time.Now().UnixMilli()) >= expires
}

// adoptNewerMainOAuthCredential 从主代理采用更新的 OAuth 凭证。
func adoptNewerMainOAuthCredential(
	store *types.AuthProfileStore,
	profileId, agentDir string,
	cred map[string]interface{},
	cliReaders map[string]ExternalCliCredentialReader,
) map[string]interface{} {
	if agentDir == "" {
		return nil
	}
	mainStore := EnsureAuthProfileStore("", cliReaders)
	mainCred := mainStore.Profiles[profileId]
	if mainCred == nil {
		return nil
	}
	mainType, _ := mainCred["type"].(string)
	mainProvider, _ := mainCred["provider"].(string)
	credProvider, _ := cred["provider"].(string)
	if mainType != "oauth" || mainProvider != credProvider {
		return nil
	}
	mainExpires := GetFloat64FromMap(mainCred, "expires")
	credExpires := GetFloat64FromMap(cred, "expires")
	if mainExpires > 0 && (credExpires == 0 || mainExpires > credExpires) {
		store.Profiles[profileId] = mainCred
		_ = SaveAuthProfileStore(store, agentDir)
		log.Printf("[auth-profiles] 从主代理采用了更新的 OAuth 凭证: profileId=%s, agentDir=%s", profileId, agentDir)
		return mainCred
	}
	return nil
}

// ResolveApiKeyForProfile 解析指定 Profile 的 API 密钥。
// 对应 TS: resolveApiKeyForProfile()
func ResolveApiKeyForProfile(
	cfg *OpenClawConfig,
	store *types.AuthProfileStore,
	profileId string,
	agentDir string,
	cliReaders map[string]ExternalCliCredentialReader,
	refresher OAuthTokenRefresher,
) (*ApiKeyProfileResult, error) {
	cred := store.Profiles[profileId]
	if cred == nil {
		return nil, nil
	}

	credType, _ := cred["type"].(string)
	credProvider, _ := cred["provider"].(string)

	if !isProfileConfigCompatible(cfg, profileId, credProvider, credType) {
		return nil, nil
	}

	switch credType {
	case "api_key":
		key, _ := cred["key"].(string)
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, nil
		}
		email, _ := cred["email"].(string)
		return &ApiKeyProfileResult{ApiKey: key, Provider: credProvider, Email: email}, nil

	case "token":
		token, _ := cred["token"].(string)
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, nil
		}
		expires := GetFloat64FromMap(cred, "expires")
		if isExpiredCredential(expires) {
			return nil, nil
		}
		email, _ := cred["email"].(string)
		return &ApiKeyProfileResult{ApiKey: token, Provider: credProvider, Email: email}, nil

	case "oauth":
		// 尝试采用主代理更新的凭证
		oauthCred := adoptNewerMainOAuthCredential(store, profileId, agentDir, cred, cliReaders)
		if oauthCred == nil {
			oauthCred = cred
		}

		access, _ := oauthCred["access"].(string)
		expires := GetFloat64FromMap(oauthCred, "expires")
		email, _ := oauthCred["email"].(string)
		projectId, _ := oauthCred["projectId"].(string)

		if float64(time.Now().UnixMilli()) < expires {
			apiKey := BuildOAuthApiKey(credProvider, access, projectId)
			return &ApiKeyProfileResult{ApiKey: apiKey, Provider: credProvider, Email: email}, nil
		}

		// 尝试刷新
		if refresher != nil {
			refresh, _ := oauthCred["refresh"].(string)
			oauthCreds := types.OAuthCredentials{
				Access:  access,
				Refresh: refresh,
				Expires: int64(expires),
			}
			newCreds, err := refresher.RefreshOAuthToken(credProvider, oauthCreds)
			if err != nil {
				// 回退：尝试主代理
				if agentDir != "" {
					mainStore := EnsureAuthProfileStore("", cliReaders)
					mainCred := mainStore.Profiles[profileId]
					if mainCred != nil {
						mainType, _ := mainCred["type"].(string)
						mainExpires := GetFloat64FromMap(mainCred, "expires")
						if mainType == "oauth" && float64(time.Now().UnixMilli()) < mainExpires {
							mainAccess, _ := mainCred["access"].(string)
							mainProjectId, _ := mainCred["projectId"].(string)
							mainEmail, _ := mainCred["email"].(string)
							store.Profiles[profileId] = mainCred
							_ = SaveAuthProfileStore(store, agentDir)
							apiKey := BuildOAuthApiKey(credProvider, mainAccess, mainProjectId)
							return &ApiKeyProfileResult{ApiKey: apiKey, Provider: credProvider, Email: mainEmail}, nil
						}
					}
				}

				hint := FormatAuthDoctorHint(cfg, store, credProvider, profileId)
				errMsg := fmt.Sprintf("OAuth token refresh failed for %s: %s. Please try again or re-authenticate.", credProvider, err.Error())
				if hint != "" {
					errMsg += "\n\n" + hint
				}
				return nil, fmt.Errorf("%s", errMsg)
			}

			// 保存刷新后的凭证
			store.Profiles[profileId] = map[string]interface{}{
				"type":     "oauth",
				"provider": credProvider,
				"access":   newCreds.Access,
				"refresh":  newCreds.Refresh,
				"expires":  newCreds.Expires,
			}
			if email != "" {
				store.Profiles[profileId]["email"] = email
			}
			if projectId != "" {
				store.Profiles[profileId]["projectId"] = projectId
			}
			_ = SaveAuthProfileStore(store, agentDir)

			apiKey := BuildOAuthApiKey(credProvider, newCreds.Access, projectId)
			return &ApiKeyProfileResult{ApiKey: apiKey, Provider: credProvider, Email: email}, nil
		}

		return nil, nil
	}

	return nil, nil
}
