package daemon

// GatewayServiceRuntime 描述守护进程服务的运行时状态
// 对应 TS: service-runtime.ts
type GatewayServiceRuntime struct {
	Status         string `json:"status,omitempty"`         // "running" | "stopped" | "unknown"
	State          string `json:"state,omitempty"`          // 平台特定状态字符串
	SubState       string `json:"subState,omitempty"`       // systemd sub-state
	PID            int    `json:"pid,omitempty"`            // 进程 ID
	LastExitStatus int    `json:"lastExitStatus,omitempty"` // 上次退出码
	LastExitReason string `json:"lastExitReason,omitempty"` // 上次退出原因
	LastRunResult  string `json:"lastRunResult,omitempty"`  // 上次运行结果
	LastRunTime    string `json:"lastRunTime,omitempty"`    // 上次运行时间
	Detail         string `json:"detail,omitempty"`         // 附加详情
	CachedLabel    bool   `json:"cachedLabel,omitempty"`    // 是否使用缓存标签
	MissingUnit    bool   `json:"missingUnit,omitempty"`    // systemd unit 文件缺失
}

// GatewayServiceInstallArgs 是安装守护进程服务时的参数
// 对应 TS: service.ts GatewayServiceInstallArgs
type GatewayServiceInstallArgs struct {
	Env              map[string]string // 环境变量
	ProgramArguments []string          // 命令行参数
	WorkingDirectory string            // 工作目录
	Environment      map[string]string // 服务的环境变量（写入配置文件）
	Description      string            // 服务描述
}

// GatewayServiceCommand 描述已安装服务的命令配置
// 对应 TS: service-audit.ts GatewayServiceCommand
type GatewayServiceCommand struct {
	ProgramArguments []string          `json:"programArguments"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	SourcePath       string            `json:"sourcePath,omitempty"`
}

// GatewayService 定义守护进程服务的平台无关接口
// 对应 TS: service.ts GatewayService
type GatewayService interface {
	// Label 返回服务标签（如 "LaunchAgent", "systemd", "Scheduled Task"）
	Label() string
	// LoadedText 返回已加载状态的显示文本
	LoadedText() string
	// NotLoadedText 返回未加载状态的显示文本
	NotLoadedText() string
	// Install 安装服务
	Install(args GatewayServiceInstallArgs) error
	// Uninstall 卸载服务
	Uninstall(env map[string]string) error
	// Stop 停止服务
	Stop(env map[string]string) error
	// Restart 重启服务
	Restart(env map[string]string) error
	// IsLoaded 检查服务是否已加载/启用
	IsLoaded(env map[string]string) (bool, error)
	// ReadCommand 读取已安装服务的命令配置
	ReadCommand(env map[string]string) (*GatewayServiceCommand, error)
	// ReadRuntime 读取服务运行时状态
	ReadRuntime(env map[string]string) (GatewayServiceRuntime, error)
}

// ServiceConfigIssue 表示服务配置审计发现的问题
// 对应 TS: service-audit.ts ServiceConfigIssue
type ServiceConfigIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Level   string `json:"level,omitempty"` // "recommended" | "aggressive"
}

// ServiceConfigAudit 表示服务配置审计结果
// 对应 TS: service-audit.ts ServiceConfigAudit
type ServiceConfigAudit struct {
	OK     bool                 `json:"ok"`
	Issues []ServiceConfigIssue `json:"issues"`
}

// ExtraGatewayService 描述系统中发现的额外 OpenAcosmi 服务实例
// 对应 TS: inspect.ts ExtraGatewayService
type ExtraGatewayService struct {
	Platform string `json:"platform"` // "darwin" | "linux" | "windows"
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	Scope    string `json:"scope"`            // "user" | "system"
	Marker   string `json:"marker,omitempty"` // "openacosmi" | "clawdbot" | "moltbot"
	Legacy   bool   `json:"legacy,omitempty"`
}

// GatewayProgramArgs 描述服务启动的命令行参数
// 对应 TS: program-args.ts GatewayProgramArgs
type GatewayProgramArgs struct {
	ProgramArguments []string `json:"programArguments"`
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
}

// MinimalServicePathOptions 控制 GetMinimalServicePathParts 的选项
// 对应 TS: service-env.ts MinimalServicePathOptions
type MinimalServicePathOptions struct {
	Platform  string            // runtime.GOOS 值
	ExtraDirs []string          // 额外的 PATH 目录
	Home      string            // 用户 HOME 目录
	Env       map[string]string // 环境变量
}
