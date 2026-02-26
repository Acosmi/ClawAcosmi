package cron

import "strings"

// ============================================================================
// 投递计划 — 解析 CronJob 的投递配置
// 对应 TS: cron/delivery.ts (78L)
// ============================================================================

// CronDeliveryPlan 投递计划（解析后的投递意图）
// TS 对照: cron/delivery.ts L3-9
type CronDeliveryPlan struct {
	Mode       CronDeliveryMode `json:"mode"`
	Channel    string           `json:"channel,omitempty"`
	To         string           `json:"to,omitempty"`
	BestEffort bool             `json:"bestEffort"`
	Source     string           `json:"source,omitempty"` // "delivery" | "payload"
	Requested  bool             `json:"requested"`
}

// ResolveCronDeliveryPlan 根据 CronJob 配置解析投递计划
// TS 对照: cron/delivery.ts L30-77 resolveCronDeliveryPlan()
//
// 完整实现——覆盖 delivery 配置和 legacy payload 级两种路径。
func ResolveCronDeliveryPlan(job *CronJob) CronDeliveryPlan {
	plan := CronDeliveryPlan{
		Mode:       DeliveryModeNone,
		BestEffort: false,
	}

	if job == nil {
		return plan
	}

	// 标准化 channel 和 to
	payloadChannel := normalizeDeliveryChannel(job.Payload.Channel)
	payloadTo := strings.TrimSpace(job.Payload.To)

	var deliveryChannel, deliveryTo string
	if job.Delivery != nil {
		deliveryChannel = normalizeDeliveryChannel(job.Delivery.Channel)
		deliveryTo = strings.TrimSpace(job.Delivery.To)
	}

	channel := deliveryChannel
	if channel == "" {
		channel = payloadChannel
	}
	if channel == "" {
		channel = "last"
	}
	to := deliveryTo
	if to == "" {
		to = payloadTo
	}

	// BestEffort 解析
	if job.Delivery != nil && job.Delivery.BestEffort != nil {
		plan.BestEffort = *job.Delivery.BestEffort
	} else if job.Payload.BestEffortDeliver != nil {
		plan.BestEffort = *job.Payload.BestEffortDeliver
	}

	plan.Channel = channel
	plan.To = to

	// 路径一: 优先使用 delivery 配置
	if job.Delivery != nil {
		mode := job.Delivery.Mode
		// TS: "deliver" 映射到 "announce"; 空值默认 "announce"
		if mode == "" {
			mode = DeliveryModeAnnounce
		}
		plan.Mode = mode
		plan.Source = "delivery"
		plan.Requested = mode == DeliveryModeAnnounce
		return plan
	}

	// 路径二: 回退到 payload 中的遗留字段
	plan.Source = "payload"
	if job.Payload.Kind != PayloadKindAgentTurn {
		return plan
	}

	// TS legacy 逻辑: deliver=true → explicit, deliver=false → off, undefined → auto
	legacyMode := "auto"
	if job.Payload.Deliver != nil {
		if *job.Payload.Deliver {
			legacyMode = "explicit"
		} else {
			legacyMode = "off"
		}
	}
	hasExplicitTarget := to != ""
	requested := legacyMode == "explicit" || (legacyMode == "auto" && hasExplicitTarget)

	if requested {
		plan.Mode = DeliveryModeAnnounce
	}
	plan.Requested = requested
	return plan
}

// normalizeDeliveryChannel 规范化渠道名
func normalizeDeliveryChannel(ch string) string {
	return strings.ToLower(strings.TrimSpace(ch))
}

// IsDeliveryPlanActive 判断投递计划是否激活
func IsDeliveryPlanActive(plan CronDeliveryPlan) bool {
	return plan.Mode != DeliveryModeNone
}
