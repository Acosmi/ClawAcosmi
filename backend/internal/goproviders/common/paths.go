// common/paths.go — Auth 存储路径解析工具
// 对应 TS 文件: src/agents/auth-profiles/paths.ts
package common

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// resolveUserPath 将路径中的 ~ 展开为用户主目录。
// 对应 TS: resolveUserPath()
func resolveUserPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[1:])
	}
	return p
}

// resolveOpenClawAgentDir 解析 OpenAcosmi 代理目录。
// 优先使用环境变量 OPENACOSMI_AGENT_DIR / OPENCLAW_AGENT_DIR / PI_CODING_AGENT_DIR，
// 否则使用 OPENACOSMI_STATE_DIR/agents/main/agent（默认 ~/.openacosmi/state）。
func resolveOpenClawAgentDir() string {
	if dir := os.Getenv("OPENACOSMI_AGENT_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("OPENCLAW_AGENT_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return dir
	}
	stateDir := os.Getenv("OPENACOSMI_STATE_DIR")
	if stateDir == "" {
		stateDir = os.Getenv("OPENCLAW_STATE_DIR")
	}
	if stateDir == "" {
		home, _ := os.UserHomeDir()
		stateDir = filepath.Join(home, ".openacosmi", "state")
	}
	return filepath.Join(stateDir, "agents", "main", "agent")
}

// ResolveAuthStorePath 解析 Auth Profile 存储文件的完整路径。
// 对应 TS: resolveAuthStorePath()
func ResolveAuthStorePath(agentDir string) string {
	if agentDir == "" {
		agentDir = resolveOpenClawAgentDir()
	}
	resolved := resolveUserPath(agentDir)
	return filepath.Join(resolved, AuthProfileFilename)
}

// ResolveLegacyAuthStorePath 解析旧版 Auth 存储文件路径。
// 对应 TS: resolveLegacyAuthStorePath()
func ResolveLegacyAuthStorePath(agentDir string) string {
	if agentDir == "" {
		agentDir = resolveOpenClawAgentDir()
	}
	resolved := resolveUserPath(agentDir)
	return filepath.Join(resolved, LegacyAuthFilename)
}

// ResolveAuthStorePathForDisplay 解析用于显示的 Auth 存储路径。
// 对应 TS: resolveAuthStorePathForDisplay()
func ResolveAuthStorePathForDisplay(agentDir string) string {
	pathname := ResolveAuthStorePath(agentDir)
	if strings.HasPrefix(pathname, "~") {
		return pathname
	}
	return resolveUserPath(pathname)
}

// EnsureAuthStoreFile 确保 Auth 存储文件存在，若不存在则创建默认空存储。
// 对应 TS: ensureAuthStoreFile()
func EnsureAuthStoreFile(pathname string) error {
	if _, err := os.Stat(pathname); err == nil {
		return nil // 文件已存在
	}

	payload := types.AuthProfileStore{
		Version:  AuthStoreVersion,
		Profiles: make(map[string]map[string]interface{}),
	}

	dir := filepath.Dir(pathname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(pathname, data, 0o644)
}
