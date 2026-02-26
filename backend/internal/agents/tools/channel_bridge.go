// tools/channel_bridge.go — 频道工具桥接。
// TS 参考：src/agents/channel-tools.ts (121L)
package tools

// ChannelAction 频道支持的操作。
type ChannelAction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ChannelPlugin 频道插件接口。
type ChannelPlugin interface {
	ID() string
	SupportedActions() []ChannelAction
	AgentTools() []*AgentTool
}

// ListChannelSupportedActions 列出单个频道支持的操作。
// TS 参考: channel-tools.ts listChannelSupportedActions
func ListChannelSupportedActions(plugin ChannelPlugin) []ChannelAction {
	if plugin == nil {
		return nil
	}
	return plugin.SupportedActions()
}

// ListAllChannelSupportedActions 列出所有频道插件支持的操作。
// TS 参考: channel-tools.ts listAllChannelSupportedActions
func ListAllChannelSupportedActions(plugins []ChannelPlugin) map[string][]ChannelAction {
	result := make(map[string][]ChannelAction)
	for _, p := range plugins {
		actions := p.SupportedActions()
		if len(actions) > 0 {
			result[p.ID()] = actions
		}
	}
	return result
}

// ListChannelAgentTools 收集所有频道插件的工具列表。
// TS 参考: channel-tools.ts listChannelAgentTools
func ListChannelAgentTools(plugins []ChannelPlugin) []*AgentTool {
	var result []*AgentTool
	for _, p := range plugins {
		tools := p.AgentTools()
		result = append(result, tools...)
	}
	return result
}

// ToolHint 工具提示（频道消息相关）。
type ToolHint struct {
	ToolName string `json:"tool_name"`
	Hint     string `json:"hint,omitempty"`
}

// ChannelToolHintResolver 频道工具提示解析器。
type ChannelToolHintResolver interface {
	ResolveToolHints(messageType string) []ToolHint
}

// ResolveChannelMessageToolHints 从频道解析消息工具提示。
// TS 参考: channel-tools.ts resolveChannelMessageToolHints
func ResolveChannelMessageToolHints(resolver ChannelToolHintResolver, messageType string) []ToolHint {
	if resolver == nil {
		return nil
	}
	return resolver.ResolveToolHints(messageType)
}
