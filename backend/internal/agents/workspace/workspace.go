package workspace

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Acosmi/ClawAcosmi/internal/agents/scope"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 工作区工具 ----------

// TS 参考: src/agents/workspace.ts (306 行) + workspace-run.ts (107 行)

const (
	DefaultAgentsFilename    = "AGENTS.md"
	DefaultSoulFilename      = "SOUL.md"
	DefaultToolsFilename     = "TOOLS.md"
	DefaultIdentityFilename  = "IDENTITY.md"
	DefaultUserFilename      = "USER.md"
	DefaultHeartbeatFilename = "HEARTBEAT.md"
	DefaultBootstrapFilename = "BOOTSTRAP.md"
	DefaultMemoryFilename    = "MEMORY.md"
	DefaultMemoryAltFilename = "memory.md"
)

// ---------- 命令通道 ----------

// TS 参考: src/agents/lanes.ts (5 行) + src/process/lanes.ts

// CommandLane 命令执行通道。
type CommandLane string

const (
	LaneNested   CommandLane = "nested"
	LaneSubagent CommandLane = "subagent"
)

// ---------- 工作区引导文件 ----------

// WorkspaceBootstrapFileName 引导文件名类型。
type WorkspaceBootstrapFileName string

const (
	BootstrapAgents    WorkspaceBootstrapFileName = "AGENTS.md"
	BootstrapSoul      WorkspaceBootstrapFileName = "SOUL.md"
	BootstrapTools     WorkspaceBootstrapFileName = "TOOLS.md"
	BootstrapIdentity  WorkspaceBootstrapFileName = "IDENTITY.md"
	BootstrapUser      WorkspaceBootstrapFileName = "USER.md"
	BootstrapHeartbeat WorkspaceBootstrapFileName = "HEARTBEAT.md"
	BootstrapBootstrap WorkspaceBootstrapFileName = "BOOTSTRAP.md"
	BootstrapMemory    WorkspaceBootstrapFileName = "MEMORY.md"
)

// WorkspaceBootstrapFile 引导文件信息。
type WorkspaceBootstrapFile struct {
	Name    WorkspaceBootstrapFileName `json:"name"`
	Path    string                     `json:"path"`
	Content string                     `json:"content,omitempty"`
	Missing bool                       `json:"missing"`
}

// SubagentBootstrapAllowlist 子代理可访问的引导文件。
var SubagentBootstrapAllowlist = map[WorkspaceBootstrapFileName]bool{
	BootstrapAgents: true,
	BootstrapTools:  true,
}

// LoadWorkspaceBootstrapFiles 加载工作区引导文件。
// TS 参考: workspace.ts → loadWorkspaceBootstrapFiles()
func LoadWorkspaceBootstrapFiles(dir string) []WorkspaceBootstrapFile {
	filenames := []WorkspaceBootstrapFileName{
		BootstrapAgents,
		BootstrapSoul,
		BootstrapTools,
		BootstrapIdentity,
		BootstrapUser,
		BootstrapHeartbeat,
		BootstrapBootstrap,
		BootstrapMemory,
	}

	var result []WorkspaceBootstrapFile
	for _, name := range filenames {
		filePath := filepath.Join(dir, string(name))
		content, err := os.ReadFile(filePath)
		if err != nil {
			result = append(result, WorkspaceBootstrapFile{
				Name:    name,
				Path:    filePath,
				Missing: true,
			})
			continue
		}
		stripped := StripFrontMatter(string(content))
		result = append(result, WorkspaceBootstrapFile{
			Name:    name,
			Path:    filePath,
			Content: strings.TrimSpace(stripped),
			Missing: false,
		})
	}
	return result
}

// FilterBootstrapFilesForSession 为特定会话过滤引导文件。
// 子代理会话只获得 AGENTS.md 和 TOOLS.md。
func FilterBootstrapFilesForSession(files []WorkspaceBootstrapFile, sessionKey string) []WorkspaceBootstrapFile {
	if sessionKey == "" || !isSubagentSessionKey(sessionKey) {
		return files
	}
	var filtered []WorkspaceBootstrapFile
	for _, f := range files {
		if SubagentBootstrapAllowlist[f.Name] {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// StripFrontMatter 移除 YAML front matter。
// TS 参考: workspace.ts → stripFrontMatter()
func StripFrontMatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	idx := strings.Index(content[3:], "---")
	if idx < 0 {
		return content
	}
	return strings.TrimSpace(content[3+idx+3:])
}

// isSubagentSessionKey 简单检查是否为子代理会话 key。
func isSubagentSessionKey(key string) bool {
	return strings.Contains(key, ":subagent:")
}

// ---------- 工作区运行解析 ----------

// WorkspaceFallbackReason 工作区回退原因。
type WorkspaceFallbackReason string

const (
	FallbackMissing     WorkspaceFallbackReason = "missing"
	FallbackBlank       WorkspaceFallbackReason = "blank"
	FallbackInvalidType WorkspaceFallbackReason = "invalid_type"
)

// AgentIdSource Agent ID 来源。
type AgentIdSource string

const (
	AgentIdExplicit   AgentIdSource = "explicit"
	AgentIdSessionKey AgentIdSource = "session_key"
	AgentIdDefault    AgentIdSource = "default"
)

// ResolveRunWorkspaceResult 工作区解析结果。
type ResolveRunWorkspaceResult struct {
	WorkspaceDir   string                  `json:"workspaceDir"`
	UsedFallback   bool                    `json:"usedFallback"`
	FallbackReason WorkspaceFallbackReason `json:"fallbackReason,omitempty"`
	AgentId        string                  `json:"agentId"`
	AgentIdSource  AgentIdSource           `json:"agentIdSource"`
}

// ResolveRunWorkspaceDir 解析运行时工作区目录。
// TS 参考: workspace-run.ts → resolveRunWorkspaceDir()
func ResolveRunWorkspaceDir(workspaceDir, sessionKey, agentId string, cfg *types.OpenAcosmiConfig) ResolveRunWorkspaceResult {
	resolvedAgentId, source := resolveRunAgentId(sessionKey, agentId, cfg)

	trimmed := strings.TrimSpace(workspaceDir)
	if trimmed != "" {
		return ResolveRunWorkspaceResult{
			WorkspaceDir:  resolveUserPath(trimmed),
			UsedFallback:  false,
			AgentId:       resolvedAgentId,
			AgentIdSource: source,
		}
	}

	var fallbackReason WorkspaceFallbackReason
	if workspaceDir == "" {
		fallbackReason = FallbackMissing
	} else {
		fallbackReason = FallbackBlank
	}

	fallbackDir := scope.ResolveAgentWorkspaceDir(cfg, resolvedAgentId)
	return ResolveRunWorkspaceResult{
		WorkspaceDir:   resolveUserPath(fallbackDir),
		UsedFallback:   true,
		FallbackReason: fallbackReason,
		AgentId:        resolvedAgentId,
		AgentIdSource:  source,
	}
}

// resolveRunAgentId 解析运行时 agent ID。
func resolveRunAgentId(sessionKey, agentId string, cfg *types.OpenAcosmiConfig) (string, AgentIdSource) {
	// Explicit agent ID
	trimmed := strings.TrimSpace(agentId)
	if trimmed != "" {
		return scope.NormalizeAgentId(trimmed), AgentIdExplicit
	}

	// From session key
	if sessionKey != "" {
		resolved := scope.ResolveSessionAgentId(sessionKey, cfg)
		if resolved != "" {
			return resolved, AgentIdSessionKey
		}
	}

	// Default
	defaultId := scope.ResolveDefaultAgentId(cfg)
	return defaultId, AgentIdDefault
}

// resolveUserPath 解析用户路径（~ 展开）。
func resolveUserPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// ---------- 工作区确保 ----------

// TS 参考: workspace.ts → ensureAgentWorkspace() (L127-200)

// EnsureAgentWorkspaceParams 创建/确保工作区的参数。
type EnsureAgentWorkspaceParams struct {
	// Dir 工作区目录，空值使用默认路径。
	Dir string
	// EnsureBootstrapFiles 是否写入引导模板文件。
	EnsureBootstrapFiles bool
}

// EnsureAgentWorkspaceResult 工作区确保结果。
type EnsureAgentWorkspaceResult struct {
	Dir           string `json:"dir"`
	AgentsPath    string `json:"agentsPath,omitempty"`
	SoulPath      string `json:"soulPath,omitempty"`
	ToolsPath     string `json:"toolsPath,omitempty"`
	IdentityPath  string `json:"identityPath,omitempty"`
	UserPath      string `json:"userPath,omitempty"`
	HeartbeatPath string `json:"heartbeatPath,omitempty"`
	BootstrapPath string `json:"bootstrapPath,omitempty"`
}

// EnsureAgentWorkspace 创建工作区目录，可选写入引导模板文件 + git init。
// 行为等同于 TS ensureAgentWorkspace()：
//  1. 创建目录（MkdirAll）
//  2. 若 EnsureBootstrapFiles=true，写入 7 个模板文件（仅当不存在时）
//  3. 全新工作区（7 个文件均不存在）时额外写入 BOOTSTRAP.md 并 git init
func EnsureAgentWorkspace(params EnsureAgentWorkspaceParams) (EnsureAgentWorkspaceResult, error) {
	rawDir := strings.TrimSpace(params.Dir)
	if rawDir == "" {
		rawDir = scope.ResolveAgentWorkspaceDir(nil, "")
	}
	dir := resolveUserPath(rawDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return EnsureAgentWorkspaceResult{}, err
	}

	if !params.EnsureBootstrapFiles {
		return EnsureAgentWorkspaceResult{Dir: dir}, nil
	}

	agentsPath := filepath.Join(dir, DefaultAgentsFilename)
	soulPath := filepath.Join(dir, DefaultSoulFilename)
	toolsPath := filepath.Join(dir, DefaultToolsFilename)
	identityPath := filepath.Join(dir, DefaultIdentityFilename)
	userPath := filepath.Join(dir, DefaultUserFilename)
	heartbeatPath := filepath.Join(dir, DefaultHeartbeatFilename)
	bootstrapPath := filepath.Join(dir, DefaultBootstrapFilename)

	// 检测是否全新工作区（所有引导文件均不存在）
	checkPaths := []string{agentsPath, soulPath, toolsPath, identityPath, userPath, heartbeatPath}
	isBrandNew := true
	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			isBrandNew = false
			break
		}
	}

	// 加载并写入模板
	templates := []struct {
		name string
		path string
	}{
		{DefaultAgentsFilename, agentsPath},
		{DefaultSoulFilename, soulPath},
		{DefaultToolsFilename, toolsPath},
		{DefaultIdentityFilename, identityPath},
		{DefaultUserFilename, userPath},
		{DefaultHeartbeatFilename, heartbeatPath},
	}
	for _, t := range templates {
		content, err := loadTemplate(t.name)
		if err != nil {
			// 模板加载失败不阻塞工作区创建
			continue
		}
		writeFileIfMissing(t.path, content)
	}

	// 全新工作区额外写入 BOOTSTRAP.md + git init
	if isBrandNew {
		if content, err := loadTemplate(DefaultBootstrapFilename); err == nil {
			writeFileIfMissing(bootstrapPath, content)
		}
		ensureGitRepo(dir)
	}

	return EnsureAgentWorkspaceResult{
		Dir:           dir,
		AgentsPath:    agentsPath,
		SoulPath:      soulPath,
		ToolsPath:     toolsPath,
		IdentityPath:  identityPath,
		UserPath:      userPath,
		HeartbeatPath: heartbeatPath,
		BootstrapPath: bootstrapPath,
	}, nil
}

// writeFileIfMissing 仅在文件不存在时写入（O_EXCL 语义）。
// TS 参考: workspace.ts → writeFileIfMissing()
func writeFileIfMissing(filePath, content string) {
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		// EEXIST 或其他错误均静默忽略
		return
	}
	defer f.Close()
	f.WriteString(content)
}

// hasGitRepo 检查目录是否已有 .git。
func hasGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// ensureGitRepo 为全新工作区初始化 git 仓库。
// TS 参考: workspace.ts → ensureGitRepo()
func ensureGitRepo(dir string) {
	if hasGitRepo(dir) {
		return
	}
	// 检查 git 是否可用
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	_ = cmd.Run() // git init 失败不影响工作区创建
}

// ---------- 模板加载 ----------

// TS 参考: workspace-templates.ts (69 行)

var (
	templateDir     string
	templateDirOnce sync.Once
)

// resolveTemplateDir 解析模板目录。
// 优先级: 可执行文件同级 docs/reference/templates → 工作目录 docs/reference/templates
func resolveTemplateDir() string {
	templateDirOnce.Do(func() {
		// 尝试从可执行文件位置推算
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "..", "docs", "reference", "templates")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				templateDir = candidate
				return
			}
		}
		// 尝试从工作目录
		if cwd, err := os.Getwd(); err == nil {
			candidate := filepath.Join(cwd, "docs", "reference", "templates")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				templateDir = candidate
				return
			}
		}
		// 后备：相对路径
		templateDir = filepath.Join("docs", "reference", "templates")
	})
	return templateDir
}

// ResetTemplateDirCache 重置模板目录缓存（仅用于测试）。
func ResetTemplateDirCache() {
	templateDirOnce = sync.Once{}
	templateDir = ""
}

// SetTemplateDir 覆盖模板目录（仅用于测试）。
func SetTemplateDir(dir string) {
	templateDirOnce.Do(func() {})
	templateDir = dir
}

// loadTemplate 从模板目录加载文件内容（去除 front matter）。
// TS 参考: workspace.ts → loadTemplate()
func loadTemplate(name string) (string, error) {
	tmplDir := resolveTemplateDir()
	data, err := os.ReadFile(filepath.Join(tmplDir, name))
	if err != nil {
		return "", err
	}
	return StripFrontMatter(string(data)), nil
}

// ---------- 文档路径 ----------

// TS 参考: src/agents/docs-path.ts (31 行)

// ResolveDocsPath 解析文档目录路径。
func ResolveDocsPath(workspaceDir string) string {
	if dir := strings.TrimSpace(workspaceDir); dir != "" {
		docsPath := filepath.Join(dir, "docs")
		if info, err := os.Stat(docsPath); err == nil && info.IsDir() {
			return docsPath
		}
	}
	return ""
}

// ---------- 临时辅助 ----------

// init 用于兼容 Go 的 init 随机种子（Go 1.20+ 自动初始化）。
var _ = rand.Int
