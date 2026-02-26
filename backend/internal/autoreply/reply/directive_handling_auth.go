package reply

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// TS 对照: auto-reply/reply/directive-handling.auth.ts (247L)

// ModelAuthDetailMode 认证信息展示模式。
// TS 对照: directive-handling.auth.ts ModelAuthDetailMode
type ModelAuthDetailMode string

const (
	ModelAuthDetailCompact ModelAuthDetailMode = "compact"
	ModelAuthDetailVerbose ModelAuthDetailMode = "verbose"
)

// AuthProfileEntry 认证档案条目。
type AuthProfileEntry struct {
	// "api_key" | "token" | "oauth"
	Type     string
	Key      string // api_key
	Token    string // token/oauth
	Provider string
	Expires  int64 // unix ms, 0 = no expiry
}

// AuthProfileStore 认证档案存储。
type AuthProfileStore struct {
	Profiles   map[string]*AuthProfileEntry
	LastGood   map[string]string // provider → profileId
	UsageStats map[string]*ProfileUsageStats
}

// ProfileUsageStats 档案使用统计。
type ProfileUsageStats struct {
	CooldownUntil int64 // unix ms
}

// AuthLabel 认证标签结果。
type AuthLabel struct {
	Label  string
	Source string
}

// maskAPIKey 遮盖 API key 中间部分。
// TS 对照: directive-handling.auth.ts maskApiKey (L18-27)
func maskAPIKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "missing"
	}
	if len(trimmed) <= 16 {
		return trimmed
	}
	return trimmed[:8] + "..." + trimmed[len(trimmed)-8:]
}

// formatUntil 格式化剩余时间。
// TS 对照: directive-handling.auth.ts formatUntil (L57-72)
func formatUntil(timestampMs int64) string {
	now := time.Now().UnixMilli()
	remainingMs := timestampMs - now
	if remainingMs < 0 {
		remainingMs = 0
	}
	minutes := int(math.Round(float64(remainingMs) / 60_000))
	if minutes < 1 {
		return "soon"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(math.Round(float64(minutes) / 60))
	if hours < 48 {
		return fmt.Sprintf("%dh", hours)
	}
	days := int(math.Round(float64(hours) / 24))
	return fmt.Sprintf("%dd", days)
}

// isProfileInCooldown 检查档案是否在冷却中。
func isProfileInCooldown(store *AuthProfileStore, profileId string) bool {
	if store == nil || store.UsageStats == nil {
		return false
	}
	stats, ok := store.UsageStats[profileId]
	if !ok || stats == nil {
		return false
	}
	return stats.CooldownUntil > time.Now().UnixMilli()
}

// ResolveAuthLabelParams 解析认证标签参数。
type ResolveAuthLabelParams struct {
	Provider string
	Store    *AuthProfileStore
	// ProfileOrder 按优先级排序的 profileId 列表
	ProfileOrder []string
	// EnvAPIKey 环境变量 API key（可选）
	EnvAPIKey    string
	EnvAPISource string
	// CustomKey 自定义 provider API key
	CustomKey  string
	ModelsPath string
	Mode       ModelAuthDetailMode
}

// ResolveAuthLabel 解析认证标签。
// TS 对照: directive-handling.auth.ts resolveAuthLabel (L29-214)
func ResolveAuthLabel(params ResolveAuthLabelParams) AuthLabel {
	mode := params.Mode
	if mode == "" {
		mode = ModelAuthDetailCompact
	}
	now := time.Now().UnixMilli()
	store := params.Store
	order := params.ProfileOrder

	if len(order) > 0 && store != nil {
		if mode == ModelAuthDetailCompact {
			profileId := order[0]
			if profileId == "" {
				return AuthLabel{Label: "missing", Source: "missing"}
			}
			profile := store.Profiles[profileId]
			if profile == nil {
				more := ""
				if len(order) > 1 {
					more = fmt.Sprintf(" (+%d)", len(order)-1)
				}
				return AuthLabel{Label: fmt.Sprintf("%s missing%s", profileId, more), Source: ""}
			}
			more := ""
			if len(order) > 1 {
				more = fmt.Sprintf(" (+%d)", len(order)-1)
			}
			switch profile.Type {
			case "api_key":
				return AuthLabel{
					Label:  fmt.Sprintf("%s api-key %s%s", profileId, maskAPIKey(profile.Key), more),
					Source: "",
				}
			case "token":
				exp := resolveExpirySuffix(profile.Expires, now)
				return AuthLabel{
					Label:  fmt.Sprintf("%s token %s%s%s", profileId, maskAPIKey(profile.Token), exp, more),
					Source: "",
				}
			default: // oauth
				exp := resolveExpirySuffix(profile.Expires, now)
				return AuthLabel{
					Label:  fmt.Sprintf("%s oauth%s%s", profileId, exp, more),
					Source: "",
				}
			}
		}

		// verbose mode
		var labels []string
		for _, profileId := range order {
			profile := store.Profiles[profileId]
			var flags []string
			if profileId == order[0] {
				flags = append(flags, "next")
			}
			if store.LastGood != nil {
				for _, v := range store.LastGood {
					if v == profileId {
						flags = append(flags, "lastGood")
						break
					}
				}
			}
			if isProfileInCooldown(store, profileId) {
				if store.UsageStats != nil {
					if stats, ok := store.UsageStats[profileId]; ok && stats != nil {
						until := stats.CooldownUntil
						if until > now {
							flags = append(flags, fmt.Sprintf("cooldown %s", formatUntil(until)))
						} else {
							flags = append(flags, "cooldown")
						}
					}
				}
			}
			if profile == nil {
				suffix := ""
				if len(flags) > 0 {
					suffix = fmt.Sprintf(" (%s)", strings.Join(flags, ", "))
				}
				labels = append(labels, fmt.Sprintf("%s=missing%s", profileId, suffix))
				continue
			}
			switch profile.Type {
			case "api_key":
				suffix := ""
				if len(flags) > 0 {
					suffix = fmt.Sprintf(" (%s)", strings.Join(flags, ", "))
				}
				labels = append(labels, fmt.Sprintf("%s=%s%s", profileId, maskAPIKey(profile.Key), suffix))
			case "token":
				if profile.Expires > 0 {
					if profile.Expires <= now {
						flags = append(flags, "expired")
					} else {
						flags = append(flags, fmt.Sprintf("exp %s", formatUntil(profile.Expires)))
					}
				}
				suffix := ""
				if len(flags) > 0 {
					suffix = fmt.Sprintf(" (%s)", strings.Join(flags, ", "))
				}
				labels = append(labels, fmt.Sprintf("%s=token:%s%s", profileId, maskAPIKey(profile.Token), suffix))
			default: // oauth
				if profile.Expires > 0 {
					if profile.Expires <= now {
						flags = append(flags, "expired")
					} else {
						flags = append(flags, fmt.Sprintf("exp %s", formatUntil(profile.Expires)))
					}
				}
				suffix := ""
				if len(flags) > 0 {
					suffix = fmt.Sprintf(" (%s)", strings.Join(flags, ", "))
				}
				labels = append(labels, fmt.Sprintf("%s=OAuth%s", profileId, suffix))
			}
		}
		source := ""
		if params.ModelsPath != "" {
			source = fmt.Sprintf("auth-profiles.json: %s", params.ModelsPath)
		}
		return AuthLabel{Label: strings.Join(labels, ", "), Source: source}
	}

	// 环境变量 API key
	if params.EnvAPIKey != "" {
		src := params.EnvAPISource
		isOAuth := strings.Contains(src, "ANTHROPIC_OAUTH_TOKEN") ||
			strings.Contains(strings.ToLower(src), "oauth")
		label := maskAPIKey(params.EnvAPIKey)
		if isOAuth {
			label = "OAuth (env)"
		}
		srcLabel := ""
		if mode == ModelAuthDetailVerbose {
			srcLabel = src
		}
		return AuthLabel{Label: label, Source: srcLabel}
	}

	// 自定义 key
	if params.CustomKey != "" {
		srcLabel := ""
		if mode == ModelAuthDetailVerbose && params.ModelsPath != "" {
			srcLabel = fmt.Sprintf("models.json: %s", params.ModelsPath)
		}
		return AuthLabel{Label: maskAPIKey(params.CustomKey), Source: srcLabel}
	}

	return AuthLabel{Label: "missing", Source: "missing"}
}

// resolveExpirySuffix 根据过期时间戳生成过期标签。
func resolveExpirySuffix(expires, now int64) string {
	if expires <= 0 {
		return ""
	}
	if expires <= now {
		return " expired"
	}
	return fmt.Sprintf(" exp %s", formatUntil(expires))
}

// FormatAuthLabel 格式化认证标签为可读字符串。
// TS 对照: directive-handling.auth.ts formatAuthLabel (L216-221)
func FormatAuthLabel(auth AuthLabel) string {
	if auth.Source == "" || auth.Source == auth.Label || auth.Source == "missing" {
		return auth.Label
	}
	return fmt.Sprintf("%s (%s)", auth.Label, auth.Source)
}

// ResolveProfileOverrideResult 解析 profile 覆盖结果。
type ResolveProfileOverrideResult struct {
	ProfileID string
	Error     string
}

// ResolveProfileOverride 解析 auth profile 覆盖设置。
// TS 对照: directive-handling.auth.ts resolveProfileOverride (L223-246)
func ResolveProfileOverride(rawProfile, provider string, store *AuthProfileStore) ResolveProfileOverrideResult {
	raw := strings.TrimSpace(rawProfile)
	if raw == "" {
		return ResolveProfileOverrideResult{}
	}
	if store == nil || store.Profiles == nil {
		return ResolveProfileOverrideResult{
			Error: fmt.Sprintf("Auth profile %q not found.", raw),
		}
	}
	profile, ok := store.Profiles[raw]
	if !ok || profile == nil {
		return ResolveProfileOverrideResult{
			Error: fmt.Sprintf("Auth profile %q not found.", raw),
		}
	}
	if profile.Provider != provider {
		return ResolveProfileOverrideResult{
			Error: fmt.Sprintf("Auth profile %q is for %s, not %s.", raw, profile.Provider, provider),
		}
	}
	return ResolveProfileOverrideResult{ProfileID: raw}
}
