// Package services — 记忆压缩与主动遗忘。
// 定期压缩老旧记忆，降低 Token 消耗。
// Rust 预留：压缩算法可后续迁移到 nexus-decay。
package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// 压缩配置常量
const (
	ArchiveThreshold     = 0.05 // decay_factor 低于此值 → 归档候选
	ArchiveInactiveDays  = 30   // 未访问天数超过此值 → 归档候选
	CompressionBatchSize = 20   // 每次压缩处理的最大记忆数
	MinMergeGroupSize    = 2    // 最少几条记忆才触发合并
	DefaultTickerMinutes = 60   // 默认压缩周期（分钟）
)

// CompressionStats 记录一次压缩周期的统计结果。
type CompressionStats struct {
	Archived       int     `json:"archived"`         // 被归档的记忆数
	Merged         int     `json:"merged"`           // 被合并的记忆数
	CreatedSummary int     `json:"created_summary"`  // 新建的摘要数
	TokensSaved    int     `json:"tokens_saved"`     // 估算节省的 Token 数
	Duration       float64 `json:"duration_seconds"` // 耗时（秒）
}

// MemoryCompressor 负责定期压缩和归档记忆。
type MemoryCompressor struct {
	db          *gorm.DB
	llm         LLMProvider
	vectorStore *VectorStoreService
}

// NewMemoryCompressor 创建压缩器实例。
func NewMemoryCompressor(db *gorm.DB, llm LLMProvider, vs *VectorStoreService) *MemoryCompressor {
	return &MemoryCompressor{db: db, llm: llm, vectorStore: vs}
}

// RunCompressionCycle 执行一次完整的压缩周期。
// 1. 归档过期记忆  2. 合并相似低衰减记忆  3. 返回统计
func (mc *MemoryCompressor) RunCompressionCycle(ctx context.Context, userID string) (*CompressionStats, error) {
	start := time.Now()
	stats := &CompressionStats{}

	// 阶段 1：归档过期记忆
	archived, err := mc.ArchiveExpiredMemories(userID)
	if err != nil {
		slog.Warn("归档阶段出错", "error", err, "user_id", userID)
	}
	stats.Archived = archived

	// 阶段 2：合并相似低衰减记忆
	merged, summaries, tokensSaved, err := mc.MergeSimilarMemories(ctx, userID)
	if err != nil {
		slog.Warn("合并阶段出错", "error", err, "user_id", userID)
	}
	stats.Merged = merged
	stats.CreatedSummary = summaries
	stats.TokensSaved = tokensSaved

	stats.Duration = time.Since(start).Seconds()
	slog.Info("压缩周期完成",
		"user_id", userID,
		"archived", stats.Archived,
		"merged", stats.Merged,
		"tokens_saved", stats.TokensSaved,
		"duration_s", stats.Duration,
	)
	return stats, nil
}

// ArchiveExpiredMemories 归档过期记忆（soft delete）。
// 条件：decay_factor < ArchiveThreshold 且 超过 ArchiveInactiveDays 未访问。
func (mc *MemoryCompressor) ArchiveExpiredMemories(userID string) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -ArchiveInactiveDays)

	result := mc.db.Model(&models.Memory{}).
		Where("user_id = ? AND decay_factor < ? AND (last_accessed_at IS NULL OR last_accessed_at < ?) AND memory_type NOT IN ?",
			userID, ArchiveThreshold, cutoff, append([]string{MemoryTypeReflection}, ProtectedMemoryTypes...)).
		Update("memory_type", "archived")
	if result.Error != nil {
		return 0, fmt.Errorf("archive memories: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		slog.Info("归档过期记忆", "user_id", userID, "count", count)
	}
	return count, nil
}

// MergeSimilarMemories 合并相似的低衰减记忆为摘要。
// 返回 (合并数, 新摘要数, token节省量, error)。
func (mc *MemoryCompressor) MergeSimilarMemories(
	ctx context.Context, userID string,
) (int, int, int, error) {
	// 获取合并候选
	candidates, err := GetConsolidationCandidates(mc.db, userID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("get consolidation candidates: %w", err)
	}
	if len(candidates) < MinMergeGroupSize {
		return 0, 0, 0, nil
	}

	// 限制批次大小
	if len(candidates) > CompressionBatchSize {
		candidates = candidates[:CompressionBatchSize]
	}

	// 按类别分组
	groups := make(map[string][]models.Memory)
	for _, m := range candidates {
		groups[m.Category] = append(groups[m.Category], m)
	}

	totalMerged := 0
	totalSummaries := 0
	totalTokensSaved := 0

	for category, group := range groups {
		if len(group) < MinMergeGroupSize {
			continue
		}

		// 构建内容文本
		var contents []string
		var ids []uuid.UUID
		originalTokens := 0
		for _, m := range group {
			contents = append(contents, m.Content)
			ids = append(ids, m.ID)
			originalTokens += len(m.Content) / 4 // 粗略估算
		}

		// LLM 摘要合并
		summary, err := mc.summarizeMemories(ctx, contents, category)
		if err != nil {
			slog.Warn("LLM 摘要失败", "category", category, "error", err)
			continue
		}

		// 创建摘要记忆
		summaryMemory := &models.Memory{
			Content:         summary,
			UserID:          userID,
			MemoryType:      MemoryTypeReflection,
			Category:        CategorySummary,
			ImportanceScore: 0.6,
		}
		if err := mc.db.Create(summaryMemory).Error; err != nil {
			slog.Warn("创建摘要记忆失败", "error", err)
			continue
		}

		// soft delete 原始记忆
		mc.db.Model(&models.Memory{}).Where("id IN ?", ids).
			Update("memory_type", "archived")

		summaryTokens := len(summary) / 4
		saved := originalTokens - summaryTokens
		if saved < 0 {
			saved = 0
		}

		totalMerged += len(group)
		totalSummaries++
		totalTokensSaved += saved
	}

	return totalMerged, totalSummaries, totalTokensSaved, nil
}

// GetCompressionStats 获取用户的压缩统计信息。
func (mc *MemoryCompressor) GetCompressionStats(userID string) (*CompressionStats, error) {
	stats := &CompressionStats{}

	// 统计已归档数
	var archivedCount int64
	mc.db.Model(&models.Memory{}).
		Where("user_id = ? AND memory_type = ?", userID, "archived").
		Count(&archivedCount)
	stats.Archived = int(archivedCount)

	// 统计摘要数
	var summaryCount int64
	mc.db.Model(&models.Memory{}).
		Where("user_id = ? AND category = ?", userID, CategorySummary).
		Count(&summaryCount)
	stats.CreatedSummary = int(summaryCount)

	return stats, nil
}

// --- Internal ---

const summarizePrompt = `Summarize these %d related memories (category: %s) into a single concise statement that preserves all key information.

Memories:
%s

Respond with ONLY the summary text, no explanation.`

func (mc *MemoryCompressor) summarizeMemories(
	ctx context.Context, contents []string, category string,
) (string, error) {
	if mc.llm == nil {
		// 无 LLM 时简单拼接
		return strings.Join(contents, "; "), nil
	}

	memoriesText := ""
	for i, c := range contents {
		memoriesText += fmt.Sprintf("%d. %s\n", i+1, c)
	}

	prompt := fmt.Sprintf(summarizePrompt, len(contents), category, memoriesText)
	result, err := mc.llm.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM summarize: %w", err)
	}

	return strings.TrimSpace(result), nil
}
