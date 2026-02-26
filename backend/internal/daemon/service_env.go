package daemon

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ResolveSystemPathDirs 返回平台对应的系统 PATH 目录
// 对应 TS: service-env.ts resolveSystemPathDirs
func ResolveSystemPathDirs(platform string) []string {
	switch platform {
	case "darwin":
		return []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"}
	case "linux":
		return []string{"/usr/local/bin", "/usr/bin", "/bin"}
	default:
		return nil
	}
}

// ResolveLinuxUserBinDirs 解析 Linux 用户 bin 目录
// 包括 npm 全局安装、nvm、fnm、volta 等版本管理器的路径
// 对应 TS: service-env.ts resolveLinuxUserBinDirs
func ResolveLinuxUserBinDirs(home string, env map[string]string) []string {
	if home == "" {
		return nil
	}

	var dirs []string
	add := func(dir string) {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	appendSubdir := func(base, subdir string) string {
		if base == "" {
			return ""
		}
		if strings.HasSuffix(base, "/"+subdir) {
			return base
		}
		return filepath.Join(base, subdir)
	}

	// 环境变量配置的 bin 根目录（覆盖默认值）
	add(env["PNPM_HOME"])
	add(appendSubdir(env["NPM_CONFIG_PREFIX"], "bin"))
	add(appendSubdir(env["BUN_INSTALL"], "bin"))
	add(appendSubdir(env["VOLTA_HOME"], "bin"))
	add(appendSubdir(env["ASDF_DATA_DIR"], "shims"))
	add(appendSubdir(env["NVM_DIR"], "current/bin"))
	add(appendSubdir(env["FNM_DIR"], "current/bin"))

	// 常用用户 bin 目录
	dirs = append(dirs,
		filepath.Join(home, ".local/bin"),      // XDG 标准
		filepath.Join(home, ".npm-global/bin"), // npm 自定义前缀
		filepath.Join(home, "bin"),             // 个人 bin
	)

	// Node 版本管理器目录
	dirs = append(dirs,
		filepath.Join(home, ".nvm/current/bin"),
		filepath.Join(home, ".fnm/current/bin"),
		filepath.Join(home, ".volta/bin"),
		filepath.Join(home, ".asdf/shims"),
		filepath.Join(home, ".local/share/pnpm"),
		filepath.Join(home, ".bun/bin"),
	)

	return dirs
}

// GetMinimalServicePathParts 构建服务最小 PATH 的各部分
// 对应 TS: service-env.ts getMinimalServicePathParts
func GetMinimalServicePathParts(opts MinimalServicePathOptions) []string {
	platform := opts.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform == "windows" {
		return nil
	}

	var parts []string
	seen := make(map[string]bool)
	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		parts = append(parts, dir)
	}

	// 额外目录优先
	for _, dir := range opts.ExtraDirs {
		add(dir)
	}

	// Linux 用户 bin 目录（npm 全局等）
	if platform == "linux" {
		for _, dir := range ResolveLinuxUserBinDirs(opts.Home, opts.Env) {
			add(dir)
		}
	}

	// 系统目录
	for _, dir := range ResolveSystemPathDirs(platform) {
		add(dir)
	}

	return parts
}

// GetMinimalServicePathPartsFromEnv 使用环境变量构建服务最小 PATH
// 对应 TS: service-env.ts getMinimalServicePathPartsFromEnv
func GetMinimalServicePathPartsFromEnv(opts MinimalServicePathOptions) []string {
	if opts.Home == "" && opts.Env != nil {
		opts.Home = opts.Env["HOME"]
	}
	return GetMinimalServicePathParts(opts)
}

// BuildMinimalServicePath 构建服务最小 PATH 字符串
// 对应 TS: service-env.ts buildMinimalServicePath
func BuildMinimalServicePath(opts MinimalServicePathOptions) string {
	platform := opts.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform == "windows" {
		if opts.Env != nil {
			return opts.Env["PATH"]
		}
		return ""
	}
	parts := GetMinimalServicePathPartsFromEnv(opts)
	return strings.Join(parts, ":")
}

// BuildServiceEnvironment 构建 gateway 服务的环境变量
// 对应 TS: service-env.ts buildServiceEnvironment
func BuildServiceEnvironment(env map[string]string, port int, token string) map[string]string {
	profile := env["OPENACOSMI_PROFILE"]
	launchdLabel := ""
	if runtime.GOOS == "darwin" {
		launchdLabel = ResolveGatewayLaunchAgentLabel(profile)
	}
	systemdUnit := ResolveGatewaySystemdServiceName(profile) + ".service"

	result := map[string]string{
		"HOME":                      env["HOME"],
		"PATH":                      BuildMinimalServicePath(MinimalServicePathOptions{Env: env}),
		"OPENACOSMI_PROFILE":        profile,
		"OPENACOSMI_STATE_DIR":      env["OPENACOSMI_STATE_DIR"],
		"OPENACOSMI_CONFIG_PATH":    env["OPENACOSMI_CONFIG_PATH"],
		"OPENACOSMI_GATEWAY_PORT":   intToStr(port),
		"OPENACOSMI_LAUNCHD_LABEL":  launchdLabel,
		"OPENACOSMI_SYSTEMD_UNIT":   systemdUnit,
		"OPENACOSMI_SERVICE_MARKER": GatewayServiceMarker,
		"OPENACOSMI_SERVICE_KIND":   GatewayServiceKind,
	}
	if token != "" {
		result["OPENACOSMI_GATEWAY_TOKEN"] = token
	}
	return result
}

// BuildNodeServiceEnvironment 构建 node 服务的环境变量
// 对应 TS: service-env.ts buildNodeServiceEnvironment
func BuildNodeServiceEnvironment(env map[string]string) map[string]string {
	return map[string]string{
		"HOME":                         env["HOME"],
		"PATH":                         BuildMinimalServicePath(MinimalServicePathOptions{Env: env}),
		"OPENACOSMI_STATE_DIR":         env["OPENACOSMI_STATE_DIR"],
		"OPENACOSMI_CONFIG_PATH":       env["OPENACOSMI_CONFIG_PATH"],
		"OPENACOSMI_LAUNCHD_LABEL":     ResolveNodeLaunchAgentLabel(),
		"OPENACOSMI_SYSTEMD_UNIT":      ResolveNodeSystemdServiceName(),
		"OPENACOSMI_WINDOWS_TASK_NAME": ResolveNodeWindowsTaskName(),
		"OPENACOSMI_TASK_SCRIPT_NAME":  NodeWindowsTaskScriptName,
		"OPENACOSMI_LOG_PREFIX":        "node",
		"OPENACOSMI_SERVICE_MARKER":    NodeServiceMarker,
		"OPENACOSMI_SERVICE_KIND":      NodeServiceKind,
	}
}

// intToStr 将 int 转为字符串
func intToStr(n int) string {
	return strconv.Itoa(n)
}
