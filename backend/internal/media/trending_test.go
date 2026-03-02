package media

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ---------- Mock TrendingSource ----------

type mockTrendingSource struct {
	name   string
	topics []TrendingTopic
	err    error
}

func (m *mockTrendingSource) Fetch(_ context.Context, category string, limit int) ([]TrendingTopic, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := m.topics
	if category != "" && category != "all" {
		var filtered []TrendingTopic
		for _, t := range result {
			if t.Category == category {
				filtered = append(filtered, t)
			}
		}
		result = filtered
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockTrendingSource) Name() string { return m.name }

// ---------- Tests ----------

func TestTrendingAggregator_EmptySources(t *testing.T) {
	agg := NewTrendingAggregator()
	topics, results := agg.FetchAll(context.Background(), "", 10)
	if topics != nil {
		t.Errorf("expected nil topics, got %v", topics)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestTrendingAggregator_MultipleSources(t *testing.T) {
	src1 := &mockTrendingSource{
		name: "weibo",
		topics: []TrendingTopic{
			{Title: "Weibo #1", Source: "weibo", HeatScore: 100, FetchedAt: time.Now()},
			{Title: "Weibo #2", Source: "weibo", HeatScore: 50, FetchedAt: time.Now()},
		},
	}
	src2 := &mockTrendingSource{
		name: "baidu",
		topics: []TrendingTopic{
			{Title: "Baidu #1", Source: "baidu", HeatScore: 80, FetchedAt: time.Now()},
		},
	}

	agg := NewTrendingAggregator(src1, src2)

	topics, results := agg.FetchAll(context.Background(), "", 0)
	if len(topics) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(topics))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify sorted by HeatScore descending.
	if topics[0].HeatScore != 100 {
		t.Errorf("topics[0].HeatScore: got %.0f, want 100", topics[0].HeatScore)
	}
	if topics[1].HeatScore != 80 {
		t.Errorf("topics[1].HeatScore: got %.0f, want 80", topics[1].HeatScore)
	}
	if topics[2].HeatScore != 50 {
		t.Errorf("topics[2].HeatScore: got %.0f, want 50", topics[2].HeatScore)
	}
}

func TestTrendingAggregator_PartialFailure(t *testing.T) {
	srcOK := &mockTrendingSource{
		name: "baidu",
		topics: []TrendingTopic{
			{Title: "OK Topic", Source: "baidu", HeatScore: 90, FetchedAt: time.Now()},
		},
	}
	srcFail := &mockTrendingSource{
		name: "weibo",
		err:  fmt.Errorf("network timeout"),
	}

	agg := NewTrendingAggregator(srcOK, srcFail)
	topics, results := agg.FetchAll(context.Background(), "", 0)

	// Should still return topics from the successful source.
	if len(topics) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(topics))
	}
	if topics[0].Title != "OK Topic" {
		t.Errorf("topic title: got %q, want %q", topics[0].Title, "OK Topic")
	}

	// Verify error is captured in results.
	var errCount int
	for _, r := range results {
		if r.Err != nil {
			errCount++
		}
	}
	if errCount != 1 {
		t.Errorf("expected 1 error in results, got %d", errCount)
	}
}

func TestTrendingAggregator_GlobalLimit(t *testing.T) {
	src := &mockTrendingSource{
		name: "weibo",
		topics: []TrendingTopic{
			{Title: "A", HeatScore: 100, FetchedAt: time.Now()},
			{Title: "B", HeatScore: 90, FetchedAt: time.Now()},
			{Title: "C", HeatScore: 80, FetchedAt: time.Now()},
			{Title: "D", HeatScore: 70, FetchedAt: time.Now()},
		},
	}

	agg := NewTrendingAggregator(src)
	topics, _ := agg.FetchAll(context.Background(), "", 2)

	if len(topics) != 2 {
		t.Fatalf("expected 2 topics with limit=2, got %d", len(topics))
	}
	if topics[0].Title != "A" || topics[1].Title != "B" {
		t.Errorf("expected top 2 by score, got %q, %q", topics[0].Title, topics[1].Title)
	}
}

func TestTrendingAggregator_AddSource(t *testing.T) {
	agg := NewTrendingAggregator()
	if names := agg.SourceNames(); len(names) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(names))
	}

	agg.AddSource(&mockTrendingSource{name: "zhihu"})
	agg.AddSource(nil) // Should be ignored.

	names := agg.SourceNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 source, got %d", len(names))
	}
	if names[0] != "zhihu" {
		t.Errorf("source name: got %q, want %q", names[0], "zhihu")
	}
}

func TestTrendingAggregator_FetchBySource(t *testing.T) {
	src := &mockTrendingSource{
		name: "baidu",
		topics: []TrendingTopic{
			{Title: "Baidu Topic", HeatScore: 75, FetchedAt: time.Now()},
		},
	}

	agg := NewTrendingAggregator(src)

	// Existing source.
	topics, err := agg.FetchBySource(context.Background(), "baidu", "", 10)
	if err != nil {
		t.Fatalf("FetchBySource: %v", err)
	}
	if len(topics) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(topics))
	}

	// Non-existent source.
	_, err = agg.FetchBySource(context.Background(), "nonexistent", "", 10)
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
}
