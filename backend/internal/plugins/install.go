package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InstallPluginResult 插件安装结果
// 对应 TS: install.ts InstallPluginResult
type InstallPluginResult struct {
	OK           bool     `json:"ok"`
	PluginID     string   `json:"pluginId,omitempty"`
	TargetDir    string   `json:"targetDir,omitempty"`
	ManifestName string   `json:"manifestName,omitempty"`
	Version      string   `json:"version,omitempty"`
	Extensions   []string `json:"extensions,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// InstallLogger 安装日志接口
type InstallLogger struct {
	Info func(message string)
	Warn func(message string)
}

// DefaultInstallLogger 默认空日志
var DefaultInstallLogger = InstallLogger{}

// --- ID / 路径安全校验 ---

// ValidatePluginID 校验插件 ID
// 对应 TS: install.ts validatePluginId
func ValidatePluginID(pluginID string) error {
	if pluginID == "" {
		return fmt.Errorf("invalid plugin name: missing")
	}
	if pluginID == "." || pluginID == ".." {
		return fmt.Errorf("invalid plugin name: reserved path segment")
	}
	if strings.ContainsAny(pluginID, "/\\") {
		return fmt.Errorf("invalid plugin name: path separators not allowed")
	}
	return nil
}

// UnscopedPackageName 去除 npm scope 前缀
// 对应 TS: install.ts unscopedPackageName
func UnscopedPackageName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return trimmed
	}
	if strings.Contains(trimmed, "/") {
		parts := strings.Split(trimmed, "/")
		return parts[len(parts)-1]
	}
	return trimmed
}

// SafeDirName 安全目录名（替换路径分隔符）
func SafeDirName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return trimmed
	}
	r := strings.NewReplacer("/", "__", "\\", "__")
	return r.Replace(trimmed)
}

// IsPathInside 检查 candidatePath 是否在 basePath 内部
func IsPathInside(basePath, candidatePath string) bool {
	base, _ := filepath.Abs(basePath)
	candidate, _ := filepath.Abs(candidatePath)
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
		rel != ".." && !filepath.IsAbs(rel))
}

// ResolvePluginInstallDir 解析插件安装目标目录
// 对应 TS: install.ts resolvePluginInstallDir
func ResolvePluginInstallDir(pluginID string, extensionsDir string) (string, error) {
	base := extensionsDir
	if base == "" {
		base = filepath.Join(resolveConfigDir(), "extensions")
	} else {
		base = resolveUserPath(base)
	}
	if err := ValidatePluginID(pluginID); err != nil {
		return "", err
	}
	return resolveSafeInstallDir(base, pluginID)
}

func resolveSafeInstallDir(extensionsDir, pluginID string) (string, error) {
	targetDir := filepath.Join(extensionsDir, SafeDirName(pluginID))
	resolvedBase, _ := filepath.Abs(extensionsDir)
	resolvedTarget, _ := filepath.Abs(targetDir)
	rel, err := filepath.Rel(resolvedBase, resolvedTarget)
	if err != nil || rel == "" || rel == ".." ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid plugin name: path traversal detected")
	}
	return targetDir, nil
}

// --- 核心安装函数 ---

// InstallPluginFromPackageDir 从解压后的包目录安装插件
// 对应 TS: install.ts installPluginFromPackageDir
func InstallPluginFromPackageDir(params InstallFromPkgDirParams) InstallPluginResult {
	logger := params.Logger
	if logger.Info == nil && logger.Warn == nil {
		logger = DefaultInstallLogger
	}
	mode := params.Mode
	if mode == "" {
		mode = "install"
	}

	// 读取 package.json
	pkg := readPackageJSON(params.PackageDir)
	if pkg == nil {
		return InstallPluginResult{OK: false, Error: "extracted package missing package.json"}
	}

	// 校验 openacosmi.extensions
	extensions := resolvePackageExtensions(pkg)
	if len(extensions) == 0 {
		return InstallPluginResult{OK: false, Error: "package.json missing or empty openacosmi.extensions"}
	}

	pkgName := strings.TrimSpace(pkg.Name)
	pluginID := "plugin"
	if pkgName != "" {
		pluginID = UnscopedPackageName(pkgName)
	}
	if err := ValidatePluginID(pluginID); err != nil {
		return InstallPluginResult{OK: false, Error: err.Error()}
	}
	if params.ExpectedPluginID != "" && params.ExpectedPluginID != pluginID {
		return InstallPluginResult{
			OK:    false,
			Error: fmt.Sprintf("plugin id mismatch: expected %s, got %s", params.ExpectedPluginID, pluginID),
		}
	}

	// 安全扫描（简化：仅日志警告，完整扫描器留 Phase 7）
	packageDir, _ := filepath.Abs(params.PackageDir)
	for _, entry := range extensions {
		resolvedEntry, _ := filepath.Abs(filepath.Join(packageDir, entry))
		if !IsPathInside(packageDir, resolvedEntry) {
			if logger.Warn != nil {
				logger.Warn("extension entry escapes plugin directory: " + entry)
			}
		}
	}

	// 确定安装目录
	extensionsBase := params.ExtensionsDir
	if extensionsBase == "" {
		extensionsBase = filepath.Join(resolveConfigDir(), "extensions")
	} else {
		extensionsBase = resolveUserPath(extensionsBase)
	}
	if err := os.MkdirAll(extensionsBase, 0o755); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to create extensions dir: " + err.Error()}
	}

	targetDir, err := resolveSafeInstallDir(extensionsBase, pluginID)
	if err != nil {
		return InstallPluginResult{OK: false, Error: err.Error()}
	}

	// install 模式下检查已存在
	if mode == "install" {
		if _, err := os.Stat(targetDir); err == nil {
			return InstallPluginResult{
				OK:    false,
				Error: fmt.Sprintf("plugin already exists: %s (delete it first)", targetDir),
			}
		}
	}

	// dry-run
	if params.DryRun {
		return InstallPluginResult{
			OK:           true,
			PluginID:     pluginID,
			TargetDir:    targetDir,
			ManifestName: pkgName,
			Version:      strings.TrimSpace(pkg.Version),
			Extensions:   extensions,
		}
	}

	if logger.Info != nil {
		logger.Info("Installing to " + targetDir + "…")
	}

	// update 模式：备份
	var backupDir string
	if mode == "update" {
		if _, err := os.Stat(targetDir); err == nil {
			backupDir = fmt.Sprintf("%s.backup-%d", targetDir, time.Now().UnixMilli())
			if err := os.Rename(targetDir, backupDir); err != nil {
				return InstallPluginResult{OK: false, Error: "failed to backup: " + err.Error()}
			}
		}
	}

	// 复制
	if err := copyDir(packageDir, targetDir); err != nil {
		if backupDir != "" {
			_ = os.RemoveAll(targetDir)
			_ = os.Rename(backupDir, targetDir)
		}
		return InstallPluginResult{OK: false, Error: "failed to copy plugin: " + err.Error()}
	}

	// npm install 依赖
	if len(pkg.Dependencies) > 0 {
		if logger.Info != nil {
			logger.Info("Installing plugin dependencies…")
		}
		timeoutMs := params.TimeoutMs
		if timeoutMs == 0 {
			timeoutMs = 300_000
		}
		if err := runNpmInstall(targetDir, timeoutMs); err != nil {
			if backupDir != "" {
				_ = os.RemoveAll(targetDir)
				_ = os.Rename(backupDir, targetDir)
			}
			return InstallPluginResult{OK: false, Error: "npm install failed: " + err.Error()}
		}
	}

	// 清理备份
	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}

	return InstallPluginResult{
		OK:           true,
		PluginID:     pluginID,
		TargetDir:    targetDir,
		ManifestName: pkgName,
		Version:      strings.TrimSpace(pkg.Version),
		Extensions:   extensions,
	}
}

// InstallFromPkgDirParams 安装参数
type InstallFromPkgDirParams struct {
	PackageDir       string
	ExtensionsDir    string
	TimeoutMs        int64
	Logger           InstallLogger
	Mode             string // "install" | "update"
	DryRun           bool
	ExpectedPluginID string
}

// InstallPluginFromNpmSpec 从 npm 包规格安装插件
// 对应 TS: install.ts installPluginFromNpmSpec
func InstallPluginFromNpmSpec(params InstallFromNpmParams) InstallPluginResult {
	logger := params.Logger
	spec := strings.TrimSpace(params.Spec)
	if spec == "" {
		return InstallPluginResult{OK: false, Error: "missing npm spec"}
	}

	timeoutMs := params.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 120_000
	}
	mode := params.Mode
	if mode == "" {
		mode = "install"
	}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "openacosmi-npm-pack-")
	if err != nil {
		return InstallPluginResult{OK: false, Error: "failed to create temp dir: " + err.Error()}
	}
	defer os.RemoveAll(tmpDir)

	if logger.Info != nil {
		logger.Info("Downloading " + spec + "…")
	}

	// npm pack
	packTimeout := timeoutMs
	if packTimeout < 300_000 {
		packTimeout = 300_000
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(packTimeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "pack", spec)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "COREPACK_ENABLE_DOWNLOAD_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return InstallPluginResult{
			OK:    false,
			Error: fmt.Sprintf("npm pack failed: %s", strings.TrimSpace(string(output))),
		}
	}

	// 获取生成的 tarball 文件名
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	packed := ""
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			packed = trimmed
			break
		}
	}
	if packed == "" {
		return InstallPluginResult{OK: false, Error: "npm pack produced no archive"}
	}

	archivePath := filepath.Join(tmpDir, packed)

	// 解压
	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to create extract dir: " + err.Error()}
	}
	if err := extractTarGz(archivePath, extractDir); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to extract archive: " + err.Error()}
	}

	// 找到解压后的根目录（npm pack 通常会创建 package/ 子目录）
	packageDir, err := resolvePackedRootDir(extractDir)
	if err != nil {
		return InstallPluginResult{OK: false, Error: err.Error()}
	}

	return InstallPluginFromPackageDir(InstallFromPkgDirParams{
		PackageDir:       packageDir,
		ExtensionsDir:    params.ExtensionsDir,
		TimeoutMs:        timeoutMs,
		Logger:           logger,
		Mode:             mode,
		DryRun:           params.DryRun,
		ExpectedPluginID: params.ExpectedPluginID,
	})
}

// InstallFromNpmParams npm 安装参数
type InstallFromNpmParams struct {
	Spec             string
	ExtensionsDir    string
	TimeoutMs        int64
	Logger           InstallLogger
	Mode             string // "install" | "update"
	DryRun           bool
	ExpectedPluginID string
}

// InstallPluginFromDir 从本地目录安装插件
// 对应 TS: install.ts installPluginFromDir
func InstallPluginFromDir(params InstallFromDirParams) InstallPluginResult {
	dirPath := resolveUserPath(params.DirPath)
	info, err := os.Stat(dirPath)
	if err != nil {
		return InstallPluginResult{OK: false, Error: "directory not found: " + dirPath}
	}
	if !info.IsDir() {
		return InstallPluginResult{OK: false, Error: "not a directory: " + dirPath}
	}
	return InstallPluginFromPackageDir(InstallFromPkgDirParams{
		PackageDir:       dirPath,
		ExtensionsDir:    params.ExtensionsDir,
		TimeoutMs:        params.TimeoutMs,
		Logger:           params.Logger,
		Mode:             params.Mode,
		DryRun:           params.DryRun,
		ExpectedPluginID: params.ExpectedPluginID,
	})
}

// InstallFromDirParams 目录安装参数
type InstallFromDirParams struct {
	DirPath          string
	ExtensionsDir    string
	TimeoutMs        int64
	Logger           InstallLogger
	Mode             string
	DryRun           bool
	ExpectedPluginID string
}

// InstallPluginFromFile 从单文件安装插件
// 对应 TS: install.ts installPluginFromFile
func InstallPluginFromFile(params InstallFromFileParams) InstallPluginResult {
	logger := params.Logger
	mode := params.Mode
	if mode == "" {
		mode = "install"
	}

	filePath := resolveUserPath(params.FilePath)
	if _, err := os.Stat(filePath); err != nil {
		return InstallPluginResult{OK: false, Error: "file not found: " + filePath}
	}

	extensionsDir := params.ExtensionsDir
	if extensionsDir == "" {
		extensionsDir = filepath.Join(resolveConfigDir(), "extensions")
	} else {
		extensionsDir = resolveUserPath(extensionsDir)
	}
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to create extensions dir: " + err.Error()}
	}

	base := fileBaseName(filepath.Base(filePath))
	pluginID := base
	if pluginID == "" {
		pluginID = "plugin"
	}
	if err := ValidatePluginID(pluginID); err != nil {
		return InstallPluginResult{OK: false, Error: err.Error()}
	}

	targetFile := filepath.Join(extensionsDir, SafeDirName(pluginID)+filepath.Ext(filePath))

	if mode == "install" {
		if _, err := os.Stat(targetFile); err == nil {
			return InstallPluginResult{
				OK:    false,
				Error: fmt.Sprintf("plugin already exists: %s (delete it first)", targetFile),
			}
		}
	}

	if params.DryRun {
		return InstallPluginResult{
			OK:         true,
			PluginID:   pluginID,
			TargetDir:  targetFile,
			Extensions: []string{filepath.Base(targetFile)},
		}
	}

	if logger.Info != nil {
		logger.Info("Installing to " + targetFile + "…")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return InstallPluginResult{OK: false, Error: "failed to read file: " + err.Error()}
	}
	if err := os.WriteFile(targetFile, data, 0o644); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to write file: " + err.Error()}
	}

	return InstallPluginResult{
		OK:         true,
		PluginID:   pluginID,
		TargetDir:  targetFile,
		Extensions: []string{filepath.Base(targetFile)},
	}
}

// InstallFromFileParams 文件安装参数
type InstallFromFileParams struct {
	FilePath      string
	ExtensionsDir string
	Logger        InstallLogger
	Mode          string
	DryRun        bool
}

// InstallPluginFromPath 从路径安装（智能路由：目录/归档/文件）
// 对应 TS: install.ts installPluginFromPath
func InstallPluginFromPath(params InstallFromPathParams) InstallPluginResult {
	resolved := resolveUserPath(params.Path)
	info, err := os.Stat(resolved)
	if err != nil {
		return InstallPluginResult{OK: false, Error: "path not found: " + resolved}
	}

	if info.IsDir() {
		return InstallPluginFromDir(InstallFromDirParams{
			DirPath:          resolved,
			ExtensionsDir:    params.ExtensionsDir,
			TimeoutMs:        params.TimeoutMs,
			Logger:           params.Logger,
			Mode:             params.Mode,
			DryRun:           params.DryRun,
			ExpectedPluginID: params.ExpectedPluginID,
		})
	}

	// 检查是否为归档文件
	if isArchiveFile(resolved) {
		return installPluginFromArchive(resolved, params)
	}

	return InstallPluginFromFile(InstallFromFileParams{
		FilePath:      resolved,
		ExtensionsDir: params.ExtensionsDir,
		Logger:        params.Logger,
		Mode:          params.Mode,
		DryRun:        params.DryRun,
	})
}

// InstallFromPathParams 路径安装参数
type InstallFromPathParams struct {
	Path             string
	ExtensionsDir    string
	TimeoutMs        int64
	Logger           InstallLogger
	Mode             string
	DryRun           bool
	ExpectedPluginID string
}

func installPluginFromArchive(archivePath string, params InstallFromPathParams) InstallPluginResult {
	logger := params.Logger
	timeoutMs := params.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 120_000
	}

	tmpDir, err := os.MkdirTemp("", "openacosmi-plugin-")
	if err != nil {
		return InstallPluginResult{OK: false, Error: "failed to create temp dir: " + err.Error()}
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to create extract dir: " + err.Error()}
	}

	if logger.Info != nil {
		logger.Info("Extracting " + archivePath + "…")
	}

	if err := extractTarGz(archivePath, extractDir); err != nil {
		return InstallPluginResult{OK: false, Error: "failed to extract archive: " + err.Error()}
	}

	packageDir, err := resolvePackedRootDir(extractDir)
	if err != nil {
		return InstallPluginResult{OK: false, Error: err.Error()}
	}

	return InstallPluginFromPackageDir(InstallFromPkgDirParams{
		PackageDir:       packageDir,
		ExtensionsDir:    params.ExtensionsDir,
		TimeoutMs:        timeoutMs,
		Logger:           logger,
		Mode:             params.Mode,
		DryRun:           params.DryRun,
		ExpectedPluginID: params.ExpectedPluginID,
	})
}
