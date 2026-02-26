package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// Hook 安装管线 — 从本地路径/npm/archive 安装钩子包
// 对应 TS: hooks/install.ts (500L)
// ============================================================================

// InstallHooksResult 安装结果
// 对应 TS: install.ts InstallHooksResult
type InstallHooksResult struct {
	OK         bool     `json:"ok"`
	HookPackID string   `json:"hookPackId,omitempty"`
	Hooks      []string `json:"hooks,omitempty"`
	TargetDir  string   `json:"targetDir,omitempty"`
	Version    string   `json:"version,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// HookInstallLogger 安装日志接口
type HookInstallLogger struct {
	Info func(msg string)
	Warn func(msg string)
}

func installLogInfo(logger *HookInstallLogger, msg string) {
	if logger != nil && logger.Info != nil {
		logger.Info(msg)
	}
}

// hookPackageManifest package.json 中的钩子声明
type hookPackageManifest struct {
	Name         string            `json:"name,omitempty"`
	Version      string            `json:"version,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	OpenAcosmi   *struct {
		Hooks []string `json:"hooks,omitempty"`
	} `json:"openacosmi,omitempty"`
}

// --- 路径安全 ---

// unscopedPackageName 取 npm scoped package 的末尾名称
func unscopedPackageName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return trimmed
	}
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

// safeDirName 将路径分隔符替换为 __
func safeDirName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return trimmed
	}
	trimmed = strings.ReplaceAll(trimmed, "/", "__")
	trimmed = strings.ReplaceAll(trimmed, "\\", "__")
	return trimmed
}

// ValidateHookID 校验钩子 ID
// 对应 TS: install.ts validateHookId
func ValidateHookID(hookID string) error {
	if hookID == "" {
		return fmt.Errorf("invalid hook name: missing")
	}
	if hookID == "." || hookID == ".." {
		return fmt.Errorf("invalid hook name: reserved path segment")
	}
	if strings.Contains(hookID, "/") || strings.Contains(hookID, "\\") {
		return fmt.Errorf("invalid hook name: path separators not allowed")
	}
	return nil
}

// resolveSafeInstallDir 解析安全的安装目标目录（防止路径穿越）
// 对应 TS: install.ts resolveSafeInstallDir
func resolveSafeInstallDir(hooksDir, hookID string) (string, error) {
	targetDir := filepath.Join(hooksDir, safeDirName(hookID))
	resolvedBase, _ := filepath.Abs(hooksDir)
	resolvedTarget, _ := filepath.Abs(targetDir)
	rel, err := filepath.Rel(resolvedBase, resolvedTarget)
	if err != nil || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid hook name: path traversal detected")
	}
	return targetDir, nil
}

// ResolveHookInstallDir 解析钩子安装目录
// 对应 TS: install.ts resolveHookInstallDir
func ResolveHookInstallDir(hookID, hooksDir string) (string, error) {
	if hooksDir == "" {
		hooksDir = filepath.Join(ResolveConfigDir(), "hooks")
	} else {
		hooksDir = resolveUserPath(hooksDir)
	}
	if err := ValidateHookID(hookID); err != nil {
		return "", err
	}
	return resolveSafeInstallDir(hooksDir, hookID)
}

// --- 目录验证 ---

// validateHookDir 验证钩子目录包含 HOOK.md 和 handler 文件
func validateHookDir(hookDir string) error {
	hookMdPath := filepath.Join(hookDir, "HOOK.md")
	if _, err := os.Stat(hookMdPath); err != nil {
		return fmt.Errorf("HOOK.md missing in %s", hookDir)
	}
	candidates := []string{"handler.go", "handler.ts", "handler.js", "index.ts", "index.js"}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(hookDir, c)); err == nil {
			return nil
		}
	}
	return fmt.Errorf("handler.ts/handler.js/index.ts/index.js missing in %s", hookDir)
}

// resolveHookNameFromDir 从 HOOK.md frontmatter 中读取钩子名称
func resolveHookNameFromDir(hookDir string) (string, error) {
	hookMdPath := filepath.Join(hookDir, "HOOK.md")
	data, err := os.ReadFile(hookMdPath)
	if err != nil {
		return "", fmt.Errorf("HOOK.md missing in %s", hookDir)
	}
	fm := ParseFrontmatter(string(data))
	if name := fm["name"]; name != "" {
		return name, nil
	}
	return filepath.Base(hookDir), nil
}

// --- 安装管线 ---

// InstallHookFromDir 从单个钩子目录安装
// 对应 TS: install.ts installHookFromDir
func InstallHookFromDir(hookDir, hooksDir, mode string, dryRun bool, expectedID string, logger *HookInstallLogger) InstallHooksResult {
	if err := validateHookDir(hookDir); err != nil {
		return InstallHooksResult{OK: false, Error: err.Error()}
	}
	hookName, err := resolveHookNameFromDir(hookDir)
	if err != nil {
		return InstallHooksResult{OK: false, Error: err.Error()}
	}
	if err := ValidateHookID(hookName); err != nil {
		return InstallHooksResult{OK: false, Error: err.Error()}
	}
	if expectedID != "" && expectedID != hookName {
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("hook id mismatch: expected %s, got %s", expectedID, hookName)}
	}

	if hooksDir == "" {
		hooksDir = filepath.Join(ResolveConfigDir(), "hooks")
	} else {
		hooksDir = resolveUserPath(hooksDir)
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("mkdir hooks: %v", err)}
	}

	targetDir, err := resolveSafeInstallDir(hooksDir, hookName)
	if err != nil {
		return InstallHooksResult{OK: false, Error: err.Error()}
	}
	if mode == "" {
		mode = "install"
	}
	if mode == "install" {
		if _, err := os.Stat(targetDir); err == nil {
			return InstallHooksResult{OK: false, Error: fmt.Sprintf("hook already exists: %s (delete it first)", targetDir)}
		}
	}

	if dryRun {
		return InstallHooksResult{OK: true, HookPackID: hookName, Hooks: []string{hookName}, TargetDir: targetDir}
	}

	installLogInfo(logger, fmt.Sprintf("Installing to %s…", targetDir))
	backupDir := ""
	if mode == "update" {
		if _, err := os.Stat(targetDir); err == nil {
			backupDir = fmt.Sprintf("%s.backup-%d", targetDir, time.Now().UnixMilli())
			if err := os.Rename(targetDir, backupDir); err != nil {
				return InstallHooksResult{OK: false, Error: fmt.Sprintf("backup failed: %v", err)}
			}
		}
	}

	if err := copyDir(hookDir, targetDir); err != nil {
		if backupDir != "" {
			_ = os.RemoveAll(targetDir)
			_ = os.Rename(backupDir, targetDir)
		}
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("failed to copy hook: %v", err)}
	}

	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}

	return InstallHooksResult{OK: true, HookPackID: hookName, Hooks: []string{hookName}, TargetDir: targetDir}
}

// InstallHookPackageFromDir 从包含 package.json 的钩子包目录安装
// 对应 TS: install.ts installHookPackageFromDir
func InstallHookPackageFromDir(packageDir, hooksDir, mode string, timeoutMs int, dryRun bool, expectedID string, logger *HookInstallLogger) InstallHooksResult {
	manifestPath := filepath.Join(packageDir, "package.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return InstallHooksResult{OK: false, Error: "package.json missing"}
	}

	var manifest hookPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("invalid package.json: %v", err)}
	}

	if manifest.OpenAcosmi == nil || len(manifest.OpenAcosmi.Hooks) == 0 {
		return InstallHooksResult{OK: false, Error: "package.json missing openacosmi.hooks"}
	}
	var hookEntries []string
	for _, e := range manifest.OpenAcosmi.Hooks {
		trimmed := strings.TrimSpace(e)
		if trimmed != "" {
			hookEntries = append(hookEntries, trimmed)
		}
	}
	if len(hookEntries) == 0 {
		return InstallHooksResult{OK: false, Error: "package.json openacosmi.hooks is empty"}
	}

	pkgName := strings.TrimSpace(manifest.Name)
	hookPackID := unscopedPackageName(pkgName)
	if hookPackID == "" {
		hookPackID = filepath.Base(packageDir)
	}
	if err := ValidateHookID(hookPackID); err != nil {
		return InstallHooksResult{OK: false, Error: err.Error()}
	}
	if expectedID != "" && expectedID != hookPackID {
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("hook pack id mismatch: expected %s, got %s", expectedID, hookPackID)}
	}

	if hooksDir == "" {
		hooksDir = filepath.Join(ResolveConfigDir(), "hooks")
	} else {
		hooksDir = resolveUserPath(hooksDir)
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("mkdir hooks: %v", err)}
	}

	targetDir, err := resolveSafeInstallDir(hooksDir, hookPackID)
	if err != nil {
		return InstallHooksResult{OK: false, Error: err.Error()}
	}
	if mode == "" {
		mode = "install"
	}
	if mode == "install" {
		if _, err := os.Stat(targetDir); err == nil {
			return InstallHooksResult{OK: false, Error: fmt.Sprintf("hook pack already exists: %s (delete it first)", targetDir)}
		}
	}

	// Validate and resolve hook names
	resolvedHooks := make([]string, 0, len(hookEntries))
	for _, entry := range hookEntries {
		hookDir := filepath.Join(packageDir, entry)
		if err := validateHookDir(hookDir); err != nil {
			return InstallHooksResult{OK: false, Error: err.Error()}
		}
		hookName, err := resolveHookNameFromDir(hookDir)
		if err != nil {
			return InstallHooksResult{OK: false, Error: err.Error()}
		}
		resolvedHooks = append(resolvedHooks, hookName)
	}

	if dryRun {
		return InstallHooksResult{
			OK:         true,
			HookPackID: hookPackID,
			Hooks:      resolvedHooks,
			TargetDir:  targetDir,
			Version:    manifest.Version,
		}
	}

	installLogInfo(logger, fmt.Sprintf("Installing to %s…", targetDir))
	backupDir := ""
	if mode == "update" {
		if _, err := os.Stat(targetDir); err == nil {
			backupDir = fmt.Sprintf("%s.backup-%d", targetDir, time.Now().UnixMilli())
			if err := os.Rename(targetDir, backupDir); err != nil {
				return InstallHooksResult{OK: false, Error: fmt.Sprintf("backup failed: %v", err)}
			}
		}
	}

	if err := copyDir(packageDir, targetDir); err != nil {
		if backupDir != "" {
			_ = os.RemoveAll(targetDir)
			_ = os.Rename(backupDir, targetDir)
		}
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("failed to copy hook pack: %v", err)}
	}

	// npm install if dependencies exist
	if len(manifest.Dependencies) > 0 {
		installLogInfo(logger, "Installing hook pack dependencies…")
		timeout := time.Duration(timeoutMs) * time.Millisecond
		if timeout < 300*time.Second {
			timeout = 300 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "npm", "install", "--omit=dev", "--silent")
		cmd.Dir = targetDir
		cmd.Env = append(os.Environ(), "COREPACK_ENABLE_DOWNLOAD_PROMPT=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			if backupDir != "" {
				_ = os.RemoveAll(targetDir)
				_ = os.Rename(backupDir, targetDir)
			}
			return InstallHooksResult{OK: false, Error: fmt.Sprintf("npm install failed: %s", strings.TrimSpace(string(out)))}
		}
	}

	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}

	return InstallHooksResult{
		OK:         true,
		HookPackID: hookPackID,
		Hooks:      resolvedHooks,
		TargetDir:  targetDir,
		Version:    manifest.Version,
	}
}

// InstallHooksFromPath 从本地路径安装钩子
// 对应 TS: install.ts installHooksFromPath
func InstallHooksFromPath(hookPath, hooksDir, mode string, timeoutMs int, dryRun bool, expectedID string, logger *HookInstallLogger) InstallHooksResult {
	resolved := resolveUserPath(hookPath)
	info, err := os.Stat(resolved)
	if err != nil {
		return InstallHooksResult{OK: false, Error: fmt.Sprintf("path not found: %s", resolved)}
	}

	if info.IsDir() {
		// Check if it's a package (has package.json)
		manifestPath := filepath.Join(resolved, "package.json")
		if _, err := os.Stat(manifestPath); err == nil {
			return InstallHookPackageFromDir(resolved, hooksDir, mode, timeoutMs, dryRun, expectedID, logger)
		}
		return InstallHookFromDir(resolved, hooksDir, mode, dryRun, expectedID, logger)
	}

	return InstallHooksResult{OK: false, Error: fmt.Sprintf("unsupported hook file: %s", resolved)}
}

// --- 辅助函数 ---

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}
