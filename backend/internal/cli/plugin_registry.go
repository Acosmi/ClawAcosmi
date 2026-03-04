package cli

import (
	"log/slog"
	"sync"

	"github.com/Acosmi/ClawAcosmi/internal/plugins"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// 对应 TS src/cli/plugin-registry.ts — 插件注册表 singleton
// 审计修复项: CLI-P2-1 (Plugin registry singleton)

// PluginRegistryDeps 插件注册表加载所需的依赖。
// 由外部注入以避免循环导入。
type PluginRegistryDeps struct {
	// LoadConfig 加载当前配置。
	LoadConfig func() *types.OpenAcosmiConfig
	// ResolveWorkspaceDir 解析工作空间目录。
	ResolveWorkspaceDir func(cfg *types.OpenAcosmiConfig) string
	// CoreGatewayHandlers 核心网关处理器(可为 nil)。
	CoreGatewayHandlers map[string]plugins.GatewayRequestHandler
}

var (
	registryOnce sync.Once
	registryDeps *PluginRegistryDeps
	// globalRegistry 全局插件注册表实例（加载后可读）
	globalRegistry *plugins.PluginRegistry
)

// SetPluginRegistryDeps 设置插件注册表依赖。
// 必须在 EnsurePluginRegistryLoaded 之前调用。
func SetPluginRegistryDeps(deps *PluginRegistryDeps) {
	registryDeps = deps
}

// EnsurePluginRegistryLoaded 确保插件注册表已加载（singleton，仅执行一次）。
// 对应 TS ensurePluginRegistryLoaded()。
func EnsurePluginRegistryLoaded() {
	registryOnce.Do(func() {
		if registryDeps == nil {
			slog.Warn("[cli] plugin registry deps not set, skipping plugin load")
			return
		}
		cfg := registryDeps.LoadConfig()
		workspaceDir := registryDeps.ResolveWorkspaceDir(cfg)
		logger := plugins.PluginLogger{
			Info:  func(msg string) { slog.Info("[plugins] " + msg) },
			Warn:  func(msg string) { slog.Warn("[plugins] " + msg) },
			Error: func(msg string) { slog.Error("[plugins] " + msg) },
			Debug: func(msg string) { slog.Debug("[plugins] " + msg) },
		}
		result := plugins.LoadOpenAcosmiPlugins(plugins.PluginLoadOptions{
			Config:              cfg,
			WorkspaceDir:        workspaceDir,
			Logger:              logger,
			CoreGatewayHandlers: registryDeps.CoreGatewayHandlers,
			Cache:               true,
		})
		globalRegistry = result.Registry
	})
}

// GetGlobalPluginRegistry 返回全局插件注册表（可能为 nil）。
func GetGlobalPluginRegistry() *plugins.PluginRegistry {
	return globalRegistry
}

// ResetPluginRegistryForTest 重置 singleton 状态（仅用于测试）。
func ResetPluginRegistryForTest() {
	registryOnce = sync.Once{}
	registryDeps = nil
	globalRegistry = nil
}
