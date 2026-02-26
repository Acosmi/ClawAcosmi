package types

// Agent 配置类型 — 继承自 src/config/types.agents.ts (83 行)

// AgentModelConfig Agent 模型配置
// 原版可以是 string 或 {primary, fallbacks} 对象
// Go 中使用结构体表示，string 场景通过 Primary 字段覆盖
type AgentModelConfig struct {
	Primary   string    `json:"primary,omitempty"`
	Fallbacks *[]string `json:"fallbacks,omitempty"`
}

// AgentSubagentsConfig Agent 子代理配置
type AgentSubagentsConfig struct {
	AllowAgents []string    `json:"allowAgents,omitempty"` // 使用 "*" 允许全部
	Model       interface{} `json:"model,omitempty"`       // string | {primary, fallbacks}
}

// AgentSandboxConfig Agent 沙箱配置
type AgentSandboxConfig struct {
	Mode                   string                  `json:"mode,omitempty"`                   // "off"|"non-main"|"all"
	WorkspaceAccess        string                  `json:"workspaceAccess,omitempty"`        // "none"|"ro"|"rw"
	SessionToolsVisibility string                  `json:"sessionToolsVisibility,omitempty"` // "spawned"|"all"
	Scope                  string                  `json:"scope,omitempty"`                  // "session"|"agent"|"shared"
	PerSession             *bool                   `json:"perSession,omitempty"`             // @deprecated 使用 Scope
	WorkspaceRoot          string                  `json:"workspaceRoot,omitempty"`
	Docker                 *SandboxDockerSettings  `json:"docker,omitempty"`
	Browser                *SandboxBrowserSettings `json:"browser,omitempty"`
	Prune                  *SandboxPruneSettings   `json:"prune,omitempty"`
}

// AgentListItemConfig 单个 Agent 配置
// 原版: export type AgentConfig (重命名避免与 Phase 0 的 AgentConfig 冲突)
type AgentListItemConfig struct {
	ID           string                `json:"id"`
	Default      *bool                 `json:"default,omitempty"`
	Name         string                `json:"name,omitempty"`
	Workspace    string                `json:"workspace,omitempty"`
	AgentDir     string                `json:"agentDir,omitempty"`
	Model        *AgentModelConfig     `json:"model,omitempty"`
	Skills       []string              `json:"skills,omitempty"`
	MemorySearch *MemorySearchConfig   `json:"memorySearch,omitempty"`
	HumanDelay   *HumanDelayConfig     `json:"humanDelay,omitempty"`
	Heartbeat    *HeartbeatConfig      `json:"heartbeat,omitempty"`
	Identity     *IdentityConfig       `json:"identity,omitempty"`
	GroupChat    *GroupChatConfig      `json:"groupChat,omitempty"`
	Subagents    *AgentSubagentsConfig `json:"subagents,omitempty"`
	Sandbox      *AgentSandboxConfig   `json:"sandbox,omitempty"`
	Tools        *AgentToolsConfig     `json:"tools,omitempty"`
}

// AgentsConfig Agent 总配置
// 原版: export type AgentsConfig
type AgentsConfig struct {
	Defaults *AgentDefaultsConfig  `json:"defaults,omitempty"`
	List     []AgentListItemConfig `json:"list,omitempty"`
}

// AgentBindingPeerMatch Agent 绑定的对等匹配条件
type AgentBindingPeerMatch struct {
	Kind ChatType `json:"kind"`
	ID   string   `json:"id"`
}

// AgentBindingMatch Agent 绑定匹配条件
type AgentBindingMatch struct {
	Channel   string                 `json:"channel"`
	AccountID string                 `json:"accountId,omitempty"`
	Peer      *AgentBindingPeerMatch `json:"peer,omitempty"`
	GuildID   string                 `json:"guildId,omitempty"`
	TeamID    string                 `json:"teamId,omitempty"`
}

// AgentBinding Agent 路由绑定
type AgentBinding struct {
	AgentID string            `json:"agentId"`
	Match   AgentBindingMatch `json:"match"`
}
