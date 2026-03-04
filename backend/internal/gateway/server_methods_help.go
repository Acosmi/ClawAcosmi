package gateway

// server_methods_help.go — 三级指挥体系 Phase 4: 子智能体求助 RPC
//
// WebSocket RPC: subagent.help.resolve — 前端/主智能体回复子智能体求助
// 事件: subagent.help.requested（广播，由 server.go SpawnSubagent goroutine 触发）

import (
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/agents/runner"
)

func timeNowMs() int64 {
	return time.Now().UnixMilli()
}

// agentChannelRef 活跃子智能体通道引用。
type agentChannelRef struct {
	Channel    *runner.AgentChannel
	ContractID string
	Label      string
}

// RegisterHelpChannel 注册 help request ID → AgentChannel 映射。
// 由 server.go SpawnSubagent 的 toParent 监听 goroutine 调用。
func (s *GatewayState) RegisterHelpChannel(helpMsgID string, ch *runner.AgentChannel, contractID, label string) {
	s.agentChannelsMu.Lock()
	defer s.agentChannelsMu.Unlock()
	if s.agentChannels == nil {
		s.agentChannels = make(map[string]*agentChannelRef)
	}
	s.agentChannels[helpMsgID] = &agentChannelRef{
		Channel:    ch,
		ContractID: contractID,
		Label:      label,
	}
}

// UnregisterHelpChannel 移除 help request ID → AgentChannel 映射。
func (s *GatewayState) UnregisterHelpChannel(helpMsgID string) {
	s.agentChannelsMu.Lock()
	defer s.agentChannelsMu.Unlock()
	delete(s.agentChannels, helpMsgID)
}

// LookupHelpChannel 查找 help request ID 对应的 AgentChannel。
func (s *GatewayState) LookupHelpChannel(helpMsgID string) *agentChannelRef {
	s.agentChannelsMu.RLock()
	defer s.agentChannelsMu.RUnlock()
	return s.agentChannels[helpMsgID]
}

// CleanupAgentChannels 清理指定合约的所有 help channel 映射。
// 返回被清理的 help request ID 列表（用于广播通知前端清除弹窗）。
func (s *GatewayState) CleanupAgentChannels(contractID string) []string {
	s.agentChannelsMu.Lock()
	defer s.agentChannelsMu.Unlock()
	var removed []string
	for id, ref := range s.agentChannels {
		if ref.ContractID == contractID {
			removed = append(removed, id)
			delete(s.agentChannels, id)
		}
	}
	return removed
}

// SubagentHelpHandlers 返回子智能体求助相关的 RPC 处理器。
func SubagentHelpHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		// subagent.help.resolve — 前端/用户回复子智能体求助
		// params: { id: string, response: string }
		"subagent.help.resolve": func(ctx *MethodHandlerContext) {
			helpID, _ := ctx.Params["id"].(string)
			response, _ := ctx.Params["response"].(string)
			if helpID == "" || response == "" {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "missing id or response"))
				return
			}

			ref := ctx.Context.State.LookupHelpChannel(helpID)
			if ref == nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "no pending help request with id: "+helpID))
				return
			}

			// 发送回复到子智能体
			if err := ref.Channel.SendHelpResponse(helpID, response); err != nil {
				ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to send response: "+err.Error()))
				return
			}

			// 清理映射
			ctx.Context.State.UnregisterHelpChannel(helpID)

			// 广播 resolved 事件
			if ctx.Context.Broadcaster != nil {
				ctx.Context.Broadcaster.Broadcast("subagent.help.resolved", map[string]interface{}{
					"id":       helpID,
					"response": response,
					"ts":       timeNowMs(),
				}, nil)
			}

			ctx.Respond(true, map[string]interface{}{"ok": true}, nil)
		},
	}
}
