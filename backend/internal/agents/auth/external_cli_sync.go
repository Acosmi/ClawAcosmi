package auth

// external_cli_sync.go — 外部 CLI 凭据同步
// 对应 TS src/agents/auth-profiles/external-cli-sync.ts (136L)
//
// 从外部 CLI 工具（如 Qwen Code CLI、MiniMax CLI）同步 OAuth 凭据。

import (
	"log/slog"
	"time"
)

// 常量
const (
	QwenCliProfileID    = "qwen-portal:qwen-cli"
	MiniMaxCliProfileID = "minimax-portal:minimax-cli"

	// 外部 CLI 凭据近过期阈值
	ExternalCliNearExpiryMs int64 = 5 * 60 * 1000 // 5 分钟

	// 外部 CLI 同步 TTL
	ExternalCliSyncTTLMs int64 = 30 * 1000 // 30 秒
)

// ExternalCliCredentialReader 外部 CLI 凭据读取器接口。
type ExternalCliCredentialReader interface {
	ReadCredentials() *OAuthCredential
}

// OAuthCredential 外部 OAuth 凭据（用于 CLI 同步）。
type OAuthCredential struct {
	Provider      string `json:"provider"`
	Access        string `json:"access"`
	Refresh       string `json:"refresh,omitempty"`
	Expires       int64  `json:"expires"`
	Email         string `json:"email,omitempty"`
	EnterpriseUrl string `json:"enterpriseUrl,omitempty"`
	ProjectId     string `json:"projectId,omitempty"`
	AccountId     string `json:"accountId,omitempty"`
}

// shallowEqualOAuthCredentials 浅比较两个 OAuth 凭据。
func shallowEqualOAuthCredentials(a *AuthProfileCredential, b *OAuthCredential) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Type != CredentialOAuth {
		return false
	}
	return a.Provider == b.Provider &&
		a.Token == b.Access &&
		a.Key == b.Refresh &&
		(a.Expires != nil && *a.Expires == b.Expires) &&
		a.Email == b.Email
}

// isExternalProfileFresh 检查外部 profile 是否新鲜（未过期）。
func isExternalProfileFresh(cred *AuthProfileCredential, now int64) bool {
	if cred == nil {
		return false
	}
	if cred.Type != CredentialOAuth && cred.Type != CredentialToken {
		return false
	}
	if cred.Provider != "qwen-portal" && cred.Provider != "minimax-portal" {
		return false
	}
	if cred.Expires == nil {
		return true
	}
	return *cred.Expires > now+ExternalCliNearExpiryMs
}

// SyncExternalCliCredentialsForProvider 同步指定 provider 的外部 CLI 凭据。
// 对应 TS: syncExternalCliCredentialsForProvider
func SyncExternalCliCredentialsForProvider(
	store *AuthProfileStore,
	profileId string,
	provider string,
	reader ExternalCliCredentialReader,
	now int64,
) bool {
	if store == nil || reader == nil {
		return false
	}

	existing := store.Profiles[profileId]
	shouldSync := existing == nil ||
		existing.Provider != provider ||
		!isExternalProfileFresh(existing, now)

	if !shouldSync {
		return false
	}

	creds := reader.ReadCredentials()
	if creds == nil {
		return false
	}

	shouldUpdate := existing == nil ||
		existing.Type != CredentialOAuth ||
		existing.Provider != provider ||
		(existing.Expires != nil && *existing.Expires <= now) ||
		(creds.Expires > 0 && (existing.Expires == nil || creds.Expires > *existing.Expires))

	if shouldUpdate && !shallowEqualOAuthCredentials(existing, creds) {
		store.Profiles[profileId] = &AuthProfileCredential{
			Type:     CredentialOAuth,
			Provider: creds.Provider,
			Token:    creds.Access,
			Key:      creds.Refresh,
			Expires:  &creds.Expires,
			Email:    creds.Email,
		}
		slog.Info("synced external cli credentials",
			"profileId", profileId,
			"provider", provider,
			"expires", time.UnixMilli(creds.Expires).Format(time.RFC3339))
		return true
	}

	return false
}

// SyncExternalCliCredentials 同步所有外部 CLI 工具凭据。
// 对应 TS: syncExternalCliCredentials
//
// 调用方需提供凭据读取器注册表。
// 返回是否有任何凭据被更新。
func SyncExternalCliCredentials(store *AuthProfileStore, readers map[string]ExternalCliCredentialReader) bool {
	if store == nil || len(readers) == 0 {
		return false
	}

	now := time.Now().UnixMilli()
	mutated := false

	// Qwen CLI
	if reader, ok := readers["qwen"]; ok {
		if SyncExternalCliCredentialsForProvider(store, QwenCliProfileID, "qwen-portal", reader, now) {
			mutated = true
		}
	}

	// MiniMax CLI
	if reader, ok := readers["minimax"]; ok {
		if SyncExternalCliCredentialsForProvider(store, MiniMaxCliProfileID, "minimax-portal", reader, now) {
			mutated = true
		}
	}

	return mutated
}
