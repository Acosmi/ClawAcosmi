package outbound

import "strings"

// ============================================================================
// Agent 投递计划解析
// 对应 TS: infra/outbound/agent-delivery.ts
// ============================================================================

// InternalMessageChannel 内部消息频道标识（WebChat / 内部路由）。
// TS 参考: utils/message-channel.ts → INTERNAL_MESSAGE_CHANNEL
const InternalMessageChannel = "internal"

// AgentDeliveryPlan Agent 投递计划。
// TS 参考: agent-delivery.ts → AgentDeliveryPlan
type AgentDeliveryPlan struct {
	// BaseDelivery 基础会话投递目标（来自会话记录）。
	BaseDelivery SessionDeliveryTarget
	// ResolvedChannel 最终解析出的出站频道。
	ResolvedChannel string
	// ResolvedTo 最终解析出的接收方标识。
	ResolvedTo string
	// ResolvedAccountID 最终解析出的账户 ID。
	ResolvedAccountID string
	// ResolvedThreadID 最终解析出的线程 ID。
	ResolvedThreadID string
	// DeliveryTargetMode 目标模式（explicit / implicit / heartbeat）。
	DeliveryTargetMode ChannelOutboundTargetMode
}

// ResolveAgentDeliveryPlanParams 解析 Agent 投递计划的参数。
type ResolveAgentDeliveryPlanParams struct {
	// SessionDelivery 来自会话的基础投递目标（可为零值）。
	SessionDelivery *SessionDeliveryTarget
	// RequestedChannel 请求的频道（空字符串表示使用 "last"）。
	RequestedChannel string
	// ExplicitTo 明确指定的接收方。
	ExplicitTo string
	// ExplicitThreadID 明确指定的线程 ID。
	ExplicitThreadID string
	// AccountID 账户 ID（可选）。
	AccountID string
	// WantsDelivery 是否期望执行真实投递（false 时默认使用内部频道）。
	WantsDelivery bool
	// DefaultChatChannel 没有历史记录时使用的默认频道（空则不 fallback）。
	DefaultChatChannel string
}

// isDeliverableChannel 判断频道是否为可投递的真实渠道。
func isDeliverableChannel(ch string) bool {
	return ch != "" && ch != InternalMessageChannel && ch != string(OutboundChannelNone)
}

// normalizeChannelID 规范化频道标识（trim + lowercase）。
func normalizeChannelID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// normalizeAccountID 规范化账户 ID（trim + lowercase）。
func normalizeAccountID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ResolveAgentDeliveryPlan 解析 Agent 投递计划。
// TS 参考: agent-delivery.ts → resolveAgentDeliveryPlan()
func ResolveAgentDeliveryPlan(params ResolveAgentDeliveryPlanParams) AgentDeliveryPlan {
	requestedRaw := strings.TrimSpace(params.RequestedChannel)
	normalizedRequested := normalizeChannelID(requestedRaw)
	requestedChannel := normalizedRequested
	if requestedChannel == "" {
		requestedChannel = "last"
	}

	explicitTo := strings.TrimSpace(params.ExplicitTo)
	if explicitTo == "" {
		explicitTo = ""
	}

	// 构建基础会话目标
	var baseDelivery SessionDeliveryTarget
	if params.SessionDelivery != nil {
		sd := *params.SessionDelivery
		// 如果是 "internal" 频道，视同 "last"
		resolveChannel := requestedChannel
		if requestedChannel == InternalMessageChannel {
			resolveChannel = "last"
		}
		if resolveChannel != "last" && sd.Channel != "" {
			// 若请求的频道与会话频道不同，不继承 To
			if string(sd.Channel) != resolveChannel {
				sd.To = ""
				sd.AccountID = ""
				sd.ThreadID = ""
			}
		}
		baseDelivery = sd
	}

	// 解析 resolvedChannel
	resolvedChannel := func() string {
		if requestedChannel == InternalMessageChannel {
			return InternalMessageChannel
		}
		if requestedChannel == "last" {
			if baseDelivery.Channel != "" && string(baseDelivery.Channel) != InternalMessageChannel {
				return string(baseDelivery.Channel)
			}
			if params.WantsDelivery && params.DefaultChatChannel != "" {
				return params.DefaultChatChannel
			}
			return InternalMessageChannel
		}
		// 明确请求的频道
		if isDeliverableChannel(requestedChannel) {
			return requestedChannel
		}
		if baseDelivery.Channel != "" && string(baseDelivery.Channel) != InternalMessageChannel {
			return string(baseDelivery.Channel)
		}
		if params.WantsDelivery && params.DefaultChatChannel != "" {
			return params.DefaultChatChannel
		}
		return InternalMessageChannel
	}()

	// 目标模式
	deliveryTargetMode := ChannelOutboundTargetMode("")
	if explicitTo != "" {
		deliveryTargetMode = TargetModeExplicit
	} else if isDeliverableChannel(resolvedChannel) {
		deliveryTargetMode = TargetModeImplicit
	}

	// 账户 ID
	resolvedAccountID := normalizeAccountID(params.AccountID)
	if resolvedAccountID == "" && deliveryTargetMode == TargetModeImplicit {
		resolvedAccountID = baseDelivery.AccountID
	}

	// 接收方
	resolvedTo := explicitTo
	if resolvedTo == "" && isDeliverableChannel(resolvedChannel) && resolvedChannel == string(baseDelivery.Channel) {
		resolvedTo = baseDelivery.LastTo
	}

	// 线程 ID（优先会话继承）
	resolvedThreadID := strings.TrimSpace(params.ExplicitThreadID)
	if resolvedThreadID == "" {
		resolvedThreadID = baseDelivery.ThreadID
	}

	return AgentDeliveryPlan{
		BaseDelivery:       baseDelivery,
		ResolvedChannel:    resolvedChannel,
		ResolvedTo:         resolvedTo,
		ResolvedAccountID:  resolvedAccountID,
		ResolvedThreadID:   resolvedThreadID,
		DeliveryTargetMode: deliveryTargetMode,
	}
}

// ResolveAgentOutboundTargetParams 解析 Agent 出站目标的参数。
type ResolveAgentOutboundTargetParams struct {
	// Plan 已解析的 Agent 投递计划。
	Plan AgentDeliveryPlan
	// TargetMode 覆盖目标模式（可选）。
	TargetMode ChannelOutboundTargetMode
	// ValidateExplicitTarget 是否对明确目标执行校验。
	ValidateExplicitTarget bool
	// TargetResolver 可选外部目标解析函数（如需校验）。
	TargetResolver func(channel, to, accountID string, mode ChannelOutboundTargetMode) OutboundTargetResolution
}

// ResolveAgentOutboundTargetResult 解析结果。
type ResolveAgentOutboundTargetResult struct {
	ResolvedTarget *OutboundTargetResolution
	ResolvedTo     string
	TargetMode     ChannelOutboundTargetMode
}

// ResolveAgentOutboundTarget 解析 Agent 出站目标。
// TS 参考: agent-delivery.ts → resolveAgentOutboundTarget()
func ResolveAgentOutboundTarget(params ResolveAgentOutboundTargetParams) ResolveAgentOutboundTargetResult {
	plan := params.Plan
	targetMode := params.TargetMode
	if targetMode == "" {
		if plan.DeliveryTargetMode != "" {
			targetMode = plan.DeliveryTargetMode
		} else if plan.ResolvedTo != "" {
			targetMode = TargetModeExplicit
		} else {
			targetMode = TargetModeImplicit
		}
	}

	if !isDeliverableChannel(plan.ResolvedChannel) {
		return ResolveAgentOutboundTargetResult{
			ResolvedTarget: nil,
			ResolvedTo:     plan.ResolvedTo,
			TargetMode:     targetMode,
		}
	}

	// 若未要求校验且已有 resolvedTo，直接返回
	if !params.ValidateExplicitTarget && plan.ResolvedTo != "" {
		return ResolveAgentOutboundTargetResult{
			ResolvedTarget: nil,
			ResolvedTo:     plan.ResolvedTo,
			TargetMode:     targetMode,
		}
	}

	if params.TargetResolver != nil {
		resolution := params.TargetResolver(plan.ResolvedChannel, plan.ResolvedTo, plan.ResolvedAccountID, targetMode)
		resolvedTo := plan.ResolvedTo
		if resolution.OK {
			resolvedTo = resolution.To
		}
		return ResolveAgentOutboundTargetResult{
			ResolvedTarget: &resolution,
			ResolvedTo:     resolvedTo,
			TargetMode:     targetMode,
		}
	}

	return ResolveAgentOutboundTargetResult{
		ResolvedTarget: nil,
		ResolvedTo:     plan.ResolvedTo,
		TargetMode:     targetMode,
	}
}
