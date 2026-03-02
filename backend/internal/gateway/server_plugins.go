package gateway

// TS 对照: src/gateway/server-plugins.ts (50L)
// Gateway 插件 HTTP 层 — 加载插件并合并 gateway 方法列表。

import (
	"log/slog"

	"github.com/openacosmi/claw-acismi/internal/plugins"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// PluginDiagnostic 插件加载诊断信息。
type PluginDiagnostic struct {
	PluginID string
	Source   string
	Message  string
	Level    string // "error" | "info"
}

// PluginRegistry 插件注册表。
type PluginRegistry struct {
	// GatewayHandlers 插件提供的 gateway 方法处理器。
	GatewayHandlers map[string]interface{}
	// Diagnostics 加载诊断信息。
	Diagnostics []PluginDiagnostic
	// PluginsResult 底层 plugins 包加载结果（供需要完整信息的调用方使用）。
	PluginsResult *plugins.PluginLoadResult
}

// GatewayPluginsConfig 插件加载配置。
type GatewayPluginsConfig struct {
	WorkspaceDir        string
	Logger              *slog.Logger
	CoreGatewayHandlers map[string]interface{}
	BaseMethods         []string
	Config              *types.OpenAcosmiConfig
}

// GatewayPluginsResult 插件加载结果。
type GatewayPluginsResult struct {
	PluginRegistry *PluginRegistry
	GatewayMethods []string
}

// LoadGatewayPlugins 加载 gateway 插件并返回合并后的方法列表。
// TS 对照: server-plugins.ts loadGatewayPlugins (L5-49)
func LoadGatewayPlugins(cfg GatewayPluginsConfig) GatewayPluginsResult {
	// 构建 plugins 包所需的 GatewayRequestHandler map
	coreHandlers := make(map[string]plugins.GatewayRequestHandler)
	// 注意：gateway 的 handler 类型与 plugins 的不同，此处仅传递方法名（用于冲突检测）
	// 真正的 handler 映射在 WS attachGatewayWsHandlers 中完成
	for name := range cfg.CoreGatewayHandlers {
		coreHandlers[name] = func(params map[string]interface{}) (interface{}, error) { return nil, nil }
	}

	// 构建 PluginLogger
	logger := plugins.PluginLogger{
		Info:  func(msg string) { cfg.Logger.Info(msg) },
		Warn:  func(msg string) { cfg.Logger.Warn(msg) },
		Error: func(msg string) { cfg.Logger.Error(msg) },
		Debug: func(msg string) { cfg.Logger.Debug(msg) },
	}
	if cfg.Logger == nil {
		logger = plugins.PluginLogger{
			Info:  func(msg string) {},
			Warn:  func(msg string) {},
			Error: func(msg string) {},
			Debug: func(msg string) {},
		}
	}

	// 调用 plugins 包的完整加载逻辑
	loadResult := plugins.LoadOpenAcosmiPlugins(plugins.PluginLoadOptions{
		Config:              cfg.Config,
		WorkspaceDir:        cfg.WorkspaceDir,
		Logger:              logger,
		CoreGatewayHandlers: coreHandlers,
		Cache:               true,
	})

	// 转换 diagnostics
	var diagnostics []PluginDiagnostic
	if loadResult != nil && loadResult.Registry != nil {
		for _, d := range loadResult.Registry.Diagnostics {
			diagnostics = append(diagnostics, PluginDiagnostic{
				PluginID: d.PluginID,
				Source:   d.Source,
				Message:  d.Message,
				Level:    d.Level,
			})
		}
	}

	// 提取 gateway handlers
	gatewayHandlers := make(map[string]interface{})
	if loadResult != nil && loadResult.Registry != nil {
		for name, handler := range loadResult.Registry.GatewayHandlers {
			gatewayHandlers[name] = handler
		}
	}

	registry := &PluginRegistry{
		GatewayHandlers: gatewayHandlers,
		Diagnostics:     diagnostics,
		PluginsResult:   loadResult,
	}

	// 合并 gateway 方法列表
	methodSet := make(map[string]bool, len(cfg.BaseMethods))
	for _, m := range cfg.BaseMethods {
		methodSet[m] = true
	}
	for m := range registry.GatewayHandlers {
		methodSet[m] = true
	}

	methods := make([]string, 0, len(methodSet))
	for m := range methodSet {
		methods = append(methods, m)
	}

	// 输出诊断信息
	if cfg.Logger != nil {
		for _, diag := range registry.Diagnostics {
			details := ""
			if diag.PluginID != "" {
				details += "plugin=" + diag.PluginID
			}
			if diag.Source != "" {
				if details != "" {
					details += ", "
				}
				details += "source=" + diag.Source
			}
			msg := "[plugins] " + diag.Message
			if details != "" {
				msg += " (" + details + ")"
			}
			if diag.Level == "error" {
				cfg.Logger.Error(msg)
			} else {
				cfg.Logger.Info(msg)
			}
		}
	}

	return GatewayPluginsResult{
		PluginRegistry: registry,
		GatewayMethods: methods,
	}
}
