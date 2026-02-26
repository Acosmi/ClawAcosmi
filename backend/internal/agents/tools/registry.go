// tools/registry.go — 工具注册表 + 聚合。
// TS 参考：src/agents/openacosmi-tools.ts (170L)
package tools

import (
	"sort"
	"sync"
)

// ToolRegistry 工具注册表（线程安全）。
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*AgentTool
}

// NewToolRegistry 创建新注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*AgentTool),
	}
}

// Register 注册工具。
func (r *ToolRegistry) Register(tool *AgentTool) {
	if tool == nil || tool.Name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Get 按名称获取工具。
func (r *ToolRegistry) Get(name string) *AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List 列出所有已注册工具（按名称排序）。
func (r *ToolRegistry) List() []*AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]*AgentTool, 0, len(names))
	for _, name := range names {
		result = append(result, r.tools[name])
	}
	return result
}

// Names 返回所有工具名称。
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Has 检查工具是否存在。
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// Count 返回已注册工具数量。
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// ---------- OpenAcosmi 工具集构建 ----------

// OpenAcosmiToolsOptions 构建 OpenAcosmi 工具集的选项。
// TS 参考: openacosmi-tools.ts L11-20
type OpenAcosmiToolsOptions struct {
	Workspace      string
	AgentID        string
	SessionKey     string
	Sandbox        bool
	SandboxRoot    string
	EnableBrowser  bool
	EnableCanvas   bool
	EnableCron     bool
	EnableGateway  bool
	EnableMemory   bool
	EnableNodes    bool
	EnableMessage  bool
	EnableTTS      bool
	EnableWebTools bool
	EnableImage    bool
	EnableSessions bool
}

// CreateOpenAcosmiTools 构建 OpenAcosmi 的全量工具集。
// TS 参考: openacosmi-tools.ts createOpenAcosmiTools
func CreateOpenAcosmiTools(opts OpenAcosmiToolsOptions) *ToolRegistry {
	reg := NewToolRegistry()

	// 核心工具（始终包含）
	// Session 状态查询
	if opts.EnableSessions {
		registerSessionTools(reg, opts)
	}

	// 文件读写工具
	registerFileTools(reg, opts)

	// 可选工具
	if opts.EnableBrowser {
		registerBrowserTool(reg, opts)
	}
	if opts.EnableCanvas {
		registerCanvasTool(reg, opts)
	}
	if opts.EnableCron {
		registerCronTool(reg, opts)
	}
	if opts.EnableGateway {
		registerGatewayTool(reg, opts)
	}
	if opts.EnableMemory {
		registerMemoryTool(reg, opts)
	}
	if opts.EnableMessage {
		registerMessageTool(reg, opts)
	}
	if opts.EnableNodes {
		registerNodesTool(reg, opts)
	}
	if opts.EnableTTS {
		registerTTSTool(reg, opts)
	}
	if opts.EnableWebTools {
		registerWebTools(reg, opts)
	}
	if opts.EnableImage {
		registerImageTool(reg, opts)
	}

	return reg
}

// ---------- 工具注册实现 ----------
// 以下函数将已实现的 Create*Tool() 构造器挂载到 ToolRegistry。
// 需要外部依赖的构造器使用 nil 占位 — 运行时由模块初始化代码注入实例。
// 这与 TS 原版模式一致：注册时只挂定义，依赖按需注入。

func registerSessionTools(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// sessions.go: 5 个会话工具
	// SessionManager 由运行时注入，此处 nil 占位使工具定义可被发现。
	reg.Register(CreateSessionsListTool(nil))
	reg.Register(CreateSessionsHistoryTool(nil))
	reg.Register(CreateSessionsSendTool(nil))
	reg.Register(CreateSessionsSpawnTool(nil))
	reg.Register(CreateSessionStatusTool(nil))
}

func registerFileTools(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// read.go: 文件读取工具
	reg.Register(CreateReadTool(ReadToolOptions{
		Sandbox:     opts.Sandbox,
		SandboxRoot: opts.SandboxRoot,
	}))
}

func registerBrowserTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// browser_tool.go: 浏览器控制工具
	// BrowserController 由运行时注入。
	reg.Register(CreateBrowserTool(nil))
}

func registerCanvasTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// canvas_tool.go: Canvas 工具
	// CanvasProvider 由运行时注入。
	reg.Register(CreateCanvasTool(nil))
}

func registerCronTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// cron_tool.go: 定时任务工具
	// CronManager 由运行时注入。
	reg.Register(CreateCronTool(nil))
}

func registerGatewayTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// gateway.go: Gateway 工具
	reg.Register(CreateGatewayTool(DefaultGatewayOptions()))
}

func registerMemoryTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// memory_tool.go: 记忆搜索 + 获取工具
	// MemoryStore 由运行时注入。
	reg.Register(CreateMemorySearchTool(nil))
	reg.Register(CreateMemoryGetTool(nil))
}

func registerMessageTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// message_tool.go: 消息发送工具
	// MessageSender 由运行时注入。
	reg.Register(CreateMessageTool(nil))
}

func registerNodesTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// nodes_tool.go: 节点管理工具
	// NodeStore 由运行时注入。
	reg.Register(CreateNodesTool(nil))
}

func registerTTSTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// tts_tool.go: TTS 工具
	// TTSProvider 由运行时注入。
	reg.Register(CreateTTSTool(nil))
}

func registerWebTools(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// web_fetch.go: 网页抓取 + 搜索工具
	reg.Register(CreateWebFetchTool(DefaultWebFetchOptions()))
	// WebSearchProvider 由运行时注入。
	reg.Register(CreateWebSearchTool(nil))
}

func registerImageTool(reg *ToolRegistry, opts OpenAcosmiToolsOptions) {
	// image_tool.go: 图像生成工具
	// ImageProvider 由运行时注入, workspace 从 opts 获取。
	reg.Register(CreateImageTool(nil, opts.Workspace))
}
