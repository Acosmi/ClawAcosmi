package skills

// refresh.go — 技能快照版本管理 + 文件监视
// 对应 TS: agents/skills/refresh.ts (185L)
//
// 管理 skills 快照版本号，支持文件监视和事件通知。

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// SkillsChangeEvent 技能变更事件。
type SkillsChangeEvent struct {
	WorkspaceDir string
	Reason       string // "watch"|"manual"|"remote-node"
	ChangedPath  string
}

// SkillsChangeListener 技能变更监听器。
type SkillsChangeListener func(event SkillsChangeEvent)

var (
	mu                sync.RWMutex
	listeners         []SkillsChangeListener
	workspaceVersions = make(map[string]int64)
	globalVersion     int64
)

// DefaultSkillsWatchIgnored 默认忽略的目录模式。
var DefaultSkillsWatchIgnored = []string{
	".git", "node_modules", "dist",
	".venv", "venv", "__pycache__",
	".mypy_cache", ".pytest_cache",
	"build", ".cache",
}

// RegisterSkillsChangeListener 注册技能变更监听器。
// 对应 TS: registerSkillsChangeListener
func RegisterSkillsChangeListener(listener SkillsChangeListener) func() {
	mu.Lock()
	defer mu.Unlock()
	listeners = append(listeners, listener)
	idx := len(listeners) - 1
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if idx < len(listeners) {
			listeners = append(listeners[:idx], listeners[idx+1:]...)
		}
	}
}

// BumpSkillsSnapshotVersion 提升技能快照版本号。
// 对应 TS: bumpSkillsSnapshotVersion
func BumpSkillsSnapshotVersion(workspaceDir string, reason string, changedPath string) int64 {
	mu.Lock()
	defer mu.Unlock()

	if reason == "" {
		reason = "manual"
	}

	now := time.Now().UnixMilli()

	if workspaceDir != "" {
		current := workspaceVersions[workspaceDir]
		next := now
		if now <= current {
			next = current + 1
		}
		workspaceVersions[workspaceDir] = next
		emitLocked(SkillsChangeEvent{WorkspaceDir: workspaceDir, Reason: reason, ChangedPath: changedPath})
		return next
	}

	if now <= globalVersion {
		globalVersion++
	} else {
		globalVersion = now
	}
	emitLocked(SkillsChangeEvent{Reason: reason, ChangedPath: changedPath})
	return globalVersion
}

// GetSkillsSnapshotVersion 获取技能快照版本号。
// 对应 TS: getSkillsSnapshotVersion
func GetSkillsSnapshotVersion(workspaceDir string) int64 {
	mu.RLock()
	defer mu.RUnlock()

	if workspaceDir == "" {
		return globalVersion
	}
	local := workspaceVersions[workspaceDir]
	if local > globalVersion {
		return local
	}
	return globalVersion
}

func emitLocked(event SkillsChangeEvent) {
	for _, l := range listeners {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("skills change listener panicked", "error", r)
				}
			}()
			l(event)
		}()
	}
}

// ResolveWatchPaths 解析需要监视的路径列表。
// 对应 TS: resolveWatchPaths
func ResolveWatchPaths(workspaceDir string, config *types.OpenAcosmiConfig) []string {
	var paths []string
	if workspaceDir != "" {
		paths = append(paths, filepath.Join(workspaceDir, "skills"))
	}
	// 用户级技能目录
	if home, err := homeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".config", "openacosmi", "skills"))
	}
	// 额外目录
	if config != nil && config.Skills != nil && config.Skills.Load != nil {
		for _, dir := range config.Skills.Load.ExtraDirs {
			if dir != "" {
				paths = append(paths, dir)
			}
		}
	}
	return paths
}

// EnsureSkillsWatcher 确保技能文件监视器运行。
// 对应 TS: ensureSkillsWatcher
//
// 注意: Go 版本使用轮询而非 chokidar（与 TS 版本的差异）。
// 完整的 fsnotify 集成属于 Phase 14+。
func EnsureSkillsWatcher(workspaceDir string, config *types.OpenAcosmiConfig) {
	if workspaceDir == "" {
		return
	}

	watchEnabled := true
	if config != nil && config.Skills != nil && config.Skills.Load != nil {
		if config.Skills.Load.Watch != nil && !*config.Skills.Load.Watch {
			watchEnabled = false
		}
	}
	if !watchEnabled {
		return
	}

	// 当前为 stub 日志 — 完整 fsnotify 实现在 Phase 14+
	slog.Debug("skills watcher: would watch paths",
		"workspaceDir", workspaceDir,
		"paths", ResolveWatchPaths(workspaceDir, config))
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	return home, err
}
