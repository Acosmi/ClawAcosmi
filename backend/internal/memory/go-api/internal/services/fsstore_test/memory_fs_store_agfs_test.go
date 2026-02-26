// Package fsstoretest — isolated tests for FSStoreService with AGFS localfs backend.
// Separated from services package to avoid CGO/FFI linkage requirements.
//
// Phase C: Tests use a mock AGFSClient to validate all VFS operations
// without requiring a real AGFS Server.
package fsstoretest

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Mock AGFS Client ---

type mockFile struct {
	data    []byte
	isDir   bool
	modTime time.Time
}

type mockAGFS struct {
	mu    sync.Mutex
	files map[string]*mockFile
}

func newMockAGFS() *mockAGFS {
	return &mockAGFS{files: make(map[string]*mockFile)}
}

func (m *mockAGFS) Mkdir(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = &mockFile{isDir: true, modTime: time.Now()}
	return nil
}

func (m *mockAGFS) WriteFile(path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Auto-create parent dirs.
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if dir == "" {
			continue
		}
		if _, ok := m.files[dir]; !ok {
			m.files[dir] = &mockFile{isDir: true, modTime: time.Now()}
		}
	}
	m.files[path] = &mockFile{data: data, modTime: time.Now()}
	return nil
}

func (m *mockAGFS) ReadFile(path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return f.data, nil
}

type mockFileInfo struct {
	Name    string
	IsDir   bool
	ModTime time.Time
}

func (m *mockAGFS) ListDir(path string) ([]mockFileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	prefix := path + "/"
	seen := make(map[string]bool)
	var result []mockFileInfo

	for p, f := range m.files {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := p[len(prefix):]
		// Only immediate children (no nested slashes).
		if idx := strings.Index(rest, "/"); idx >= 0 {
			name := rest[:idx]
			if !seen[name] {
				seen[name] = true
				result = append(result, mockFileInfo{Name: name, IsDir: true, ModTime: f.modTime})
			}
		} else {
			if !seen[rest] {
				seen[rest] = true
				result = append(result, mockFileInfo{Name: rest, IsDir: f.isDir, ModTime: f.modTime})
			}
		}
	}
	return result, nil
}

func (m *mockAGFS) RemoveAll(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := path + "/"
	for p := range m.files {
		if p == path || strings.HasPrefix(p, prefix) {
			delete(m.files, p)
		}
	}
	return nil
}

func (m *mockAGFS) FileExists(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[path]
	return ok
}

// --- Minimal FSStoreService replica for testing ---

const vfsPrefix = "/vfs"

type FSStoreService struct {
	agfs *mockAGFS
}

func NewFSStoreService(agfs *mockAGFS) *FSStoreService {
	agfs.Mkdir(vfsPrefix)
	return &FSStoreService{agfs: agfs}
}

func userRoot(tenant, user string) string {
	return fmt.Sprintf("%s/%s/%s", vfsPrefix, tenant, user)
}

func (s *FSStoreService) WriteMemory(tenant, user, memID, section, category, content, l0, l1 string) error {
	root := userRoot(tenant, user)
	dir := fmt.Sprintf("%s/%s/%s/%s", root, section, category, memID)

	s.agfs.Mkdir(root)
	s.agfs.Mkdir(fmt.Sprintf("%s/%s", root, section))
	s.agfs.Mkdir(fmt.Sprintf("%s/%s/%s", root, section, category))
	s.agfs.Mkdir(dir)

	if err := s.agfs.WriteFile(dir+"/l0.txt", []byte(l0)); err != nil {
		return err
	}
	if err := s.agfs.WriteFile(dir+"/l1.txt", []byte(l1)); err != nil {
		return err
	}
	if err := s.agfs.WriteFile(dir+"/l2.txt", []byte(content)); err != nil {
		return err
	}

	meta := map[string]interface{}{
		"memory_id": memID, "section": section, "category": category,
		"created_at": time.Now().Unix(),
	}
	metaBytes, _ := json.Marshal(meta)
	return s.agfs.WriteFile(dir+"/meta.json", metaBytes)
}

func (s *FSStoreService) ReadMemory(tenant, user, path string, level int) (string, error) {
	var filename string
	switch level {
	case 0:
		filename = "l0.txt"
	case 1:
		filename = "l1.txt"
	default:
		filename = "l2.txt"
	}
	fullPath := fmt.Sprintf("%s/%s/%s", userRoot(tenant, user), path, filename)
	data, err := s.agfs.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *FSStoreService) DeleteMemory(tenant, user, memID string) error {
	root := userRoot(tenant, user)
	entries, err := s.agfs.ListDir(root)
	if err != nil {
		return err
	}
	for _, section := range entries {
		if !section.IsDir || section.Name == "archives" {
			continue
		}
		sPath := fmt.Sprintf("%s/%s", root, section.Name)
		cats, cerr := s.agfs.ListDir(sPath)
		if cerr != nil {
			continue
		}
		for _, cat := range cats {
			if !cat.IsDir {
				continue
			}
			memDir := fmt.Sprintf("%s/%s/%s", sPath, cat.Name, memID)
			if s.agfs.FileExists(memDir) {
				return s.agfs.RemoveAll(memDir)
			}
		}
	}
	return fmt.Errorf("memory %s not found", memID)
}

func (s *FSStoreService) MemoryCount(tenant, user string) int {
	count := 0
	root := userRoot(tenant, user)
	sections, err := s.agfs.ListDir(root)
	if err != nil {
		return 0
	}
	for _, section := range sections {
		if !section.IsDir || section.Name == "archives" {
			continue
		}
		cats, cerr := s.agfs.ListDir(fmt.Sprintf("%s/%s", root, section.Name))
		if cerr != nil {
			continue
		}
		for _, cat := range cats {
			if !cat.IsDir {
				continue
			}
			mems, merr := s.agfs.ListDir(fmt.Sprintf("%s/%s/%s", root, section.Name, cat.Name))
			if merr != nil {
				continue
			}
			for _, m := range mems {
				if m.IsDir {
					count++
				}
			}
		}
	}
	return count
}

func (s *FSStoreService) CreateArchiveDir(tenant, user string, index int, l0, l1 string) error {
	archDir := fmt.Sprintf("%s/archives", userRoot(tenant, user))
	s.agfs.Mkdir(archDir)
	dir := fmt.Sprintf("%s/%d", archDir, index)
	s.agfs.Mkdir(dir)
	if err := s.agfs.WriteFile(dir+"/l0.txt", []byte(l0)); err != nil {
		return err
	}
	return s.agfs.WriteFile(dir+"/l1.txt", []byte(l1))
}

func (s *FSStoreService) NextArchiveIndex(tenant, user string) int {
	archDir := fmt.Sprintf("%s/archives", userRoot(tenant, user))
	entries, err := s.agfs.ListDir(archDir)
	if err != nil {
		return 0
	}
	maxIdx := -1
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		var idx int
		if _, perr := fmt.Sscanf(e.Name, "%d", &idx); perr == nil && idx > maxIdx {
			maxIdx = idx
		}
	}
	return maxIdx + 1
}

func (s *FSStoreService) SearchMemories(tenant, user, query string, maxResults int) int {
	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		return 0
	}
	count := 0
	root := userRoot(tenant, user)
	sections, _ := s.agfs.ListDir(root)
	for _, sec := range sections {
		if !sec.IsDir || sec.Name == "archives" {
			continue
		}
		cats, _ := s.agfs.ListDir(fmt.Sprintf("%s/%s", root, sec.Name))
		for _, cat := range cats {
			if !cat.IsDir {
				continue
			}
			mems, _ := s.agfs.ListDir(fmt.Sprintf("%s/%s/%s", root, sec.Name, cat.Name))
			for _, m := range mems {
				if !m.IsDir {
					continue
				}
				l0Path := fmt.Sprintf("%s/%s/%s/%s/l0.txt", root, sec.Name, cat.Name, m.Name)
				l0Data, rerr := s.agfs.ReadFile(l0Path)
				if rerr != nil {
					continue
				}
				l0Lower := strings.ToLower(string(l0Data))
				for _, kw := range keywords {
					if strings.Contains(l0Lower, kw) {
						count++
						break
					}
				}
				if count >= maxResults {
					return count
				}
			}
		}
	}
	return count
}

// --- Tests ---

func TestFSStore_WriteAndReadMemory(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	err := s.WriteMemory("t1", "u1", "mem-001", "permanent", "fact",
		"Go is a compiled language", "Go language", "Go is compiled and statically typed")
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}

	// L0
	l0, err := s.ReadMemory("t1", "u1", "permanent/fact/mem-001", 0)
	if err != nil {
		t.Fatalf("ReadMemory L0: %v", err)
	}
	if l0 != "Go language" {
		t.Errorf("L0 = %q, want %q", l0, "Go language")
	}

	// L1
	l1, err := s.ReadMemory("t1", "u1", "permanent/fact/mem-001", 1)
	if err != nil {
		t.Fatalf("ReadMemory L1: %v", err)
	}
	if l1 != "Go is compiled and statically typed" {
		t.Errorf("L1 = %q, want %q", l1, "Go is compiled and statically typed")
	}

	// L2
	l2, err := s.ReadMemory("t1", "u1", "permanent/fact/mem-001", 2)
	if err != nil {
		t.Fatalf("ReadMemory L2: %v", err)
	}
	if l2 != "Go is a compiled language" {
		t.Errorf("L2 = %q, want %q", l2, "Go is a compiled language")
	}
}

func TestFSStore_DeleteMemory(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	_ = s.WriteMemory("t1", "u1", "mem-del", "permanent", "habit",
		"Daily coding", "Coding habit", "Codes daily")

	// Delete
	if err := s.DeleteMemory("t1", "u1", "mem-del"); err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}

	// Read should fail
	_, err := s.ReadMemory("t1", "u1", "permanent/habit/mem-del", 0)
	if err == nil {
		t.Error("Expected error reading deleted memory, got nil")
	}
}

func TestFSStore_DeleteMemory_NotFound(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	err := s.DeleteMemory("t1", "u1", "nonexistent")
	if err == nil {
		t.Error("Expected error deleting non-existent memory, got nil")
	}
}

func TestFSStore_MemoryCount(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	_ = s.WriteMemory("t1", "u1", "m1", "permanent", "fact", "c1", "l0", "l1")
	_ = s.WriteMemory("t1", "u1", "m2", "permanent", "skill", "c2", "l0", "l1")
	_ = s.WriteMemory("t1", "u1", "m3", "episodic", "event", "c3", "l0", "l1")

	count := s.MemoryCount("t1", "u1")
	if count != 3 {
		t.Errorf("MemoryCount = %d, want 3", count)
	}
}

func TestFSStore_MemoryCount_Empty(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	count := s.MemoryCount("t1", "u1")
	if count != 0 {
		t.Errorf("MemoryCount = %d, want 0", count)
	}
}

func TestFSStore_Archive(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// First archive
	idx := s.NextArchiveIndex("t1", "u1")
	if idx != 0 {
		t.Errorf("NextArchiveIndex = %d, want 0", idx)
	}

	if err := s.CreateArchiveDir("t1", "u1", 0, "Session summary 1", "Full overview 1"); err != nil {
		t.Fatalf("CreateArchiveDir: %v", err)
	}

	// Second archive
	idx = s.NextArchiveIndex("t1", "u1")
	if idx != 1 {
		t.Errorf("NextArchiveIndex = %d, want 1", idx)
	}

	if err := s.CreateArchiveDir("t1", "u1", 1, "Session summary 2", "Full overview 2"); err != nil {
		t.Fatalf("CreateArchiveDir: %v", err)
	}

	idx = s.NextArchiveIndex("t1", "u1")
	if idx != 2 {
		t.Errorf("NextArchiveIndex = %d, want 2", idx)
	}
}

func TestFSStore_SearchMemories(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	_ = s.WriteMemory("t1", "u1", "m1", "permanent", "fact",
		"Go is a compiled language", "Go compiled language", "overview")
	_ = s.WriteMemory("t1", "u1", "m2", "permanent", "skill",
		"Python is interpreted", "Python scripting", "overview")
	_ = s.WriteMemory("t1", "u1", "m3", "permanent", "fact",
		"Rust is safe", "Rust memory safe", "overview")

	// Search for "Go"
	hits := s.SearchMemories("t1", "u1", "Go", 10)
	if hits != 1 {
		t.Errorf("SearchMemories(Go) = %d hits, want 1", hits)
	}

	// Search for "compiled"
	hits = s.SearchMemories("t1", "u1", "compiled", 10)
	if hits != 1 {
		t.Errorf("SearchMemories(compiled) = %d hits, want 1", hits)
	}

	// Search for "safe"
	hits = s.SearchMemories("t1", "u1", "safe", 10)
	if hits != 1 {
		t.Errorf("SearchMemories(safe) = %d hits, want 1", hits)
	}

	// Empty query
	hits = s.SearchMemories("t1", "u1", "", 10)
	if hits != 0 {
		t.Errorf("SearchMemories('') = %d hits, want 0", hits)
	}
}

func TestFSStore_MultiTenantIsolation(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	_ = s.WriteMemory("tenantA", "user1", "m1", "permanent", "fact", "c1", "l0", "l1")
	_ = s.WriteMemory("tenantB", "user1", "m2", "permanent", "fact", "c2", "l0", "l1")

	countA := s.MemoryCount("tenantA", "user1")
	countB := s.MemoryCount("tenantB", "user1")

	if countA != 1 {
		t.Errorf("tenantA count = %d, want 1", countA)
	}
	if countB != 1 {
		t.Errorf("tenantB count = %d, want 1", countB)
	}
}

// --- L0Entry and BatchReadL0 (Phase 1: Progressive Loading) ---

// L0Entry represents a lightweight L0 abstract entry.
type L0Entry struct {
	URI        string `json:"uri"`
	MemoryID   string `json:"memory_id"`
	L0Abstract string `json:"l0_abstract"`
	MemoryType string `json:"memory_type"`
	Category   string `json:"category"`
	CreatedAt  int64  `json:"created_at"`
}

// BatchReadL0 批量读取一组 URI 的 L0 摘要。
// URI 格式: "{section}/{category}/{memoryID}"
func (s *FSStoreService) BatchReadL0(tenant, user string, uris []string) ([]L0Entry, error) {
	if len(uris) == 0 {
		return nil, nil
	}

	results := make([]L0Entry, 0, len(uris))
	root := userRoot(tenant, user)

	for _, uri := range uris {
		// Read l0.txt
		l0Path := fmt.Sprintf("%s/%s/l0.txt", root, uri)
		l0Data, err := s.agfs.ReadFile(l0Path)
		if err != nil {
			continue // skip missing URIs
		}

		// Parse URI components: "{section}/{category}/{memoryID}"
		parts := strings.SplitN(uri, "/", 3)
		entry := L0Entry{
			URI:        uri,
			L0Abstract: string(l0Data),
		}
		if len(parts) >= 3 {
			entry.MemoryType = parts[0]
			entry.Category = parts[1]
			entry.MemoryID = parts[2]
		} else if len(parts) >= 1 {
			entry.MemoryID = parts[len(parts)-1]
		}

		// Try to read meta.json for created_at
		metaPath := fmt.Sprintf("%s/%s/meta.json", root, uri)
		metaData, merr := s.agfs.ReadFile(metaPath)
		if merr == nil {
			var meta map[string]interface{}
			if jerr := json.Unmarshal(metaData, &meta); jerr == nil {
				if ts, ok := meta["created_at"]; ok {
					if v, ok := ts.(float64); ok {
						entry.CreatedAt = int64(v)
					}
				}
				if section, ok := meta["section"].(string); ok && entry.MemoryType == "" {
					entry.MemoryType = section
				}
				if cat, ok := meta["category"].(string); ok && entry.Category == "" {
					entry.Category = cat
				}
			}
		}

		results = append(results, entry)
	}
	return results, nil
}

// --- BatchReadL0 Tests ---

func TestFSStore_BatchReadL0_Normal(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// Write 3 memories
	_ = s.WriteMemory("t1", "u1", "mem-001", "permanent", "fact",
		"Go is compiled", "Go语言摘要", "Go详细概述")
	_ = s.WriteMemory("t1", "u1", "mem-002", "permanent", "skill",
		"Rust is safe", "Rust安全摘要", "Rust详细概述")
	_ = s.WriteMemory("t1", "u1", "mem-003", "episodic", "event",
		"Meeting today", "会议摘要", "会议概述")

	// Batch read L0
	uris := []string{
		"permanent/fact/mem-001",
		"permanent/skill/mem-002",
		"episodic/event/mem-003",
	}
	entries, err := s.BatchReadL0("t1", "u1", uris)
	if err != nil {
		t.Fatalf("BatchReadL0: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("BatchReadL0 returned %d entries, want 3", len(entries))
	}

	// Verify first entry
	if entries[0].L0Abstract != "Go语言摘要" {
		t.Errorf("entries[0].L0Abstract = %q, want %q", entries[0].L0Abstract, "Go语言摘要")
	}
	if entries[0].MemoryID != "mem-001" {
		t.Errorf("entries[0].MemoryID = %q, want %q", entries[0].MemoryID, "mem-001")
	}
	if entries[0].MemoryType != "permanent" {
		t.Errorf("entries[0].MemoryType = %q, want %q", entries[0].MemoryType, "permanent")
	}
	if entries[0].Category != "fact" {
		t.Errorf("entries[0].Category = %q, want %q", entries[0].Category, "fact")
	}

	// Verify third entry (different section)
	if entries[2].L0Abstract != "会议摘要" {
		t.Errorf("entries[2].L0Abstract = %q, want %q", entries[2].L0Abstract, "会议摘要")
	}
	if entries[2].MemoryType != "episodic" {
		t.Errorf("entries[2].MemoryType = %q, want %q", entries[2].MemoryType, "episodic")
	}
}

func TestFSStore_BatchReadL0_Empty(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	entries, err := s.BatchReadL0("t1", "u1", []string{})
	if err != nil {
		t.Fatalf("BatchReadL0 empty: %v", err)
	}
	if entries != nil {
		t.Errorf("BatchReadL0 empty returned %v, want nil", entries)
	}

	// nil input
	entries, err = s.BatchReadL0("t1", "u1", nil)
	if err != nil {
		t.Fatalf("BatchReadL0 nil: %v", err)
	}
	if entries != nil {
		t.Errorf("BatchReadL0 nil returned %v, want nil", entries)
	}
}

func TestFSStore_BatchReadL0_NonExistent(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// Write one memory but request non-existent URIs
	_ = s.WriteMemory("t1", "u1", "exists", "permanent", "fact", "c", "l0", "l1")

	uris := []string{
		"permanent/fact/nonexistent-1",
		"permanent/fact/nonexistent-2",
		"episodic/event/nonexistent-3",
	}
	entries, err := s.BatchReadL0("t1", "u1", uris)
	if err != nil {
		t.Fatalf("BatchReadL0 nonexistent: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("BatchReadL0 nonexistent returned %d entries, want 0", len(entries))
	}
}

func TestFSStore_BatchReadL0_CJK(t *testing.T) {
	s := NewFSStoreService(newMockAGFS())

	// Write memories with CJK content
	_ = s.WriteMemory("t1", "u1", "cjk-1", "permanent", "fact",
		"用户喜欢中国传统文化，尤其是书法和水墨画",
		"中国传统文化偏好：书法、水墨画",
		"用户对中国传统文化有浓厚兴趣")
	_ = s.WriteMemory("t1", "u1", "cjk-2", "permanent", "fact",
		"ユーザーは日本語が話せます",
		"日本語スキル",
		"日本語の能力")
	_ = s.WriteMemory("t1", "u1", "cjk-3", "permanent", "fact",
		"사용자는 한국 드라마를 좋아합니다",
		"한국 드라마 선호",
		"한국 드라마에 대한 관심")

	uris := []string{
		"permanent/fact/cjk-1",
		"permanent/fact/cjk-2",
		"permanent/fact/cjk-3",
	}
	entries, err := s.BatchReadL0("t1", "u1", uris)
	if err != nil {
		t.Fatalf("BatchReadL0 CJK: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("BatchReadL0 CJK returned %d entries, want 3", len(entries))
	}

	// Verify CJK content integrity
	if entries[0].L0Abstract != "中国传统文化偏好：书法、水墨画" {
		t.Errorf("CJK Chinese L0 = %q, want %q", entries[0].L0Abstract, "中国传统文化偏好：书法、水墨画")
	}
	if entries[1].L0Abstract != "日本語スキル" {
		t.Errorf("CJK Japanese L0 = %q, want %q", entries[1].L0Abstract, "日本語スキル")
	}
	if entries[2].L0Abstract != "한국 드라마 선호" {
		t.Errorf("CJK Korean L0 = %q, want %q", entries[2].L0Abstract, "한국 드라마 선호")
	}
}
