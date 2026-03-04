package plugins

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// --- Plugin Loader ---
// 对应 TS: plugins/loader.ts

// PluginLoadOptions 加载选项
type PluginLoadOptions struct {
	Config              *types.OpenAcosmiConfig
	WorkspaceDir        string
	Logger              PluginLogger
	CoreGatewayHandlers map[string]GatewayRequestHandler
	Cache               bool
	Mode                string // "full"|"validate"
}

// PluginLoadResult 加载结果
type PluginLoadResult struct {
	Registry    *PluginRegistry
	Diagnostics []PluginDiagnostic
	CacheKey    string
	CacheHit    bool
}

var (
	registryCacheMu sync.RWMutex
	registryCache   = make(map[string]*PluginRegistry)
)

// LoadOpenAcosmiPlugins 加载并注册所有 OpenAcosmi 插件
// 对应 TS: loader.ts loadOpenAcosmiPlugins
func LoadOpenAcosmiPlugins(options PluginLoadOptions) *PluginLoadResult {
	logger := options.Logger
	if logger.Info == nil {
		logger = defaultPluginLogger()
	}

	cfg := options.Config
	if cfg == nil {
		cfg = &types.OpenAcosmiConfig{}
	}

	// 规范化插件配置（使用 config_state.go 中的 NormalizePluginsConfig）
	normalized := normalizeFromTypedConfig(cfg)

	// 构建缓存 key
	cacheKey := buildLoaderCacheKey(options.WorkspaceDir, normalized)

	// 缓存命中检查
	if options.Cache {
		registryCacheMu.RLock()
		if cached, ok := registryCache[cacheKey]; ok {
			registryCacheMu.RUnlock()
			return &PluginLoadResult{
				Registry:    cached,
				Diagnostics: cached.Diagnostics,
				CacheKey:    cacheKey,
				CacheHit:    true,
			}
		}
		registryCacheMu.RUnlock()
	}

	// 创建运行时和注册表
	slogger := slog.Default()
	rt := NewDefaultPluginRuntime(slogger)
	registry := NewPluginRegistry(rt, logger, options.CoreGatewayHandlers)

	// 发现插件
	discovery := DiscoverPlugins(options.WorkspaceDir, extractLoadPaths(cfg))

	// 合并诊断
	for _, diag := range discovery.Diagnostics {
		registry.PushDiagnostic(diag)
	}

	// 处理每个候选插件
	for _, candidate := range discovery.Candidates {
		loadSingleCandidate(registry, candidate, normalized, cfg, options, logger)
	}

	// 写入缓存
	if options.Cache {
		registryCacheMu.Lock()
		registryCache[cacheKey] = registry
		registryCacheMu.Unlock()
	}

	// 设置活动注册表
	SetActivePluginRegistry(registry, cacheKey)

	return &PluginLoadResult{
		Registry:    registry,
		Diagnostics: registry.Diagnostics,
		CacheKey:    cacheKey,
		CacheHit:    false,
	}
}

func loadSingleCandidate(
	registry *PluginRegistry,
	candidate PluginCandidate,
	normalized NormalizedPluginsConfig,
	cfg *types.OpenAcosmiConfig,
	options PluginLoadOptions,
	logger PluginLogger,
) {
	// 加载 manifest
	manifestResult := LoadPluginManifest(candidate.RootDir)
	if !manifestResult.OK {
		registry.PushDiagnostic(PluginDiagnostic{
			PluginID: candidate.IDHint,
			Level:    "warn",
			Message:  "Failed to load manifest: " + manifestResult.Error,
			Source:   candidate.Source,
		})
		return
	}

	manifest := manifestResult.Manifest
	pluginID := manifest.ID
	if pluginID == "" {
		pluginID = candidate.IDHint
	}
	if pluginID == "" {
		return
	}

	// 检查是否启用
	enableState := resolveEnableFromNormalized(pluginID, normalized)
	if !enableState.Enabled {
		return
	}

	// 读取插件配置
	pluginConfig := resolvePluginEntryConfig(pluginID, cfg)

	// 验证配置 schema（Go 简化版，JSON Schema 动态验证需外部库）
	if manifest.ConfigSchema != nil && options.Mode == "validate" {
		if logger.Info != nil {
			logger.Info("Plugin " + pluginID + " has config schema (validation deferred)")
		}
	}

	// 创建 PluginRecord
	record := &PluginRecord{
		ID:           pluginID,
		Name:         manifest.Name,
		Description:  manifest.Description,
		Version:      candidate.PackageVersion,
		Source:       candidate.Source,
		Origin:       candidate.Origin,
		WorkspaceDir: options.WorkspaceDir,
		Enabled:      true,
		ConfigSchema: manifest.ConfigSchema != nil,
	}

	registry.Plugins = append(registry.Plugins, *record)

	// Go 内部插件通过编译时注册，动态 JS 模块加载不适用
	if registrar, ok := builtinPluginRegistrars[pluginID]; ok {
		api := registry.CreateAPI(record, pluginConfig)
		if err := registrar(api); err != nil {
			registry.PushDiagnostic(PluginDiagnostic{
				PluginID: pluginID,
				Level:    "error",
				Message:  "Plugin registration failed: " + err.Error(),
				Source:   candidate.Source,
			})
		}
	}
}

// --- 内置插件注册 ---

// BuiltinPluginRegistrar 内置插件注册函数
type BuiltinPluginRegistrar func(api *PluginAPI) error

var builtinPluginRegistrars = make(map[string]BuiltinPluginRegistrar)

// RegisterBuiltinPlugin 注册内置插件
// 在 init() 中调用以编译时注册
func RegisterBuiltinPlugin(pluginID string, registrar BuiltinPluginRegistrar) {
	builtinPluginRegistrars[pluginID] = registrar
}

// --- 辅助函数 ---

func defaultPluginLogger() PluginLogger {
	return PluginLogger{
		Info:  func(msg string) { slog.Info(msg) },
		Warn:  func(msg string) { slog.Warn(msg) },
		Error: func(msg string) { slog.Error(msg) },
	}
}

func buildLoaderCacheKey(workspaceDir string, normalized NormalizedPluginsConfig) string {
	data, _ := json.Marshal(struct {
		WorkspaceDir string
		Config       NormalizedPluginsConfig
	}{workspaceDir, normalized})
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8])
}

func extractLoadPaths(cfg *types.OpenAcosmiConfig) []string {
	if cfg == nil || cfg.Plugins == nil || cfg.Plugins.Load == nil {
		return nil
	}
	return cfg.Plugins.Load.Paths
}

func resolvePluginEntryConfig(pluginID string, cfg *types.OpenAcosmiConfig) map[string]interface{} {
	if cfg == nil || cfg.Plugins == nil || cfg.Plugins.Entries == nil {
		return nil
	}
	entry := cfg.Plugins.Entries[pluginID]
	if entry == nil {
		return nil
	}
	return entry.Config
}

// normalizeFromTypedConfig 从 typed config 转换为 NormalizedPluginsConfig
// 适配 config_state.go 的 NormalizePluginsConfig 接受 map[string]interface{} 的签名
func normalizeFromTypedConfig(cfg *types.OpenAcosmiConfig) NormalizedPluginsConfig {
	if cfg == nil || cfg.Plugins == nil {
		return NormalizePluginsConfig(nil)
	}
	// 将 PluginsConfig 序列化为 map
	data, err := json.Marshal(cfg.Plugins)
	if err != nil {
		return NormalizePluginsConfig(nil)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return NormalizePluginsConfig(nil)
	}
	return NormalizePluginsConfig(raw)
}

// resolveEnableFromNormalized 从规范化配置解析启用状态
func resolveEnableFromNormalized(pluginID string, normalized NormalizedPluginsConfig) PluginEnableState {
	return resolvePluginEnableState(pluginID, normalized)
}

// PluginEnableState 插件启用状态
type PluginEnableState struct {
	Enabled bool
	Reason  string
}

// resolvePluginEnableState 解析插件启用状态
func resolvePluginEnableState(pluginID string, normalized NormalizedPluginsConfig) PluginEnableState {
	if !normalized.Enabled {
		return PluginEnableState{Enabled: false, Reason: "plugins globally disabled"}
	}

	// Deny 列表
	for _, pattern := range normalized.Deny {
		if matchPluginPattern(pluginID, pattern) {
			return PluginEnableState{Enabled: false, Reason: "denied by pattern: " + pattern}
		}
	}

	// Allow 列表（如果非空，需匹配）
	if len(normalized.Allow) > 0 {
		for _, pattern := range normalized.Allow {
			if matchPluginPattern(pluginID, pattern) {
				return PluginEnableState{Enabled: true}
			}
		}
		return PluginEnableState{Enabled: false, Reason: "not in allow list"}
	}

	return PluginEnableState{Enabled: true}
}

func matchPluginPattern(pluginID, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(pluginID) >= len(prefix) && pluginID[:len(prefix)] == prefix
	}
	return pluginID == pattern
}

// ValidatePluginConfig 验证插件配置
// 对应 TS: loader.ts validatePluginConfig
// 注：完整 JSON Schema 验证需外部库（如 gojsonschema），暂用简化版
func ValidatePluginConfig(schema map[string]interface{}, value interface{}) (bool, []string) {
	if schema == nil {
		return true, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false, []string{"invalid JSON: " + err.Error()}
	}
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false, []string{"invalid JSON structure: " + err.Error()}
	}
	return true, nil
}

// PluginDefinitionExport 插件定义导出
// 对应 TS: loader.ts resolvePluginModuleExport
// Go 中使用编译时注册，留此接口供未来 WASM 插件使用
type PluginDefinitionExport struct {
	ID          string
	Name        string
	Description string
	Version     string
	Register    func(api *PluginAPI) error
}

// ClearRegistryCache 清空注册表缓存
func ClearRegistryCache() {
	registryCacheMu.Lock()
	registryCache = make(map[string]*PluginRegistry)
	registryCacheMu.Unlock()
}
