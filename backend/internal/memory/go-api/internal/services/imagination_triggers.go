// Package services — 想象记忆事件驱动触发器。
// 参考: Mem0 消息对触发 + Zep/Graphiti 增量社区检测 + 认知科学事件型前瞻记忆。
// 三种触发器：活跃度突增、新实体簇形成、话题漂移检测。
package services

import (
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// ImaginationTrigger 定义事件驱动的想象记忆触发接口。
// 每种触发器独立检测条件，命中任一即可触发 ImaginationEngine.Run()。
type ImaginationTrigger interface {
	// ShouldTrigger 检查指定用户是否满足触发条件。
	ShouldTrigger(db *gorm.DB, userID string) bool
	// Name 返回触发器标识名。
	Name() string
}

// ============================================================================
// Trigger A: ActivityBurstTrigger — 活跃度突增检测
// ============================================================================

// ActivityBurstTrigger 检测用户短时间内密集写入记忆的活跃度突增。
// 检测条件: 过去 1 小时内写入 ≥ 5 条记忆（正常基线 ~1-2 条/小时）。
// 冷却: 6 小时内不重复触发。
// 来源: Mem0 消息对模式 + 认知科学事件型前瞻记忆。
type ActivityBurstTrigger struct {
	Threshold    int           // 触发阈值（默认 5）
	Window       time.Duration // 检测窗口（默认 1 小时）
	CooldownTime time.Duration // 冷却时间（默认 6 小时）
	lastTrigger  map[string]time.Time
}

// NewActivityBurstTrigger 创建活跃度突增触发器（使用默认参数）。
func NewActivityBurstTrigger() *ActivityBurstTrigger {
	return &ActivityBurstTrigger{
		Threshold:    5,
		Window:       1 * time.Hour,
		CooldownTime: 6 * time.Hour,
		lastTrigger:  make(map[string]time.Time),
	}
}

func (t *ActivityBurstTrigger) Name() string { return "activity_burst" }

func (t *ActivityBurstTrigger) ShouldTrigger(db *gorm.DB, userID string) bool {
	if db == nil || userID == "" {
		return false
	}

	// 冷却检查
	if last, ok := t.lastTrigger[userID]; ok && time.Since(last) < t.CooldownTime {
		return false
	}

	// 统计检测窗口内的新记忆数
	cutoff := time.Now().UTC().Add(-t.Window)
	var count int64
	if err := db.Model(&models.Memory{}).
		Where("user_id = ? AND created_at >= ?", userID, cutoff).
		Count(&count).Error; err != nil {
		slog.Warn("ActivityBurstTrigger: count query failed", "user_id", userID, "error", err)
		return false
	}

	if count >= int64(t.Threshold) {
		t.lastTrigger[userID] = time.Now()
		slog.Info("ActivityBurstTrigger: 触发", "user_id", userID, "count", count, "threshold", t.Threshold)
		return true
	}
	return false
}

// ============================================================================
// Trigger B: EntityClusterTrigger — 新实体簇形成检测
// ============================================================================

// EntityClusterTrigger 检测知识图谱中新增的高连接实体簇。
// 检测条件: 过去 24h 内新增实体 ≥ 3 个且属于同一 community_id。
// 冷却: 24 小时。
// 来源: Zep/Graphiti 增量社区检测。
type EntityClusterTrigger struct {
	MinEntities  int           // 最少新实体数（默认 3）
	Window       time.Duration // 检测窗口（默认 24 小时）
	CooldownTime time.Duration // 冷却时间（默认 24 小时）
	lastTrigger  map[string]time.Time
}

// NewEntityClusterTrigger 创建新实体簇触发器。
func NewEntityClusterTrigger() *EntityClusterTrigger {
	return &EntityClusterTrigger{
		MinEntities:  3,
		Window:       24 * time.Hour,
		CooldownTime: 24 * time.Hour,
		lastTrigger:  make(map[string]time.Time),
	}
}

func (t *EntityClusterTrigger) Name() string { return "entity_cluster" }

func (t *EntityClusterTrigger) ShouldTrigger(db *gorm.DB, userID string) bool {
	if db == nil || userID == "" {
		return false
	}

	// 冷却检查
	if last, ok := t.lastTrigger[userID]; ok && time.Since(last) < t.CooldownTime {
		return false
	}

	cutoff := time.Now().UTC().Add(-t.Window)

	// 查找在时间窗口内新增的实体，按 community_id 分组
	// 找到任何一个 community 有 >= MinEntities 个新实体即触发
	type clusterCount struct {
		CommunityID int
		Count       int64
	}
	var clusters []clusterCount
	if err := db.Model(&models.Entity{}).
		Select("community_id, COUNT(*) as count").
		Where("user_id = ? AND created_at >= ? AND community_id IS NOT NULL", userID, cutoff).
		Group("community_id").
		Having("COUNT(*) >= ?", t.MinEntities).
		Scan(&clusters).Error; err != nil {
		slog.Warn("EntityClusterTrigger: query failed", "user_id", userID, "error", err)
		return false
	}

	if len(clusters) > 0 {
		t.lastTrigger[userID] = time.Now()
		slog.Info("EntityClusterTrigger: 触发",
			"user_id", userID,
			"clusters", len(clusters),
			"top_cluster_size", clusters[0].Count,
		)
		return true
	}
	return false
}

// ============================================================================
// Trigger C: TopicDriftTrigger — 话题漂移检测
// ============================================================================

// TopicDriftTrigger 检测用户关注点的领域转移。
// 检测条件: 最近 10 条记忆的 top category 与之前 50 条的 top category 不同。
// 冷却: 12 小时。
// 来源: Mem0 提取-更新管线中的冲突检测思想。
type TopicDriftTrigger struct {
	RecentCount  int           // 最近记忆数（默认 10）
	BaseCount    int           // 基线记忆数（默认 50）
	CooldownTime time.Duration // 冷却时间（默认 12 小时）
	lastTrigger  map[string]time.Time
}

// NewTopicDriftTrigger 创建话题漂移触发器。
func NewTopicDriftTrigger() *TopicDriftTrigger {
	return &TopicDriftTrigger{
		RecentCount:  10,
		BaseCount:    50,
		CooldownTime: 12 * time.Hour,
		lastTrigger:  make(map[string]time.Time),
	}
}

func (t *TopicDriftTrigger) Name() string { return "topic_drift" }

func (t *TopicDriftTrigger) ShouldTrigger(db *gorm.DB, userID string) bool {
	if db == nil || userID == "" {
		return false
	}

	// 冷却检查
	if last, ok := t.lastTrigger[userID]; ok && time.Since(last) < t.CooldownTime {
		return false
	}

	// 获取最近 (RecentCount + BaseCount) 条记忆的 category
	totalNeeded := t.RecentCount + t.BaseCount
	var categories []string
	if err := db.Model(&models.Memory{}).
		Select("category").
		Where("user_id = ? AND memory_type NOT IN ?", userID, ProtectedMemoryTypes).
		Order("created_at DESC").
		Limit(totalNeeded).
		Pluck("category", &categories).Error; err != nil {
		slog.Warn("TopicDriftTrigger: query failed", "user_id", userID, "error", err)
		return false
	}

	// 数据不足，无法判断漂移
	if len(categories) < t.RecentCount+t.BaseCount {
		return false
	}

	// 计算最近 RecentCount 条的 top category
	recentCats := categories[:t.RecentCount]
	baseCats := categories[t.RecentCount:]

	recentTop := topCategory(recentCats)
	baseTop := topCategory(baseCats)

	if recentTop != "" && baseTop != "" && recentTop != baseTop {
		t.lastTrigger[userID] = time.Now()
		slog.Info("TopicDriftTrigger: 触发",
			"user_id", userID,
			"recent_top", recentTop,
			"base_top", baseTop,
		)
		return true
	}
	return false
}

// topCategory 返回类别列表中出现次数最多的类别。
func topCategory(categories []string) string {
	counts := make(map[string]int)
	for _, c := range categories {
		if c != "" {
			counts[c]++
		}
	}
	maxCount := 0
	maxCat := ""
	for cat, count := range counts {
		if count > maxCount {
			maxCount = count
			maxCat = cat
		}
	}
	return maxCat
}
