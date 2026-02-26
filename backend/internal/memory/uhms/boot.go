package uhms

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BootFile 是系统全局启动文件，单文件记录系统地图 + 上次会话 + 检索指南。
// 路径: ~/.openacosmi/memory/boot.json
type BootFile struct {
	Version     string          `json:"version"`
	UpdatedAt   time.Time       `json:"updated_at"`
	LastSession *BootSession    `json:"last_session,omitempty"`
	SystemMap   BootSystemMap   `json:"system_map"`
	SearchGuide BootSearchGuide `json:"search_guide"`
}

type BootSession struct {
	Summary     string    `json:"summary"`
	ActiveTasks []string  `json:"active_tasks,omitempty"`
	EndedAt     time.Time `json:"ended_at"`
}

type BootSystemMap struct {
	Skills   BootSkillsInfo   `json:"skills"`
	Plugins  BootPluginsInfo  `json:"plugins"`
	Memory   BootMemoryInfo   `json:"memory"`
	Sessions BootSessionsInfo `json:"sessions,omitempty"`
}

type BootSkillsInfo struct {
	SourceDir        string    `json:"source_dir"`
	VFSDir           string    `json:"vfs_dir"`
	Categories       []string  `json:"categories,omitempty"`
	TotalCount       int       `json:"total_count"`
	Indexed          bool      `json:"indexed"`
	QdrantCollection string    `json:"qdrant_collection"`
	LastIndexedAt    time.Time `json:"last_indexed_at,omitempty"`
}

type BootPluginsInfo struct {
	Registered []string `json:"registered,omitempty"`
	Active     []string `json:"active,omitempty"`
}

type BootMemoryInfo struct {
	VFSRoot     string `json:"vfs_root"`
	SegmentData string `json:"segment_data"`
}

type BootSessionsInfo struct {
	QdrantCollection string `json:"qdrant_collection,omitempty"`
	TotalCount       int    `json:"total_count,omitempty"`
}

// BootSearchGuide 提供检索优先级指南，指导 agent 在记忆层次中的查找顺序。
type BootSearchGuide struct {
	// Priority 检索优先级列表，按顺序尝试
	Priority []SearchPriorityEntry `json:"priority"`
	// Tips 其他检索提示
	Tips []string `json:"tips,omitempty"`
}

// SearchPriorityEntry 描述一个检索层级。
type SearchPriorityEntry struct {
	Order       int    `json:"order"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// ---------- Boot file operations ----------

// LoadBootFile 从指定路径读取并反序列化 BootFile。
// 文件不存在或 JSON 损坏时返回 (nil, nil)，仅 I/O 错误（非 NotExist）返回 error。
func LoadBootFile(path string) (*BootFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read boot file %s: %w", path, err)
	}

	var boot BootFile
	if err := json.Unmarshal(data, &boot); err != nil {
		// JSON 损坏 — 容错处理，视为不存在
		return nil, nil
	}
	return &boot, nil
}

// SaveBootFile 原子写入 BootFile 到指定路径（写临时文件 → os.Rename）。
// 自动更新 UpdatedAt 字段。
func SaveBootFile(path string, boot *BootFile) error {
	if boot == nil {
		return fmt.Errorf("save boot file: boot is nil")
	}

	boot.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(boot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal boot file: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create boot file dir %s: %w", dir, err)
	}

	// 原子写: 先写临时文件，再 rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp boot file %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		// 清理临时文件（忽略错误）
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp boot file to %s: %w", path, err)
	}

	return nil
}

// UpdateBootSession 读取 → 更新 LastSession → 保存。
// 如果 boot.json 不存在，则创建一个默认 BootFile。
func UpdateBootSession(path string, session *BootSession) error {
	boot, err := LoadBootFile(path)
	if err != nil {
		return fmt.Errorf("update boot session: %w", err)
	}
	if boot == nil {
		boot = &BootFile{
			Version:     "0.2.0",
			SearchGuide: DefaultSearchGuide(),
		}
	}

	boot.LastSession = session
	return SaveBootFile(path, boot)
}

// UpdateBootSkillsInfo 读取 → 更新 SystemMap.Skills → 保存。
// 如果 boot.json 不存在，则创建一个默认 BootFile。
func UpdateBootSkillsInfo(path string, info BootSkillsInfo) error {
	boot, err := LoadBootFile(path)
	if err != nil {
		return fmt.Errorf("update boot skills info: %w", err)
	}
	if boot == nil {
		boot = &BootFile{
			Version:     "0.2.0",
			SearchGuide: DefaultSearchGuide(),
		}
	}

	boot.SystemMap.Skills = info
	return SaveBootFile(path, boot)
}

// DefaultSearchGuide 返回默认检索指南。
func DefaultSearchGuide() BootSearchGuide {
	return BootSearchGuide{
		Priority: []SearchPriorityEntry{
			{Order: 1, Source: "L0-working", Description: "当前会话工作记忆 (最新上下文)"},
			{Order: 2, Source: "L1-active", Description: "高频活跃记忆 (FTS5 全文检索)"},
			{Order: 3, Source: "L2-archive", Description: "归档长期记忆 (向量检索 / 衰减排序)"},
			{Order: 4, Source: "skills-index", Description: "技能索引 (Qdrant 语义搜索)"},
			{Order: 5, Source: "session-history", Description: "历史会话摘要 (按时间倒序)"},
		},
		Tips: []string{
			"优先使用 L0 工作记忆，避免不必要的向量检索",
			"技能检索使用语义搜索，支持自然语言查询",
			"L2 归档记忆按 FSRS-6 稳定性排序，高稳定性优先",
		},
	}
}

// ============================================================================
// BootManager — 带缓存的 Boot 文件管理器（线程安全）
// ============================================================================

// BootManager 封装 boot.json 的读写，持有内存缓存以支持 IsSkillsIndexed() 零 I/O 快速检查。
type BootManager struct {
	path    string
	mu      sync.RWMutex
	current *BootFile
}

// NewBootManager 创建 BootManager，bootFilePath 由 UHMSConfig.ResolvedBootFilePath() 提供。
func NewBootManager(bootFilePath string) *BootManager {
	return &BootManager{path: bootFilePath}
}

// Load 读取 boot.json 并缓存结果。返回 (*BootFile, true) 表示成功；(nil, false) 表示不存在或损坏。
func (bm *BootManager) Load() (*BootFile, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	boot, err := LoadBootFile(bm.path)
	if err != nil || boot == nil {
		return nil, false
	}
	bm.current = boot
	return boot, true
}

// IsSkillsIndexed 检查技能是否已分级（使用缓存，不读磁盘）。
func (bm *BootManager) IsSkillsIndexed() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.current != nil && bm.current.SystemMap.Skills.Indexed
}

// MarkSkillsIndexed 将技能分级状态写入 boot.json，并更新内存缓存。
func (bm *BootManager) MarkSkillsIndexed(totalCount int) error {
	info := BootSkillsInfo{
		SourceDir:        "docs/skills/",
		VFSDir:           "_system/skills/",
		QdrantCollection: "sys_skills",
		Indexed:          true,
		TotalCount:       totalCount,
		LastIndexedAt:    time.Now(),
	}
	if err := UpdateBootSkillsInfo(bm.path, info); err != nil {
		return err
	}
	bm.mu.Lock()
	if bm.current == nil {
		bm.current = &BootFile{}
	}
	bm.current.SystemMap.Skills = info
	bm.mu.Unlock()
	return nil
}

// UpdateLastSession 更新上次会话摘要。
func (bm *BootManager) UpdateLastSession(summary string, activeTasks []string) error {
	session := &BootSession{
		Summary:     summary,
		ActiveTasks: activeTasks,
		EndedAt:     time.Now(),
	}
	return UpdateBootSession(bm.path, session)
}

// Current 返回当前缓存的 BootFile 副本（未加载时返回 nil）。
func (bm *BootManager) Current() *BootFile {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	if bm.current == nil {
		return nil
	}
	cp := *bm.current
	return &cp
}

// Reset 删除 boot.json 并清空缓存，下次启动将触发全量重扫描。
func (bm *BootManager) Reset() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.current = nil
	if err := os.Remove(bm.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("uhms/boot: reset: %w", err)
	}
	return nil
}

// NewBootFile 创建初始 BootFile，从配置中提取系统地图信息。
func NewBootFile(cfg UHMSConfig) *BootFile {
	return &BootFile{
		Version:   "0.2.0",
		UpdatedAt: time.Now(),
		SystemMap: BootSystemMap{
			Skills: BootSkillsInfo{
				SourceDir:        "docs/skills",
				VFSDir:           filepath.Join(cfg.ResolvedVFSPath(), "skills"),
				QdrantCollection: "oa_skills",
			},
			Plugins: BootPluginsInfo{},
			Memory: BootMemoryInfo{
				VFSRoot:     cfg.ResolvedVFSPath(),
				SegmentData: filepath.Join(cfg.ResolvedVFSPath(), "segment_data"),
			},
			Sessions: BootSessionsInfo{
				QdrantCollection: "oa_sessions",
			},
		},
		SearchGuide: DefaultSearchGuide(),
	}
}
