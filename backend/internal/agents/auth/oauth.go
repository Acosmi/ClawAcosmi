package auth

// oauth.go — OAuth 认证流程
// 对应 TS src/agents/auth-profiles/oauth.ts (286L)
//
// 提供 OAuth provider 识别、API key 构建、token 刷新、
// profile resolve 等核心 OAuth 管线能力。

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// OAuthProvider OAuth 提供商标识。
type OAuthProvider string

// OAuthCredentials OAuth 凭据（刷新 / 访问 token）。
type OAuthCredentials struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh,omitempty"`
	Expires int64  `json:"expires,omitempty"`
	Email   string `json:"email,omitempty"`
}

// 已知 OAuth 提供商集合（对应 TS OAUTH_PROVIDER_IDS）。
var knownOAuthProviders = map[string]bool{
	"anthropic":      true,
	"google":         true,
	"openai":         true,
	"github-copilot": true,
	"qwen-portal":    true,
}

// IsOAuthProvider 检查是否为已知 OAuth 提供商。
// 对应 TS: isOAuthProvider
func IsOAuthProvider(provider string) bool {
	return knownOAuthProviders[strings.ToLower(strings.TrimSpace(provider))]
}

// ResolveOAuthProvider 解析 OAuth 提供商，返回规范化名称。
// 对应 TS: resolveOAuthProvider
func ResolveOAuthProvider(provider string) string {
	norm := strings.ToLower(strings.TrimSpace(provider))
	if knownOAuthProviders[norm] {
		return norm
	}
	return ""
}

// BuildOAuthApiKey 从 OAuth 凭据构建 API key。
// 对应 TS: buildOAuthApiKey
func BuildOAuthApiKey(provider string, creds *OAuthCredentials) string {
	if creds == nil || creds.Access == "" {
		return ""
	}
	return creds.Access
}

// RefreshOAuthTokenResult 刷新 token 的结果。
type RefreshOAuthTokenResult struct {
	ApiKey         string
	NewCredentials *OAuthCredentials
}

// OAuthTokenRefresher 接口 — 外部注入的 token 刷新器。
type OAuthTokenRefresher interface {
	RefreshToken(provider string, refreshToken string) (*OAuthCredentials, error)
}

// RefreshOAuthTokenWithLock 带锁刷新 OAuth token。
// 对应 TS: refreshOAuthTokenWithLock
func RefreshOAuthTokenWithLock(store *AuthStore, profileId string, refresher OAuthTokenRefresher) *RefreshOAuthTokenResult {
	if store == nil || profileId == "" || refresher == nil {
		return nil
	}

	profile := store.GetProfile(profileId)
	if profile == nil || profile.Type != CredentialOAuth {
		return nil
	}

	// 检查 access token 是否已过期
	now := time.Now().UnixMilli()
	if profile.Expires != nil && *profile.Expires > now {
		// 尚未过期 — 直接返回
		return &RefreshOAuthTokenResult{
			ApiKey: BuildOAuthApiKey(profile.Provider, &OAuthCredentials{
				Access:  profile.Token,
				Refresh: profile.Key, // refresh token 存在 Key 字段
				Expires: *profile.Expires,
				Email:   profile.Email,
			}),
		}
	}

	// 需要刷新
	refreshToken := profile.Key // OAuth refresh token 存储在 Key 字段
	if refreshToken == "" {
		slog.Warn("oauth: no refresh token available", "profileId", profileId)
		return nil
	}

	newCreds, err := refresher.RefreshToken(profile.Provider, refreshToken)
	if err != nil {
		slog.Error("oauth: token refresh failed",
			"profileId", profileId,
			"error", err)
		return nil
	}

	// 更新 store
	store.Update(func(s *AuthProfileStore) bool {
		p := s.Profiles[profileId]
		if p == nil {
			return false
		}
		p.Token = newCreds.Access
		if newCreds.Refresh != "" {
			p.Key = newCreds.Refresh
		}
		if newCreds.Expires > 0 {
			p.Expires = &newCreds.Expires
		}
		if newCreds.Email != "" {
			p.Email = newCreds.Email
		}
		return true
	})

	return &RefreshOAuthTokenResult{
		ApiKey:         BuildOAuthApiKey(profile.Provider, newCreds),
		NewCredentials: newCreds,
	}
}

// TryResolveOAuthProfile 尝试解析 OAuth profile 并返回 API key。
// 对应 TS: tryResolveOAuthProfile
func TryResolveOAuthProfile(store *AuthStore, profileId string, refresher OAuthTokenRefresher) *ResolvedProfile {
	if store == nil || profileId == "" {
		return nil
	}

	profile := store.GetProfile(profileId)
	if profile == nil {
		return nil
	}

	if profile.Type != CredentialOAuth {
		return nil
	}

	// 尝试用现有 token
	if profile.Token != "" {
		now := time.Now().UnixMilli()
		if profile.Expires == nil || *profile.Expires > now {
			return &ResolvedProfile{
				ApiKey:   BuildOAuthApiKey(profile.Provider, &OAuthCredentials{Access: profile.Token}),
				Provider: profile.Provider,
				Email:    profile.Email,
			}
		}
	}

	// 尝试刷新
	result := RefreshOAuthTokenWithLock(store, profileId, refresher)
	if result == nil {
		return nil
	}
	return &ResolvedProfile{
		ApiKey:   result.ApiKey,
		Provider: profile.Provider,
		Email:    profile.Email,
	}
}

// ResolvedProfile 已解析的 profile。
type ResolvedProfile struct {
	ApiKey   string
	Provider string
	Email    string
}

// ResolveApiKeyForProfile 为指定 profile 解析 API key。
// 对应 TS: resolveApiKeyForProfile
func ResolveApiKeyForProfile(store *AuthStore, profileId string, refresher OAuthTokenRefresher) *ResolvedProfile {
	if store == nil || profileId == "" {
		return nil
	}

	profile := store.GetProfile(profileId)
	if profile == nil {
		return nil
	}

	switch profile.Type {
	case CredentialAPIKey:
		if strings.TrimSpace(profile.Key) == "" {
			return nil
		}
		return &ResolvedProfile{
			ApiKey:   profile.Key,
			Provider: profile.Provider,
			Email:    profile.Email,
		}

	case CredentialToken:
		if strings.TrimSpace(profile.Token) == "" {
			return nil
		}
		// 检查过期
		if profile.Expires != nil {
			now := time.Now().UnixMilli()
			if *profile.Expires > 0 && now >= *profile.Expires {
				slog.Debug("auth: token expired",
					"profileId", profileId,
					"expiresAt", *profile.Expires)
				return nil
			}
		}
		return &ResolvedProfile{
			ApiKey:   profile.Token,
			Provider: profile.Provider,
			Email:    profile.Email,
		}

	case CredentialOAuth:
		return TryResolveOAuthProfile(store, profileId, refresher)

	default:
		slog.Debug("auth: unknown credential type",
			"profileId", profileId,
			"type", profile.Type)
		return nil
	}
}

// NormalizeProviderId 规范化 provider ID。
// 对应 TS: model-selection.ts → normalizeProviderId
func NormalizeProviderId(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	// 常见别名映射
	aliases := map[string]string{
		"anthropic":      "anthropic",
		"claude":         "anthropic",
		"openai":         "openai",
		"gpt":            "openai",
		"google":         "google",
		"gemini":         "google",
		"qwen-portal":    "qwen-portal",
		"minimax-portal": "minimax-portal",
	}
	if norm, ok := aliases[p]; ok {
		return norm
	}
	return p
}

// FormatProfileId 格式化 profile ID。
func FormatProfileId(provider, suffix string) string {
	return fmt.Sprintf("%s:%s", NormalizeProviderId(provider), suffix)
}
