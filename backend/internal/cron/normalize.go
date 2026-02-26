package cron

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ============================================================================
// 输入规范化 — 验证和规范化 cron job 输入
// 对应 TS: cron/normalize.ts (485L)
// ============================================================================

const (
	defaultSessionTarget = SessionTargetMain
	defaultWakeMode      = WakeModeNextHeartbeat
	defaultDeliveryMode  = DeliveryModeAnnounce
)

// --- 调度规范化 ---

// NormalizeCronSchedule 规范化调度配置
func NormalizeCronSchedule(raw map[string]interface{}) (*CronSchedule, error) {
	if raw == nil {
		return nil, fmt.Errorf("schedule is required")
	}

	kind, _ := raw["kind"].(string)
	kind = strings.TrimSpace(strings.ToLower(kind))

	switch CronScheduleKind(kind) {
	case ScheduleKindAt:
		return normalizeAtSchedule(raw)
	case ScheduleKindEvery:
		return normalizeEverySchedule(raw)
	case ScheduleKindCron:
		return normalizeCronSchedule(raw)
	default:
		return nil, fmt.Errorf("invalid schedule.kind: %q (expected at, every, cron)", kind)
	}
}

func normalizeAtSchedule(raw map[string]interface{}) (*CronSchedule, error) {
	at, _ := raw["at"].(string)
	at = strings.TrimSpace(at)
	if at == "" {
		return nil, fmt.Errorf("schedule.at is required for kind=at")
	}
	ms := ParseAbsoluteTimeMs(at)
	if ms < 0 {
		return nil, fmt.Errorf("invalid schedule.at: %q", at)
	}
	return &CronSchedule{Kind: ScheduleKindAt, At: at}, nil
}

func normalizeEverySchedule(raw map[string]interface{}) (*CronSchedule, error) {
	everyMs, ok := readNumberInt64(raw, "everyMs")
	if !ok || everyMs <= 0 {
		return nil, fmt.Errorf("schedule.everyMs must be a positive number")
	}

	sched := &CronSchedule{Kind: ScheduleKindEvery, EveryMs: everyMs}

	// 锚点
	if anchorRaw, exists := raw["anchorMs"]; exists {
		if anchorMs, ok := toInt64(anchorRaw); ok && anchorMs > 0 {
			sched.AnchorMs = &anchorMs
		}
	}

	return sched, nil
}

func normalizeCronSchedule(raw map[string]interface{}) (*CronSchedule, error) {
	expr, _ := raw["expr"].(string)
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("schedule.expr is required for kind=cron")
	}

	sched := &CronSchedule{Kind: ScheduleKindCron, Expr: expr}

	if tz, ok := raw["tz"].(string); ok {
		tz = strings.TrimSpace(tz)
		if tz != "" {
			sched.Tz = tz
		}
	}

	return sched, nil
}

// --- 负载规范化 ---

// NormalizeCronPayload 规范化负载配置
func NormalizeCronPayload(raw map[string]interface{}, sessionTarget CronSessionTarget) (*CronPayload, error) {
	if raw == nil {
		return nil, fmt.Errorf("payload is required")
	}

	kind, _ := raw["kind"].(string)
	kind = strings.TrimSpace(strings.ToLower(kind))

	// 推断 kind（如果缺失）
	if kind == "" {
		if sessionTarget == SessionTargetIsolated {
			kind = string(PayloadKindAgentTurn)
		} else {
			kind = string(PayloadKindSystemEvent)
		}
	}

	switch CronPayloadKind(kind) {
	case PayloadKindSystemEvent:
		return normalizeSystemEventPayload(raw)
	case PayloadKindAgentTurn:
		return normalizeAgentTurnPayload(raw)
	default:
		return nil, fmt.Errorf("invalid payload.kind: %q", kind)
	}
}

func normalizeSystemEventPayload(raw map[string]interface{}) (*CronPayload, error) {
	text, _ := raw["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		// 尝试 message 字段（兼容旧格式）
		if msg, ok := raw["message"].(string); ok && strings.TrimSpace(msg) != "" {
			text = strings.TrimSpace(msg)
		}
	}
	return &CronPayload{Kind: PayloadKindSystemEvent, Text: text}, nil
}

func normalizeAgentTurnPayload(raw map[string]interface{}) (*CronPayload, error) {
	p := &CronPayload{Kind: PayloadKindAgentTurn}

	p.Message, _ = raw["message"].(string)
	p.Message = strings.TrimSpace(p.Message)
	if p.Message == "" {
		if text, ok := raw["text"].(string); ok && strings.TrimSpace(text) != "" {
			p.Message = strings.TrimSpace(text)
		}
	}

	p.Model, _ = raw["model"].(string)
	p.Thinking, _ = raw["thinking"].(string)

	if ts, ok := readNumberInt(raw, "timeoutSeconds"); ok && ts > 0 {
		p.TimeoutSeconds = &ts
	}

	if v, ok := raw["allowUnsafeExternalContent"].(bool); ok {
		p.AllowUnsafeExternalContent = &v
	}

	if v, ok := raw["deliver"].(bool); ok {
		p.Deliver = &v
	}

	if ch, ok := raw["channel"].(string); ok {
		p.Channel = strings.TrimSpace(strings.ToLower(ch))
	}
	if to, ok := raw["to"].(string); ok {
		p.To = strings.TrimSpace(to)
	}
	if v, ok := raw["bestEffortDeliver"].(bool); ok {
		p.BestEffortDeliver = &v
	}

	return p, nil
}

// --- 投递规范化 ---

// NormalizeCronDelivery 规范化投递配置
func NormalizeCronDelivery(raw map[string]interface{}) *CronDelivery {
	if raw == nil {
		return nil
	}

	delivery := &CronDelivery{}

	if mode, ok := raw["mode"].(string); ok {
		mode = strings.TrimSpace(strings.ToLower(mode))
		switch CronDeliveryMode(mode) {
		case DeliveryModeNone, DeliveryModeAnnounce:
			delivery.Mode = CronDeliveryMode(mode)
		default:
			delivery.Mode = defaultDeliveryMode
		}
	} else {
		delivery.Mode = defaultDeliveryMode
	}

	if ch, ok := raw["channel"].(string); ok {
		delivery.Channel = strings.TrimSpace(strings.ToLower(ch))
	}
	if to, ok := raw["to"].(string); ok {
		delivery.To = strings.TrimSpace(to)
	}
	if v, ok := raw["bestEffort"].(bool); ok {
		delivery.BestEffort = &v
	}

	return delivery
}

// --- 顶层规范化 ---

// NormalizeCronJobInput 规范化完整的 cron job 输入（松散 map → 强类型）
func NormalizeCronJobInput(raw map[string]interface{}) (*CronJobCreate, error) {
	if raw == nil {
		return nil, fmt.Errorf("input is required")
	}

	result := &CronJobCreate{}

	// name
	if name, ok := raw["name"].(string); ok {
		result.Name = strings.TrimSpace(name)
	}

	// description
	if desc, ok := raw["description"].(string); ok {
		result.Description = strings.TrimSpace(desc)
	}

	// agentId
	if agentID, ok := raw["agentId"].(string); ok {
		result.AgentID = strings.TrimSpace(agentID)
	}

	// enabled（默认 true）
	enabled := true
	if v, ok := raw["enabled"].(bool); ok {
		enabled = v
	}
	result.Enabled = &enabled

	// deleteAfterRun
	if v, ok := raw["deleteAfterRun"].(bool); ok {
		result.DeleteAfterRun = &v
	}

	// sessionTarget
	result.SessionTarget = defaultSessionTarget
	if st, ok := raw["sessionTarget"].(string); ok {
		st = strings.TrimSpace(strings.ToLower(st))
		switch CronSessionTarget(st) {
		case SessionTargetMain, SessionTargetIsolated:
			result.SessionTarget = CronSessionTarget(st)
		}
	}

	// wakeMode
	result.WakeMode = defaultWakeMode
	if wm, ok := raw["wakeMode"].(string); ok {
		wm = strings.TrimSpace(strings.ToLower(wm))
		switch CronWakeMode(wm) {
		case WakeModeNextHeartbeat, WakeModeNow:
			result.WakeMode = CronWakeMode(wm)
		}
	}

	// schedule
	scheduleRaw, _ := raw["schedule"].(map[string]interface{})
	sched, err := NormalizeCronSchedule(scheduleRaw)
	if err != nil {
		return nil, fmt.Errorf("schedule: %w", err)
	}
	result.Schedule = *sched

	// payload
	payloadRaw, _ := raw["payload"].(map[string]interface{})
	payload, err := NormalizeCronPayload(payloadRaw, result.SessionTarget)
	if err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	result.Payload = *payload

	// delivery
	if deliveryRaw, ok := raw["delivery"].(map[string]interface{}); ok {
		result.Delivery = NormalizeCronDelivery(deliveryRaw)
	}

	return result, nil
}

// NormalizeCronJobCreate 规范化 CronJobCreate 结构体
// 类型安全版本（与 NormalizeCronJobInput 不同，不做类型转换）
func NormalizeCronJobCreate(input *CronJobCreate) error {
	if input == nil {
		return fmt.Errorf("input is required")
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.AgentID = strings.TrimSpace(input.AgentID)

	if input.SessionTarget == "" {
		input.SessionTarget = defaultSessionTarget
	}
	if input.WakeMode == "" {
		input.WakeMode = defaultWakeMode
	}

	return nil
}

// NormalizeCronJobPatch 规范化 CronJobPatch 结构体
func NormalizeCronJobPatch(patch *CronJobPatch) error {
	if patch == nil {
		return fmt.Errorf("patch is required")
	}

	if patch.Name != nil {
		trimmed := strings.TrimSpace(*patch.Name)
		patch.Name = &trimmed
	}
	if patch.Description != nil {
		trimmed := strings.TrimSpace(*patch.Description)
		patch.Description = &trimmed
	}
	if patch.AgentID != nil {
		trimmed := strings.TrimSpace(*patch.AgentID)
		patch.AgentID = &trimmed
	}

	return nil
}

// --- 数值工具 ---

func readNumberInt64(m map[string]interface{}, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	return toInt64(v)
}

func readNumberInt(m map[string]interface{}, key string) (int, bool) {
	v, ok := readNumberInt64(m, key)
	if !ok {
		return 0, false
	}
	return int(v), true
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case string:
		if i, err := strconv.ParseInt(n, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return int64(f), true
		}
	}
	return 0, false
}
