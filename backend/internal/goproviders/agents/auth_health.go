// agents/auth_health.go — 认证健康检查
// 对应 TS 文件: src/agents/auth-health.ts
package agents

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/authprofile"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// AuthProfileSource Profile 来源。
type AuthProfileSource string

const AuthProfileSourceStore AuthProfileSource = "store"

// AuthProfileHealthStatus Profile 健康状态。
type AuthProfileHealthStatus string

const (
	HealthStatusOk       AuthProfileHealthStatus = "ok"
	HealthStatusExpiring AuthProfileHealthStatus = "expiring"
	HealthStatusExpired  AuthProfileHealthStatus = "expired"
	HealthStatusMissing  AuthProfileHealthStatus = "missing"
	HealthStatusStatic   AuthProfileHealthStatus = "static"
)

// AuthProfileHealth 单个 Profile 的健康信息。
type AuthProfileHealth struct {
	ProfileId   string                  `json:"profileId"`
	Provider    string                  `json:"provider"`
	Type        string                  `json:"type"`
	Status      AuthProfileHealthStatus `json:"status"`
	ExpiresAt   *int64                  `json:"expiresAt,omitempty"`
	RemainingMs *int64                  `json:"remainingMs,omitempty"`
	Source      AuthProfileSource       `json:"source"`
	Label       string                  `json:"label"`
}

// AuthProviderHealth Provider 级别的健康信息。
type AuthProviderHealth struct {
	Provider    string                  `json:"provider"`
	Status      AuthProfileHealthStatus `json:"status"`
	ExpiresAt   *int64                  `json:"expiresAt,omitempty"`
	RemainingMs *int64                  `json:"remainingMs,omitempty"`
	Profiles    []AuthProfileHealth     `json:"profiles"`
}

// AuthHealthSummary 认证健康摘要。
type AuthHealthSummary struct {
	Now         int64                `json:"now"`
	WarnAfterMs int64                `json:"warnAfterMs"`
	Profiles    []AuthProfileHealth  `json:"profiles"`
	Providers   []AuthProviderHealth `json:"providers"`
}

// DefaultOAuthWarnMs 默认 OAuth 警告阈值（24 小时）。
const DefaultOAuthWarnMs = 24 * 60 * 60 * 1000

// FormatRemainingShort 格式化剩余时间为简短字符串。
// 对应 TS: formatRemainingShort()
func FormatRemainingShort(remainingMs *int64, underMinuteLabel string) string {
	if remainingMs == nil {
		return "unknown"
	}
	r := *remainingMs
	if r <= 0 {
		return "0m"
	}
	roundedMinutes := int64(math.Round(float64(r) / 60000))
	if roundedMinutes < 1 {
		if underMinuteLabel != "" {
			return underMinuteLabel
		}
		return "1m"
	}
	if roundedMinutes < 60 {
		return fmt.Sprintf("%dm", roundedMinutes)
	}
	hours := int64(math.Round(float64(roundedMinutes) / 60))
	if hours < 48 {
		return fmt.Sprintf("%dh", hours)
	}
	days := int64(math.Round(float64(hours) / 24))
	return fmt.Sprintf("%dd", days)
}

// resolveOAuthStatus 解析 OAuth 状态。
func resolveOAuthStatus(expiresAt *int64, now, warnAfterMs int64) (AuthProfileHealthStatus, *int64) {
	if expiresAt == nil || *expiresAt <= 0 {
		return HealthStatusMissing, nil
	}
	remainingMs := *expiresAt - now
	if remainingMs <= 0 {
		return HealthStatusExpired, &remainingMs
	}
	if remainingMs <= warnAfterMs {
		return HealthStatusExpiring, &remainingMs
	}
	return HealthStatusOk, &remainingMs
}

// resolveDisplayLabel 解析显示标签。
func resolveDisplayLabel(cfg *authprofile.OpenClawConfig, store *types.AuthProfileStore, profileId string) string {
	// 优先使用配置中的 email
	if cfg != nil && cfg.Auth != nil && cfg.Auth.Profiles != nil {
		if pc, ok := cfg.Auth.Profiles[profileId]; ok {
			email := strings.TrimSpace(pc.Email)
			if email != "" {
				return fmt.Sprintf("%s (%s)", profileId, email)
			}
		}
	}
	// 回退到存储中的 email
	if cred, ok := store.Profiles[profileId]; ok {
		if email, ok := cred["email"].(string); ok {
			trimmed := strings.TrimSpace(email)
			if trimmed != "" {
				return fmt.Sprintf("%s (%s)", profileId, trimmed)
			}
		}
	}
	return profileId
}

// buildProfileHealth 构建单个 Profile 的健康信息。
func buildProfileHealth(
	profileId string,
	credential map[string]interface{},
	store *types.AuthProfileStore,
	cfg *authprofile.OpenClawConfig,
	now, warnAfterMs int64,
) AuthProfileHealth {
	label := resolveDisplayLabel(cfg, store, profileId)
	credType, _ := credential["type"].(string)
	credProvider, _ := credential["provider"].(string)

	if credType == "api_key" {
		return AuthProfileHealth{
			ProfileId: profileId, Provider: credProvider, Type: "api_key",
			Status: HealthStatusStatic, Source: AuthProfileSourceStore, Label: label,
		}
	}

	if credType == "token" {
		expires := authprofile.GetFloat64FromMap(credential, "expires")
		if expires <= 0 {
			return AuthProfileHealth{
				ProfileId: profileId, Provider: credProvider, Type: "token",
				Status: HealthStatusStatic, Source: AuthProfileSourceStore, Label: label,
			}
		}
		expiresAt := int64(expires)
		status, remainingMs := resolveOAuthStatus(&expiresAt, now, warnAfterMs)
		return AuthProfileHealth{
			ProfileId: profileId, Provider: credProvider, Type: "token",
			Status: status, ExpiresAt: &expiresAt, RemainingMs: remainingMs,
			Source: AuthProfileSourceStore, Label: label,
		}
	}

	// oauth
	refresh, _ := credential["refresh"].(string)
	hasRefreshToken := strings.TrimSpace(refresh) != ""
	expires := authprofile.GetFloat64FromMap(credential, "expires")
	expiresAt := int64(expires)
	rawStatus, remainingMs := resolveOAuthStatus(&expiresAt, now, warnAfterMs)
	status := rawStatus
	if hasRefreshToken && (rawStatus == HealthStatusExpired || rawStatus == HealthStatusExpiring) {
		status = HealthStatusOk
	}

	return AuthProfileHealth{
		ProfileId: profileId, Provider: credProvider, Type: "oauth",
		Status: status, ExpiresAt: &expiresAt, RemainingMs: remainingMs,
		Source: AuthProfileSourceStore, Label: label,
	}
}

// BuildAuthHealthSummary 构建认证健康摘要。
// 对应 TS: buildAuthHealthSummary()
func BuildAuthHealthSummary(
	store *types.AuthProfileStore,
	cfg *authprofile.OpenClawConfig,
	warnAfterMs *int64,
	providers []string,
) AuthHealthSummary {
	now := time.Now().UnixMilli()
	warn := int64(DefaultOAuthWarnMs)
	if warnAfterMs != nil {
		warn = *warnAfterMs
	}

	var providerFilter map[string]bool
	if providers != nil {
		providerFilter = make(map[string]bool)
		for _, p := range providers {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				providerFilter[trimmed] = true
			}
		}
	}

	var profiles []AuthProfileHealth
	for profileId, credential := range store.Profiles {
		credProvider, _ := credential["provider"].(string)
		if providerFilter != nil && !providerFilter[credProvider] {
			continue
		}
		profiles = append(profiles, buildProfileHealth(profileId, credential, store, cfg, now, warn))
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Provider != profiles[j].Provider {
			return profiles[i].Provider < profiles[j].Provider
		}
		return profiles[i].ProfileId < profiles[j].ProfileId
	})

	// 按 Provider 分组
	providersMap := make(map[string]*AuthProviderHealth)
	for _, profile := range profiles {
		existing, ok := providersMap[profile.Provider]
		if !ok {
			providersMap[profile.Provider] = &AuthProviderHealth{
				Provider: profile.Provider, Status: HealthStatusMissing,
				Profiles: []AuthProfileHealth{profile},
			}
		} else {
			existing.Profiles = append(existing.Profiles, profile)
		}
	}

	if providerFilter != nil {
		for p := range providerFilter {
			if _, ok := providersMap[p]; !ok {
				providersMap[p] = &AuthProviderHealth{
					Provider: p, Status: HealthStatusMissing, Profiles: nil,
				}
			}
		}
	}

	// 计算 Provider 级状态
	for _, provider := range providersMap {
		if len(provider.Profiles) == 0 {
			provider.Status = HealthStatusMissing
			continue
		}

		var expirable []AuthProfileHealth
		hasApiKey := false
		for _, p := range provider.Profiles {
			if p.Type == "oauth" || p.Type == "token" {
				expirable = append(expirable, p)
			}
			if p.Type == "api_key" {
				hasApiKey = true
			}
		}

		if len(expirable) == 0 {
			if hasApiKey {
				provider.Status = HealthStatusStatic
			} else {
				provider.Status = HealthStatusMissing
			}
			continue
		}

		var minExpiry *int64
		for _, p := range expirable {
			if p.ExpiresAt != nil {
				if minExpiry == nil || *p.ExpiresAt < *minExpiry {
					val := *p.ExpiresAt
					minExpiry = &val
				}
			}
		}
		if minExpiry != nil {
			provider.ExpiresAt = minExpiry
			remaining := *minExpiry - now
			provider.RemainingMs = &remaining
		}

		statuses := make(map[AuthProfileHealthStatus]bool)
		for _, p := range expirable {
			statuses[p.Status] = true
		}
		if statuses[HealthStatusExpired] || statuses[HealthStatusMissing] {
			provider.Status = HealthStatusExpired
		} else if statuses[HealthStatusExpiring] {
			provider.Status = HealthStatusExpiring
		} else {
			provider.Status = HealthStatusOk
		}
	}

	providerList := make([]AuthProviderHealth, 0, len(providersMap))
	for _, p := range providersMap {
		providerList = append(providerList, *p)
	}
	sort.Slice(providerList, func(i, j int) bool {
		return providerList[i].Provider < providerList[j].Provider
	})

	return AuthHealthSummary{
		Now: now, WarnAfterMs: warn,
		Profiles: profiles, Providers: providerList,
	}
}
