package plugins

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
)

// --- Plugin Runtime 实现 ---
// 对应 TS: plugins/runtime/index.ts createPluginRuntime
//
// TS 版通过 runtime 对象暴露 100+ 内部函数引用（如 sendMessageSlack、probeDiscord 等），
// 供 JS 动态插件在运行时调用。
//
// Go 版采用不同架构：
// 1. 内部插件通过编译时 import 直接引用其他包，无需运行时中心化桥接
// 2. PluginRuntime 接口仅提供跨越编译边界需要的公共能力（版本、日志、配置发现）
// 3. 未来第三方插件（go-plugin / WASM）使用 gRPC/ABI 边界，由 PluginBridge 处理
//
// 因此 Go 的 DefaultPluginRuntime 仅实现核心 utility 功能，
// 不复制 TS 的 100+ 函数引用。

// DefaultPluginRuntime 默认插件运行时
type DefaultPluginRuntime struct {
	version string
	logger  *slog.Logger
}

// NewDefaultPluginRuntime 创建默认运行时
func NewDefaultPluginRuntime(logger *slog.Logger) *DefaultPluginRuntime {
	if logger == nil {
		logger = slog.Default()
	}
	return &DefaultPluginRuntime{
		version: resolveOpenAcosmiVersion(),
		logger:  logger,
	}
}

// Version 返回 OpenAcosmi 版本
func (r *DefaultPluginRuntime) Version() string {
	return r.version
}

// GetLogger 创建子系统日志
func (r *DefaultPluginRuntime) GetLogger(bindings map[string]interface{}) PluginLogger {
	attrs := make([]any, 0, len(bindings)*2)
	for k, v := range bindings {
		attrs = append(attrs, k, v)
	}
	child := r.logger.With(attrs...)
	return PluginLogger{
		Info:  func(msg string) { child.Info(msg) },
		Warn:  func(msg string) { child.Warn(msg) },
		Error: func(msg string) { child.Error(msg) },
		Debug: func(msg string) { child.Debug(msg) },
	}
}

// --- Version Resolver ---

func resolveOpenAcosmiVersion() string {
	// 1. 环境变量覆盖
	if v := os.Getenv("OPENACOSMI_VERSION"); v != "" {
		return v
	}
	// 2. 读取 package.json（开发模式）
	if v := readVersionFromPackageJSON(); v != "" {
		return v
	}
	// 3. 编译信息
	return resolveGoVersion()
}

func readVersionFromPackageJSON() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(filepath.Dir(exe), "..", "package.json"),
		filepath.Join(filepath.Dir(exe), "package.json"),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var pkg struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Version != "" {
			return pkg.Version
		}
	}
	return ""
}

func resolveGoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "0.0.0-dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "0.0.0-dev"
}
