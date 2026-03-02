package media

// ============================================================================
// media/media_registry.go — oa-media 子智能体工具注册扩展
// 独立于主系统的 tools/registry.go，在集成阶段合并。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P0-4
// ============================================================================

import (
	"log/slog"
)

// ---------- 配置 ----------

// MediaToolsConfig 媒体工具集注册配置。
// 所有字段为可选依赖，nil 时对应工具注册但执行时返回 "not configured"。
type MediaToolsConfig struct {
	DraftStore     DraftStore          // P0-5 草稿存储
	Aggregator     *TrendingAggregator // P0-6 热点聚合
	Workspace      string              // 工作目录
	EnablePublish  bool                // 是否启用发布工具（默认 false，Phase 2+ 启用）
	EnableInteract bool                // 是否启用互动工具（默认 false，Phase 3+ 启用）
}

// ---------- 工具名常量 ----------

const (
	ToolTrendingTopics = "trending_topics"
	ToolContentCompose = "content_compose"
	ToolMediaPublish   = "media_publish"
	ToolSocialInteract = "social_interact"
)

// ---------- 工具定义（占位） ----------

// MediaToolDef 媒体工具定义占位符。
// Phase 1+ 阶段实现具体 AgentTool 构造器时替换。
type MediaToolDef struct {
	Name        string
	Description string
	Enabled     bool
}

// DefaultMediaToolDefs 返回 oa-media 全量工具定义清单。
func DefaultMediaToolDefs(cfg MediaToolsConfig) []MediaToolDef {
	defs := []MediaToolDef{
		{
			Name:        ToolTrendingTopics,
			Description: "Discover trending topics from multiple sources (Weibo, Baidu, Zhihu, etc.)",
			Enabled:     true,
		},
		{
			Name:        ToolContentCompose,
			Description: "Draft, preview, and revise content for specific platforms",
			Enabled:     true,
		},
		{
			Name:        ToolMediaPublish,
			Description: "Publish approved content to WeChat MP, Xiaohongshu, or website",
			Enabled:     cfg.EnablePublish,
		},
		{
			Name:        ToolSocialInteract,
			Description: "Manage comments and DMs on social platforms",
			Enabled:     cfg.EnableInteract,
		},
	}
	return defs
}

// LogMediaToolsRegistration 记录媒体工具注册状态（用于启动日志）。
func LogMediaToolsRegistration(cfg MediaToolsConfig) {
	defs := DefaultMediaToolDefs(cfg)
	enabled := 0
	for _, d := range defs {
		if d.Enabled {
			enabled++
		}
	}
	slog.Info("media tools registration",
		"total", len(defs),
		"enabled", enabled,
		"hasDraftStore", cfg.DraftStore != nil,
		"hasAggregator", cfg.Aggregator != nil,
	)
}
