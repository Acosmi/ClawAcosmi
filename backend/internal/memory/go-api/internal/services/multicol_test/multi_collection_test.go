// Package multicol_test tests the multi-collection routing logic.
// Separated from services package to avoid CGO/FFI linkage.
package multicol_test

import (
	"testing"
)

// Replicate the routing logic from vector_store.go for isolated testing.
var legacyTypeMap = map[string]string{
	"observation": "episodic",
	"dialogue":    "episodic",
	"reflection":  "semantic",
	"plan":        "procedural",
}

func normalizeMemoryType(t string) string {
	if mapped, ok := legacyTypeMap[t]; ok {
		return mapped
	}
	return t
}

var collectionMap = map[string]string{
	"episodic":    "mem_episodic",
	"semantic":    "mem_semantic",
	"procedural":  "mem_procedural",
	"permanent":   "mem_permanent",
	"imagination": "mem_permanent",
}

func collectionForType(memoryType string) string {
	normalized := normalizeMemoryType(memoryType)
	if c, ok := collectionMap[normalized]; ok {
		return c
	}
	return "mem_episodic"
}

func allCollectionNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, col := range collectionMap {
		if !seen[col] {
			seen[col] = true
			names = append(names, col)
		}
	}
	return names
}

func collectionsForSearch(memoryTypes []string) []string {
	if len(memoryTypes) == 0 {
		return allCollectionNames()
	}
	seen := map[string]bool{}
	var cols []string
	for _, mt := range memoryTypes {
		c := collectionForType(mt)
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	return cols
}

// TestCollectionForType_CognitiveTypes — cognitive types map correctly.
func TestCollectionForType_CognitiveTypes(t *testing.T) {
	cases := []struct {
		memoryType string
		want       string
	}{
		{"episodic", "mem_episodic"},
		{"semantic", "mem_semantic"},
		{"procedural", "mem_procedural"},
		{"permanent", "mem_permanent"},
		{"imagination", "mem_permanent"},
	}
	for _, tc := range cases {
		got := collectionForType(tc.memoryType)
		if got != tc.want {
			t.Errorf("collectionForType(%q) = %q, want %q", tc.memoryType, got, tc.want)
		}
	}
}

// TestCollectionForType_LegacyTypes — legacy types normalize then map.
func TestCollectionForType_LegacyTypes(t *testing.T) {
	cases := []struct {
		memoryType string
		want       string
	}{
		{"observation", "mem_episodic"},
		{"dialogue", "mem_episodic"},
		{"reflection", "mem_semantic"},
		{"plan", "mem_procedural"},
	}
	for _, tc := range cases {
		got := collectionForType(tc.memoryType)
		if got != tc.want {
			t.Errorf("collectionForType(%q) = %q, want %q", tc.memoryType, got, tc.want)
		}
	}
}

// TestCollectionForType_Unknown — unknown type falls back to mem_episodic.
func TestCollectionForType_Unknown(t *testing.T) {
	got := collectionForType("unknown_type")
	if got != "mem_episodic" {
		t.Errorf("collectionForType(unknown_type) = %q, want mem_episodic", got)
	}
}

// TestAllCollectionNames — returns exactly 4 unique collections.
func TestAllCollectionNames(t *testing.T) {
	names := allCollectionNames()
	if len(names) != 4 {
		t.Errorf("expected 4 unique collections, got %d: %v", len(names), names)
	}
	expected := map[string]bool{
		"mem_episodic":   false,
		"mem_semantic":   false,
		"mem_procedural": false,
		"mem_permanent":  false,
	}
	for _, n := range names {
		if _, ok := expected[n]; !ok {
			t.Errorf("unexpected collection name: %s", n)
		}
		expected[n] = true
	}
	for n, found := range expected {
		if !found {
			t.Errorf("missing expected collection: %s", n)
		}
	}
}

// TestCollectionsForSearch_Specified — specific types deduplicate to collections.
func TestCollectionsForSearch_Specified(t *testing.T) {
	// observation + dialogue both → mem_episodic (deduped)
	cols := collectionsForSearch([]string{"observation", "dialogue"})
	if len(cols) != 1 || cols[0] != "mem_episodic" {
		t.Errorf("expected [mem_episodic], got %v", cols)
	}

	// episodic + permanent → 2 collections
	cols = collectionsForSearch([]string{"episodic", "permanent"})
	if len(cols) != 2 {
		t.Errorf("expected 2 collections, got %d: %v", len(cols), cols)
	}
}

// TestCollectionsForSearch_Empty — empty returns all 4 collections.
func TestCollectionsForSearch_Empty(t *testing.T) {
	cols := collectionsForSearch(nil)
	if len(cols) != 4 {
		t.Errorf("expected 4 collections for empty memoryTypes, got %d", len(cols))
	}
}
