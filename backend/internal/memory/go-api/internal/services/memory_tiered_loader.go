// Package services — TieredLoader: LLM-based progressive memory loading.
//
// Phase 2: Implements L0 → LLM filter → L1 two-step pipeline.
// Reduces token consumption by ~86% (150k → ~21k tokens).
//
// Memory tier policies:
//   - permanent  → AlwaysL1 (skip L0 filter, always load L1)
//   - imagination → L0Only  (never expand beyond L0)
//   - others     → Standard (L0 → LLM filter → L1)
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// --- Tier Policy ---

// TierPolicy defines how a memory type participates in tiered loading.
type TierPolicy int

const (
	// TierStandard: normal L0 → LLM filter → L1 flow.
	TierStandard TierPolicy = iota
	// TierAlwaysL1: skip L0 filter, always load L1 (permanent memories).
	TierAlwaysL1
	// TierL0Only: never expand beyond L0 (imagination memories).
	TierL0Only
)

// --- Configuration ---

// TieredLoaderConfig holds tunable parameters for the progressive loading pipeline.
type TieredLoaderConfig struct {
	MaxL0Count       int  // Maximum L0 entries to consider (default 50)
	TopK             int  // Number of entries to keep after LLM filtering (default 8)
	TokenBudget      int  // Token budget ceiling for L1 contents (default 20000)
	LLMFilterEnabled bool // Whether to use LLM for filtering (default true)
}

// DefaultTieredLoaderConfig returns production-ready defaults.
func DefaultTieredLoaderConfig() TieredLoaderConfig {
	return TieredLoaderConfig{
		MaxL0Count:       50,
		TopK:             8,
		TokenBudget:      20000,
		LLMFilterEnabled: true,
	}
}

// --- TieredLoader ---

// TieredLoader orchestrates L0 → LLM filter → L1 progressive loading.
// Injected into MemoryManager via WithTieredLoader().
type TieredLoader struct {
	llm     LLMProvider
	fsStore *FSStoreService
	config  TieredLoaderConfig
}

// NewTieredLoader creates a TieredLoader with the given dependencies.
func NewTieredLoader(llm LLMProvider, fsStore *FSStoreService, cfg TieredLoaderConfig) *TieredLoader {
	return &TieredLoader{
		llm:     llm,
		fsStore: fsStore,
		config:  cfg,
	}
}

// ClassifyMemoryTier determines the tier policy for a given memory type.
//
//	permanent   → TierAlwaysL1  (critical memories, never filtered out)
//	imagination → TierL0Only    (prevent hallucination amplification)
//	others      → TierStandard  (normal L0→LLM→L1 pipeline)
func ClassifyMemoryTier(memoryType string) TierPolicy {
	switch memoryType {
	case MemoryTypePermanent:
		return TierAlwaysL1
	case MemoryTypeImagination:
		return TierL0Only
	default:
		return TierStandard
	}
}

// FilterByL0 uses the LLM to select the Top-K most relevant L0 entries
// for a given query. Returns the URIs of selected entries.
//
// Fallback: if LLM is unavailable or call fails, returns all URIs (no filtering).
func (tl *TieredLoader) FilterByL0(
	ctx context.Context,
	query string,
	l0Entries []L0Entry,
) ([]string, error) {
	if len(l0Entries) == 0 {
		return nil, nil
	}

	// If LLM filtering is disabled or LLM not available, return all URIs.
	if !tl.config.LLMFilterEnabled || tl.llm == nil {
		return allURIs(l0Entries), nil
	}

	// If entries <= TopK, no need to filter.
	if len(l0Entries) <= tl.config.TopK {
		return allURIs(l0Entries), nil
	}

	// Build prompt for LLM filtering.
	prompt := tl.buildFilterPrompt(query, l0Entries)

	// Call LLM.
	response, err := tl.llm.Generate(ctx, prompt)
	if err != nil {
		slog.Warn("TieredLoader: LLM 筛选调用失败，回退全量返回",
			"error", err,
			"entry_count", len(l0Entries),
		)
		return allURIs(l0Entries), nil
	}

	// Parse LLM response: expect a JSON array of URI strings.
	selectedURIs, parseErr := tl.parseFilterResponse(response, l0Entries)
	if parseErr != nil {
		slog.Warn("TieredLoader: LLM 响应解析失败，回退全量返回",
			"error", parseErr,
			"raw_response", truncate(response, 200),
		)
		return allURIs(l0Entries), nil
	}

	slog.Info("TieredLoader: LLM L0 筛选完成",
		"query", truncate(query, 50),
		"input_count", len(l0Entries),
		"selected_count", len(selectedURIs),
	)

	return selectedURIs, nil
}

// buildFilterPrompt constructs the prompt for LLM-based L0 filtering.
func (tl *TieredLoader) buildFilterPrompt(query string, entries []L0Entry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		`You are a memory relevance filter. Given a user query and a list of memory summaries, select the top %d most relevant memories.

User Query: "%s"

Memory Summaries:
`, tl.config.TopK, query))

	for i, entry := range entries {
		sb.WriteString(fmt.Sprintf("[%d] URI: %s | Type: %s | Summary: %s\n",
			i+1, entry.URI, entry.MemoryType, entry.L0Abstract))
	}

	sb.WriteString(fmt.Sprintf(`
Select exactly %d memories most relevant to the query.
Return ONLY a JSON array of URI strings, nothing else.
Example: ["%s", "%s"]`,
		tl.config.TopK,
		entries[0].URI,
		func() string {
			if len(entries) > 1 {
				return entries[1].URI
			}
			return entries[0].URI
		}(),
	))

	return sb.String()
}

// parseFilterResponse parses the LLM response into a list of URIs.
// Validates that returned URIs actually exist in the original entries.
func (tl *TieredLoader) parseFilterResponse(response string, entries []L0Entry) ([]string, error) {
	// Clean response: strip markdown code fences if present.
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON array.
	var uris []string
	if err := json.Unmarshal([]byte(response), &uris); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w (response: %s)", err, truncate(response, 100))
	}

	if len(uris) == 0 {
		return nil, fmt.Errorf("LLM returned empty URI list")
	}

	// Build valid URI set for validation.
	validURIs := make(map[string]bool, len(entries))
	for _, e := range entries {
		validURIs[e.URI] = true
	}

	// Filter to only valid URIs.
	validated := make([]string, 0, len(uris))
	for _, uri := range uris {
		if validURIs[uri] {
			validated = append(validated, uri)
		} else {
			slog.Debug("TieredLoader: LLM 返回了无效 URI，已跳过", "uri", uri)
		}
	}

	if len(validated) == 0 {
		return nil, fmt.Errorf("no valid URIs in LLM response")
	}

	return validated, nil
}

// --- Helpers ---

// allURIs extracts all URIs from L0 entries.
func allURIs(entries []L0Entry) []string {
	uris := make([]string, len(entries))
	for i, e := range entries {
		uris[i] = e.URI
	}
	return uris
}

// truncate truncates a string to maxLen characters, adding "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// --- Phase 3: Token Budget Controller ---

// estimateTokens provides a CJK-safe token count estimate for a string.
// Heuristic: CJK characters ≈ 2 tokens each, English words ≈ 1.3 tokens each.
// This is intentionally conservative to avoid exceeding the budget.
func estimateTokens(s string) int {
	runes := []rune(s)
	count := 0
	for _, r := range runes {
		if isCJKRune(r) {
			count += 2 // CJK characters are 2 tokens on average
		} else if r == ' ' || r == '\n' || r == '\t' {
			continue // whitespace doesn't count directly
		} else {
			count++ // ASCII characters ≈ 0.25 tokens per char, but words ≈ 1.3
		}
	}
	// Rough adjustment: ASCII chars overcount, divide by 4 and add back CJK portion.
	// Simplified: just use rune count as a reasonable proxy.
	if count == 0 {
		count = len(runes)
	}
	return count
}

// isCJKRune checks if a rune is a CJK character.
func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility
		(r >= 0x3040 && r <= 0x30FF) || // Hiragana + Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul
}

// ApplyTokenBudget enforces the token budget on L1 entries.
// Entries are processed in order; once the cumulative token count exceeds
// the budget, remaining entries are truncated (L1Overview cleared).
// Returns the (potentially truncated) entries.
func (tl *TieredLoader) ApplyTokenBudget(entries []L1Entry) []L1Entry {
	if tl.config.TokenBudget <= 0 || len(entries) == 0 {
		return entries
	}

	totalTokens := 0
	truncatedCount := 0

	for i := range entries {
		entryTokens := estimateTokens(entries[i].L1Overview)
		if totalTokens+entryTokens > tl.config.TokenBudget {
			// Over budget — clear L1Overview to signal degradation
			entries[i].L1Overview = ""
			truncatedCount++
		} else {
			totalTokens += entryTokens
		}
	}

	if truncatedCount > 0 {
		slog.Info("Phase 3 Token 预算控制: L1 条目已降级",
			"budget", tl.config.TokenBudget,
			"total_tokens", totalTokens,
			"kept", len(entries)-truncatedCount,
			"degraded", truncatedCount,
		)
	}

	return entries
}

// AvailableLevelsForType returns the available tier levels for a given memory type.
//
//	imagination → [0]         (L0 only — prevent hallucination)
//	others      → [0, 1, 2]   (full tier access)
func AvailableLevelsForType(memoryType string) []int {
	if ClassifyMemoryTier(memoryType) == TierL0Only {
		return []int{0}
	}
	return []int{0, 1, 2}
}
