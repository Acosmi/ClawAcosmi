package gateway

// server_methods_agent_files.go — agents.files.* 方法处理器
// 对应 TS: src/gateway/server-methods/agents.ts (L368-L508)
//
// 提供 Agent 工作区文件的列表、读取、写入功能。
// 依赖: scope.ResolveAgentWorkspaceDir, workspace 常量

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/scope"
	"github.com/Acosmi/ClawAcosmi/internal/agents/workspace"
)

// bootstrapFileNames 对应 TS BOOTSTRAP_FILE_NAMES 常量。
var bootstrapFileNames = []string{
	workspace.DefaultAgentsFilename,
	workspace.DefaultSoulFilename,
	workspace.DefaultToolsFilename,
	workspace.DefaultIdentityFilename,
	workspace.DefaultUserFilename,
	workspace.DefaultHeartbeatFilename,
	workspace.DefaultBootstrapFilename,
}

// allowedFileNames 允许通过 agents.files.get/set 访问的文件名集合。
// 对应 TS ALLOWED_FILE_NAMES = Set([...BOOTSTRAP_FILE_NAMES, ...MEMORY_FILE_NAMES])
var allowedFileNames = func() map[string]bool {
	m := make(map[string]bool, len(bootstrapFileNames)+2)
	for _, name := range bootstrapFileNames {
		m[name] = true
	}
	m[workspace.DefaultMemoryFilename] = true
	m[workspace.DefaultMemoryAltFilename] = true
	return m
}()

// agentFileMeta 文件元数据。
type agentFileMeta struct {
	Size        int64 `json:"size"`
	UpdatedAtMs int64 `json:"updatedAtMs"`
}

// statAgentFile 获取文件元数据，不存在返回 nil。
func statAgentFile(filePath string) *agentFileMeta {
	info, err := os.Stat(filePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return &agentFileMeta{
		Size:        info.Size(),
		UpdatedAtMs: info.ModTime().UnixMilli(),
	}
}

// listAgentWorkspaceFiles 列出工作区引导 + 记忆文件。
// 对应 TS listAgentFiles() (agents.ts L80-L132)
func listAgentWorkspaceFiles(workspaceDir string) []map[string]interface{} {
	files := make([]map[string]interface{}, 0, len(bootstrapFileNames)+1)

	// 引导文件
	for _, name := range bootstrapFileNames {
		fp := filepath.Join(workspaceDir, name)
		meta := statAgentFile(fp)
		if meta != nil {
			files = append(files, map[string]interface{}{
				"name":        name,
				"path":        fp,
				"missing":     false,
				"size":        meta.Size,
				"updatedAtMs": meta.UpdatedAtMs,
			})
		} else {
			files = append(files, map[string]interface{}{
				"name":    name,
				"path":    fp,
				"missing": true,
			})
		}
	}

	// 记忆文件：优先 MEMORY.md，回退 memory.md
	primaryPath := filepath.Join(workspaceDir, workspace.DefaultMemoryFilename)
	primaryMeta := statAgentFile(primaryPath)
	if primaryMeta != nil {
		files = append(files, map[string]interface{}{
			"name":        workspace.DefaultMemoryFilename,
			"path":        primaryPath,
			"missing":     false,
			"size":        primaryMeta.Size,
			"updatedAtMs": primaryMeta.UpdatedAtMs,
		})
	} else {
		altPath := filepath.Join(workspaceDir, workspace.DefaultMemoryAltFilename)
		altMeta := statAgentFile(altPath)
		if altMeta != nil {
			files = append(files, map[string]interface{}{
				"name":        workspace.DefaultMemoryAltFilename,
				"path":        altPath,
				"missing":     false,
				"size":        altMeta.Size,
				"updatedAtMs": altMeta.UpdatedAtMs,
			})
		} else {
			files = append(files, map[string]interface{}{
				"name":    workspace.DefaultMemoryFilename,
				"path":    primaryPath,
				"missing": true,
			})
		}
	}

	return files
}

// AgentFilesHandlers 返回 agents.files.* 方法处理器映射。

func AgentFilesHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"agents.files.list": handleAgentFilesList,
		"agents.files.get":  handleAgentFilesGet,
		"agents.files.set":  handleAgentFilesSet,
	}
}

// ---------- agents.files.list ----------

func handleAgentFilesList(ctx *MethodHandlerContext) {
	cfg := resolveConfigFromContext(ctx)
	if cfg == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config not available"))
		return
	}

	agentIdRaw, _ := ctx.Params["agentId"].(string)
	if strings.TrimSpace(agentIdRaw) == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "agentId is required"))
		return
	}

	agentId := scope.NormalizeAgentId(agentIdRaw)
	allowed := make(map[string]bool)
	for _, id := range scope.ListAgentIds(cfg) {
		allowed[id] = true
	}
	if !allowed[agentId] {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown agent id"))
		return
	}

	workspaceDir := scope.ResolveAgentWorkspaceDir(cfg, agentId)
	files := listAgentWorkspaceFiles(workspaceDir)

	ctx.Respond(true, map[string]interface{}{
		"agentId":   agentId,
		"workspace": workspaceDir,
		"files":     files,
	}, nil)
}

// ---------- agents.files.get ----------

func handleAgentFilesGet(ctx *MethodHandlerContext) {
	cfg := resolveConfigFromContext(ctx)
	if cfg == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config not available"))
		return
	}

	agentIdRaw, _ := ctx.Params["agentId"].(string)
	if strings.TrimSpace(agentIdRaw) == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "agentId is required"))
		return
	}

	agentId := scope.NormalizeAgentId(agentIdRaw)
	allowed := make(map[string]bool)
	for _, id := range scope.ListAgentIds(cfg) {
		allowed[id] = true
	}
	if !allowed[agentId] {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown agent id"))
		return
	}

	nameRaw, _ := ctx.Params["name"].(string)
	name := strings.TrimSpace(nameRaw)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "name is required"))
		return
	}
	if !allowedFileNames[name] {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unsupported file \""+name+"\""))
		return
	}

	workspaceDir := scope.ResolveAgentWorkspaceDir(cfg, agentId)
	filePath := filepath.Join(workspaceDir, name)
	meta := statAgentFile(filePath)

	if meta == nil {
		ctx.Respond(true, map[string]interface{}{
			"agentId":   agentId,
			"workspace": workspaceDir,
			"file": map[string]interface{}{
				"name":    name,
				"path":    filePath,
				"missing": true,
			},
		}, nil)
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to read file: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"agentId":   agentId,
		"workspace": workspaceDir,
		"file": map[string]interface{}{
			"name":        name,
			"path":        filePath,
			"missing":     false,
			"size":        meta.Size,
			"updatedAtMs": meta.UpdatedAtMs,
			"content":     string(content),
		},
	}, nil)
}

// ---------- agents.files.set ----------

func handleAgentFilesSet(ctx *MethodHandlerContext) {
	cfg := resolveConfigFromContext(ctx)
	if cfg == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config not available"))
		return
	}

	agentIdRaw, _ := ctx.Params["agentId"].(string)
	if strings.TrimSpace(agentIdRaw) == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "agentId is required"))
		return
	}

	agentId := scope.NormalizeAgentId(agentIdRaw)
	allowed := make(map[string]bool)
	for _, id := range scope.ListAgentIds(cfg) {
		allowed[id] = true
	}
	if !allowed[agentId] {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unknown agent id"))
		return
	}

	nameRaw, _ := ctx.Params["name"].(string)
	name := strings.TrimSpace(nameRaw)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "name is required"))
		return
	}
	if !allowedFileNames[name] {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "unsupported file \""+name+"\""))
		return
	}

	contentRaw, _ := ctx.Params["content"].(string)

	workspaceDir := scope.ResolveAgentWorkspaceDir(cfg, agentId)
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to create workspace: "+err.Error()))
		return
	}

	filePath := filepath.Join(workspaceDir, name)
	if err := os.WriteFile(filePath, []byte(contentRaw), 0o644); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "failed to write file: "+err.Error()))
		return
	}

	meta := statAgentFile(filePath)

	result := map[string]interface{}{
		"ok":        true,
		"agentId":   agentId,
		"workspace": workspaceDir,
		"file": map[string]interface{}{
			"name":    name,
			"path":    filePath,
			"missing": false,
			"content": contentRaw,
		},
	}
	if meta != nil {
		fileMap := result["file"].(map[string]interface{})
		fileMap["size"] = meta.Size
		fileMap["updatedAtMs"] = meta.UpdatedAtMs
	}

	ctx.Respond(true, result, nil)
}
