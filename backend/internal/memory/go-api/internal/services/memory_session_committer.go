// Package services — SessionCommitter: 会话自动提交记忆提炼服务。
//
// 借鉴 ByteDance OpenViking 的 session.commit() 三步机制：
//  1. 对话压缩 (Archive)  → LLM 结构化摘要 → 写入 VFS session/archives/
//  2. 记忆提取 (Extract)   → LLM 6 分类候选记忆
//  3. 去重决策 (Dedup)     → 向量预过滤 + skip/create/merge
//  4. 写入    (Persist)    → DB + Vector + VFS
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/models"
)

// --- 6 分类记忆提取（OpenViking 风格）---

// MemoryCandidate 表示 LLM 从对话中提取的候选记忆。
type MemoryCandidate struct {
	Content  string `json:"content"`  // 记忆内容
	Category string `json:"category"` // 6 分类之一
	Owner    string `json:"owner"`    // "user" | "agent"
}

// SessionCommitResult 记录一次 session.commit() 的执行结果。
type SessionCommitResult struct {
	ArchiveIndex   int     `json:"archive_index"`    // 本次归档索引
	SummaryContent string  `json:"summary_content"`  // 对话摘要
	ExtractedCount int     `json:"extracted_count"`  // LLM 提取的候选数
	CreatedCount   int     `json:"created_count"`    // 新创建的记忆数
	MergedCount    int     `json:"merged_count"`     // 合并更新的记忆数
	SkippedCount   int     `json:"skipped_count"`    // 跳过的重复记忆数
	Duration       float64 `json:"duration_seconds"` // 耗时（秒）
}

// SessionCommitter 负责会话结束时的自动记忆提炼。
// 与 MemoryArchiver（ProMem 4 维度冷归档）互补，提供 6 分类热提炼 + 去重。
type SessionCommitter struct {
	db          *gorm.DB
	llm         LLMProvider
	vectorStore *VectorStoreService
	fsStore     *FSStoreService // 可选：VFS 文件系统存储
	storageMode string          // "vector" | "fs" | "hybrid"
}

// NewSessionCommitter 创建 SessionCommitter 实例。
func NewSessionCommitter(db *gorm.DB, llm LLMProvider, vs *VectorStoreService) *SessionCommitter {
	return &SessionCommitter{
		db:          db,
		llm:         llm,
		vectorStore: vs,
		storageMode: StorageModeVector,
	}
}

// WithFSStore 挂载文件系统存储并设置存储模式。
func (sc *SessionCommitter) WithFSStore(fs *FSStoreService, mode string) *SessionCommitter {
	sc.fsStore = fs
	if mode != "" {
		sc.storageMode = mode
	}
	return sc
}

// --- LLM Prompt 模板 ---

// 对话摘要 Prompt — 结构化输出
const sessionSummaryPrompt = `你是一个高效的对话摘要助手。请对以下对话内容生成结构化摘要。

对话内容：
%s

请严格以 JSON 格式返回：
{
  "one_line": "一行核心总结（15字以内）",
  "analysis": "详细分析（主要讨论了什么，达成了什么共识）",
  "core_intent": "用户的核心意图",
  "key_concepts": ["关键概念1", "关键概念2"],
  "action_items": ["待办1", "待办2"]
}

仅返回 JSON，不要附加解释。`

// SessionSummary 结构化会话摘要。
type SessionSummary struct {
	OneLine     string   `json:"one_line"`
	Analysis    string   `json:"analysis"`
	CoreIntent  string   `json:"core_intent"`
	KeyConcepts []string `json:"key_concepts"`
	ActionItems []string `json:"action_items"`
}

// 记忆提取 Prompt — 6 分类
const memoryExtractionPrompt = `你是一个记忆提取助手。请从以下对话中提取值得长期记忆的信息。

对话内容：
%s

请将提取的信息分为以下 6 个类别：
1. profile — 用户身份属性（如姓名、职业、角色）
2. preferences — 用户偏好（如工具偏好、风格偏好）
3. entities — 涉及的重要实体（人、项目、组织）
4. events — 具体事件和决策
5. cases — 问题及其解决方案（经验总结）
6. patterns — 可复用的模式、技巧或最佳实践

每条记忆标注归属方：user（用户相关）或 agent（助手经验）。

以 JSON 数组格式返回：
[
  {"content": "记忆内容", "category": "profile", "owner": "user"},
  {"content": "使用 RRF 融合搜索效果更好", "category": "patterns", "owner": "agent"}
]

如果没有值得提取的记忆，返回空数组 []。仅返回 JSON。`

// --- 核心方法 ---

// Commit 执行会话提交：对话压缩 → 记忆提取 → 去重决策 → 写入。
// 这是异步安全的方法，不会阻塞调用方。
func (sc *SessionCommitter) Commit(
	ctx context.Context,
	sessionID, userID string,
	messages string,
) (*SessionCommitResult, error) {
	start := time.Now()

	if strings.TrimSpace(messages) == "" {
		return &SessionCommitResult{}, nil
	}

	result := &SessionCommitResult{}

	// ── 步骤一：对话压缩 ──────────────────────────────
	summary, err := sc.compressDialogue(ctx, messages)
	if err != nil {
		slog.Warn("会话压缩失败，继续提取", "error", err, "session_id", sessionID)
		// 降级：使用原始消息前 200 字作为摘要
		oneLine := messages
		if len(oneLine) > 200 {
			oneLine = oneLine[:200] + "…"
		}
		summary = &SessionSummary{OneLine: oneLine, Analysis: messages}
	}
	result.SummaryContent = summary.OneLine

	// 写入 VFS archive 目录
	sc.writeArchive(ctx, userID, sessionID, summary, result)

	// ── 步骤二：记忆提取 ──────────────────────────────
	candidates, err := sc.extractMemories(ctx, messages)
	if err != nil {
		slog.Warn("记忆提取失败", "error", err, "session_id", sessionID)
		result.Duration = time.Since(start).Seconds()
		return result, nil
	}
	result.ExtractedCount = len(candidates)

	if len(candidates) == 0 {
		slog.Info("会话提交：无可提取记忆", "session_id", sessionID)
		result.Duration = time.Since(start).Seconds()
		return result, nil
	}

	// ── 步骤三：去重决策 ──────────────────────────────
	// 将候选转为 Fact 格式以复用现有 DeduplicateFacts 管线
	facts := make([]Fact, len(candidates))
	for i, c := range candidates {
		facts[i] = Fact{
			Content:    c.Content,
			Category:   sc.mapCategoryToInternal(c.Category),
			Confidence: 0.8,
		}
	}

	dedupResult := DeduplicateFacts(ctx, facts, userID, sc.vectorStore, 0.85)

	// 合并冲突事实
	if len(dedupResult.UpdatePairs) > 0 {
		merged := MergeFacts(ctx, sc.llm, dedupResult.UpdatePairs)
		for _, pair := range merged {
			if pair.ExistingID != "" {
				targetUUID, parseErr := uuid.Parse(pair.ExistingID)
				if parseErr == nil {
					if dbErr := sc.db.Model(&models.Memory{}).Where("id = ?", targetUUID).
						Update("content", pair.NewFact.Content).Error; dbErr != nil {
						slog.Warn("会话提交：合并更新 DB 失败", "id", targetUUID, "error", dbErr)
					} else {
						result.MergedCount++
						// 重新向量化
						if sc.vectorStore != nil {
							go func(id uuid.UUID, content string) {
								_ = sc.vectorStore.AddMemory(
									context.Background(), id, content,
									userID, MemoryTypeEpisodic, 0.6, nil,
								)
							}(targetUUID, pair.NewFact.Content)
						}
					}
				}
			}
		}
	}
	result.SkippedCount = dedupResult.SkippedCount

	// ── 步骤四：写入新记忆 ─────────────────────────────
	for _, fact := range dedupResult.NewFacts {
		if sc.db == nil {
			slog.Warn("会话提交：DB 未初始化，跳过写入", "content", fact.Content)
			result.CreatedCount++
			continue
		}

		memory := &models.Memory{
			Content:         fact.Content,
			UserID:          userID,
			MemoryType:      MemoryTypeEpisodic,
			Category:        fact.Category,
			ImportanceScore: 0.6,
		}
		metadata := map[string]any{
			"source":     "session_commit",
			"session_id": sessionID,
		}
		memory.Metadata = &metadata

		if err := sc.db.Create(memory).Error; err != nil {
			slog.Warn("会话提交：创建记忆失败", "error", err)
			continue
		}
		result.CreatedCount++

		// 异步写入向量存储
		if sc.vectorStore != nil {
			go func(m *models.Memory) {
				_ = sc.vectorStore.AddMemory(
					context.Background(), m.ID, m.Content,
					userID, m.MemoryType, m.ImportanceScore, nil,
				)
			}(memory)
		}

		// 异步写入 VFS
		if (sc.storageMode == StorageModeFS || sc.storageMode == StorageModeHybrid) && sc.fsStore != nil {
			go func(m *models.Memory, cat string) {
				section, subCat := memoryTypeToVFSPath(m.MemoryType)
				// 使用候选的原始分类作为子目录（如果匹配 VFS 结构）
				tenantID := middleware.TenantFromCtx(ctx)
				l0 := m.Content
				if len(l0) > 80 {
					l0 = l0[:80] + "…"
				}
				if writeErr := sc.fsStore.WriteMemoryTo(
					context.Background(), tenantID, userID, m.ID,
					section, subCat, m.Content, l0, m.Content,
				); writeErr != nil {
					slog.Error("会话提交 VFS 写入失败", "error", writeErr)
				}
			}(memory, fact.Category)
		}
	}

	result.Duration = time.Since(start).Seconds()

	slog.Info("会话提交完成",
		"session_id", sessionID,
		"user_id", userID,
		"extracted", result.ExtractedCount,
		"created", result.CreatedCount,
		"merged", result.MergedCount,
		"skipped", result.SkippedCount,
		"duration_s", result.Duration,
	)

	return result, nil
}

// --- 内部方法 ---

// compressDialogue 使用 LLM 生成结构化对话摘要。
func (sc *SessionCommitter) compressDialogue(ctx context.Context, messages string) (*SessionSummary, error) {
	if sc.llm == nil {
		// 无 LLM 时降级
		oneLine := messages
		if len(oneLine) > 100 {
			oneLine = oneLine[:100] + "…"
		}
		return &SessionSummary{OneLine: oneLine, Analysis: messages}, nil
	}

	prompt := fmt.Sprintf(sessionSummaryPrompt, messages)
	raw, err := sc.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM 对话摘要: %w", err)
	}

	var summary SessionSummary
	raw = strings.TrimSpace(raw)
	// 尝试提取 JSON
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return nil, fmt.Errorf("解析摘要 JSON: %w (raw: %s)", err, raw)
	}
	return &summary, nil
}

// extractMemories 使用 LLM 从对话中提取 6 分类候选记忆。
func (sc *SessionCommitter) extractMemories(ctx context.Context, messages string) ([]MemoryCandidate, error) {
	if sc.llm == nil {
		return nil, nil
	}

	prompt := fmt.Sprintf(memoryExtractionPrompt, messages)
	raw, err := sc.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM 记忆提取: %w", err)
	}

	raw = strings.TrimSpace(raw)
	// 尝试提取 JSON 数组
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var candidates []MemoryCandidate
	if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
		return nil, fmt.Errorf("解析候选记忆 JSON: %w (raw: %s)", err, raw)
	}

	// 过滤空内容
	valid := make([]MemoryCandidate, 0, len(candidates))
	for _, c := range candidates {
		c.Content = strings.TrimSpace(c.Content)
		if c.Content != "" {
			valid = append(valid, c)
		}
	}

	return valid, nil
}

// writeArchive 将对话摘要写入 VFS session/archives/ 目录。
func (sc *SessionCommitter) writeArchive(
	ctx context.Context,
	userID, sessionID string,
	summary *SessionSummary,
	result *SessionCommitResult,
) {
	if sc.fsStore == nil || sc.storageMode == StorageModeVector {
		return
	}

	tenantID := middleware.TenantFromCtx(ctx)

	// 构建 L1 概述
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("会话 ID: %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("核心意图: %s\n", summary.CoreIntent))
	sb.WriteString(fmt.Sprintf("分析: %s\n", summary.Analysis))
	if len(summary.KeyConcepts) > 0 {
		sb.WriteString("关键概念: " + strings.Join(summary.KeyConcepts, ", ") + "\n")
	}
	if len(summary.ActionItems) > 0 {
		sb.WriteString("待办事项: " + strings.Join(summary.ActionItems, ", ") + "\n")
	}

	// 原子操作: 在同一把分布式锁内读取下一个索引并写入 archive 目录，
	// 消除 NextArchiveIndex + CreateArchiveDir 两步之间的 TOCTOU 竞态。
	nextIdx, err := sc.fsStore.CreateNextArchive(
		tenantID, userID,
		summary.OneLine,
		sb.String(),
	)
	if err != nil {
		slog.Warn("创建 archive 目录失败", "error", err)
		return
	}
	result.ArchiveIndex = nextIdx
}

// mapCategoryToInternal 将 OpenViking 6 分类映射为 UHMS 内部分类。
func (sc *SessionCommitter) mapCategoryToInternal(category string) string {
	switch strings.ToLower(category) {
	case "profile":
		return CategoryProfile
	case "preferences":
		return CategoryPreference
	case "entities":
		return CategoryRelationship
	case "events":
		return CategoryEvent
	case "cases":
		return CategoryInsight
	case "patterns":
		return CategorySkill
	default:
		return CategoryFact
	}
}
