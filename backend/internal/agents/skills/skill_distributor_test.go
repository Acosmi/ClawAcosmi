package skills

import (
	"context"
	"testing"

	"github.com/anthropic/open-acosmi/internal/memory/uhms"
)

func TestDistributeSkillsEmpty(t *testing.T) {
	dir := t.TempDir()
	vfs, err := uhms.NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	result, err := DistributeSkills(context.Background(), vfs, nil, nil)
	if err != nil {
		t.Fatalf("DistributeSkills: %v", err)
	}
	if result.Indexed != 0 {
		t.Errorf("expected indexed=0, got %d", result.Indexed)
	}
	if result.Skipped != 0 {
		t.Errorf("expected skipped=0, got %d", result.Skipped)
	}
}

func TestDistributeSkillsBasic(t *testing.T) {
	dir := t.TempDir()
	vfs, err := uhms.NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	entries := []SkillEntry{
		{
			Skill: Skill{
				Name:        "test-skill",
				Description: "A test skill",
				Content:     "---\ntags: testing,demo\n---\n# Test Skill\nFull content here.",
				Dir:         "/tmp/skills/tools/test-skill",
			},
			Enabled: true,
		},
	}

	result, err := DistributeSkills(context.Background(), vfs, nil, entries)
	if err != nil {
		t.Fatalf("DistributeSkills: %v", err)
	}
	if result.Indexed != 1 {
		t.Errorf("expected indexed=1, got %d", result.Indexed)
	}

	// Verify VFS write
	l0, err := vfs.ReadSystemL0("skills", "tools", "test-skill")
	if err != nil {
		t.Fatalf("ReadSystemL0: %v", err)
	}
	if l0 == "" {
		t.Error("expected non-empty L0")
	}

	l2, err := vfs.ReadSystemL2("skills", "tools", "test-skill")
	if err != nil {
		t.Fatalf("ReadSystemL2: %v", err)
	}
	if l2 != entries[0].Skill.Content {
		t.Error("L2 should be full content")
	}

	meta, err := vfs.ReadSystemMeta("skills", "tools", "test-skill")
	if err != nil {
		t.Fatalf("ReadSystemMeta: %v", err)
	}
	if meta["distributed"] != true {
		t.Error("expected meta.distributed=true")
	}
	if meta["tags"] != "testing,demo" {
		t.Errorf("expected meta.tags='testing,demo', got %v", meta["tags"])
	}
}

func TestDistributeSkillsIncremental(t *testing.T) {
	dir := t.TempDir()
	vfs, err := uhms.NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	entries := []SkillEntry{
		{
			Skill: Skill{
				Name:        "incremental",
				Description: "Test incremental",
				Content:     "# Incremental Test",
				Dir:         "/tmp/skills/general/incremental",
			},
			Enabled: true,
		},
	}

	// First distribute
	r1, err := DistributeSkills(context.Background(), vfs, nil, entries)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Indexed != 1 {
		t.Errorf("first run: expected indexed=1, got %d", r1.Indexed)
	}

	// Second distribute with same content → should skip
	r2, err := DistributeSkills(context.Background(), vfs, nil, entries)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Skipped != 1 {
		t.Errorf("second run: expected skipped=1, got %d", r2.Skipped)
	}
	if r2.Indexed != 0 {
		t.Errorf("second run: expected indexed=0, got %d", r2.Indexed)
	}

	// Third distribute with changed content → should re-index
	entries[0].Skill.Content = "# Incremental Test v2"
	r3, err := DistributeSkills(context.Background(), vfs, nil, entries)
	if err != nil {
		t.Fatal(err)
	}
	if r3.Indexed != 1 {
		t.Errorf("third run: expected indexed=1, got %d", r3.Indexed)
	}
}

func TestGenerateSkillL0(t *testing.T) {
	entry := SkillEntry{
		Skill: Skill{
			Name:        "browser",
			Description: "Integrated browser control and web automation",
			Content:     "---\ntags: web,automation\n---\n# Browser",
		},
		Metadata: &OpenAcosmiSkillMetadata{
			Emoji: "🌐",
		},
	}

	l0 := generateSkillL0(entry)
	if l0 == "" {
		t.Fatal("expected non-empty L0")
	}
	// Should contain name
	if !containsSubstring(l0, "browser") {
		t.Errorf("L0 should contain skill name, got: %s", l0)
	}
	// Should contain tags
	if !containsSubstring(l0, "web,automation") {
		t.Errorf("L0 should contain tags, got: %s", l0)
	}
}

func TestGenerateSkillL1Truncation(t *testing.T) {
	// Create content longer than 8000 runes
	longContent := ""
	for i := 0; i < 10000; i++ {
		longContent += "A"
	}

	entry := SkillEntry{
		Skill: Skill{Content: longContent},
	}

	l1 := generateSkillL1(entry)
	runes := []rune(l1)
	// Should be truncated to ~8000 runes + suffix
	if len(runes) > 8100 {
		t.Errorf("L1 should be truncated, got %d runes", len(runes))
	}
}

func TestExtractTags(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"---\ntags: web,automation\n---\n# Content", "web,automation"},
		{"---\nname: test\ntags: a,b,c\n---\n# Content", "a,b,c"},
		{"# No frontmatter", ""},
		{"---\nname: test\n---\n# No tags", ""},
	}

	for _, tt := range tests {
		result := extractTags(tt.content)
		if result != tt.expected {
			t.Errorf("extractTags(%q) = %q, want %q", tt.content[:min(30, len(tt.content))], result, tt.expected)
		}
	}
}

func TestComputeContentHash(t *testing.T) {
	h1 := computeContentHash("hello")
	h2 := computeContentHash("hello")
	h3 := computeContentHash("world")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 32 {
		t.Errorf("expected 32-char hex hash, got %d chars", len(h1))
	}
}

func TestResolveSkillCategory(t *testing.T) {
	tests := []struct {
		dir      string
		name     string
		expected string
	}{
		{"/tmp/docs/skills/tools/browser", "browser", "tools"},
		{"/tmp/docs/skills/providers/openai", "openai", "providers"},
		{"/tmp/skills/my-skill", "my-skill", "general"},
		{"", "orphan", "general"},
	}

	for _, tt := range tests {
		entry := SkillEntry{
			Skill: Skill{Name: tt.name, Dir: tt.dir},
		}
		result := ResolveSkillCategory(entry)
		if result != tt.expected {
			t.Errorf("ResolveSkillCategory(dir=%q, name=%q) = %q, want %q", tt.dir, tt.name, result, tt.expected)
		}
	}
}

func TestDeterministicSkillID(t *testing.T) {
	id1 := deterministicSkillID("browser")
	id2 := deterministicSkillID("browser")
	id3 := deterministicSkillID("coder")

	if id1 != id2 {
		t.Error("same name should produce same ID")
	}
	if id1 == id3 {
		t.Error("different names should produce different IDs")
	}
	// Should be UUID-like format
	if len(id1) != 36 {
		t.Errorf("expected 36-char UUID-like ID, got %d chars: %s", len(id1), id1)
	}
}

func TestCollectDistributedCategories(t *testing.T) {
	entries := []SkillEntry{
		{Skill: Skill{Name: "a", Dir: "/tmp/skills/tools/a"}},
		{Skill: Skill{Name: "b", Dir: "/tmp/skills/tools/b"}},
		{Skill: Skill{Name: "c", Dir: "/tmp/skills/providers/c"}},
	}

	cats := CollectDistributedCategories(entries)
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d: %v", len(cats), cats)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
