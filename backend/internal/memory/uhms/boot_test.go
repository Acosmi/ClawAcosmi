package uhms

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBootFileLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")

	boot := &BootFile{
		Version:   "0.2.0",
		UpdatedAt: time.Now(),
		SystemMap: BootSystemMap{
			Skills: BootSkillsInfo{
				Indexed:    true,
				TotalCount: 42,
			},
		},
		SearchGuide: DefaultSearchGuide(),
	}

	if err := SaveBootFile(path, boot); err != nil {
		t.Fatalf("SaveBootFile: %v", err)
	}

	loaded, err := LoadBootFile(path)
	if err != nil {
		t.Fatalf("LoadBootFile: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadBootFile returned nil")
	}
	if loaded.Version != "0.2.0" {
		t.Errorf("expected version=0.2.0, got %s", loaded.Version)
	}
	if !loaded.SystemMap.Skills.Indexed {
		t.Error("expected Skills.Indexed=true")
	}
	if loaded.SystemMap.Skills.TotalCount != 42 {
		t.Errorf("expected TotalCount=42, got %d", loaded.SystemMap.Skills.TotalCount)
	}
}

func TestBootFileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	boot, err := LoadBootFile(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if boot != nil {
		t.Fatalf("expected nil boot for missing file, got: %+v", boot)
	}
}

func TestBootFileCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")

	// Write corrupt JSON
	if err := writeFile(path, "not json"); err != nil {
		t.Fatal(err)
	}

	boot, err := LoadBootFile(path)
	if err != nil {
		t.Fatalf("expected nil error for corrupt file, got: %v", err)
	}
	if boot != nil {
		t.Fatal("expected nil boot for corrupt file")
	}
}

func TestUpdateBootSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")

	session := &BootSession{
		Summary:     "Implemented feature X",
		ActiveTasks: []string{"task-1", "task-2"},
		EndedAt:     time.Now(),
	}

	if err := UpdateBootSession(path, session); err != nil {
		t.Fatalf("UpdateBootSession: %v", err)
	}

	// Should create boot file if not exists
	boot, err := LoadBootFile(path)
	if err != nil {
		t.Fatalf("LoadBootFile: %v", err)
	}
	if boot == nil {
		t.Fatal("expected boot file to be created")
	}
	if boot.LastSession == nil {
		t.Fatal("expected LastSession to be set")
	}
	if boot.LastSession.Summary != "Implemented feature X" {
		t.Errorf("expected summary='Implemented feature X', got %q", boot.LastSession.Summary)
	}
}

func TestUpdateBootSkillsInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")

	info := BootSkillsInfo{
		SourceDir:        "docs/skills/",
		VFSDir:           "_system/skills/",
		TotalCount:       71,
		Indexed:          true,
		QdrantCollection: "sys_skills",
		Categories:       []string{"tools", "providers", "general"},
	}

	if err := UpdateBootSkillsInfo(path, info); err != nil {
		t.Fatalf("UpdateBootSkillsInfo: %v", err)
	}

	boot, err := LoadBootFile(path)
	if err != nil {
		t.Fatalf("LoadBootFile: %v", err)
	}
	if boot == nil {
		t.Fatal("expected boot file to be created")
	}
	if !boot.SystemMap.Skills.Indexed {
		t.Error("expected Skills.Indexed=true")
	}
	if boot.SystemMap.Skills.TotalCount != 71 {
		t.Errorf("expected TotalCount=71, got %d", boot.SystemMap.Skills.TotalCount)
	}
}

func TestBootManagerCaching(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")

	bm := NewBootManager(path)

	// Before load
	if bm.IsSkillsIndexed() {
		t.Error("expected IsSkillsIndexed=false before load")
	}

	// Mark indexed (creates file)
	if err := bm.MarkSkillsIndexed(42); err != nil {
		t.Fatalf("MarkSkillsIndexed: %v", err)
	}

	if !bm.IsSkillsIndexed() {
		t.Error("expected IsSkillsIndexed=true after MarkSkillsIndexed")
	}

	// New manager reading from same file
	bm2 := NewBootManager(path)
	_, ok := bm2.Load()
	if !ok {
		t.Fatal("expected Load to succeed")
	}
	if !bm2.IsSkillsIndexed() {
		t.Error("expected IsSkillsIndexed=true from loaded file")
	}
}

func TestBootManagerReset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")

	bm := NewBootManager(path)
	if err := bm.MarkSkillsIndexed(10); err != nil {
		t.Fatal(err)
	}

	if err := bm.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if bm.IsSkillsIndexed() {
		t.Error("expected IsSkillsIndexed=false after reset")
	}

	// File should be gone
	boot, _ := LoadBootFile(path)
	if boot != nil {
		t.Error("expected boot file to be deleted after reset")
	}
}

func TestDefaultSearchGuide(t *testing.T) {
	guide := DefaultSearchGuide()
	if len(guide.Priority) == 0 {
		t.Error("expected non-empty priority list")
	}
	if len(guide.Tips) == 0 {
		t.Error("expected non-empty tips")
	}
}
