package types

// 沙箱配置类型 — 继承自 src/config/types.sandbox.ts

// SandboxDockerSettings Docker 沙箱设置
// 原版: export type SandboxDockerSettings
type SandboxDockerSettings struct {
	Image           string                 `json:"image,omitempty"`
	ContainerPrefix string                 `json:"containerPrefix,omitempty"`
	Workdir         string                 `json:"workdir,omitempty"` // 默认 /workspace
	ReadOnlyRoot    *bool                  `json:"readOnlyRoot,omitempty"`
	Tmpfs           []string               `json:"tmpfs,omitempty"`
	Network         string                 `json:"network,omitempty"` // bridge|none|custom
	User            string                 `json:"user,omitempty"`    // uid:gid
	CapDrop         []string               `json:"capDrop,omitempty"`
	Env             map[string]string      `json:"env,omitempty"`
	SetupCommand    string                 `json:"setupCommand,omitempty"`
	PidsLimit       *int                   `json:"pidsLimit,omitempty"`
	Memory          interface{}            `json:"memory,omitempty"`     // string|number
	MemorySwap      interface{}            `json:"memorySwap,omitempty"` // string|number
	Cpus            *float64               `json:"cpus,omitempty"`
	Ulimits         map[string]interface{} `json:"ulimits,omitempty"` // 复杂联合类型
	SeccompProfile  string                 `json:"seccompProfile,omitempty"`
	ApparmorProfile string                 `json:"apparmorProfile,omitempty"`
	DNS             []string               `json:"dns,omitempty"`
	ExtraHosts      []string               `json:"extraHosts,omitempty"`
	Binds           []string               `json:"binds,omitempty"`
}

// SandboxBrowserSettings 沙箱浏览器设置
type SandboxBrowserSettings struct {
	Enabled            *bool  `json:"enabled,omitempty"`
	Image              string `json:"image,omitempty"`
	ContainerPrefix    string `json:"containerPrefix,omitempty"`
	CDPPort            *int   `json:"cdpPort,omitempty"`
	VNCPort            *int   `json:"vncPort,omitempty"`
	NoVNCPort          *int   `json:"noVncPort,omitempty"`
	Headless           *bool  `json:"headless,omitempty"`
	EnableNoVNC        *bool  `json:"enableNoVnc,omitempty"`
	AllowHostControl   *bool  `json:"allowHostControl,omitempty"`
	AutoStart          *bool  `json:"autoStart,omitempty"`
	AutoStartTimeoutMs *int   `json:"autoStartTimeoutMs,omitempty"`
}

// SandboxPruneSettings 沙箱自动清理设置
type SandboxPruneSettings struct {
	IdleHours  *int `json:"idleHours,omitempty"`  // 空闲超过N小时清理
	MaxAgeDays *int `json:"maxAgeDays,omitempty"` // 超过N天清理
}
