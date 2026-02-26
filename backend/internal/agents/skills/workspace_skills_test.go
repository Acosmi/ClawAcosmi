package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildWorkspaceSkillSnapshot_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	snap := BuildWorkspaceSkillSnapshot(BuildSnapshotParams{
		WorkspaceDir: tmpDir,
	})
	if len(snap.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(snap.Skills))
	}
	if snap.Prompt != "" {
		t.Errorf("expected empty prompt, got %q", snap.Prompt)
	}
}

func TestBuildWorkspaceSkillSnapshot_WithSkills(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, ".agent", "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: A test skill\n---\n# Test Skill\nInstructions here."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := BuildWorkspaceSkillSnapshot(BuildSnapshotParams{
		WorkspaceDir: tmpDir,
	})

	if len(snap.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(snap.Skills))
	}
	if snap.Skills[0].Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", snap.Skills[0].Name)
	}
	if snap.Prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestBuildWorkspaceSkillSnapshot_WithFilter(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		skillDir := filepath.Join(tmpDir, ".agent", "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: "+name+"\n---\nContent"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	snap := BuildWorkspaceSkillSnapshot(BuildSnapshotParams{
		WorkspaceDir: tmpDir,
		SkillFilter:  []string{"alpha", "gamma"},
	})
	if len(snap.Skills) != 2 {
		t.Fatalf("expected 2 skills after filter, got %d", len(snap.Skills))
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"no frontmatter", "# Hello", ""},
		{"with desc", "---\ndescription: My Skill\n---\nBody", "My Skill"},
		{"no desc field", "---\nname: test\n---\nBody", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDescription(tt.content)
			if got != tt.want {
				t.Errorf("extractDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildWorkspaceSkillSnapshot_PreloadedEntries(t *testing.T) {
	v := 42
	snap := BuildWorkspaceSkillSnapshot(BuildSnapshotParams{
		WorkspaceDir:    "/nonexistent",
		SnapshotVersion: &v,
		Entries: []SkillEntry{
			{
				Skill:   Skill{Name: "preloaded", Description: "desc"},
				Enabled: true,
			},
		},
	})
	if len(snap.Skills) != 1 || snap.Skills[0].Name != "preloaded" {
		t.Errorf("unexpected skills: %+v", snap.Skills)
	}
	if snap.Version == nil || *snap.Version != 42 {
		t.Error("version not set")
	}
}
