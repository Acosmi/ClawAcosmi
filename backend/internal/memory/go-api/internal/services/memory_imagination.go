// Package services — L4 想象记忆 (Imaginative Memory) 引擎。
// 实现前瞻性个性化记忆生成：基于知识图谱高频实体，结合用户偏好进行融合分析。
// 参考：Stanford Generative Agents (Planning)、Meta JEPA、DeepMind Dreamer。
// 通过 MemoryPlugin 接口实现架构隔离，支持 Open Core 商业化分离。
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// ============================================================================
// MemoryPlugin 接口 — 插件化记忆扩展的抽象层
// ============================================================================

// MemoryPlugin 定义可插拔的记忆扩展层接口。
// L4 想象记忆作为首个插件实现，后续可扩展更多记忆层。
// 设计目标：源码层面的天然隔离，便于 Open Core 商业化输出。
type MemoryPlugin interface {
	// Name 返回插件名称标识。
	Name() string
	// Run 执行插件的记忆生成逻辑，返回新生成的记忆列表。
	Run(ctx context.Context, db *gorm.DB, userID string) ([]*models.Memory, error)
}

// ============================================================================
// ImaginationEngine — L4 想象记忆引擎
// ============================================================================

// ImaginationEngine 实现 L4 想象记忆的四步流水线：
//
//	Step 1: GetTrendingEntities → 提取高热度实体（Value Gating）
//	Step 2: GenerateProbe       → LLM 构建"未来预演"探索问题
//	Step 3: FetchExternalContext → 获取外部上下文（MVP: LLM 模拟）
//	Step 4: GenerateImagination  → 融合 CoreMemory 偏好，生成 Predictive 记忆块
type ImaginationEngine struct {
	graphStore  *GraphStoreService
	vectorStore *VectorStoreService
	llm         LLMProvider
	searcher    WebSearchProvider
	triggers    []ImaginationTrigger // 事件驱动触发器
}

// Compile-time check: ImaginationEngine implements MemoryPlugin.
var _ MemoryPlugin = (*ImaginationEngine)(nil)

// NewImaginationEngine 创建想象记忆引擎实例。
func NewImaginationEngine(gs *GraphStoreService, vs *VectorStoreService, llm LLMProvider, searcher WebSearchProvider) *ImaginationEngine {
	return &ImaginationEngine{
		graphStore:  gs,
		vectorStore: vs,
		llm:         llm,
		searcher:    searcher,
	}
}

// RegisterTrigger 注册一个事件驱动触发器。
// 触发器在 CheckTriggers 中被轮询，命中任一即可触发 Run()。
func (e *ImaginationEngine) RegisterTrigger(t ImaginationTrigger) {
	e.triggers = append(e.triggers, t)
}

// CheckTriggers 检查所有已注册的触发器，返回第一个命中的触发器名。
// 如果无触发器命中，返回空字符串。
func (e *ImaginationEngine) CheckTriggers(db *gorm.DB, userID string) string {
	for _, t := range e.triggers {
		if t.ShouldTrigger(db, userID) {
			return t.Name()
		}
	}
	return ""
}

// Name 返回插件标识名。
func (e *ImaginationEngine) Name() string {
	return "imagination"
}

// --- Prompt 模板 ---

// probePrompt 意图探针 Prompt — 让 LLM 基于高频实体生成探索性问题。
const probePrompt = `你是一个前瞻性思维助手。基于以下用户长期关注的话题，生成一个用户可能关心的未来趋势或前沿发展方向的探索性问题。

话题信息：
- 实体名称：%s
- 实体类型：%s
- 实体描述：%s
- 关注持续天数：%d 天
- 关联关系数：%d

请生成一个具体的、有深度的探索性问题。问题应该：
1. 聚焦于该话题的未来发展方向或最新趋势
2. 结合用户的长期关注度，推测用户可能感兴趣的切入角度
3. 避免过于宽泛，保持具体和可探索性

仅返回一个问题，不要附加其他内容。`

// externalContextPrompt 外部信息检索 Prompt（MVP 阶段用 LLM 生成模拟外部数据）。
const externalContextPrompt = `你是一个信息调研助手。请针对以下探索性问题提供简洁的分析和最新的相关信息。

问题：%s

请提供：
1. 该领域的最新发展动态（2-3 点）
2. 主流观点或趋势判断
3. 潜在的机遇或风险点

以简洁的要点形式输出，每点不超过一句话。`

// imaginationPrompt 想象记忆融合生成 Prompt。
// N2 自编辑: 增加核心记忆追加指令（仅允许 append）。
const imaginationPrompt = `你是一个个性化预测分析师。基于以下信息，为用户生成一段前瞻性的个性化分析。

## 用户长期关注的话题
%s

## 探索性问题
%s

## 最新相关信息
%s

## 用户个人偏好
%s

请生成一个 JSON 响应，结构如下：
{
  "imagination": "你的前瞻性分析内容（150-300字）",
  "core_memory_appends": [
    {"section": "preferences", "mode": "append", "content": "新发现的用户兴趣或偏好"}
  ]
}

分析要求：
1. 结合用户的个人偏好和兴趣特点进行个性化解读
2. 给出基于当前趋势的合理推测
3. 提供可能对用户有价值的行动建议

core_memory_appends 规则：
1. 仅在发现明确的新用户兴趣或偏好时才添加
2. mode 必须为 "append"（不允许 replace）
3. section 必须为 "persona" 或 "preferences"
4. 如果没有需要追加的内容，使用空数组: "core_memory_appends": []

返回纯 JSON，无 markdown 格式。`

// --- Run 核心执行流 ---

// Run 执行 L4 想象记忆生成的完整四步流水线。
func (e *ImaginationEngine) Run(ctx context.Context, db *gorm.DB, userID string) ([]*models.Memory, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if e.llm == nil {
		return nil, errors.New("LLM provider is required for imagination")
	}
	if e.graphStore == nil {
		return nil, errors.New("graph store is required for imagination")
	}

	// Step 1: Value Gating — 提取高热度实体
	trending, err := e.graphStore.GetTrendingEntities(db, userID, 3, 3)
	if err != nil {
		return nil, fmt.Errorf("get trending entities: %w", err)
	}
	if len(trending) == 0 {
		slog.Info("L4 想象记忆：无符合条件的高热度实体，跳过", "user_id", userID)
		return nil, nil
	}

	// 获取用户 Core Memory 偏好（用于 Step 4 融合）
	preferences := ""
	cm, err := GetCoreMemory(db, userID)
	if err == nil && cm != nil {
		preferences = cm.Preferences
		if cm.Persona != "" {
			preferences = "用户画像: " + cm.Persona + "\n偏好: " + preferences
		}
	}
	if preferences == "" {
		preferences = "(未设置个人偏好)"
	}

	// 遍历高热度实体，为每个生成想象记忆
	var memories []*models.Memory
	for _, t := range trending {
		mem, err := e.generateForEntity(ctx, db, userID, t, preferences)
		if err != nil {
			slog.Warn("L4 想象记忆生成失败，跳过该实体",
				"entity", t.Entity.Name, "error", err)
			continue
		}
		if mem != nil {
			memories = append(memories, mem)
		}
	}

	slog.Info("L4 想象记忆生成完成",
		"user_id", userID,
		"trending_count", len(trending),
		"generated_count", len(memories),
	)

	return memories, nil
}

// generateForEntity 为单个高热度实体执行想象记忆生成流水线。
func (e *ImaginationEngine) generateForEntity(
	ctx context.Context, db *gorm.DB,
	userID string, t TrendingEntity, preferences string,
) (*models.Memory, error) {
	entityDesc := ""
	if t.Entity.Description != nil {
		entityDesc = *t.Entity.Description
	}
	if entityDesc == "" {
		entityDesc = "（无描述）"
	}

	// Step 2: 意图探针 — 生成探索性问题
	probe, err := e.llm.Generate(ctx, fmt.Sprintf(probePrompt,
		t.Entity.Name, t.Entity.EntityType, entityDesc, t.DaysSpan, t.RelationCount))
	if err != nil {
		return nil, fmt.Errorf("generate probe: %w", err)
	}
	probe = strings.TrimSpace(probe)
	if probe == "" {
		return nil, fmt.Errorf("empty probe for entity %s", t.Entity.Name)
	}

	// Step 3: 外部数据引入（真实搜索 API）
	var externalCtx string
	var sourceType string
	var sourceURLs []string

	if e.searcher != nil {
		searchResult, err := e.searcher.Search(ctx, probe)
		if err != nil {
			slog.Warn("L4 外部搜索失败，使用 LLM 降级", "error", err)
			// 降级为 LLM 生成
			externalCtx, err = e.llm.Generate(ctx, fmt.Sprintf(externalContextPrompt, probe))
			if err != nil {
				return nil, fmt.Errorf("fetch external context: %w", err)
			}
			sourceType = "llm_inference"
			sourceURLs = []string{}
		} else {
			externalCtx = searchResult.Summary
			sourceType = searchResult.Provider
			sourceURLs = searchResult.SourceURLs
		}
	} else {
		// 无搜索提供者，使用 LLM 模拟（兼容 MVP 行为）
		externalCtx, err = e.llm.Generate(ctx, fmt.Sprintf(externalContextPrompt, probe))
		if err != nil {
			return nil, fmt.Errorf("fetch external context: %w", err)
		}
		sourceType = "llm_inference"
		sourceURLs = []string{}
	}
	externalCtx = strings.TrimSpace(externalCtx)
	if sourceURLs == nil {
		sourceURLs = []string{}
	}

	// Step 4: 融合生成 Predictive 记忆块
	topicSummary := fmt.Sprintf("话题: %s (%s)\n描述: %s\n关注度: %d 天, %d 个关联",
		t.Entity.Name, t.Entity.EntityType, entityDesc, t.DaysSpan, t.RelationCount)

	rawResponse, err := e.llm.Generate(ctx, fmt.Sprintf(imaginationPrompt,
		topicSummary, probe, externalCtx, preferences))
	if err != nil {
		return nil, fmt.Errorf("generate imagination: %w", err)
	}
	rawResponse = strings.TrimSpace(rawResponse)
	if rawResponse == "" {
		return nil, fmt.Errorf("empty imagination for entity %s", t.Entity.Name)
	}

	// N2 自编辑: 尝试解析结构化 JSON 响应
	var imagination string
	var editsApplied int
	parsedJSON, parseErr := ParseLLMJSON(rawResponse)
	if parseErr == nil {
		// 提取 imagination 文本
		if imag, ok := parsedJSON["imagination"].(string); ok && imag != "" {
			imagination = imag
		} else {
			imagination = rawResponse // fallback
		}

		// 提取并执行核心记忆追加（强制 append-only）
		if appendsRaw, ok := parsedJSON["core_memory_appends"]; ok {
			var appends []CoreMemoryEditAction
			appendsJSON, _ := json.Marshal(appendsRaw)
			if json.Unmarshal(appendsJSON, &appends) == nil && len(appends) > 0 {
				applied, applyErr := ApplyCoreMemoryEdits(db, userID, "imagination", appends, true) // forceAppendOnly=true
				if applyErr != nil {
					slog.Error("Core Memory 自编辑（想象）失败", "error", applyErr, "user_id", userID)
				}
				editsApplied = applied
			}
		}
	} else {
		// JSON 解析失败 — 纯文本想象（向后兼容）
		imagination = rawResponse
	}

	// 构建防污染元数据断言
	metadata := map[string]any{
		"simulation_depth":        1,             // 模拟深度标识
		"source_type":             sourceType,    // 来源类型
		"source_urls":             sourceURLs,    // 证据链
		"imagination_probe":       probe,         // 触发该想象的探索问题
		"trending_entity":         t.Entity.Name, // 关联的高热度实体名
		"entity_type":             t.Entity.EntityType,
		"heat_score":              t.HeatScore, // 触发时的热度评分
		"days_span":               t.DaysSpan,  // 关注跨度
		"generated_at":            time.Now().UTC().Format(time.RFC3339),
		"core_memory_edits_count": editsApplied, // N2: 核心记忆追加次数
	}

	// 创建想象记忆记录
	memory := &models.Memory{
		Content:         imagination,
		UserID:          userID,
		MemoryType:      MemoryTypeImagination,
		Category:        CategoryInsight,
		ImportanceScore: 0.6,            // 想象记忆初始重要度适中
		DecayFactor:     MaxDecayFactor, // 受保护，不参与衰减
		Metadata:        &metadata,
	}

	if err := db.Create(memory).Error; err != nil {
		return nil, fmt.Errorf("create imagination memory: %w", err)
	}

	slog.Info("L4 想象记忆已创建",
		"id", memory.ID,
		"entity", t.Entity.Name,
		"user_id", userID,
		"content_len", len(imagination),
		"core_memory_edits", editsApplied,
	)

	// 异步写入向量存储
	if e.vectorStore != nil {
		go func() {
			if err := e.vectorStore.AddMemory(
				context.Background(), memory.ID, imagination,
				userID, MemoryTypeImagination, memory.ImportanceScore, nil,
			); err != nil {
				slog.Error("想象记忆向量写入失败", "error", err, "id", memory.ID)
			}
		}()
	}

	return memory, nil
}

// --- Helper: 想象记忆查询 ---

// ImaginationResult 想象记忆查询结果。
type ImaginationResult struct {
	Memories []models.Memory `json:"memories"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// ListImaginations 分页查询指定用户的想象记忆。
func ListImaginations(db *gorm.DB, userID string, page, pageSize int) (*ImaginationResult, error) {
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := db.Model(&models.Memory{}).
		Where("user_id = ? AND memory_type = ?", userID, MemoryTypeImagination).
		Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count imaginations: %w", err)
	}

	var memories []models.Memory
	if err := db.Where("user_id = ? AND memory_type = ?", userID, MemoryTypeImagination).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&memories).Error; err != nil {
		return nil, fmt.Errorf("list imaginations: %w", err)
	}

	return &ImaginationResult{
		Memories: memories,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// --- Helper: 解析想象记忆元数据 ---

// ImaginationMeta 想象记忆的结构化元数据。
type ImaginationMeta struct {
	SimulationDepth int      `json:"simulation_depth"`
	SourceType      string   `json:"source_type"`
	SourceURLs      []string `json:"source_urls"`
	Probe           string   `json:"imagination_probe"`
	TrendingEntity  string   `json:"trending_entity"`
	EntityType      string   `json:"entity_type"`
	HeatScore       float64  `json:"heat_score"`
	DaysSpan        int      `json:"days_span"`
	GeneratedAt     string   `json:"generated_at"`
}

// ParseImaginationMeta 从记忆的 Metadata 字段解析想象记忆专属元数据。
func ParseImaginationMeta(metadata *map[string]any) (*ImaginationMeta, error) {
	if metadata == nil {
		return nil, errors.New("metadata is nil")
	}
	raw, err := json.Marshal(*metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	var meta ImaginationMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal imagination meta: %w", err)
	}
	return &meta, nil
}
