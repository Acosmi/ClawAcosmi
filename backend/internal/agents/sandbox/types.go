// types.go — Docker sandbox 类型与常量定义。
//
// TS 对照: agents/sandbox/types.ts (86L),
//
//	agents/sandbox/types.docker.ts (23L),
//	agents/sandbox/constants.ts (52L)
//
// 定义沙箱配置、Docker 配置、浏览器配置、
// 工具策略、注册表条目以及所有默认常量。
package sandbox

import "time"

// ---------- Scope / Access 枚举 ----------

// SandboxScope 沙箱作用域。
// TS 对照: types.ts SandboxScope
type SandboxScope string

const (
	ScopeSession SandboxScope = "session"
	ScopeAgent   SandboxScope = "agent"
	ScopeShared  SandboxScope = "shared"
)

// SandboxWorkspaceAccess 工作区访问级别。
// TS 对照: types.ts SandboxWorkspaceAccess
type SandboxWorkspaceAccess string

const (
	AccessReadWrite SandboxWorkspaceAccess = "read-write"
	AccessReadOnly  SandboxWorkspaceAccess = "read-only"
	AccessNone      SandboxWorkspaceAccess = "none"
)

// ---------- Docker 配置 ----------

// SandboxDockerConfig Docker 容器配置。
// TS 对照: types.docker.ts SandboxDockerConfig
type SandboxDockerConfig struct {
	Image           string            `json:"image,omitempty"`
	Workdir         string            `json:"workdir,omitempty"`
	ReadOnlyRoot    bool              `json:"readOnlyRoot,omitempty"`
	Tmpfs           []string          `json:"tmpfs,omitempty"`
	Network         string            `json:"network,omitempty"`
	User            string            `json:"user,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	MemoryMB        int               `json:"memoryMb,omitempty"`
	CPUs            float64           `json:"cpus,omitempty"`
	SeccompPolicy   string            `json:"seccompPolicy,omitempty"`
	ApparmorProfile string            `json:"apparmorProfile,omitempty"`
}

// ---------- 浏览器配置 ----------

// SandboxBrowserConfig 沙箱浏览器配置。
// TS 对照: types.ts SandboxBrowserConfig
type SandboxBrowserConfig struct {
	Enabled            bool   `json:"enabled,omitempty"`
	Image              string `json:"image,omitempty"`
	ContainerPrefix    string `json:"containerPrefix,omitempty"`
	CDPPort            int    `json:"cdpPort,omitempty"`
	VNCPort            int    `json:"vncPort,omitempty"`
	NoVNCPort          int    `json:"noVncPort,omitempty"`
	Headless           bool   `json:"headless,omitempty"`
	EnableNoVNC        bool   `json:"enableNoVnc,omitempty"`
	AllowHostControl   bool   `json:"allowHostControl,omitempty"`
	AutoStart          bool   `json:"autoStart,omitempty"`
	AutoStartTimeoutMs int    `json:"autoStartTimeoutMs,omitempty"`
	IdleTimeout        int    `json:"idleTimeout,omitempty"`
}

// ---------- 工具策略 ----------

// SandboxToolPolicy 沙箱工具策略。
// TS 对照: types.ts SandboxToolPolicy
type SandboxToolPolicy struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// ---------- 剪枝配置 ----------

// SandboxPruneConfig 沙箱清理配置。
// TS 对照: types.ts (embedded in SandboxConfig)
type SandboxPruneConfig struct {
	IdleHours  int `json:"idleHours,omitempty"`
	MaxAgeDays int `json:"maxAgeDays,omitempty"`
}

// ---------- 沙箱主配置 ----------

// SandboxConfig 沙箱配置。
// TS 对照: types.ts SandboxConfig
type SandboxConfig struct {
	Enabled   bool                   `json:"enabled,omitempty"`
	Scope     SandboxScope           `json:"scope,omitempty"`
	Workspace SandboxWorkspaceAccess `json:"workspace,omitempty"`
	Docker    SandboxDockerConfig    `json:"docker,omitempty"`
	Browser   SandboxBrowserConfig   `json:"browser,omitempty"`
	Tools     SandboxToolPolicy      `json:"tools,omitempty"`
	Prune     SandboxPruneConfig     `json:"prune,omitempty"`
}

// ---------- 沙箱上下文 ----------

// SandboxContext 已解析的沙箱上下文（运行时使用）。
// TS 对照: types.ts SandboxContext
type SandboxContext struct {
	ContainerName string                 `json:"containerName"`
	SessionKey    string                 `json:"sessionKey"`
	WorkspaceDir  string                 `json:"workspaceDir,omitempty"`
	AgentID       string                 `json:"agentId"`
	Browser       *SandboxBrowserContext `json:"browser,omitempty"`
}

// SandboxBrowserContext 浏览器上下文。
// TS 对照: browser.ts 返回的 SandboxBrowserContext
type SandboxBrowserContext struct {
	ContainerName string `json:"containerName"`
	BridgeURL     string `json:"bridgeUrl"`
	NoVNCURL      string `json:"noVncUrl,omitempty"`
	CDPPort       int    `json:"cdpPort,omitempty"`
}

// ---------- 运行时状态 ----------

// SandboxRuntimeStatus 沙箱运行时状态。
// TS 对照: runtime-status.ts SandboxRuntimeStatus
type SandboxRuntimeStatus struct {
	AgentID     string             `json:"agentId"`
	Mode        string             `json:"mode"` // "enforced" | "advisory" | "off"
	IsSandboxed bool               `json:"isSandboxed"`
	ToolPolicy  *SandboxToolPolicy `json:"toolPolicy,omitempty"`
}

// ---------- 注册表条目 ----------

// RegistryEntry 沙箱容器注册表条目。
// TS 对照: registry.ts SandboxRegistryEntry
type RegistryEntry struct {
	ContainerName string `json:"containerName"`
	SessionKey    string `json:"sessionKey"`
	CreatedAtMs   int64  `json:"createdAtMs"`
	LastUsedAtMs  int64  `json:"lastUsedAtMs"`
	Image         string `json:"image"`
	ConfigHash    string `json:"configHash,omitempty"`
}

// BrowserRegistryEntry 浏览器注册表条目。
// TS 对照: registry.ts BrowserRegistryEntry
type BrowserRegistryEntry struct {
	ContainerName string `json:"containerName"`
	SessionKey    string `json:"sessionKey"`
	CreatedAtMs   int64  `json:"createdAtMs"`
	LastUsedAtMs  int64  `json:"lastUsedAtMs"`
	Image         string `json:"image"`
	CDPPort       int    `json:"cdpPort"`
}

// Registry 注册表数据文件。
type Registry struct {
	Entries []RegistryEntry `json:"entries"`
}

// BrowserRegistry 浏览器注册表数据文件。
type BrowserRegistry struct {
	Entries []BrowserRegistryEntry `json:"entries"`
}

// ---------- 容器信息 ----------

// ContainerState Docker 容器状态。
// TS 对照: docker.ts ContainerState
type ContainerState struct {
	Exists  bool   `json:"exists"`
	Running bool   `json:"running"`
	Status  string `json:"status,omitempty"`
}

// ContainerInfo 容器列表信息。
// TS 对照: manage.ts SandboxContainerInfo
type ContainerInfo struct {
	RegistryEntry
	Running    bool `json:"running"`
	ImageMatch bool `json:"imageMatch"`
}

// BrowserContainerInfo 浏览器容器列表信息。
// TS 对照: manage.ts SandboxBrowserInfo
type BrowserContainerInfo struct {
	BrowserRegistryEntry
	Running    bool `json:"running"`
	ImageMatch bool `json:"imageMatch"`
	NoVncPort  int  `json:"noVncPort,omitempty"`
}

// ---------- Docker 执行选项 ----------

// ExecDockerOpts Docker 命令执行选项。
type ExecDockerOpts struct {
	AllowFailure bool
	TimeoutSec   int
}

// ExecDockerResult Docker 命令执行结果。
type ExecDockerResult struct {
	Code   int
	Stdout string
	Stderr string
}

// ---------- 默认常量 ----------
// TS 对照: constants.ts

const (
	// DefaultImage 默认沙箱 Docker 镜像。
	DefaultImage = "ghcr.io/openacosmi/sandbox:latest"

	// DefaultBrowserImage 默认浏览器 Docker 镜像。
	DefaultBrowserImage = "ghcr.io/openacosmi/browser:latest"

	// DefaultContainerPrefix 容器名前缀。
	DefaultContainerPrefix = "openacosmi-sandbox-"

	// DefaultBrowserContainerPrefix 浏览器容器名前缀。
	DefaultBrowserContainerPrefix = "openacosmi-browser-"

	// DefaultWorkdir 容器内工作目录。
	DefaultWorkdir = "/workspace"

	// DefaultIdleHours 空闲清理小时数。
	DefaultIdleHours = 24

	// DefaultMaxAgeDays 容器最大存活天数。
	DefaultMaxAgeDays = 7

	// DefaultCDPPort 默认 CDP 端口。
	DefaultCDPPort = 9222

	// DefaultVNCPort 默认 VNC 端口。
	DefaultVNCPort = 5900

	// DefaultNoVNCPort 默认 noVNC 端口。
	// TS 对照: constants.ts DEFAULT_SANDBOX_BROWSER_NOVNC_PORT
	DefaultNoVNCPort = 6080

	// DefaultBrowserIdleTimeout 浏览器空闲超时秒数。
	DefaultBrowserIdleTimeout = 300

	// DefaultBrowserAutoStartTimeoutMs 浏览器自动启动超时毫秒。
	// TS 对照: constants.ts DEFAULT_SANDBOX_BROWSER_AUTOSTART_TIMEOUT_MS
	DefaultBrowserAutoStartTimeoutMs = 12000

	// DefaultMemoryMB 默认内存限制 MB。
	DefaultMemoryMB = 512

	// DefaultCPUs 默认 CPU 核数。
	DefaultCPUs = 1.0

	// DefaultExecTimeout 默认 Docker 命令超时秒数。
	DefaultExecTimeout = 30

	// CDPWaitTimeout CDP 就绪等待超时。
	CDPWaitTimeout = 30 * time.Second

	// CDPWaitInterval CDP 就绪等待轮询间隔。
	CDPWaitInterval = 150 * time.Millisecond

	// RegistryFilename 容器注册表文件名。
	RegistryFilename = "sandbox-registry.json"

	// BrowserRegistryFilename 浏览器注册表文件名。
	BrowserRegistryFilename = "browser-registry.json"

	// SandboxAgentWorkspaceMount agent 工作区挂载点。
	// TS 对照: constants.ts SANDBOX_AGENT_WORKSPACE_MOUNT
	SandboxAgentWorkspaceMount = "/agent"
)

// DefaultToolAllow 默认工具白名单。
// TS 对照: constants.ts DEFAULT_TOOL_ALLOW
var DefaultToolAllow = []string{
	"Read",
	"Grep",
	"Glob",
	"LS",
	"Write",
	"Edit",
	"MultiEdit",
	"Bash",
	"TodoRead",
	"TodoWrite",
	"WebFetch",
}

// DefaultToolDeny 默认工具黑名单。
// TS 对照: constants.ts DEFAULT_TOOL_DENY
var DefaultToolDeny = []string{
	"Computer",
	"BrowserNavigate",
	"BrowserClick",
	"BrowserType",
}
