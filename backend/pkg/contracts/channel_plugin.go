package contracts

// ChannelPlugin 契约 — 继承自 src/channels/plugins/types.plugin.ts (85 行)
// 每个频道插件实现此接口，最多支持 23 个适配器槽位

// ChannelPlugin 频道插件完整定义
type ChannelPlugin struct {
	ID           ChannelID              `json:"id"`
	Meta         ChannelMeta            `json:"meta"`
	Capabilities ChannelCapabilities    `json:"capabilities"`
	Defaults     *ChannelPluginDefaults `json:"defaults,omitempty"`
	Reload       *ChannelReloadConfig   `json:"reload,omitempty"`

	// 以下为可选适配器槽位，由具体频道实现按需提供
	Config      ChannelConfigAdapter      `json:"-"`
	Setup       ChannelSetupAdapter       `json:"-"`
	Pairing     ChannelPairingAdapter     `json:"-"`
	Security    ChannelSecurityAdapter    `json:"-"`
	Groups      ChannelGroupAdapter       `json:"-"`
	Mentions    ChannelMentionAdapter     `json:"-"`
	Outbound    ChannelOutboundAdapter    `json:"-"`
	Status      ChannelStatusAdapter      `json:"-"`
	Gateway     ChannelGatewayAdapter     `json:"-"`
	Elevated    ChannelElevatedAdapter    `json:"-"`
	Commands    *ChannelCommandAdapter    `json:"-"`
	Streaming   ChannelStreamingAdapter   `json:"-"`
	Threading   ChannelThreadingAdapter   `json:"-"`
	Directory   ChannelDirectoryAdapter   `json:"-"`
	AgentPrompt ChannelAgentPromptAdapter `json:"-"`
}

// ChannelPluginDefaults 插件默认值
type ChannelPluginDefaults struct {
	Queue *ChannelQueueDefaults `json:"queue,omitempty"`
}

// ChannelQueueDefaults 队列默认值
type ChannelQueueDefaults struct {
	DebounceMs int `json:"debounceMs,omitempty"`
}

// ChannelReloadConfig 热重载配置
type ChannelReloadConfig struct {
	ConfigPrefixes []string `json:"configPrefixes"`
	NoopPrefixes   []string `json:"noopPrefixes,omitempty"`
}

// ChannelConfigSchema 配置 schema
type ChannelConfigSchema struct {
	Schema  map[string]interface{}         `json:"schema"`
	UIHints map[string]ChannelConfigUIHint `json:"uiHints,omitempty"`
}

// ChannelConfigUIHint 配置 UI 提示
type ChannelConfigUIHint struct {
	Label       string `json:"label,omitempty"`
	Help        string `json:"help,omitempty"`
	Advanced    bool   `json:"advanced,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}
