package uhms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemEntryWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	meta := map[string]interface{}{
		"name":     "browser",
		"category": "tools",
		"tags":     "web,automation",
	}
	if err := vfs.WriteSystemEntry("skills", "tools", "browser", "L0: browser tool", "L1: overview...", "L2: full content...", meta); err != nil {
		t.Fatalf("WriteSystemEntry: %v", err)
	}

	// Read L0
	l0, err := vfs.ReadSystemL0("skills", "tools", "browser")
	if err != nil {
		t.Fatalf("ReadSystemL0: %v", err)
	}
	if l0 != "L0: browser tool" {
		t.Errorf("expected L0='L0: browser tool', got %q", l0)
	}

	// Read L1
	l1, err := vfs.ReadSystemL1("skills", "tools", "browser")
	if err != nil {
		t.Fatalf("ReadSystemL1: %v", err)
	}
	if l1 != "L1: overview..." {
		t.Errorf("expected L1='L1: overview...', got %q", l1)
	}

	// Read L2
	l2, err := vfs.ReadSystemL2("skills", "tools", "browser")
	if err != nil {
		t.Fatalf("ReadSystemL2: %v", err)
	}
	if l2 != "L2: full content..." {
		t.Errorf("expected L2='L2: full content...', got %q", l2)
	}

	// Read meta
	readMeta, err := vfs.ReadSystemMeta("skills", "tools", "browser")
	if err != nil {
		t.Fatalf("ReadSystemMeta: %v", err)
	}
	if readMeta["name"] != "browser" {
		t.Errorf("expected meta.name='browser', got %v", readMeta["name"])
	}
	if readMeta["tags"] != "web,automation" {
		t.Errorf("expected meta.tags='web,automation', got %v", readMeta["tags"])
	}
}

func TestSystemEntryExists(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	if vfs.SystemEntryExists("skills", "tools", "nonexistent") {
		t.Error("expected SystemEntryExists=false for nonexistent entry")
	}

	meta := map[string]interface{}{"name": "test"}
	if err := vfs.WriteSystemEntry("skills", "general", "test", "l0", "l1", "l2", meta); err != nil {
		t.Fatalf("WriteSystemEntry: %v", err)
	}

	if !vfs.SystemEntryExists("skills", "general", "test") {
		t.Error("expected SystemEntryExists=true after write")
	}
}

func TestSystemEntryDelete(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	meta := map[string]interface{}{"name": "ephemeral"}
	if err := vfs.WriteSystemEntry("skills", "temp", "ephemeral", "l0", "l1", "l2", meta); err != nil {
		t.Fatalf("WriteSystemEntry: %v", err)
	}

	if !vfs.SystemEntryExists("skills", "temp", "ephemeral") {
		t.Fatal("expected entry to exist")
	}

	if err := vfs.DeleteSystemEntry("skills", "temp", "ephemeral"); err != nil {
		t.Fatalf("DeleteSystemEntry: %v", err)
	}

	if vfs.SystemEntryExists("skills", "temp", "ephemeral") {
		t.Error("expected entry to be deleted")
	}
}

func TestListSystemEntriesAndCategories(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	// Write entries in two categories
	meta1 := map[string]interface{}{"name": "a"}
	meta2 := map[string]interface{}{"name": "b"}
	meta3 := map[string]interface{}{"name": "c"}
	if err := vfs.WriteSystemEntry("skills", "tools", "a", "l0a", "l1a", "l2a", meta1); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteSystemEntry("skills", "tools", "b", "l0b", "l1b", "l2b", meta2); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteSystemEntry("skills", "providers", "c", "l0c", "l1c", "l2c", meta3); err != nil {
		t.Fatal(err)
	}

	// List categories
	cats, err := vfs.ListSystemCategories("skills")
	if err != nil {
		t.Fatalf("ListSystemCategories: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d: %v", len(cats), cats)
	}

	// List entries in tools
	refs, err := vfs.ListSystemEntries("skills", "tools")
	if err != nil {
		t.Fatalf("ListSystemEntries: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("expected 2 entries in tools, got %d", len(refs))
	}
}

func TestBatchReadSystemL0(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	meta1 := map[string]interface{}{"name": "x"}
	meta2 := map[string]interface{}{"name": "y"}
	if err := vfs.WriteSystemEntry("skills", "cat", "x", "L0-X", "l1", "l2", meta1); err != nil {
		t.Fatal(err)
	}
	if err := vfs.WriteSystemEntry("skills", "cat", "y", "L0-Y", "l1", "l2", meta2); err != nil {
		t.Fatal(err)
	}

	refs := []SystemEntryRef{
		{Category: "cat", ID: "x"},
		{Category: "cat", ID: "y"},
		{Category: "cat", ID: "nonexistent"},
	}
	entries := vfs.BatchReadSystemL0("skills", refs)
	if len(entries) != 2 {
		t.Errorf("expected 2 L0 entries, got %d", len(entries))
	}
}

func TestReadByVFSPath(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	meta := map[string]interface{}{"name": "path-test"}
	if err := vfs.WriteSystemEntry("skills", "cat", "path-test", "l0data", "l1data", "l2data", meta); err != nil {
		t.Fatal(err)
	}

	// ReadByVFSPath with relative path
	vfsPath := filepath.Join("_system", "skills", "cat", "path-test")
	l0, err := vfs.ReadByVFSPath(vfsPath, 0)
	if err != nil {
		t.Fatalf("ReadByVFSPath L0: %v", err)
	}
	if l0 != "l0data" {
		t.Errorf("expected l0data, got %q", l0)
	}

	l2, err := vfs.ReadByVFSPath(vfsPath, 2)
	if err != nil {
		t.Fatalf("ReadByVFSPath L2: %v", err)
	}
	if l2 != "l2data" {
		t.Errorf("expected l2data, got %q", l2)
	}
}

func TestSystemEntryOverwrite(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	meta := map[string]interface{}{"name": "overwrite-test", "version": float64(1)}
	if err := vfs.WriteSystemEntry("skills", "cat", "overwrite-test", "v1-l0", "v1-l1", "v1-l2", meta); err != nil {
		t.Fatal(err)
	}

	// Overwrite
	meta2 := map[string]interface{}{"name": "overwrite-test", "version": float64(2)}
	if err := vfs.WriteSystemEntry("skills", "cat", "overwrite-test", "v2-l0", "v2-l1", "v2-l2", meta2); err != nil {
		t.Fatal(err)
	}

	l0, _ := vfs.ReadSystemL0("skills", "cat", "overwrite-test")
	if l0 != "v2-l0" {
		t.Errorf("expected v2-l0 after overwrite, got %q", l0)
	}

	readMeta, _ := vfs.ReadSystemMeta("skills", "cat", "overwrite-test")
	if v, ok := readMeta["version"].(float64); !ok || v != 2 {
		t.Errorf("expected meta.version=2, got %v", readMeta["version"])
	}
}

func TestSystemDirPath(t *testing.T) {
	dir := t.TempDir()
	vfs, err := NewLocalVFS(dir)
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}

	expected := filepath.Join(dir, "_system", "skills", "tools", "browser")
	actual := vfs.systemDir("skills", "tools", "browser")
	if actual != expected {
		t.Errorf("systemDir mismatch:\n  expected: %s\n  actual:   %s", expected, actual)
	}

	// Verify no path traversal
	expectedSafe := filepath.Join(dir, "_system", "skills", "..%2F..%2Fetc", "passwd")
	actualSafe := vfs.systemDir("skills", "..%2F..%2Fetc", "passwd")
	if actualSafe != expectedSafe {
		t.Errorf("systemDir should not resolve path traversal:\n  expected: %s\n  actual:   %s", expectedSafe, actualSafe)
	}
	// The actual directory just won't have real data, so reading it fails gracefully
	_, readErr := vfs.ReadSystemL0("skills", "../../../etc", "passwd")
	if readErr == nil {
		// Should not be able to read outside the VFS root
		entryPath := filepath.Join(dir, "_system", "skills", "../../../etc", "passwd", "l0.txt")
		if _, statErr := os.Stat(entryPath); statErr == nil {
			t.Error("path traversal should not succeed")
		}
	}
}
