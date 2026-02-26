package cron

import "strings"

// ============================================================================
// 遗留负载迁移 — 将 provider 字段映射为 channel
// 对应 TS: cron/payload-migration.ts (41L)
// ============================================================================

// MigrateLegacyCronPayload 将遗留 payload 中的 provider 字段迁移为 channel
// 返回 true 表示发生了变更
func MigrateLegacyCronPayload(payload map[string]interface{}) bool {
	mutated := false

	channelValue, _ := payload["channel"].(string)
	providerValue, _ := payload["provider"].(string)

	var nextChannel string
	if strings.TrimSpace(channelValue) != "" {
		nextChannel = strings.ToLower(strings.TrimSpace(channelValue))
	} else if strings.TrimSpace(providerValue) != "" {
		nextChannel = strings.ToLower(strings.TrimSpace(providerValue))
	}

	if nextChannel != "" {
		if channelValue != nextChannel {
			payload["channel"] = nextChannel
			mutated = true
		}
	}

	if _, hasProvider := payload["provider"]; hasProvider {
		delete(payload, "provider")
		mutated = true
	}

	return mutated
}

// MigrateLegacyCronPayloadTyped 对强类型 CronPayload 执行 provider→channel 迁移
// 返回 true 表示发生了变更
func MigrateLegacyCronPayloadTyped(payload *CronPayload) bool {
	// 对于强类型结构体，provider 字段不存在于 CronPayload 中
	// 此函数主要用于规范化 channel 字段
	if payload.Channel != "" {
		normalized := strings.ToLower(strings.TrimSpace(payload.Channel))
		if normalized != payload.Channel {
			payload.Channel = normalized
			return true
		}
	}
	return false
}
