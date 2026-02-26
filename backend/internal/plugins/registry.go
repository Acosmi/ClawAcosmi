package plugins

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
)

// PluginToolRegistration 工具注册记录
type PluginToolRegistration struct {
	PluginID string
	Factory  PluginToolFactory
	Names    []string
	Optional bool
	Source   string
}

// PluginCliRegistration CLI 注册记录
type PluginCliRegistration struct {
	PluginID string
	Register PluginCliRegistrar
	Commands []string
	Source   string
}

// PluginHttpRegistration HTTP 处理注册
type PluginHttpRegistration struct {
	PluginID string
	Handler  PluginHttpHandler
	Source   string
}

// PluginHttpRouteRegistration HTTP 路由注册
type PluginHttpRouteRegistration struct {
	PluginID string
	Path     string
	Handler  PluginHttpRouteHandler
	Source   string
}

// PluginChannelRegistrationEntry 渠道注册记录
type PluginChannelRegistrationEntry struct {
	PluginID string
	Plugin   ChannelPlugin
	Source   string
}

// PluginProviderRegistration 提供商注册
type PluginProviderRegistration struct {
	PluginID string
	Provider ProviderPlugin
	Source   string
}

// PluginHookRegistrationEntry 钩子注册记录
type PluginHookRegistrationEntry struct {
	PluginID string
	Events   []string
	Name     string
	Source   string
}

// PluginServiceRegistration 服务注册
type PluginServiceRegistration struct {
	PluginID string
	Service  PluginService
	Source   string
}

// PluginCommandRegistration 命令注册
type PluginCommandRegistration struct {
	PluginID string
	Command  PluginCommandDefinition
	Source   string
}

// PluginRecord 插件元数据记录
// 对应 TS: registry.ts PluginRecord
type PluginRecord struct {
	ID               string
	Name             string
	Version          string
	Description      string
	Kind             PluginKind
	Source           string
	Origin           PluginOrigin
	WorkspaceDir     string
	Enabled          bool
	Status           string // "loaded" | "disabled" | "error"
	Error            string
	ToolNames        []string
	HookNames        []string
	ChannelIDs       []string
	ProviderIDs      []string
	GatewayMethods   []string
	CliCommands      []string
	Services         []string
	Commands         []string
	HttpHandlers     int
	HookCount        int
	ConfigSchema     bool
	ConfigUiHints    map[string]PluginConfigUiHint
	ConfigJsonSchema map[string]interface{}
}

// PluginRegistry 插件注册表
// 对应 TS: registry.ts PluginRegistry
type PluginRegistry struct {
	mu sync.RWMutex

	Plugins         []PluginRecord
	Tools           []PluginToolRegistration
	Hooks           []PluginHookRegistrationEntry
	TypedHooks      []PluginHookRegistration
	Channels        []PluginChannelRegistrationEntry
	Providers       []PluginProviderRegistration
	GatewayHandlers map[string]GatewayRequestHandler
	HttpHandlers    []PluginHttpRegistration
	HttpRoutes      []PluginHttpRouteRegistration
	CliRegistrars   []PluginCliRegistration
	ServiceEntries  []PluginServiceRegistration
	CommandEntries  []PluginCommandRegistration
	Diagnostics     []PluginDiagnostic

	coreGatewayMethods map[string]bool
	runtime            PluginRuntime
	logger             PluginLogger
}

// NewPluginRegistry 创建插件注册表
// 对应 TS: registry.ts createPluginRegistry
func NewPluginRegistry(runtime PluginRuntime, logger PluginLogger, coreGatewayHandlers map[string]GatewayRequestHandler) *PluginRegistry {
	core := make(map[string]bool)
	for k := range coreGatewayHandlers {
		core[k] = true
	}
	gw := make(map[string]GatewayRequestHandler)
	for k, v := range coreGatewayHandlers {
		gw[k] = v
	}

	return &PluginRegistry{
		Plugins:            make([]PluginRecord, 0),
		Tools:              make([]PluginToolRegistration, 0),
		Hooks:              make([]PluginHookRegistrationEntry, 0),
		TypedHooks:         make([]PluginHookRegistration, 0),
		Channels:           make([]PluginChannelRegistrationEntry, 0),
		Providers:          make([]PluginProviderRegistration, 0),
		GatewayHandlers:    gw,
		HttpHandlers:       make([]PluginHttpRegistration, 0),
		HttpRoutes:         make([]PluginHttpRouteRegistration, 0),
		CliRegistrars:      make([]PluginCliRegistration, 0),
		ServiceEntries:     make([]PluginServiceRegistration, 0),
		CommandEntries:     make([]PluginCommandRegistration, 0),
		Diagnostics:        make([]PluginDiagnostic, 0),
		coreGatewayMethods: core,
		runtime:            runtime,
		logger:             logger,
	}
}

// PushDiagnostic 添加诊断信息
func (r *PluginRegistry) PushDiagnostic(diag PluginDiagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Diagnostics = append(r.Diagnostics, diag)
}

// RegisterTool 注册工具
// 对应 TS: registry.ts registerTool
func (r *PluginRegistry) RegisterTool(record *PluginRecord, factory PluginToolFactory, opts *PluginToolOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var names []string
	optional := false
	if opts != nil {
		if len(opts.Names) > 0 {
			names = opts.Names
		} else if opts.Name != "" {
			names = []string{opts.Name}
		}
		optional = opts.Optional
	}

	normalized := make([]string, 0, len(names))
	for _, n := range names {
		trimmed := strings.TrimSpace(n)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}

	record.ToolNames = append(record.ToolNames, normalized...)
	r.Tools = append(r.Tools, PluginToolRegistration{
		PluginID: record.ID,
		Factory:  factory,
		Names:    normalized,
		Optional: optional,
		Source:   record.Source,
	})
}

// RegisterHook 注册钩子
// 对应 TS: registry.ts registerHook
func (r *PluginRegistry) RegisterHook(record *PluginRecord, events []string, name string, source string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalizedEvents := make([]string, 0, len(events))
	for _, e := range events {
		trimmed := strings.TrimSpace(e)
		if trimmed != "" {
			normalizedEvents = append(normalizedEvents, trimmed)
		}
	}

	if name == "" {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "warn",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  "hook registration missing name",
		})
		return
	}

	record.HookNames = append(record.HookNames, name)
	r.Hooks = append(r.Hooks, PluginHookRegistrationEntry{
		PluginID: record.ID,
		Events:   normalizedEvents,
		Name:     name,
		Source:   record.Source,
	})
}

// RegisterGatewayMethod 注册网关方法
// 对应 TS: registry.ts registerGatewayMethod
func (r *PluginRegistry) RegisterGatewayMethod(record *PluginRecord, method string, handler GatewayRequestHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	trimmed := strings.TrimSpace(method)
	if trimmed == "" {
		return
	}
	if r.coreGatewayMethods[trimmed] || r.GatewayHandlers[trimmed] != nil {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "error",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  fmt.Sprintf("gateway method already registered: %s", trimmed),
		})
		return
	}
	r.GatewayHandlers[trimmed] = handler
	record.GatewayMethods = append(record.GatewayMethods, trimmed)
}

// RegisterHttpHandler 注册 HTTP 处理函数
func (r *PluginRegistry) RegisterHttpHandler(record *PluginRecord, handler PluginHttpHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record.HttpHandlers++
	r.HttpHandlers = append(r.HttpHandlers, PluginHttpRegistration{
		PluginID: record.ID,
		Handler:  handler,
		Source:   record.Source,
	})
}

// RegisterHttpRoute 注册 HTTP 路由
func (r *PluginRegistry) RegisterHttpRoute(record *PluginRecord, path string, handler PluginHttpRouteHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalized := NormalizePluginHttpPath(path, "")
	if normalized == "" {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "warn",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  "http route registration missing path",
		})
		return
	}
	for _, entry := range r.HttpRoutes {
		if entry.Path == normalized {
			r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
				Level:    "error",
				PluginID: record.ID,
				Source:   record.Source,
				Message:  fmt.Sprintf("http route already registered: %s", normalized),
			})
			return
		}
	}
	record.HttpHandlers++
	r.HttpRoutes = append(r.HttpRoutes, PluginHttpRouteRegistration{
		PluginID: record.ID,
		Path:     normalized,
		Handler:  handler,
		Source:   record.Source,
	})
}

// RegisterChannel 注册渠道
func (r *PluginRegistry) RegisterChannel(record *PluginRecord, plugin ChannelPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := strings.TrimSpace(plugin.ID)
	if id == "" {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "error",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  "channel registration missing id",
		})
		return
	}
	record.ChannelIDs = append(record.ChannelIDs, id)
	r.Channels = append(r.Channels, PluginChannelRegistrationEntry{
		PluginID: record.ID,
		Plugin:   plugin,
		Source:   record.Source,
	})
}

// RegisterProvider 注册提供商
func (r *PluginRegistry) RegisterProvider(record *PluginRecord, provider ProviderPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := strings.TrimSpace(provider.ID)
	if id == "" {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "error",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  "provider registration missing id",
		})
		return
	}
	for _, entry := range r.Providers {
		if entry.Provider.ID == id {
			r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
				Level:    "error",
				PluginID: record.ID,
				Source:   record.Source,
				Message:  fmt.Sprintf("provider already registered: %s (%s)", id, entry.PluginID),
			})
			return
		}
	}
	record.ProviderIDs = append(record.ProviderIDs, id)
	r.Providers = append(r.Providers, PluginProviderRegistration{
		PluginID: record.ID,
		Provider: provider,
		Source:   record.Source,
	})
}

// RegisterCli 注册 CLI 命令
func (r *PluginRegistry) RegisterCli(record *PluginRecord, registrar PluginCliRegistrar, commands []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalized := make([]string, 0, len(commands))
	for _, cmd := range commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	record.CliCommands = append(record.CliCommands, normalized...)
	r.CliRegistrars = append(r.CliRegistrars, PluginCliRegistration{
		PluginID: record.ID,
		Register: registrar,
		Commands: normalized,
		Source:   record.Source,
	})
}

// RegisterService 注册服务
func (r *PluginRegistry) RegisterService(record *PluginRecord, service PluginService) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := strings.TrimSpace(service.ID)
	if id == "" {
		return
	}
	record.Services = append(record.Services, id)
	r.ServiceEntries = append(r.ServiceEntries, PluginServiceRegistration{
		PluginID: record.ID,
		Service:  service,
		Source:   record.Source,
	})
}

// RegisterCommandEntry 注册自定义命令
func (r *PluginRegistry) RegisterCommandEntry(record *PluginRecord, command PluginCommandDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := strings.TrimSpace(command.Name)
	if name == "" {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "error",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  "command registration missing name",
		})
		return
	}

	result := RegisterPluginCommand(record.ID, command)
	if !result.OK {
		r.Diagnostics = append(r.Diagnostics, PluginDiagnostic{
			Level:    "error",
			PluginID: record.ID,
			Source:   record.Source,
			Message:  fmt.Sprintf("command registration failed: %s", result.Error),
		})
		return
	}

	record.Commands = append(record.Commands, name)
	r.CommandEntries = append(r.CommandEntries, PluginCommandRegistration{
		PluginID: record.ID,
		Command:  command,
		Source:   record.Source,
	})
}

// RegisterTypedHook 注册类型化钩子
func (r *PluginRegistry) RegisterTypedHook(record *PluginRecord, hookName PluginHookName, handler interface{}, priority *int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record.HookCount++
	r.TypedHooks = append(r.TypedHooks, PluginHookRegistration{
		PluginID: record.ID,
		HookName: hookName,
		Handler:  handler,
		Priority: priority,
		Source:   record.Source,
	})
}

// CreateAPI 为插件创建 API 实例
// 对应 TS: registry.ts createApi
func (r *PluginRegistry) CreateAPI(record *PluginRecord, pluginConfig map[string]interface{}) *PluginAPI {
	return &PluginAPI{
		ID:           record.ID,
		Name:         record.Name,
		Version:      record.Version,
		Description:  record.Description,
		Source:       record.Source,
		PluginConfig: pluginConfig,
		Runtime:      r.runtime,
		Logger:       r.normalizeLogger(),
		RegisterTool: func(factory PluginToolFactory, opts *PluginToolOptions) {
			r.RegisterTool(record, factory, opts)
		},
		RegisterHook: func(events []string, handler interface{}, opts *PluginHookOptions) {
			name := ""
			if opts != nil {
				name = opts.Name
			}
			r.RegisterHook(record, events, name, record.Source)
		},
		RegisterHttpHandler: func(handler PluginHttpHandler) {
			r.RegisterHttpHandler(record, handler)
		},
		RegisterHttpRoute: func(path string, handler PluginHttpRouteHandler) {
			r.RegisterHttpRoute(record, path, handler)
		},
		RegisterChannel: func(reg PluginChannelRegistration) {
			r.RegisterChannel(record, reg.Plugin)
		},
		RegisterProvider: func(provider ProviderPlugin) {
			r.RegisterProvider(record, provider)
		},
		RegisterGatewayMethod: func(method string, handler GatewayRequestHandler) {
			r.RegisterGatewayMethod(record, method, handler)
		},
		RegisterCli: func(registrar PluginCliRegistrar, commands []string) {
			r.RegisterCli(record, registrar, commands)
		},
		RegisterService: func(service PluginService) {
			r.RegisterService(record, service)
		},
		RegisterCommand: func(command PluginCommandDefinition) {
			r.RegisterCommandEntry(record, command)
		},
		RegisterTypedHook: func(hookName PluginHookName, handler interface{}, priority *int) {
			r.RegisterTypedHook(record, hookName, handler, priority)
		},
		ResolvePath: func(input string) string {
			return resolvePluginPath(input, record.Source)
		},
	}
}

func (r *PluginRegistry) normalizeLogger() PluginLogger {
	return PluginLogger{
		Info:  r.logger.Info,
		Warn:  r.logger.Warn,
		Error: r.logger.Error,
		Debug: r.logger.Debug,
	}
}

// resolvePluginPath 解析插件路径（相对于插件源目录）
func resolvePluginPath(input, source string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "~") {
		// 留给上层处理 ~ 展开
		return trimmed
	}
	baseDir := filepath.Dir(source)
	return filepath.Join(baseDir, trimmed)
}

// GetChannelNames 返回所有已注册的插件频道 ID 列表。
func (r *PluginRegistry) GetChannelNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.Channels))
	for _, entry := range r.Channels {
		if entry.Plugin.ID != "" {
			names = append(names, entry.Plugin.ID)
		}
	}
	return names
}

func init() {
	// 初始化日志（便于调试）
	_ = slog.Default()
}
