package media

// ============================================================================
// media/trending_tool.go — 热点采集 LLM 工具
// 封装 TrendingAggregator，为子智能体提供 trending_topics 工具接口。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P1-1
//
// NOTE: 不导入 tools 包以避免循环依赖（tools → channels → media）。
// 使用 media 包内定义的 MediaTool / MediaToolResult 类型。
// 集成时由 tools 包的注册层将 MediaTool 转换为 AgentTool。
// ============================================================================

import (
	"context"
	"fmt"
	"strings"
)

// ---------- Action 常量 ----------

// TrendingAction 热点工具操作类型。
type TrendingAction string

const (
	TrendingActionFetch       TrendingAction = "fetch"
	TrendingActionAnalyze     TrendingAction = "analyze"
	TrendingActionListSources TrendingAction = "list_sources"
)

// ---------- 工具构造器 ----------

// CreateTrendingTool 创建热点采集工具。
// agg 为 nil 时工具仍可构造，但 fetch/analyze 执行时返回错误。
// stateStore 可为 nil（不做去重标记）。
func CreateTrendingTool(agg *TrendingAggregator, stateStore MediaStateStore) *MediaTool {
	return &MediaTool{
		ToolName:  ToolTrendingTopics,
		ToolLabel: "Trending Topics",
		ToolDesc: "Discover trending topics from multiple sources (Weibo, Baidu, Zhihu, etc.). " +
			"Actions: fetch (retrieve hot topics), analyze (summarize topic list), list_sources (show available sources).",
		ToolParams: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []any{"fetch", "analyze", "list_sources"},
					"description": "The trending action to perform",
				},
				"source": map[string]any{
					"type":        "string",
					"description": "Specific source name (e.g. weibo, baidu, zhihu). If omitted, fetches from all sources.",
				},
				"category": map[string]any{
					"type":        "string",
					"enum":        []any{"tech", "finance", "entertainment", "all"},
					"description": "Topic category filter (default: all)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of topics to return (default: 10)",
				},
				"topics": map[string]any{
					"type":        "string",
					"description": "JSON-encoded topic list for analyze action",
				},
			},
			"required": []any{"action"},
		},
		ToolExecute: func(ctx context.Context, toolCallID string, args map[string]any) (*MediaToolResult, error) {
			action, err := readStringArg(args, "action", true)
			if err != nil {
				return nil, err
			}

			switch TrendingAction(action) {
			case TrendingActionFetch:
				return executeTrendingFetch(ctx, agg, stateStore, args)
			case TrendingActionAnalyze:
				return executeTrendingAnalyze(ctx, agg, args)
			case TrendingActionListSources:
				return executeTrendingListSources(agg)
			default:
				return nil, fmt.Errorf("unknown trending action: %s", action)
			}
		},
	}
}

// ---------- Action 实现 ----------

// executeTrendingFetch 拉取热点话题。
func executeTrendingFetch(
	ctx context.Context,
	agg *TrendingAggregator,
	stateStore MediaStateStore,
	args map[string]any,
) (*MediaToolResult, error) {
	if agg == nil {
		return nil, fmt.Errorf("trending aggregator not configured")
	}

	category, _ := readStringArg(args, "category", false)
	if category == "" {
		category = "all"
	}

	limit := 10
	if v, ok := readIntArg(args, "limit"); ok && v > 0 {
		limit = v
	}

	source, _ := readStringArg(args, "source", false)

	// If a specific source is requested, use FetchBySource.
	if source != "" {
		topics, err := agg.FetchBySource(ctx, source, category, limit)
		if err != nil {
			return nil, fmt.Errorf("fetch from source %q: %w", source, err)
		}
		annotateProcessedTopics(topics, stateStore)
		return jsonMediaResult(map[string]any{
			"source": source,
			"count":  len(topics),
			"topics": topics,
		}), nil
	}

	// Otherwise fetch from all sources.
	topics, results := agg.FetchAll(ctx, category, limit)
	annotateProcessedTopics(topics, stateStore)

	// Build per-source status summary.
	sourceStatus := make([]map[string]any, 0, len(results))
	for _, r := range results {
		entry := map[string]any{
			"source": r.Source,
			"count":  len(r.Topics),
		}
		if r.Err != nil {
			entry["error"] = r.Err.Error()
		}
		sourceStatus = append(sourceStatus, entry)
	}

	return jsonMediaResult(map[string]any{
		"count":         len(topics),
		"topics":        topics,
		"source_status": sourceStatus,
	}), nil
}

// executeTrendingAnalyze 格式化热点话题摘要。
// 不调用 LLM，仅做结构化文本输出；LLM 分析由子智能体 session 内完成。
func executeTrendingAnalyze(
	ctx context.Context,
	agg *TrendingAggregator,
	args map[string]any,
) (*MediaToolResult, error) {
	if agg == nil {
		return nil, fmt.Errorf("trending aggregator not configured")
	}

	category, _ := readStringArg(args, "category", false)
	if category == "" {
		category = "all"
	}
	limit := 10
	if v, ok := readIntArg(args, "limit"); ok && v > 0 {
		limit = v
	}

	topics, _ := agg.FetchAll(ctx, category, limit)
	if len(topics) == 0 {
		return jsonMediaResult(map[string]any{
			"summary": "No trending topics found.",
			"count":   0,
		}), nil
	}

	// Build a ranked text summary for LLM consumption.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Top %d trending topics", len(topics)))
	if category != "all" {
		sb.WriteString(fmt.Sprintf(" (category: %s)", category))
	}
	sb.WriteString(":\n\n")

	for i, t := range topics {
		sb.WriteString(fmt.Sprintf("%d. [%.0f] %s", i+1, t.HeatScore, t.Title))
		if t.Source != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", t.Source))
		}
		if t.URL != "" {
			sb.WriteString(fmt.Sprintf("\n   URL: %s", t.URL))
		}
		sb.WriteString("\n")
	}

	return jsonMediaResult(map[string]any{
		"summary": sb.String(),
		"count":   len(topics),
	}), nil
}

// executeTrendingListSources 返回已注册的数据源清单。
func executeTrendingListSources(agg *TrendingAggregator) (*MediaToolResult, error) {
	if agg == nil {
		return jsonMediaResult(map[string]any{
			"sources": []string{},
			"count":   0,
		}), nil
	}

	names := agg.SourceNames()
	return jsonMediaResult(map[string]any{
		"sources": names,
		"count":   len(names),
	}), nil
}

// annotateProcessedTopics 标记已处理过的热点（跨会话去重）。
func annotateProcessedTopics(topics []TrendingTopic, store MediaStateStore) {
	if store == nil || len(topics) == 0 {
		return
	}
	for i := range topics {
		if store.IsTopicProcessed(topics[i].Title) {
			topics[i].Processed = true
		}
	}
}
