package plugins

// PluginAPI 插件 API 接口 — 插件通过此接口注册功能
// 对应 TS: types.ts OpenAcosmiPluginApi
type PluginAPI struct {
	ID           string
	Name         string
	Version      string
	Description  string
	Source       string
	PluginConfig map[string]interface{}
	Runtime      PluginRuntime
	Logger       PluginLogger

	// 注册回调 — 由 Registry 在 CreateAPI 中注入
	RegisterTool          func(factory PluginToolFactory, opts *PluginToolOptions)
	RegisterHook          func(events []string, handler interface{}, opts *PluginHookOptions)
	RegisterHttpHandler   func(handler PluginHttpHandler)
	RegisterHttpRoute     func(path string, handler PluginHttpRouteHandler)
	RegisterChannel       func(plugin PluginChannelRegistration)
	RegisterProvider      func(provider ProviderPlugin)
	RegisterGatewayMethod func(method string, handler GatewayRequestHandler)
	RegisterCli           func(registrar PluginCliRegistrar, commands []string)
	RegisterService       func(service PluginService)
	RegisterCommand       func(command PluginCommandDefinition)
	RegisterTypedHook     func(hookName PluginHookName, handler interface{}, priority *int)
	ResolvePath           func(input string) string
}

// PluginToolFactory 工具工厂函数
type PluginToolFactory func(ctx PluginToolContext) interface{}

// PluginHttpHandler HTTP 处理函数
type PluginHttpHandler func(req interface{}, res interface{}) bool

// PluginHttpRouteHandler HTTP 路由处理函数
type PluginHttpRouteHandler func(req interface{}, res interface{})

// PluginCliRegistrar CLI 注册函数
type PluginCliRegistrar func(ctx PluginCliContext) error

// PluginCliContext CLI 注册上下文
type PluginCliContext struct {
	Logger       PluginLogger
	WorkspaceDir string
}

// GatewayRequestHandler 网关请求处理函数
type GatewayRequestHandler func(params map[string]interface{}) (interface{}, error)

// PluginChannelRegistration 渠道注册
type PluginChannelRegistration struct {
	PluginID string
	Plugin   ChannelPlugin
}

// ChannelPlugin 渠道插件接口
type ChannelPlugin struct {
	ID string
}

// PluginRuntime 插件运行时接口
// 对应 TS: runtime/types.ts PluginRuntime
// 注意：TS 版有 100+ 函数引用，Go 版简化为接口组合
type PluginRuntime interface {
	Version() string
	GetLogger(bindings map[string]interface{}) PluginLogger
}

// NullPluginRuntime 空运行时实现（占位）
type NullPluginRuntime struct {
	VersionStr string
}

func (n *NullPluginRuntime) Version() string { return n.VersionStr }
func (n *NullPluginRuntime) GetLogger(_ map[string]interface{}) PluginLogger {
	return PluginLogger{
		Info:  func(_ string) {},
		Warn:  func(_ string) {},
		Error: func(_ string) {},
	}
}
