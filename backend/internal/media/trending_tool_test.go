package media

import (
	"context"
	"testing"
)

// ---------- TrendingTool tests (P1-1) ----------

// toolMockSource implements TrendingSource for tool-level testing.
// Named differently from mockTrendingSource in trending_test.go to avoid redeclaration.
type toolMockSource struct {
	name   string
	topics []TrendingTopic
	err    error
}

func (m *toolMockSource) Fetch(_ context.Context, _ string, limit int) ([]TrendingTopic, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := m.topics
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *toolMockSource) Name() string { return m.name }

func TestCreateTrendingTool_FetchAll(t *testing.T) {
	agg := NewTrendingAggregator(
		&toolMockSource{
			name: "weibo",
			topics: []TrendingTopic{
				{Title: "Weibo Hot 1", HeatScore: 90, Source: "weibo"},
				{Title: "Weibo Hot 2", HeatScore: 70, Source: "weibo"},
			},
		},
		&toolMockSource{
			name: "baidu",
			topics: []TrendingTopic{
				{Title: "Baidu Hot 1", HeatScore: 80, Source: "baidu"},
			},
		},
	)

	tool := CreateTrendingTool(agg)
	if tool.ToolName != ToolTrendingTopics {
		t.Fatalf("name: got %q, want %q", tool.ToolName, ToolTrendingTopics)
	}

	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "fetch",
	})
	if err != nil {
		t.Fatalf("Execute fetch: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type: got %q, want %q", result.Content[0].Type, "text")
	}
}

func TestCreateTrendingTool_FetchBySource(t *testing.T) {
	agg := NewTrendingAggregator(
		&toolMockSource{
			name: "weibo",
			topics: []TrendingTopic{
				{Title: "Weibo Topic", HeatScore: 95, Source: "weibo"},
			},
		},
		&toolMockSource{
			name:   "baidu",
			topics: []TrendingTopic{},
		},
	)

	tool := CreateTrendingTool(agg)
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "fetch",
		"source": "weibo",
		"limit":  float64(5),
	})
	if err != nil {
		t.Fatalf("Execute fetch by source: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestCreateTrendingTool_ListSources(t *testing.T) {
	agg := NewTrendingAggregator(
		&toolMockSource{name: "weibo"},
		&toolMockSource{name: "baidu"},
		&toolMockSource{name: "zhihu"},
	)

	tool := CreateTrendingTool(agg)
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "list_sources",
	})
	if err != nil {
		t.Fatalf("Execute list_sources: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestCreateTrendingTool_Analyze(t *testing.T) {
	agg := NewTrendingAggregator(
		&toolMockSource{
			name: "weibo",
			topics: []TrendingTopic{
				{Title: "AI新突破", HeatScore: 100, Source: "weibo", URL: "https://example.com/1"},
				{Title: "科技大会", HeatScore: 80, Source: "weibo"},
			},
		},
	)

	tool := CreateTrendingTool(agg)
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "analyze",
		"category": "tech",
	})
	if err != nil {
		t.Fatalf("Execute analyze: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestCreateTrendingTool_UnknownAction(t *testing.T) {
	agg := NewTrendingAggregator()
	tool := CreateTrendingTool(agg)

	_, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "invalid_action",
	})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestCreateTrendingTool_NilAggregator(t *testing.T) {
	tool := CreateTrendingTool(nil)

	// fetch should fail.
	_, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "fetch",
	})
	if err == nil {
		t.Fatal("expected error for nil aggregator")
	}

	// list_sources should succeed with empty result.
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "list_sources",
	})
	if err != nil {
		t.Fatalf("list_sources with nil agg: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for list_sources with nil agg")
	}
}
