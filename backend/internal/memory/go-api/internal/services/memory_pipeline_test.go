// Package services — unit tests for the memory deduplication pipeline.
package services

import (
	"testing"
)

// TestSparseBM25Embed verifies the BM25 sparse embedding function.
func TestSparseBM25Embed(t *testing.T) {
	indices, values := SparseBM25Embed("hello world hello")
	if len(indices) == 0 {
		t.Fatal("SparseBM25Embed should return non-empty indices")
	}
	if len(indices) != len(values) {
		t.Fatalf("indices len (%d) != values len (%d)", len(indices), len(values))
	}

	// Check normalization: sum of values should approximately equal 1.0
	var sum float32
	for _, v := range values {
		sum += v
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("BM25 values should sum to ~1.0, got %f", sum)
	}
}

// TestSparseBM25EmbedEmpty verifies empty text returns nil.
func TestSparseBM25EmbedEmpty(t *testing.T) {
	indices, values := SparseBM25Embed("")
	if indices != nil || values != nil {
		t.Fatal("SparseBM25Embed should return nil for empty text")
	}
}

// TestSparseBM25EmbedCJK verifies CJK character handling.
func TestSparseBM25EmbedCJK(t *testing.T) {
	indices, values := SparseBM25Embed("你好世界")
	if len(indices) == 0 {
		t.Fatal("SparseBM25Embed should handle CJK text")
	}
	if len(indices) != 4 {
		t.Errorf("Expected 4 CJK tokens (one per char), got %d", len(indices))
	}

	// Each CJK char should have equal weight
	expected := float32(1.0 / 4.0)
	for i, v := range values {
		if v < expected-0.01 || v > expected+0.01 {
			t.Errorf("CJK token %d: expected weight ~%.3f, got %.3f", i, expected, v)
		}
	}
}

// TestSparseBM25EmbedDuplicateWords verifies duplicate words increase frequency.
func TestSparseBM25EmbedDuplicateWords(t *testing.T) {
	indices, values := SparseBM25Embed("test test test different")

	// Should have exactly 2 unique indices (test + different)
	if len(indices) != 2 {
		t.Errorf("Expected 2 unique tokens, got %d", len(indices))
	}

	// One value should be 0.75 (test: 3/4), other 0.25 (different: 1/4)
	found075 := false
	found025 := false
	for _, v := range values {
		if v > 0.74 && v < 0.76 {
			found075 = true
		}
		if v > 0.24 && v < 0.26 {
			found025 = true
		}
	}
	if !found075 || !found025 {
		t.Errorf("Expected weights ~0.75 and ~0.25, got %v", values)
	}
}

// TestDedupActionConstants verifies dedup action string values.
func TestDedupActionConstants(t *testing.T) {
	tests := []struct {
		action MemoryAction
		want   string
	}{
		{ActionAdd, "add"},
		{ActionUpdate, "update"},
		{ActionDelete, "delete"},
		{ActionNoop, "noop"},
	}
	for _, tt := range tests {
		if string(tt.action) != tt.want {
			t.Errorf("Action %q != expected %q", tt.action, tt.want)
		}
	}
}

// TestStringOrDefault verifies the helper function.
func TestStringOrDefault(t *testing.T) {
	m := map[string]any{
		"key1": "value1",
		"key2": 123,
	}

	if v := stringOrDefault(m, "key1", "default"); v != "value1" {
		t.Errorf("Expected 'value1', got '%s'", v)
	}
	if v := stringOrDefault(m, "key2", "default"); v != "default" {
		t.Errorf("Expected 'default' for non-string value, got '%s'", v)
	}
	if v := stringOrDefault(m, "missing", "default"); v != "default" {
		t.Errorf("Expected 'default' for missing key, got '%s'", v)
	}
}
