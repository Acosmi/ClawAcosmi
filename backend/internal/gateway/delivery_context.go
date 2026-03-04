package gateway

import (
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/session"
)

// TS 对照: utils/delivery-context.ts (141L)
// 投递上下文规范化、合并、线程移除

// NormalizeDeliveryContext 规范化投递上下文。
// TS 对照: delivery-context.ts normalizeDeliveryContext (L23-58)
func NormalizeDeliveryContext(dc *session.DeliveryContext) *session.DeliveryContext {
	if dc == nil {
		return nil
	}
	// 检查是否"空"上下文
	if dc.Channel == "" && dc.To == "" && dc.AccountId == "" && dc.ThreadId == nil {
		return nil
	}
	return &session.DeliveryContext{
		Channel:   dc.Channel,
		To:        dc.To,
		AccountId: dc.AccountId,
		ThreadId:  normalizeThreadId(dc.ThreadId),
	}
}

// MergeDeliveryContext 合并两个投递上下文（primary 优先，secondary 填充空白）。
// TS 对照: delivery-context.ts mergeDeliveryContext (L60-97)
func MergeDeliveryContext(primary, secondary *session.DeliveryContext) *session.DeliveryContext {
	if primary == nil && secondary == nil {
		return nil
	}
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}

	merged := &session.DeliveryContext{
		Channel:   primary.Channel,
		To:        primary.To,
		AccountId: primary.AccountId,
		ThreadId:  primary.ThreadId,
	}

	if merged.Channel == "" {
		merged.Channel = secondary.Channel
	}
	if merged.To == "" {
		merged.To = secondary.To
	}
	if merged.AccountId == "" {
		merged.AccountId = secondary.AccountId
	}
	if merged.ThreadId == nil {
		merged.ThreadId = secondary.ThreadId
	}

	return merged
}

// RemoveThreadFromDeliveryContext 移除投递上下文中的 ThreadId。
// TS 对照: delivery-context.ts removeThreadFromDeliveryContext (L99-119)
func RemoveThreadFromDeliveryContext(dc *session.DeliveryContext) *session.DeliveryContext {
	if dc == nil {
		return nil
	}
	return &session.DeliveryContext{
		Channel:   dc.Channel,
		To:        dc.To,
		AccountId: dc.AccountId,
		ThreadId:  nil,
	}
}

// DeliveryContextFromSession 从 SessionEntry 提取投递上下文。
// TS 对照: delivery-context.ts deliveryContextFromSession (L121-140)
func DeliveryContextFromSession(entry *SessionEntry) *session.DeliveryContext {
	if entry == nil {
		return nil
	}
	dc := entry.DeliveryContext
	if dc != nil {
		return dc
	}
	// 回退到 legacy 字段
	ch := ""
	if entry.LastChannel != nil {
		ch = entry.LastChannel.Channel
	}
	if ch == "" && entry.LastTo == "" && entry.LastAccountId == "" && entry.LastThreadId == nil {
		return nil
	}
	return &session.DeliveryContext{
		Channel:   ch,
		To:        entry.LastTo,
		AccountId: entry.LastAccountId,
		ThreadId:  entry.LastThreadId,
	}
}

// ComputeDeliveryFields 从 DeliveryContext 提取标准化的 session 字段。
// TS 对照: store.ts normalizeSessionDeliveryFields (接受 DC 而非 entry)
type SessionDeliveryFields struct {
	DeliveryContext *session.DeliveryContext
	LastChannel     *session.SessionLastChannel
	LastTo          string
	LastAccountId   string
	LastThreadId    interface{}
}

// ComputeDeliveryFields 从投递上下文计算规范化字段。
func ComputeDeliveryFields(dc *session.DeliveryContext) SessionDeliveryFields {
	if dc == nil {
		return SessionDeliveryFields{}
	}
	var lastChannel *session.SessionLastChannel
	if dc.Channel != "" {
		lastChannel = &session.SessionLastChannel{
			Channel:   dc.Channel,
			AccountId: dc.AccountId,
			To:        dc.To,
		}
	}
	return SessionDeliveryFields{
		DeliveryContext: dc,
		LastChannel:     lastChannel,
		LastTo:          dc.To,
		LastAccountId:   dc.AccountId,
		LastThreadId:    dc.ThreadId,
	}
}

// normalizeThreadId 规范化 threadId 值。
func normalizeThreadId(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch tv := v.(type) {
	case string:
		if tv == "" {
			return nil
		}
		return tv
	case int:
		return tv
	case int64:
		return tv
	case float64:
		return tv
	default:
		s := fmt.Sprintf("%v", tv)
		if s == "" {
			return nil
		}
		return s
	}
}
