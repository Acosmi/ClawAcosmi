// authprofile/usage.go — 使用统计与冷却限流
// 对应 TS 文件: src/agents/auth-profiles/usage.ts
package authprofile

import (
	"math"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// FailureReasonPriority 失败原因优先级排序。
var FailureReasonPriority = []types.AuthProfileFailureReason{
	types.FailureReasonAuthPermanent,
	types.FailureReasonAuth,
	types.FailureReasonBilling,
	types.FailureReasonFormat,
	types.FailureReasonModelNotFound,
	types.FailureReasonTimeout,
	types.FailureReasonRateLimit,
	types.FailureReasonUnknown,
}

var failureReasonSet = func() map[types.AuthProfileFailureReason]bool {
	m := make(map[types.AuthProfileFailureReason]bool)
	for _, r := range FailureReasonPriority {
		m[r] = true
	}
	return m
}()

var failureReasonOrder = func() map[types.AuthProfileFailureReason]int {
	m := make(map[types.AuthProfileFailureReason]int)
	for i, r := range FailureReasonPriority {
		m[r] = i
	}
	return m
}()

// isAuthCooldownBypassedForProvider 检查是否绕过冷却（OpenRouter）。
func isAuthCooldownBypassedForProvider(provider string) bool {
	return common.NormalizeProviderId(provider) == "openrouter"
}

// ResolveProfileUnusableUntil 解析 Profile 不可用截止时间。
// 对应 TS: resolveProfileUnusableUntil()
func ResolveProfileUnusableUntil(stats *types.ProfileUsageStats) *int64 {
	if stats == nil {
		return nil
	}
	var values []int64
	if stats.CooldownUntil != nil && *stats.CooldownUntil > 0 {
		values = append(values, *stats.CooldownUntil)
	}
	if stats.DisabledUntil != nil && *stats.DisabledUntil > 0 {
		values = append(values, *stats.DisabledUntil)
	}
	if len(values) == 0 {
		return nil
	}
	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return &maxVal
}

// IsProfileInCooldown 检查 Profile 是否处于冷却期。
// 对应 TS: isProfileInCooldown()
func IsProfileInCooldown(store *types.AuthProfileStore, profileId string) bool {
	cred := store.Profiles[profileId]
	if cred != nil {
		provider, _ := cred["provider"].(string)
		if isAuthCooldownBypassedForProvider(provider) {
			return false
		}
	}
	if store.UsageStats == nil {
		return false
	}
	stats, exists := store.UsageStats[profileId]
	if !exists {
		return false
	}
	unusableUntil := ResolveProfileUnusableUntil(&stats)
	if unusableUntil == nil {
		return false
	}
	return time.Now().UnixMilli() < *unusableUntil
}

// isActiveUnusableWindow 检查不可用窗口是否活跃。
func isActiveUnusableWindow(until *int64, now int64) bool {
	if until == nil {
		return false
	}
	return *until > 0 && now < *until
}

// ResolveProfilesUnavailableReason 推断全部候选 Profile 不可用的原因。
// 对应 TS: resolveProfilesUnavailableReason()
func ResolveProfilesUnavailableReason(store *types.AuthProfileStore, profileIds []string, now *int64) types.AuthProfileFailureReason {
	var ts int64
	if now != nil {
		ts = *now
	} else {
		ts = time.Now().UnixMilli()
	}

	scores := make(map[types.AuthProfileFailureReason]float64)
	addScore := func(reason types.AuthProfileFailureReason, value float64) {
		if !failureReasonSet[reason] || value <= 0 {
			return
		}
		scores[reason] += value
	}

	for _, profileId := range profileIds {
		if store.UsageStats == nil {
			continue
		}
		stats, exists := store.UsageStats[profileId]
		if !exists {
			continue
		}

		disabledActive := isActiveUnusableWindow(stats.DisabledUntil, ts)
		if disabledActive && stats.DisabledReason != "" && failureReasonSet[stats.DisabledReason] {
			addScore(stats.DisabledReason, 1000)
			continue
		}

		cooldownActive := isActiveUnusableWindow(stats.CooldownUntil, ts)
		if !cooldownActive {
			continue
		}

		recordedReason := false
		for reason, count := range stats.FailureCounts {
			if !failureReasonSet[reason] || count <= 0 {
				continue
			}
			addScore(reason, float64(count))
			recordedReason = true
		}
		if !recordedReason {
			addScore(types.FailureReasonRateLimit, 1)
		}
	}

	if len(scores) == 0 {
		return ""
	}

	var best types.AuthProfileFailureReason
	bestScore := float64(-1)
	bestPriority := math.MaxInt
	for _, reason := range FailureReasonPriority {
		score, exists := scores[reason]
		if !exists {
			continue
		}
		priority := failureReasonOrder[reason]
		if score > bestScore || (score == bestScore && priority < bestPriority) {
			best = reason
			bestScore = score
			bestPriority = priority
		}
	}
	return best
}

// GetSoonestCooldownExpiry 获取最早的冷却过期时间。
// 对应 TS: getSoonestCooldownExpiry()
func GetSoonestCooldownExpiry(store *types.AuthProfileStore, profileIds []string) *int64 {
	var soonest *int64
	for _, id := range profileIds {
		if store.UsageStats == nil {
			continue
		}
		stats, exists := store.UsageStats[id]
		if !exists {
			continue
		}
		until := ResolveProfileUnusableUntil(&stats)
		if until == nil || *until <= 0 {
			continue
		}
		if soonest == nil || *until < *soonest {
			soonest = until
		}
	}
	return soonest
}

// ClearExpiredCooldowns 清除已过期的冷却。
// 对应 TS: clearExpiredCooldowns()
func ClearExpiredCooldowns(store *types.AuthProfileStore, now *int64) bool {
	if store.UsageStats == nil {
		return false
	}

	var ts int64
	if now != nil {
		ts = *now
	} else {
		ts = time.Now().UnixMilli()
	}

	mutated := false
	for profileId, stats := range store.UsageStats {
		profileMutated := false

		cooldownExpired := stats.CooldownUntil != nil && *stats.CooldownUntil > 0 && ts >= *stats.CooldownUntil
		disabledExpired := stats.DisabledUntil != nil && *stats.DisabledUntil > 0 && ts >= *stats.DisabledUntil

		if cooldownExpired {
			stats.CooldownUntil = nil
			profileMutated = true
		}
		if disabledExpired {
			stats.DisabledUntil = nil
			stats.DisabledReason = ""
			profileMutated = true
		}

		if profileMutated && ResolveProfileUnusableUntil(&stats) == nil {
			zero := 0
			stats.ErrorCount = &zero
			stats.FailureCounts = nil
		}

		if profileMutated {
			store.UsageStats[profileId] = stats
			mutated = true
		}
	}
	return mutated
}

// CalculateAuthProfileCooldownMs 计算冷却毫秒数。
// 冷却时间: 1min, 5min, 25min, max 1 hour。
// 对应 TS: calculateAuthProfileCooldownMs()
func CalculateAuthProfileCooldownMs(errorCount int) int64 {
	normalized := errorCount
	if normalized < 1 {
		normalized = 1
	}
	exp := normalized - 1
	if exp > 3 {
		exp = 3
	}
	result := int64(60*1000) * int64(math.Pow(5, float64(exp)))
	maxMs := int64(60 * 60 * 1000) // 1 小时
	if result > maxMs {
		return maxMs
	}
	return result
}

// resolvedAuthCooldownConfig 解析后的冷却配置。
type resolvedAuthCooldownConfig struct {
	billingBackoffMs int64
	billingMaxMs     int64
	failureWindowMs  int64
}

// resolveAuthCooldownConfig 解析冷却配置。
func resolveAuthCooldownConfig(cfg *OpenClawConfig, providerId string) resolvedAuthCooldownConfig {
	billingBackoffHours := 5.0
	billingMaxHours := 24.0
	failureWindowHours := 24.0

	resolveHours := func(value *float64, fallback float64) float64 {
		if value != nil && *value > 0 {
			return *value
		}
		return fallback
	}

	if cfg != nil && cfg.Auth != nil && cfg.Auth.Cooldowns != nil {
		cd := cfg.Auth.Cooldowns
		// 检查 Provider 特定覆盖
		if cd.BillingBackoffHoursByProvider != nil {
			for key, value := range cd.BillingBackoffHoursByProvider {
				if common.NormalizeProviderId(key) == providerId {
					billingBackoffHours = value
					break
				}
			}
		}
		billingBackoffHours = resolveHours(cd.BillingBackoffHours, billingBackoffHours)
		billingMaxHours = resolveHours(cd.BillingMaxHours, billingMaxHours)
		failureWindowHours = resolveHours(cd.FailureWindowHours, failureWindowHours)
	}

	return resolvedAuthCooldownConfig{
		billingBackoffMs: int64(billingBackoffHours * 60 * 60 * 1000),
		billingMaxMs:     int64(billingMaxHours * 60 * 60 * 1000),
		failureWindowMs:  int64(failureWindowHours * 60 * 60 * 1000),
	}
}

// calculateAuthProfileBillingDisableMsWithConfig 计算 billing 类禁用时长。
func calculateAuthProfileBillingDisableMsWithConfig(errorCount int, baseMs, maxMs int64) int64 {
	normalized := errorCount
	if normalized < 1 {
		normalized = 1
	}
	if baseMs < 60000 {
		baseMs = 60000
	}
	if maxMs < baseMs {
		maxMs = baseMs
	}
	exp := normalized - 1
	if exp > 10 {
		exp = 10
	}
	raw := float64(baseMs) * math.Pow(2, float64(exp))
	if int64(raw) > maxMs {
		return maxMs
	}
	return int64(raw)
}

// ResolveProfileUnusableUntilForDisplay 用于显示的不可用截止时间。
// 对应 TS: resolveProfileUnusableUntilForDisplay()
func ResolveProfileUnusableUntilForDisplay(store *types.AuthProfileStore, profileId string) *int64 {
	cred := store.Profiles[profileId]
	if cred != nil {
		provider, _ := cred["provider"].(string)
		if isAuthCooldownBypassedForProvider(provider) {
			return nil
		}
	}
	if store.UsageStats == nil {
		return nil
	}
	stats, exists := store.UsageStats[profileId]
	if !exists {
		return nil
	}
	return ResolveProfileUnusableUntil(&stats)
}

// resetUsageStats 重置使用统计。
func resetUsageStats(existing *types.ProfileUsageStats, lastUsed *int64) types.ProfileUsageStats {
	result := types.ProfileUsageStats{}
	if existing != nil {
		result.LastUsed = existing.LastUsed
		result.LastFailureAt = existing.LastFailureAt
	}
	zero := 0
	result.ErrorCount = &zero
	if lastUsed != nil {
		result.LastUsed = lastUsed
	}
	return result
}

// keepActiveWindowOrRecompute 保持活跃窗口或重新计算。
func keepActiveWindowOrRecompute(existingUntil *int64, now, recomputedUntil int64) int64 {
	if existingUntil != nil && *existingUntil > now {
		return *existingUntil
	}
	return recomputedUntil
}

// ComputeNextProfileUsageStats 计算失败后的下一个使用统计状态。
func ComputeNextProfileUsageStats(existing types.ProfileUsageStats, now int64, reason types.AuthProfileFailureReason, cfgResolved resolvedAuthCooldownConfig) types.ProfileUsageStats {
	windowExpired := existing.LastFailureAt != nil && *existing.LastFailureAt > 0 && now-*existing.LastFailureAt > cfgResolved.failureWindowMs

	baseErrorCount := 0
	if !windowExpired && existing.ErrorCount != nil {
		baseErrorCount = *existing.ErrorCount
	}
	nextErrorCount := baseErrorCount + 1

	failureCounts := make(map[types.AuthProfileFailureReason]int)
	if !windowExpired {
		for k, v := range existing.FailureCounts {
			failureCounts[k] = v
		}
	}
	failureCounts[reason] = failureCounts[reason] + 1

	updated := types.ProfileUsageStats{
		LastUsed:      existing.LastUsed,
		ErrorCount:    &nextErrorCount,
		FailureCounts: failureCounts,
		LastFailureAt: &now,
	}

	if reason == types.FailureReasonBilling || reason == types.FailureReasonAuthPermanent {
		billingCount := failureCounts[reason]
		if billingCount < 1 {
			billingCount = 1
		}
		backoffMs := calculateAuthProfileBillingDisableMsWithConfig(billingCount, cfgResolved.billingBackoffMs, cfgResolved.billingMaxMs)
		disabledUntil := keepActiveWindowOrRecompute(existing.DisabledUntil, now, now+backoffMs)
		updated.DisabledUntil = &disabledUntil
		updated.DisabledReason = reason
	} else {
		backoffMs := CalculateAuthProfileCooldownMs(nextErrorCount)
		cooldownUntil := keepActiveWindowOrRecompute(existing.CooldownUntil, now, now+backoffMs)
		updated.CooldownUntil = &cooldownUntil
	}

	return updated
}

// MarkAuthProfileUsed 标记 Profile 为已使用。
// 对应 TS: markAuthProfileUsed()
func MarkAuthProfileUsed(store *types.AuthProfileStore, profileId, agentDir string) {
	if store.Profiles[profileId] == nil {
		return
	}
	if store.UsageStats == nil {
		store.UsageStats = make(map[string]types.ProfileUsageStats)
	}
	now := time.Now().UnixMilli()
	store.UsageStats[profileId] = resetUsageStats(func() *types.ProfileUsageStats {
		if s, ok := store.UsageStats[profileId]; ok {
			return &s
		}
		return nil
	}(), &now)
}

// MarkAuthProfileFailure 标记 Profile 失败。
// 对应 TS: markAuthProfileFailure()
func MarkAuthProfileFailure(store *types.AuthProfileStore, profileId string, reason types.AuthProfileFailureReason, cfg *OpenClawConfig, agentDir string) {
	profile := store.Profiles[profileId]
	if profile == nil {
		return
	}
	provider, _ := profile["provider"].(string)
	if isAuthCooldownBypassedForProvider(provider) {
		return
	}

	now := time.Now().UnixMilli()
	providerKey := common.NormalizeProviderId(provider)
	cfgResolved := resolveAuthCooldownConfig(cfg, providerKey)

	if store.UsageStats == nil {
		store.UsageStats = make(map[string]types.ProfileUsageStats)
	}
	existing := store.UsageStats[profileId]
	store.UsageStats[profileId] = ComputeNextProfileUsageStats(existing, now, reason, cfgResolved)
}

// MarkAuthProfileCooldown 标记 Profile 冷却（使用 unknown 原因）。
// 对应 TS: markAuthProfileCooldown()
func MarkAuthProfileCooldown(store *types.AuthProfileStore, profileId, agentDir string) {
	MarkAuthProfileFailure(store, profileId, types.FailureReasonUnknown, nil, agentDir)
}

// ClearAuthProfileCooldown 清除 Profile 冷却。
// 对应 TS: clearAuthProfileCooldown()
func ClearAuthProfileCooldown(store *types.AuthProfileStore, profileId, agentDir string) {
	if store.UsageStats == nil {
		return
	}
	if _, exists := store.UsageStats[profileId]; !exists {
		return
	}
	existing := store.UsageStats[profileId]
	store.UsageStats[profileId] = resetUsageStats(&existing, nil)
}
