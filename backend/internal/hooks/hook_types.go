package hooks

// ============================================================================
// Hook 内部事件系统类型
// 对应 TS: hooks/types.ts + hooks/internal-hooks.ts
// ============================================================================

// HookSource 钩子来源
type HookSource string

const (
	HookSourceBundled   HookSource = "openacosmi-bundled"
	HookSourceManaged   HookSource = "openacosmi-managed"
	HookSourceWorkspace HookSource = "openacosmi-workspace"
	HookSourcePlugin    HookSource = "openacosmi-plugin"
)

// HookInstallKind 安装方式
type HookInstallKind string

const (
	HookInstallBundled HookInstallKind = "bundled"
	HookInstallNPM     HookInstallKind = "npm"
	HookInstallGit     HookInstallKind = "git"
)

// HookInstallSpec 安装规格
// 对应 TS: types.ts HookInstallSpec
type HookInstallSpec struct {
	ID         string          `json:"id,omitempty"`
	Kind       HookInstallKind `json:"kind"`
	Label      string          `json:"label,omitempty"`
	Package    string          `json:"package,omitempty"`
	Repository string          `json:"repository,omitempty"`
	Bins       []string        `json:"bins,omitempty"`
}

// HookMetadata 钩子元数据
// 对应 TS: types.ts OpenAcosmiHookMetadata
type HookMetadata struct {
	Always   *bool             `json:"always,omitempty"`
	HookKey  string            `json:"hookKey,omitempty"`
	Emoji    string            `json:"emoji,omitempty"`
	Homepage string            `json:"homepage,omitempty"`
	Events   []string          `json:"events"`
	Export   string            `json:"export,omitempty"`
	OS       []string          `json:"os,omitempty"`
	Requires *HookRequirements `json:"requires,omitempty"`
	Install  []HookInstallSpec `json:"install,omitempty"`
}

// HookRequirements 钩子需求
type HookRequirements struct {
	Bins    []string `json:"bins,omitempty"`
	AnyBins []string `json:"anyBins,omitempty"`
	Env     []string `json:"env,omitempty"`
	Config  []string `json:"config,omitempty"`
}

// HookInvocationPolicy 调用策略
type HookInvocationPolicy struct {
	Enabled bool `json:"enabled"`
}

// Hook 钩子定义
// 对应 TS: types.ts Hook
type Hook struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Source      HookSource `json:"source"`
	PluginID    string     `json:"pluginId,omitempty"`
	FilePath    string     `json:"filePath"`    // Path to HOOK.md
	BaseDir     string     `json:"baseDir"`     // Directory containing hook
	HandlerPath string     `json:"handlerPath"` // Path to handler module
}

// HookEntry 钩子条目（hook + parsed frontmatter + metadata）
// 对应 TS: types.ts HookEntry
type HookEntry struct {
	Hook        Hook                  `json:"hook"`
	Frontmatter map[string]string     `json:"frontmatter"`
	Metadata    *HookMetadata         `json:"metadata,omitempty"`
	Invocation  *HookInvocationPolicy `json:"invocation,omitempty"`
}

// HookEligibilityContext 钩子资格上下文
// 对应 TS: types.ts HookEligibilityContext
type HookEligibilityContext struct {
	Remote *HookRemoteContext `json:"remote,omitempty"`
}

// HookRemoteContext 远程执行上下文
type HookRemoteContext struct {
	Platforms []string                 `json:"platforms,omitempty"`
	HasBin    func(bin string) bool    `json:"-"`
	HasAnyBin func(bins []string) bool `json:"-"`
	Note      string                   `json:"note,omitempty"`
}

// HookSnapshot 钩子快照
// 对应 TS: types.ts HookSnapshot
type HookSnapshot struct {
	Hooks         []HookSnapshotEntry `json:"hooks"`
	ResolvedHooks []Hook              `json:"resolvedHooks,omitempty"`
	Version       *int                `json:"version,omitempty"`
}

// HookSnapshotEntry 快照条目
type HookSnapshotEntry struct {
	Name   string   `json:"name"`
	Events []string `json:"events"`
}

// InternalHookEventType 事件类型
type InternalHookEventType string

const (
	HookEventCommand InternalHookEventType = "command"
	HookEventSession InternalHookEventType = "session"
	HookEventAgent   InternalHookEventType = "agent"
	HookEventGateway InternalHookEventType = "gateway"
)

// InternalHookEvent 内部钩子事件
// 对应 TS: internal-hooks.ts InternalHookEvent
type InternalHookEvent struct {
	Type       InternalHookEventType  `json:"type"`
	Action     string                 `json:"action"`
	SessionKey string                 `json:"sessionKey"`
	Context    map[string]interface{} `json:"context"`
	Timestamp  int64                  `json:"timestamp"` // Unix ms
	Messages   []string               `json:"messages"`
}

// InternalHookHandler 内部钩子处理函数
type InternalHookHandler func(event *InternalHookEvent) error

// HookConfig 钩子配置条目（用于 config 中的 hooks.internal.entries[key]）
type HookConfig struct {
	Enabled  *bool                  `json:"enabled,omitempty"`
	Messages *int                   `json:"messages,omitempty"`
	Env      map[string]string      `json:"env,omitempty"`
	Extra    map[string]interface{} `json:"-"` // 其他自定义字段
}
