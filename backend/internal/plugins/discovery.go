package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// PluginCandidate 插件候选项
// 对应 TS: discovery.ts PluginCandidate
type PluginCandidate struct {
	IDHint             string                     `json:"idHint"`
	Source             string                     `json:"source"`
	RootDir            string                     `json:"rootDir"`
	Origin             PluginOrigin               `json:"origin"`
	WorkspaceDir       string                     `json:"workspaceDir,omitempty"`
	PackageName        string                     `json:"packageName,omitempty"`
	PackageVersion     string                     `json:"packageVersion,omitempty"`
	PackageDescription string                     `json:"packageDescription,omitempty"`
	PackageDir         string                     `json:"packageDir,omitempty"`
	PackageManifest    *OpenAcosmiPackageManifest `json:"packageManifest,omitempty"`
}

// PluginDiscoveryResult 插件发现结果
// 对应 TS: discovery.ts PluginDiscoveryResult
type PluginDiscoveryResult struct {
	Candidates  []PluginCandidate  `json:"candidates"`
	Diagnostics []PluginDiagnostic `json:"diagnostics"`
}

// Go 插件可加载的扩展名（保留 TS 发现逻辑以便 manifest 识别）
var extensionExts = map[string]bool{
	".ts":  true,
	".js":  true,
	".mts": true,
	".cts": true,
	".mjs": true,
	".cjs": true,
	".go":  true, // Go 编译时插件
	".so":  true, // Go plugin（可选）
}

// DiscoverPlugins 发现所有可用插件
// 对应 TS: discovery.ts discoverOpenAcosmiPlugins
// 扫描顺序：config 额外路径 → workspace .openacosmi/extensions/ → 全局 extensions/ → bundled 目录
func DiscoverPlugins(workspaceDir string, extraPaths []string) PluginDiscoveryResult {
	candidates := make([]PluginCandidate, 0)
	diagnostics := make([]PluginDiagnostic, 0)
	seen := make(map[string]bool)

	// 1. Config 额外路径
	for _, extraPath := range extraPaths {
		trimmed := strings.TrimSpace(extraPath)
		if trimmed == "" {
			continue
		}
		discoverFromPath(discoverFromPathParams{
			rawPath:      trimmed,
			origin:       PluginOriginConfig,
			workspaceDir: strings.TrimSpace(workspaceDir),
			candidates:   &candidates,
			diagnostics:  &diagnostics,
			seen:         seen,
		})
	}

	// 2. Workspace extensions
	wsDir := strings.TrimSpace(workspaceDir)
	if wsDir != "" {
		wsRoot := resolveUserPath(wsDir)
		extensionsDir := filepath.Join(wsRoot, ".openacosmi", "extensions")
		discoverInDirectory(discoverInDirParams{
			dir:          extensionsDir,
			origin:       PluginOriginWorkspace,
			workspaceDir: wsRoot,
			candidates:   &candidates,
			diagnostics:  &diagnostics,
			seen:         seen,
		})
	}

	// 3. 全局 extensions
	globalDir := filepath.Join(resolveConfigDir(), "extensions")
	discoverInDirectory(discoverInDirParams{
		dir:         globalDir,
		origin:      PluginOriginGlobal,
		candidates:  &candidates,
		diagnostics: &diagnostics,
		seen:        seen,
	})

	// 4. Bundled plugins
	bundledDir := ResolveBundledPluginsDir()
	if bundledDir != "" {
		discoverInDirectory(discoverInDirParams{
			dir:         bundledDir,
			origin:      PluginOriginBundled,
			candidates:  &candidates,
			diagnostics: &diagnostics,
			seen:        seen,
		})
	}

	return PluginDiscoveryResult{
		Candidates:  candidates,
		Diagnostics: diagnostics,
	}
}

// ResolveBundledPluginsDir 解析内置插件目录
// 对应 TS: bundled-dir.ts resolveBundledPluginsDir
// 优先级：环境变量 → 可执行文件同级 extensions/ → 向上搜索
func ResolveBundledPluginsDir() string {
	// 环境变量覆盖
	override := strings.TrimSpace(os.Getenv("OPENACOSMI_BUNDLED_PLUGINS_DIR"))
	if override != "" {
		return override
	}

	// 可执行文件同级目录
	execPath, err := os.Executable()
	if err == nil {
		sibling := filepath.Join(filepath.Dir(execPath), "extensions")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			return sibling
		}
	}

	// 向上搜索（从当前工作目录）
	cwd, err := os.Getwd()
	if err == nil {
		cursor := cwd
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(cursor, "extensions")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
			parent := filepath.Dir(cursor)
			if parent == cursor {
				break
			}
			cursor = parent
		}
	}

	return ""
}

// --- 发现辅助函数 ---

type discoverInDirParams struct {
	dir          string
	origin       PluginOrigin
	workspaceDir string
	candidates   *[]PluginCandidate
	diagnostics  *[]PluginDiagnostic
	seen         map[string]bool
}

func discoverInDirectory(p discoverInDirParams) {
	if _, err := os.Stat(p.dir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(p.dir)
	if err != nil {
		*p.diagnostics = append(*p.diagnostics, PluginDiagnostic{
			Level:   "warn",
			Message: "failed to read extensions dir: " + p.dir + " (" + err.Error() + ")",
			Source:  p.dir,
		})
		return
	}

	for _, entry := range entries {
		fullPath := filepath.Join(p.dir, entry.Name())

		if !entry.IsDir() {
			// 单文件插件
			if !isExtensionFile(fullPath) {
				continue
			}
			addCandidate(addCandidateParams{
				candidates:   p.candidates,
				seen:         p.seen,
				idHint:       fileBaseName(entry.Name()),
				source:       fullPath,
				rootDir:      filepath.Dir(fullPath),
				origin:       p.origin,
				workspaceDir: p.workspaceDir,
			})
			continue
		}

		// 目录：检查 package.json
		manifest := readPackageJSON(fullPath)
		extensions := resolvePackageExtensions(manifest)

		if len(extensions) > 0 {
			for _, extPath := range extensions {
				resolved, _ := filepath.Abs(filepath.Join(fullPath, extPath))
				addCandidate(addCandidateParams{
					candidates: p.candidates,
					seen:       p.seen,
					idHint: deriveIDHint(
						resolved,
						packageNameFromManifest(manifest),
						len(extensions) > 1,
					),
					source:       resolved,
					rootDir:      fullPath,
					origin:       p.origin,
					workspaceDir: p.workspaceDir,
					manifest:     manifest,
					packageDir:   fullPath,
				})
			}
			continue
		}

		// index 文件 fallback
		indexCandidates := []string{"index.ts", "index.js", "index.mjs", "index.cjs"}
		for _, idx := range indexCandidates {
			indexFile := filepath.Join(fullPath, idx)
			if _, err := os.Stat(indexFile); err == nil && isExtensionFile(indexFile) {
				addCandidate(addCandidateParams{
					candidates:   p.candidates,
					seen:         p.seen,
					idHint:       entry.Name(),
					source:       indexFile,
					rootDir:      fullPath,
					origin:       p.origin,
					workspaceDir: p.workspaceDir,
					manifest:     manifest,
					packageDir:   fullPath,
				})
				break
			}
		}
	}
}

type discoverFromPathParams struct {
	rawPath      string
	origin       PluginOrigin
	workspaceDir string
	candidates   *[]PluginCandidate
	diagnostics  *[]PluginDiagnostic
	seen         map[string]bool
}

func discoverFromPath(p discoverFromPathParams) {
	resolved := resolveUserPath(p.rawPath)
	info, err := os.Stat(resolved)
	if err != nil {
		*p.diagnostics = append(*p.diagnostics, PluginDiagnostic{
			Level:   "error",
			Message: "plugin path not found: " + resolved,
			Source:  resolved,
		})
		return
	}

	if !info.IsDir() {
		// 单文件
		if !isExtensionFile(resolved) {
			*p.diagnostics = append(*p.diagnostics, PluginDiagnostic{
				Level:   "error",
				Message: "plugin path is not a supported file: " + resolved,
				Source:  resolved,
			})
			return
		}
		addCandidate(addCandidateParams{
			candidates:   p.candidates,
			seen:         p.seen,
			idHint:       fileBaseName(filepath.Base(resolved)),
			source:       resolved,
			rootDir:      filepath.Dir(resolved),
			origin:       p.origin,
			workspaceDir: p.workspaceDir,
		})
		return
	}

	// 目录
	manifest := readPackageJSON(resolved)
	extensions := resolvePackageExtensions(manifest)

	if len(extensions) > 0 {
		for _, extPath := range extensions {
			source, _ := filepath.Abs(filepath.Join(resolved, extPath))
			addCandidate(addCandidateParams{
				candidates: p.candidates,
				seen:       p.seen,
				idHint: deriveIDHint(
					source,
					packageNameFromManifest(manifest),
					len(extensions) > 1,
				),
				source:       source,
				rootDir:      resolved,
				origin:       p.origin,
				workspaceDir: p.workspaceDir,
				manifest:     manifest,
				packageDir:   resolved,
			})
		}
		return
	}

	// index 文件 fallback
	indexCandidates := []string{"index.ts", "index.js", "index.mjs", "index.cjs"}
	for _, idx := range indexCandidates {
		indexFile := filepath.Join(resolved, idx)
		if _, err := os.Stat(indexFile); err == nil && isExtensionFile(indexFile) {
			addCandidate(addCandidateParams{
				candidates:   p.candidates,
				seen:         p.seen,
				idHint:       filepath.Base(resolved),
				source:       indexFile,
				rootDir:      resolved,
				origin:       p.origin,
				workspaceDir: p.workspaceDir,
				manifest:     manifest,
				packageDir:   resolved,
			})
			return
		}
	}

	// 作为扩展目录递归扫描
	discoverInDirectory(discoverInDirParams{
		dir:          resolved,
		origin:       p.origin,
		workspaceDir: p.workspaceDir,
		candidates:   p.candidates,
		diagnostics:  p.diagnostics,
		seen:         p.seen,
	})
}

type addCandidateParams struct {
	candidates   *[]PluginCandidate
	seen         map[string]bool
	idHint       string
	source       string
	rootDir      string
	origin       PluginOrigin
	workspaceDir string
	manifest     *packageJSON
	packageDir   string
}

func addCandidate(p addCandidateParams) {
	resolved, _ := filepath.Abs(p.source)
	if p.seen[resolved] {
		return
	}
	p.seen[resolved] = true

	candidate := PluginCandidate{
		IDHint:       p.idHint,
		Source:       resolved,
		RootDir:      absPath(p.rootDir),
		Origin:       p.origin,
		WorkspaceDir: p.workspaceDir,
		PackageDir:   p.packageDir,
	}

	if p.manifest != nil {
		candidate.PackageName = strings.TrimSpace(p.manifest.Name)
		candidate.PackageVersion = strings.TrimSpace(p.manifest.Version)
		candidate.PackageDescription = strings.TrimSpace(p.manifest.Description)
		candidate.PackageManifest = getOpenAcosmiPackageManifest(p.manifest)
	}

	*p.candidates = append(*p.candidates, candidate)
}

// --- package.json 解析 ---

// packageJSON 简化的 package.json 结构
type packageJSON struct {
	Name         string                     `json:"name"`
	Version      string                     `json:"version"`
	Description  string                     `json:"description"`
	Dependencies map[string]string          `json:"dependencies"`
	OpenAcosmi   *OpenAcosmiPackageManifest `json:"openacosmi"`
}

func readPackageJSON(dir string) *packageJSON {
	manifestPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return &pkg
}

func resolvePackageExtensions(pkg *packageJSON) []string {
	if pkg == nil || pkg.OpenAcosmi == nil {
		return nil
	}
	result := make([]string, 0, len(pkg.OpenAcosmi.Extensions))
	for _, ext := range pkg.OpenAcosmi.Extensions {
		trimmed := strings.TrimSpace(ext)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func packageNameFromManifest(pkg *packageJSON) string {
	if pkg == nil {
		return ""
	}
	return strings.TrimSpace(pkg.Name)
}

func getOpenAcosmiPackageManifest(pkg *packageJSON) *OpenAcosmiPackageManifest {
	if pkg == nil {
		return nil
	}
	return pkg.OpenAcosmi
}

// --- 路径与名称辅助函数 ---

// isExtensionFile 判断文件是否为可加载的扩展文件
func isExtensionFile(filePath string) bool {
	ext := filepath.Ext(filePath)
	if !extensionExts[ext] {
		return false
	}
	// 排除 TypeScript 声明文件
	if strings.HasSuffix(filePath, ".d.ts") {
		return false
	}
	return true
}

// deriveIDHint 从文件路径和包名导出插件 ID 提示
// 对应 TS: discovery.ts deriveIdHint
func deriveIDHint(filePath string, packageName string, hasMultipleExtensions bool) string {
	base := fileBaseName(filepath.Base(filePath))
	rawPkgName := strings.TrimSpace(packageName)
	if rawPkgName == "" {
		return base
	}

	// 去 scope: @openacosmi/voice-call → voice-call
	unscoped := rawPkgName
	if strings.Contains(rawPkgName, "/") {
		parts := strings.Split(rawPkgName, "/")
		unscoped = parts[len(parts)-1]
	}

	if !hasMultipleExtensions {
		return unscoped
	}
	return unscoped + "/" + base
}

// fileBaseName 去除扩展名的文件名
func fileBaseName(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return name[:len(name)-len(ext)]
}

// resolveConfigDir 解析配置目录
// 对应 TS: utils.ts resolveConfigDir → CONFIG_DIR
func resolveConfigDir() string {
	if override := os.Getenv("OPENACOSMI_CONFIG_DIR"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "openacosmi")
	}
	return filepath.Join(home, ".config", "openacosmi")
}

// resolveUserPath 将 ~ 前缀路径展开为绝对路径
// 对应 TS: utils.ts resolveUserPath
func resolveUserPath(rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "~/") || trimmed == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if trimmed == "~" {
				return home
			}
			return filepath.Join(home, trimmed[2:])
		}
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return trimmed
	}
	return abs
}

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
