package daemon

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// versionManagerPatterns 版本管理器路径匹配模式
var versionManagerPatterns = []string{
	"/.nvm/", "/.fnm/", "/.volta/", "/.asdf/", "/.n/",
	"/.nodenv/", "/.nodebrew/", "/nvs/",
}

// IsVersionManagedNodePath 检查 Node 路径是否来自版本管理器
// 对应 TS: runtime-paths.ts isVersionManagedNodePath
func IsVersionManagedNodePath(execPath string, platform string) bool {
	if platform == "" {
		platform = runtime.GOOS
	}
	lower := strings.ToLower(execPath)
	normalized := strings.ReplaceAll(lower, "\\", "/")

	for _, pattern := range versionManagerPatterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}
	return false
}

// IsSystemNodePath 检查 Node 路径是否为系统安装路径
// 对应 TS: runtime-paths.ts isSystemNodePath
func IsSystemNodePath(execPath string, env map[string]string, platform string) bool {
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform == "windows" {
		return false
	}

	systemDirs := ResolveSystemPathDirs(platform)
	dir := filepath.Dir(execPath)
	normalizedDir := strings.ReplaceAll(dir, "\\", "/")
	for _, sysDir := range systemDirs {
		if normalizedDir == sysDir {
			return true
		}
	}
	return false
}

// ResolveSystemNodePath 解析系统安装的 Node 路径
// 对应 TS: runtime-paths.ts resolveSystemNodePath
func ResolveSystemNodePath(env map[string]string, platform string) string {
	if platform == "" {
		platform = runtime.GOOS
	}
	if platform == "windows" {
		return ""
	}

	systemDirs := ResolveSystemPathDirs(platform)
	for _, dir := range systemDirs {
		nodePath := filepath.Join(dir, "node")
		if _, err := exec.LookPath(nodePath); err == nil {
			return nodePath
		}
	}
	return ""
}
