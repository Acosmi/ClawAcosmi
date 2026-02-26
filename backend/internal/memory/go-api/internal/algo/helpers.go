package algo

import (
	"encoding/json"
	"strings"
)

// extractCategoryFromJSON extracts the category field from an LLM JSON response.
// Handles common LLM quirks like markdown code fences.
func extractCategoryFromJSON(raw string) string {
	// Strip markdown code fences if present
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result struct {
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return "fact" // safe default
	}

	category := strings.ToLower(strings.TrimSpace(result.Category))
	if category == "" {
		return "fact"
	}

	// Validate against known categories
	validCategories := map[string]bool{
		"preference": true, "habit": true, "profile": true,
		"skill": true, "relationship": true, "event": true,
		"opinion": true, "fact": true, "goal": true,
		"task": true, "reminder": true, "insight": true,
		"summary": true,
	}
	if !validCategories[category] {
		return "fact"
	}

	return category
}

// extractCoreMemoryEdits parses an LLM reflection response for core memory edit suggestions.
// Returns nil if no edits are found.
func extractCoreMemoryEdits(raw string) []CoreMemoryEdit {
	// Strip markdown code fences
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result struct {
		Reflection      string `json:"reflection"`
		CoreMemoryEdits []struct {
			Section string `json:"section"`
			Content string `json:"content"`
			Mode    string `json:"mode"`
		} `json:"core_memory_edits"`
	}

	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}

	if len(result.CoreMemoryEdits) == 0 {
		return nil
	}

	edits := make([]CoreMemoryEdit, len(result.CoreMemoryEdits))
	for i, e := range result.CoreMemoryEdits {
		edits[i] = CoreMemoryEdit{
			Section: e.Section,
			Content: e.Content,
			Mode:    e.Mode,
		}
	}
	return edits
}

// extractReflectionText extracts the reflection text from a structured JSON response.
// Falls back to returning the raw string if parsing fails.
func extractReflectionText(raw string) string {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var result struct {
		Reflection string `json:"reflection"`
	}
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return raw // return original if not JSON
	}
	if result.Reflection == "" {
		return raw
	}
	return result.Reflection
}
