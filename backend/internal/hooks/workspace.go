package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// 工作区钩子发现 — 从 bundled/managed/workspace 目录扫描并加载钩子
// 对应 TS: workspace.ts
// ============================================================================

// LoadHookFromDir 从单个目录加载 Hook
// 对应 TS: workspace.ts loadHookFromDir
func LoadHookFromDir(hookDir string, source HookSource, pluginID string, nameHint string) *Hook {
	hookMdPath := filepath.Join(hookDir, "HOOK.md")
	if _, err := os.Stat(hookMdPath); err != nil {
		return nil
	}

	content, err := os.ReadFile(hookMdPath)
	if err != nil {
		return nil
	}

	frontmatter := ParseFrontmatter(string(content))

	name := frontmatter["name"]
	if name == "" {
		name = nameHint
	}
	if name == "" {
		name = filepath.Base(hookDir)
	}

	description := frontmatter["description"]

	// Resolve handler path
	candidates := []string{"handler.go", "handler.ts", "handler.js", "index.ts", "index.js"}
	var handlerPath string
	for _, c := range candidates {
		candidate := filepath.Join(hookDir, c)
		if _, err := os.Stat(candidate); err == nil {
			handlerPath = candidate
			break
		}
	}
	if handlerPath == "" {
		return nil
	}

	return &Hook{
		Name:        name,
		Description: description,
		Source:      source,
		PluginID:    pluginID,
		FilePath:    hookMdPath,
		BaseDir:     hookDir,
		HandlerPath: handlerPath,
	}
}

// LoadHooksFromDir 扫描目录下所有子目录中的钩子
// 对应 TS: workspace.ts loadHooksFromDir
func LoadHooksFromDir(dir string, source HookSource, pluginID string) []Hook {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var hooks []Hook
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		hookDir := filepath.Join(dir, entry.Name())

		// Check for package.json with hook declarations
		packageHooks := resolvePackageHooks(hookDir)
		if len(packageHooks) > 0 {
			for _, hookPath := range packageHooks {
				resolvedDir := filepath.Join(hookDir, hookPath)
				hook := LoadHookFromDir(resolvedDir, source, pluginID, filepath.Base(resolvedDir))
				if hook != nil {
					hooks = append(hooks, *hook)
				}
			}
			continue
		}

		hook := LoadHookFromDir(hookDir, source, pluginID, entry.Name())
		if hook != nil {
			hooks = append(hooks, *hook)
		}
	}

	return hooks
}

// LoadHookEntriesFromDir 加载目录下的 hook entries（含 frontmatter + metadata）
// 对应 TS: workspace.ts loadHookEntriesFromDir
func LoadHookEntriesFromDir(dir string, source HookSource, pluginID string) []HookEntry {
	hooks := LoadHooksFromDir(dir, source, pluginID)
	entries := make([]HookEntry, 0, len(hooks))

	for _, hook := range hooks {
		var frontmatter map[string]string
		if data, err := os.ReadFile(hook.FilePath); err == nil {
			frontmatter = ParseFrontmatter(string(data))
		}
		if frontmatter == nil {
			frontmatter = make(map[string]string)
		}
		entry := HookEntry{
			Hook:        hook,
			Frontmatter: frontmatter,
			Metadata:    ResolveOpenAcosmiMetadata(frontmatter),
		}
		entry.Invocation = ptrPolicy(ResolveHookInvocationPolicy(frontmatter))
		entries = append(entries, entry)
	}
	return entries
}

// LoadWorkspaceHookEntries 加载工作区所有来源的钩子
// 对应 TS: workspace.ts loadWorkspaceHookEntries
func LoadWorkspaceHookEntries(workspaceDir string, config map[string]interface{}) []HookEntry {
	return loadHookEntries(workspaceDir, config, "", "")
}

func loadHookEntries(workspaceDir string, config map[string]interface{}, managedHooksDir, bundledHooksDir string) []HookEntry {
	if managedHooksDir == "" {
		managedHooksDir = filepath.Join(ResolveConfigDir(), "hooks")
	}
	workspaceHooksDir := filepath.Join(workspaceDir, "hooks")
	if bundledHooksDir == "" {
		bundledHooksDir = ResolveBundledHooksDir()
	}

	// Extra dirs from config
	var extraDirs []string
	if config != nil {
		if hooks, ok := config["hooks"].(map[string]interface{}); ok {
			if internal, ok := hooks["internal"].(map[string]interface{}); ok {
				if load, ok := internal["load"].(map[string]interface{}); ok {
					if dirs, ok := load["extraDirs"].([]interface{}); ok {
						for _, d := range dirs {
							if s, ok := d.(string); ok {
								trimmed := strings.TrimSpace(s)
								if trimmed != "" {
									extraDirs = append(extraDirs, resolveUserPath(trimmed))
								}
							}
						}
					}
				}
			}
		}
	}

	// Load from all sources
	merged := make(map[string]Hook)

	// Precedence: extra < bundled < managed < workspace
	for _, dir := range extraDirs {
		for _, hook := range LoadHooksFromDir(dir, HookSourceWorkspace, "") {
			merged[hook.Name] = hook
		}
	}
	if bundledHooksDir != "" {
		for _, hook := range LoadHooksFromDir(bundledHooksDir, HookSourceBundled, "") {
			merged[hook.Name] = hook
		}
	}
	for _, hook := range LoadHooksFromDir(managedHooksDir, HookSourceManaged, "") {
		merged[hook.Name] = hook
	}
	for _, hook := range LoadHooksFromDir(workspaceHooksDir, HookSourceWorkspace, "") {
		merged[hook.Name] = hook
	}

	// Build entries
	entries := make([]HookEntry, 0, len(merged))
	for _, hook := range merged {
		var frontmatter map[string]string
		if data, err := os.ReadFile(hook.FilePath); err == nil {
			frontmatter = ParseFrontmatter(string(data))
		}
		if frontmatter == nil {
			frontmatter = make(map[string]string)
		}
		entries = append(entries, HookEntry{
			Hook:        hook,
			Frontmatter: frontmatter,
			Metadata:    ResolveOpenAcosmiMetadata(frontmatter),
			Invocation:  ptrPolicy(ResolveHookInvocationPolicy(frontmatter)),
		})
	}

	return entries
}

// BuildWorkspaceHookSnapshot 构建钩子快照
// 对应 TS: workspace.ts buildWorkspaceHookSnapshot
func BuildWorkspaceHookSnapshot(workspaceDir string, config map[string]interface{}, eligibility *HookEligibilityContext, entries []HookEntry, version *int) HookSnapshot {
	if entries == nil {
		entries = LoadWorkspaceHookEntries(workspaceDir, config)
	}

	eligible := filterHookEntries(entries, config, eligibility)

	snapshot := HookSnapshot{
		Hooks:   make([]HookSnapshotEntry, 0, len(eligible)),
		Version: version,
	}
	resolvedHooks := make([]Hook, 0, len(eligible))
	for _, entry := range eligible {
		events := entry.Metadata.eventsOrEmpty()
		snapshot.Hooks = append(snapshot.Hooks, HookSnapshotEntry{
			Name:   entry.Hook.Name,
			Events: events,
		})
		resolvedHooks = append(resolvedHooks, entry.Hook)
	}
	snapshot.ResolvedHooks = resolvedHooks
	return snapshot
}

func filterHookEntries(entries []HookEntry, config map[string]interface{}, eligibility *HookEligibilityContext) []HookEntry {
	result := make([]HookEntry, 0, len(entries))
	for _, entry := range entries {
		if ShouldIncludeHook(&entry, config, eligibility) {
			result = append(result, entry)
		}
	}
	return result
}

// --- helpers ---

func resolvePackageHooks(dir string) []string {
	manifestPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	// Check manifest key
	for _, key := range append([]string{ManifestKey}, LegacyManifestKeys...) {
		if section, ok := raw[key].(map[string]interface{}); ok {
			if hooksRaw, ok := section["hooks"].([]interface{}); ok {
				result := make([]string, 0, len(hooksRaw))
				for _, h := range hooksRaw {
					if s, ok := h.(string); ok {
						trimmed := strings.TrimSpace(s)
						if trimmed != "" {
							result = append(result, trimmed)
						}
					}
				}
				return result
			}
		}
	}
	return nil
}

func ptrPolicy(p HookInvocationPolicy) *HookInvocationPolicy {
	return &p
}

func (m *HookMetadata) eventsOrEmpty() []string {
	if m == nil {
		return nil
	}
	return m.Events
}

// ResolveConfigDir 获取用户配置目录
func ResolveConfigDir() string {
	if dir := os.Getenv("OPENACOSMI_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "openacosmi")
}

// resolveUserPath 展开 ~ 路径
func resolveUserPath(input string) string {
	if strings.HasPrefix(input, "~/") || input == "~" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, input[1:])
	}
	return input
}
