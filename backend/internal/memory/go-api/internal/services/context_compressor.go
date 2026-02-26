// Package services — Context Compressor.
// Mirrors Python services/context_compressor.py — reduces token usage by compressing search results.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// MaxCharsPerMemory is the maximum content length per memory item.
const MaxCharsPerMemory = 500

// ContextMemory represents a memory item for context compression.
type ContextMemory struct {
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	MemoryType string  `json:"memory_type"`
}

// CompressContext reduces a list of memories into a concise context string.
//
// Strategies:
//  1. Truncate individual memories to MaxCharsPerMemory
//  2. Deduplicate based on first 100 chars
//  3. (Optional) Use LLM to summarize if over token budget
func CompressContext(
	ctx context.Context,
	memories []ContextMemory,
	query string,
	llm LLMProvider, // nil to skip LLM compression
	maxTotalTokens int,
) string {
	if len(memories) == 0 {
		return ""
	}
	if maxTotalTokens <= 0 {
		maxTotalTokens = 2000
	}

	// Step 1: Truncate and deduplicate
	seen := make(map[string]bool)
	var parts []string

	for _, mem := range memories {
		content := strings.TrimSpace(mem.Content)
		if content == "" {
			continue
		}

		// Dedup key: first 100 chars lowered
		key := strings.ToLower(content)
		if len(key) > 100 {
			key = key[:100]
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		// Truncate
		if len(content) > MaxCharsPerMemory {
			content = content[:MaxCharsPerMemory] + "..."
		}

		memType := mem.MemoryType
		if memType == "" {
			memType = "observation"
		}
		parts = append(parts, fmt.Sprintf("[%s|%.2f] %s", memType, mem.Score, content))
	}

	// Step 2: Check token budget (rough: 1 token ≈ 4 chars)
	fullContext := strings.Join(parts, "\n---\n")
	estimatedTokens := len(fullContext) / 4

	if estimatedTokens <= maxTotalTokens || llm == nil {
		return fullContext
	}

	// Step 3: LLM summary compression
	summaryPrompt := fmt.Sprintf(
		`Summarize the following memory context into a concise format relevant to the query: "%s"

Memories:
%s

Provide a dense summary preserving key facts, entities, and relationships. Keep it under %d tokens.`,
		query, fullContext, maxTotalTokens,
	)

	summary, err := llm.Generate(ctx, summaryPrompt)
	if err != nil {
		slog.Warn("LLM compression failed, using truncated context", "error", err)
		// Fallback: top 5 results
		if len(parts) > 5 {
			parts = parts[:5]
		}
		return strings.Join(parts, "\n---\n")
	}

	slog.Info("Compressed context",
		"from_tokens", estimatedTokens,
		"to_tokens", len(summary)/4,
	)
	return summary
}
