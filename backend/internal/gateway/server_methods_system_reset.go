package gateway

// server_methods_system_reset.go — system.backup.list / system.backup.restore / system.reset.preview / system.reset
// 系统一键恢复: 配置备份恢复 + 运行时状态重置。

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/config"
	"github.com/Acosmi/ClawAcosmi/internal/infra"
)

// SystemResetHandlers 返回系统恢复/重置方法处理器。
func SystemResetHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"system.backup.list":    handleBackupList,
		"system.backup.restore": handleBackupRestore,
		"system.reset.preview":  handleResetPreview,
		"system.reset":          handleReset,
	}
}

// ---------- system.backup.list ----------
// 列出配置备份文件及其元信息。

type backupEntry struct {
	Index   int    `json:"index"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	Valid   bool   `json:"valid"`
}

func handleBackupList(ctx *MethodHandlerContext) {
	loader := ctx.Context.ConfigLoader
	if loader == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config loader not available"))
		return
	}

	base := loader.ConfigPath()
	entries := listBackupEntries(base)
	ctx.Respond(true, map[string]interface{}{
		"backups": entries,
	}, nil)
}

func listBackupEntries(configPath string) []backupEntry {
	var entries []backupEntry
	paths := backupPaths(configPath)

	for i, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		valid := isValidJSON5(p)
		entries = append(entries, backupEntry{
			Index:   i,
			Path:    p,
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
			Valid:   valid,
		})
	}
	return entries
}

// backupPaths 返回 .bak, .bak.1, .bak.2, .bak.3, .bak.4 路径列表。
func backupPaths(configPath string) []string {
	base := configPath + ".bak"
	paths := []string{base}
	for i := 1; i < config.ConfigBackupCount; i++ {
		paths = append(paths, fmt.Sprintf("%s.%d", base, i))
	}
	return paths
}

// isValidJSON5 尝试读取文件并验证是否为合法 JSON（简单检查）。
func isValidJSON5(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw interface{}
	return json.Unmarshal(data, &raw) == nil
}

// ---------- system.backup.restore ----------
// 从指定备份恢复配置。

func handleBackupRestore(ctx *MethodHandlerContext) {
	loader := ctx.Context.ConfigLoader
	if loader == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config loader not available"))
		return
	}

	// 解析 index 参数
	idxRaw, ok := ctx.Params["index"]
	if !ok {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "index required"))
		return
	}
	idxFloat, ok := idxRaw.(float64)
	if !ok {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "index must be a number"))
		return
	}
	idx := int(idxFloat)
	if idx < 0 || idx >= config.ConfigBackupCount {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, fmt.Sprintf("index out of range [0, %d)", config.ConfigBackupCount)))
		return
	}

	configPath := loader.ConfigPath()
	paths := backupPaths(configPath)
	bakPath := paths[idx]

	// 验证备份文件存在
	if _, err := os.Stat(bakPath); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, fmt.Sprintf("backup %d not found", idx)))
		return
	}

	// 验证备份 JSON 有效
	if !isValidJSON5(bakPath) {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, fmt.Sprintf("backup %d is not valid JSON", idx)))
		return
	}

	// 原子写入: tmpFile + rename（与 config.WriteConfigFile 一致）
	data, err := os.ReadFile(bakPath)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("read backup: %v", err)))
		return
	}
	if err := atomicWriteFile(configPath, data, 0600); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("write config: %v", err)))
		return
	}

	// 清除缓存，触发下次 LoadConfig 重新读取
	loader.ClearCache()

	// 广播配置已恢复事件
	if ctx.Context.Broadcaster != nil {
		ctx.Context.Broadcaster.Broadcast("system.config.restored", map[string]interface{}{
			"restoredFrom": idx,
			"timestamp":    time.Now().Format(time.RFC3339),
		}, nil)
	}

	slog.Info("config restored from backup", slog.Int("index", idx), slog.String("path", bakPath))
	ctx.Respond(true, map[string]interface{}{
		"ok":           true,
		"restoredFrom": idx,
	}, nil)
}

// ---------- system.reset.preview ----------
// 预览运行时重置将清除的文件。

type resetFileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Exists bool   `json:"exists"`
	Action string `json:"action"` // "delete" | "truncate"
}

func handleResetPreview(ctx *MethodHandlerContext) {
	entries := collectResetTargets()
	ctx.Respond(true, map[string]interface{}{
		"level":   1,
		"targets": entries,
	}, nil)
}

func collectResetTargets() []resetFileEntry {
	var targets []resetFileEntry

	// 1. exec-approvals.json
	eaPath := infra.ResolveExecApprovalsPath()
	targets = append(targets, fileResetEntry(eaPath, "delete"))

	// 2. escalation-audit.log
	auditPath := resolveEscalationAuditPath()
	targets = append(targets, fileResetEntry(auditPath, "truncate"))

	// 3. UHMS boot.json
	bootPath := resolveUHMSBootPath()
	targets = append(targets, fileResetEntry(bootPath, "delete"))

	return targets
}

func fileResetEntry(path, action string) resetFileEntry {
	info, err := os.Stat(path)
	if err != nil {
		return resetFileEntry{Path: path, Size: 0, Exists: false, Action: action}
	}
	return resetFileEntry{Path: path, Size: info.Size(), Exists: true, Action: action}
}

func resolveEscalationAuditPath() string {
	return filepath.Join(config.ResolveStateDir(), "escalation-audit.log")
}

func resolveUHMSBootPath() string {
	return filepath.Join(config.ResolveStateDir(), "memory", "boot.json")
}

// atomicWriteFile 原子写入: 先写 tmpFile 再 rename，与 config.WriteConfigFile 一致。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, fmt.Sprintf("%s.%d.tmp", filepath.Base(path), os.Getpid()))
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		// fallback: 直接复制
		if cpErr := os.WriteFile(path, data, perm); cpErr != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("rename failed: %w (copy fallback also failed: %v)", err, cpErr)
		}
		_ = os.Remove(tmp)
	}
	return nil
}

// ---------- system.reset ----------
// 执行运行时重置。

func handleReset(ctx *MethodHandlerContext) {
	var errors []string

	// 1. 删除 exec-approvals.json
	eaPath := infra.ResolveExecApprovalsPath()
	if err := os.Remove(eaPath); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("exec-approvals: %v", err))
	}

	// 2. 截断 escalation-audit.log
	auditPath := resolveEscalationAuditPath()
	if err := os.Truncate(auditPath, 0); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("escalation-audit: %v", err))
	}

	// 3. 重置 EscalationManager 内存状态
	if ctx.Context.EscalationMgr != nil {
		ctx.Context.EscalationMgr.Reset()
	}

	// 4. 删除 UHMS boot.json（触发下次重扫描）
	bootPath := resolveUHMSBootPath()
	if err := os.Remove(bootPath); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("uhms-boot: %v", err))
	}

	if len(errors) > 0 {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("partial reset: %v", errors)))
		return
	}

	// 广播重置完成事件
	if ctx.Context.Broadcaster != nil {
		ctx.Context.Broadcaster.Broadcast("system.reset.done", map[string]interface{}{
			"level":     1,
			"timestamp": time.Now().Format(time.RFC3339),
		}, nil)
	}

	slog.Info("runtime reset completed", slog.Int("level", 1))
	ctx.Respond(true, map[string]interface{}{
		"ok":    true,
		"level": 1,
	}, nil)
}
