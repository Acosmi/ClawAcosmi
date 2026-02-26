// Package services — FSStoreService: file-system-based permanent memory store.
//
// Phase C migration: replaced Rust FFI (nexus-memfs) with AGFS localfs HTTP API.
// All VFS data lives under /vfs/{tenantID}/{userID}/ on AGFS localfs, enabling
// multi-instance shared access via AGFS Server.
//
// Directory layout:
//
//	/vfs/{tenant}/{user}/{section}/{category}/{memoryID}/
//	  ├── l0.txt   ← L0 abstract
//	  ├── l1.txt   ← L1 overview
//	  └── l2.txt   ← L2 detail content
//	/vfs/{tenant}/{user}/archives/{index}/
//	  ├── l0.txt   ← session archive summary
//	  └── l1.txt   ← session archive overview
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/agfs"
)

// --- Storage Mode Constants ---

// MemoryStorageMode defines how permanent memories are stored.
const (
	// StorageModeVector uses only the existing vector store + reranking.
	StorageModeVector = "vector"
	// StorageModeFS uses only the AGFS VFS (shared filesystem).
	StorageModeFS = "fs"
	// StorageModeHybrid writes to both vector and VFS, merges on read.
	// Recommended default — combines semantic recall with structured browsing.
	StorageModeHybrid = "hybrid"
)

// DefaultStorageMode is the default mode for new tenants.
const DefaultStorageMode = StorageModeVector

// vfsPrefix is the base path under AGFS localfs for all VFS data.
const vfsPrefix = "/vfs"

// --- Types (compatible with existing consumers) ---

// DirEntry represents a directory entry in the VFS.
// Replaces the former ffi.MemFSDirEntry.
type DirEntry struct {
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	L0Abstract string `json:"l0_abstract"`
	CreatedAt  uint64 `json:"created_at"`
}

// SearchHit represents a search result from VFS.
// Replaces the former ffi.MemFSSearchHit.
type SearchHit struct {
	Path       string  `json:"path"`
	MemoryID   string  `json:"memory_id"`
	Category   string  `json:"category"`
	Score      float64 `json:"score"`
	L0Abstract string  `json:"l0_abstract"`
}

// TraceStep represents a step in the search trace.
// Replaces the former ffi.MemFSTraceStep.
type TraceStep struct {
	Path             string  `json:"path"`
	NodeType         string  `json:"node_type"`
	Score            float64 `json:"score"`
	ChildrenExplored int     `json:"children_explored"`
	Matched          bool    `json:"matched"`
}

// SearchTrace represents the full search trace for visualization.
// Replaces the former ffi.MemFSSearchTrace.
type SearchTrace struct {
	Query            string      `json:"query"`
	Keywords         []string    `json:"keywords"`
	Steps            []TraceStep `json:"steps"`
	TotalDirsVisited int         `json:"total_dirs_visited"`
	TotalFilesScored int         `json:"total_files_scored"`
	Hits             []SearchHit `json:"hits"`
}

// L0Entry represents a lightweight L0 abstract for progressive loading.
// Used by BatchReadL0 to return minimal metadata for LLM filtering (Phase 1).
type L0Entry struct {
	URI        string `json:"uri"` // VFS path: "{section}/{category}/{memoryID}"
	MemoryID   string `json:"memory_id"`
	L0Abstract string `json:"l0_abstract"`
	MemoryType string `json:"memory_type"` // from meta.json section field
	Category   string `json:"category"`
	CreatedAt  int64  `json:"created_at"` // Unix timestamp
}

// L1Entry represents an L1 overview entry for progressive loading.
// Used by BatchReadL1 to return overview content after LLM L0 filtering (Phase 2).
type L1Entry struct {
	URI        string `json:"uri"` // VFS path: "{section}/{category}/{memoryID}"
	MemoryID   string `json:"memory_id"`
	L1Overview string `json:"l1_overview"` // L1 overview content from l1.txt
	MemoryType string `json:"memory_type"` // from meta.json section field
	Category   string `json:"category"`
	CreatedAt  int64  `json:"created_at"` // Unix timestamp
}

// --- FSStoreService ---

// FSStoreService provides permanent memory operations via AGFS localfs.
// Phase C: all data flows through AGFS HTTP API for multi-instance sharing.
type FSStoreService struct {
	agfsClient  *agfs.AGFSClient
	initialized bool
}

var (
	fsStoreOnce sync.Once
	fsStoreInst *FSStoreService
)

// NewFSStoreService creates the singleton FSStoreService backed by AGFS.
// Replaces the former GetFSStoreService(rootPath) which required FFI.
func NewFSStoreService(agfsClient *agfs.AGFSClient) (*FSStoreService, error) {
	fsStoreOnce.Do(func() {
		// Ensure the VFS root directory exists on AGFS.
		if err := agfsClient.Mkdir(vfsPrefix); err != nil {
			slog.Debug("VFS root directory may already exist", "error", err)
		}
		fsStoreInst = &FSStoreService{
			agfsClient:  agfsClient,
			initialized: true,
		}
		slog.Info("FSStoreService 初始化完成 (AGFS localfs)")
	})
	if fsStoreInst == nil {
		return nil, fmt.Errorf("FSStoreService not initialized")
	}
	return fsStoreInst, nil
}

// GetFSStoreService is a backward-compatible alias.
// Deprecated: use NewFSStoreService(agfsClient) instead.
func GetFSStoreService(rootPath string) (*FSStoreService, error) {
	// In AGFS mode this is only valid if already initialized by NewFSStoreService.
	if fsStoreInst != nil && fsStoreInst.initialized {
		return fsStoreInst, nil
	}
	return nil, fmt.Errorf("FSStoreService not initialized; use NewFSStoreService(agfsClient)")
}

// --- Path helpers ---

func userVFSRoot(tenantID, userID string) string {
	return fmt.Sprintf("%s/%s/%s", vfsPrefix, tenantID, userID)
}

func memoryDir(tenantID, userID, section, category, memoryID string) string {
	return fmt.Sprintf("%s/%s/%s/%s", userVFSRoot(tenantID, userID), section, category, memoryID)
}

func archivesDir(tenantID, userID string) string {
	return fmt.Sprintf("%s/archives", userVFSRoot(tenantID, userID))
}

// ensureDir creates a directory path, ignoring "already exists" errors.
func (s *FSStoreService) ensureDir(path string) {
	_ = s.agfsClient.Mkdir(path)
}

// --- Write ---

// WriteMemoryTo writes a memory to the VFS under a specific section.
func (s *FSStoreService) WriteMemoryTo(
	ctx context.Context,
	tenantID, userID string,
	memoryID uuid.UUID,
	section, category, content, l0Abstract, l1Overview string,
) error {
	if !s.initialized {
		return fmt.Errorf("FSStoreService not initialized")
	}

	// Acquire per-user distributed lock to serialize VFS writes.
	lock := globalVFSLock.ForUser(tenantID, userID)
	lock.Lock()
	defer lock.Unlock()

	dir := memoryDir(tenantID, userID, section, category, memoryID.String())

	// Create directory hierarchy.
	s.ensureDir(userVFSRoot(tenantID, userID))
	s.ensureDir(fmt.Sprintf("%s/%s", userVFSRoot(tenantID, userID), section))
	s.ensureDir(fmt.Sprintf("%s/%s/%s", userVFSRoot(tenantID, userID), section, category))
	s.ensureDir(dir)

	// Write L0/L1/L2 as separate files.
	if err := s.agfsClient.WriteFile(dir+"/l0.txt", []byte(l0Abstract)); err != nil {
		return fmt.Errorf("fs write l0: %w", err)
	}
	if err := s.agfsClient.WriteFile(dir+"/l1.txt", []byte(l1Overview)); err != nil {
		return fmt.Errorf("fs write l1: %w", err)
	}
	if err := s.agfsClient.WriteFile(dir+"/l2.txt", []byte(content)); err != nil {
		return fmt.Errorf("fs write l2: %w", err)
	}

	// Write metadata.
	meta := map[string]interface{}{
		"memory_id":  memoryID.String(),
		"section":    section,
		"category":   category,
		"created_at": time.Now().Unix(),
	}
	metaBytes, _ := json.Marshal(meta)
	if err := s.agfsClient.WriteFile(dir+"/meta.json", metaBytes); err != nil {
		return fmt.Errorf("fs write meta: %w", err)
	}

	slog.Debug("记忆已写入 VFS (AGFS)",
		"tenant", tenantID,
		"user", userID,
		"memory_id", memoryID,
		"section", section,
		"category", category,
	)
	return nil
}

// WriteMemory writes a permanent memory to the VFS.
// Backward-compatible wrapper for WriteMemoryTo("permanent", ...).
func (s *FSStoreService) WriteMemory(
	ctx context.Context,
	tenantID, userID string,
	memoryID uuid.UUID,
	category, content, l0Abstract, l1Overview string,
) error {
	return s.WriteMemoryTo(ctx, tenantID, userID, memoryID, "permanent", category, content, l0Abstract, l1Overview)
}

// --- Read ---

// ReadMemory reads memory content at a given tier level.
// level: 0=L0(abstract), 1=L1(overview), 2=L2(detail).
func (s *FSStoreService) ReadMemory(
	tenantID, userID, path string,
	level int,
) (string, error) {
	if !s.initialized {
		return "", fmt.Errorf("FSStoreService not initialized")
	}

	var filename string
	switch level {
	case 0:
		filename = "l0.txt"
	case 1:
		filename = "l1.txt"
	default:
		filename = "l2.txt"
	}

	fullPath := fmt.Sprintf("%s/%s/%s", userVFSRoot(tenantID, userID), path, filename)
	data, err := s.agfsClient.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("fs read level %d at %s: %w", level, path, err)
	}
	return string(data), nil
}

// BatchReadL0 批量读取一组 URI 的 L0 摘要。
// URI 格式: "{section}/{category}/{memoryID}"
// 适用于 segment 语义搜索返回 URI 列表后，批量获取 L0 摘要以供 LLM 筛选。
// 读取失败的 URI 会被跳过（不中断整批操作）。
func (s *FSStoreService) BatchReadL0(
	tenantID, userID string,
	uris []string,
) ([]L0Entry, error) {
	if !s.initialized {
		return nil, fmt.Errorf("FSStoreService not initialized")
	}
	if len(uris) == 0 {
		return nil, nil
	}

	results := make([]L0Entry, 0, len(uris))
	root := userVFSRoot(tenantID, userID)

	for _, uri := range uris {
		// Read l0.txt
		l0Path := fmt.Sprintf("%s/%s/l0.txt", root, uri)
		l0Data, err := s.agfsClient.ReadFile(l0Path)
		if err != nil {
			slog.Debug("BatchReadL0: l0.txt 读取失败，跳过",
				"uri", uri, "error", err)
			continue
		}

		// Parse URI components: "{section}/{category}/{memoryID}"
		parts := strings.SplitN(uri, "/", 3)
		entry := L0Entry{
			URI:        uri,
			L0Abstract: string(l0Data),
		}
		if len(parts) >= 3 {
			entry.MemoryType = parts[0] // section
			entry.Category = parts[1]
			entry.MemoryID = parts[2]
		} else if len(parts) >= 1 {
			entry.MemoryID = parts[len(parts)-1]
		}

		// Try to read meta.json for created_at timestamp
		metaPath := fmt.Sprintf("%s/%s/meta.json", root, uri)
		metaData, merr := s.agfsClient.ReadFile(metaPath)
		if merr == nil {
			var meta map[string]interface{}
			if jerr := json.Unmarshal(metaData, &meta); jerr == nil {
				if ts, ok := meta["created_at"]; ok {
					switch v := ts.(type) {
					case float64:
						entry.CreatedAt = int64(v)
					case json.Number:
						if n, nerr := v.Int64(); nerr == nil {
							entry.CreatedAt = n
						}
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

	slog.Debug("BatchReadL0 完成",
		"requested", len(uris),
		"returned", len(results),
		"tenant", tenantID,
		"user", userID,
	)
	return results, nil
}

// BatchReadL1 批量读取一组 URI 的 L1 概述。
// URI 格式: "{section}/{category}/{memoryID}"
// 适用于 LLM 筛选 Top-K 后，批量获取 L1 概述内容。
// 读取失败的 URI 会被跳过（不中断整批操作）。
func (s *FSStoreService) BatchReadL1(
	tenantID, userID string,
	uris []string,
) ([]L1Entry, error) {
	if !s.initialized {
		return nil, fmt.Errorf("FSStoreService not initialized")
	}
	if len(uris) == 0 {
		return nil, nil
	}

	results := make([]L1Entry, 0, len(uris))
	root := userVFSRoot(tenantID, userID)

	for _, uri := range uris {
		// Read l1.txt
		l1Path := fmt.Sprintf("%s/%s/l1.txt", root, uri)
		l1Data, err := s.agfsClient.ReadFile(l1Path)
		if err != nil {
			slog.Debug("BatchReadL1: l1.txt 读取失败，跳过",
				"uri", uri, "error", err)
			continue
		}

		// Parse URI components: "{section}/{category}/{memoryID}"
		parts := strings.SplitN(uri, "/", 3)
		entry := L1Entry{
			URI:        uri,
			L1Overview: string(l1Data),
		}
		if len(parts) >= 3 {
			entry.MemoryType = parts[0] // section
			entry.Category = parts[1]
			entry.MemoryID = parts[2]
		} else if len(parts) >= 1 {
			entry.MemoryID = parts[len(parts)-1]
		}

		// Try to read meta.json for created_at timestamp
		metaPath := fmt.Sprintf("%s/%s/meta.json", root, uri)
		metaData, merr := s.agfsClient.ReadFile(metaPath)
		if merr == nil {
			var meta map[string]interface{}
			if jerr := json.Unmarshal(metaData, &meta); jerr == nil {
				if ts, ok := meta["created_at"]; ok {
					switch v := ts.(type) {
					case float64:
						entry.CreatedAt = int64(v)
					case json.Number:
						if n, nerr := v.Int64(); nerr == nil {
							entry.CreatedAt = n
						}
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

	slog.Debug("BatchReadL1 完成",
		"requested", len(uris),
		"returned", len(results),
		"tenant", tenantID,
		"user", userID,
	)
	return results, nil
}

// --- List Directory ---

// ListDir lists entries in a VFS directory.
func (s *FSStoreService) ListDir(
	tenantID, userID, path string,
) ([]DirEntry, error) {
	if !s.initialized {
		return nil, fmt.Errorf("FSStoreService not initialized")
	}

	fullPath := fmt.Sprintf("%s/%s", userVFSRoot(tenantID, userID), path)
	entries, err := s.agfsClient.ListDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("fs listdir %s: %w", path, err)
	}

	result := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		de := DirEntry{
			Name:  e.Name,
			IsDir: e.IsDir,
		}
		// Try to load L0 abstract for directories (memory entries).
		if e.IsDir {
			l0Path := fmt.Sprintf("%s/%s/l0.txt", fullPath, e.Name)
			if l0Data, rerr := s.agfsClient.ReadFile(l0Path); rerr == nil {
				de.L0Abstract = string(l0Data)
			}
		}
		if !e.ModTime.IsZero() {
			de.CreatedAt = uint64(e.ModTime.Unix())
		}
		result = append(result, de)
	}
	return result, nil
}

// --- Search ---

// SearchMemories searches for memories matching keywords via VFS.
// In AGFS mode, this performs L0 keyword matching across all memory entries.
// Semantic search is handled by VectorStore; this provides structured recall.
func (s *FSStoreService) SearchMemories(
	tenantID, userID, query string,
	maxResults int,
) ([]SearchHit, error) {
	if !s.initialized {
		return nil, fmt.Errorf("FSStoreService not initialized")
	}

	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		return nil, nil
	}

	var hits []SearchHit

	// Walk section directories under user root.
	userRoot := userVFSRoot(tenantID, userID)
	sections, err := s.agfsClient.ListDir(userRoot)
	if err != nil {
		return nil, fmt.Errorf("fs search list user root: %w", err)
	}

	for _, section := range sections {
		if !section.IsDir || section.Name == "archives" {
			continue
		}
		sectionPath := fmt.Sprintf("%s/%s", userRoot, section.Name)
		categories, cerr := s.agfsClient.ListDir(sectionPath)
		if cerr != nil {
			continue
		}
		for _, cat := range categories {
			if !cat.IsDir {
				continue
			}
			catPath := fmt.Sprintf("%s/%s", sectionPath, cat.Name)
			memories, merr := s.agfsClient.ListDir(catPath)
			if merr != nil {
				continue
			}
			for _, mem := range memories {
				if !mem.IsDir {
					continue
				}
				// Read L0 for keyword matching.
				l0Path := fmt.Sprintf("%s/%s/l0.txt", catPath, mem.Name)
				l0Data, rerr := s.agfsClient.ReadFile(l0Path)
				if rerr != nil {
					continue
				}
				l0Lower := strings.ToLower(string(l0Data))
				score := 0.0
				for _, kw := range keywords {
					if strings.Contains(l0Lower, kw) {
						score += 1.0
					}
				}
				if score > 0 {
					hits = append(hits, SearchHit{
						Path:       fmt.Sprintf("%s/%s/%s", section.Name, cat.Name, mem.Name),
						MemoryID:   mem.Name,
						Category:   cat.Name,
						Score:      score / float64(len(keywords)),
						L0Abstract: string(l0Data),
					})
				}
				if len(hits) >= maxResults {
					return hits, nil
				}
			}
		}
	}

	return hits, nil
}

// --- Delete ---

// DeleteMemory removes a memory from the VFS.
func (s *FSStoreService) DeleteMemory(
	tenantID, userID string,
	memoryID uuid.UUID,
) error {
	if !s.initialized {
		return fmt.Errorf("FSStoreService not initialized")
	}

	// Acquire per-user distributed lock to serialize VFS deletes.
	lock := globalVFSLock.ForUser(tenantID, userID)
	lock.Lock()
	defer lock.Unlock()

	// Search across sections for the memory ID.
	userRoot := userVFSRoot(tenantID, userID)
	sections, err := s.agfsClient.ListDir(userRoot)
	if err != nil {
		return fmt.Errorf("fs delete list: %w", err)
	}

	memID := memoryID.String()
	for _, section := range sections {
		if !section.IsDir || section.Name == "archives" {
			continue
		}
		sectionPath := fmt.Sprintf("%s/%s", userRoot, section.Name)
		categories, cerr := s.agfsClient.ListDir(sectionPath)
		if cerr != nil {
			continue
		}
		for _, cat := range categories {
			if !cat.IsDir {
				continue
			}
			memDir := fmt.Sprintf("%s/%s/%s", sectionPath, cat.Name, memID)
			if s.agfsClient.FileExists(memDir) {
				return s.agfsClient.RemoveAll(memDir)
			}
		}
	}

	return fmt.Errorf("memory %s not found in VFS", memID)
}

// --- Memory Count ---

// MemoryCount returns total memories stored in the VFS for a user.
func (s *FSStoreService) MemoryCount(
	tenantID, userID string,
) (int, error) {
	if !s.initialized {
		return 0, fmt.Errorf("FSStoreService not initialized")
	}

	count := 0
	userRoot := userVFSRoot(tenantID, userID)
	sections, err := s.agfsClient.ListDir(userRoot)
	if err != nil {
		// User directory may not exist yet.
		return 0, nil
	}

	for _, section := range sections {
		if !section.IsDir || section.Name == "archives" {
			continue
		}
		sectionPath := fmt.Sprintf("%s/%s", userRoot, section.Name)
		categories, cerr := s.agfsClient.ListDir(sectionPath)
		if cerr != nil {
			continue
		}
		for _, cat := range categories {
			if !cat.IsDir {
				continue
			}
			catPath := fmt.Sprintf("%s/%s", sectionPath, cat.Name)
			memories, merr := s.agfsClient.ListDir(catPath)
			if merr != nil {
				continue
			}
			for _, mem := range memories {
				if mem.IsDir {
					count++
				}
			}
		}
	}

	return count, nil
}

// --- Cleanup ---

// Cleanup is a no-op in AGFS mode — data is shared, not evicted.
// Retained for interface compatibility.
func (s *FSStoreService) Cleanup(tenantID, userID string) error {
	if !s.initialized {
		return fmt.Errorf("FSStoreService not initialized")
	}
	slog.Debug("Cleanup is no-op in AGFS mode", "tenant", tenantID, "user", userID)
	return nil
}

// --- Session Archives ---

// CreateArchiveDir creates a session archive directory in the VFS at a given index.
// Acquires the per-user distributed lock to prevent concurrent writes.
// Prefer CreateNextArchive when the index has not yet been determined — it
// eliminates the TOCTOU race between NextArchiveIndex and this call.
func (s *FSStoreService) CreateArchiveDir(
	tenantID, userID string, index int, l0Summary, l1Overview string,
) error {
	if !s.initialized {
		return fmt.Errorf("FSStoreService not initialized")
	}

	// Acquire per-user distributed lock to serialize concurrent archive writes.
	lock := globalVFSLock.ForUser(tenantID, userID)
	lock.Lock()
	defer lock.Unlock()

	return s.writeArchiveDir(tenantID, userID, index, l0Summary, l1Overview)
}

// CreateNextArchive atomically reads the next archive index and writes the
// archive directory in a single lock scope, eliminating the TOCTOU race
// between NextArchiveIndex and CreateArchiveDir.
// Returns the index that was assigned to this archive.
func (s *FSStoreService) CreateNextArchive(
	tenantID, userID, l0Summary, l1Overview string,
) (int, error) {
	if !s.initialized {
		return 0, fmt.Errorf("FSStoreService not initialized")
	}

	lock := globalVFSLock.ForUser(tenantID, userID)
	lock.Lock()
	defer lock.Unlock()

	// Read next index under the lock so no other instance can claim the same index.
	nextIdx, err := s.nextArchiveIndexLocked(tenantID, userID)
	if err != nil {
		return 0, fmt.Errorf("read next archive index: %w", err)
	}

	if err := s.writeArchiveDir(tenantID, userID, nextIdx, l0Summary, l1Overview); err != nil {
		return 0, err
	}
	return nextIdx, nil
}

// writeArchiveDir writes L0/L1 files into an archive directory.
// Caller must hold the per-user distributed lock.
func (s *FSStoreService) writeArchiveDir(
	tenantID, userID string, index int, l0Summary, l1Overview string,
) error {
	s.ensureDir(archivesDir(tenantID, userID))
	dir := fmt.Sprintf("%s/%d", archivesDir(tenantID, userID), index)
	s.ensureDir(dir)

	if err := s.agfsClient.WriteFile(dir+"/l0.txt", []byte(l0Summary)); err != nil {
		return fmt.Errorf("fs archive write l0: %w", err)
	}
	if err := s.agfsClient.WriteFile(dir+"/l1.txt", []byte(l1Overview)); err != nil {
		return fmt.Errorf("fs archive write l1: %w", err)
	}
	return nil
}

// nextArchiveIndexLocked reads the highest existing archive index and returns
// the next available index. Caller must hold the per-user distributed lock.
func (s *FSStoreService) nextArchiveIndexLocked(tenantID, userID string) (int, error) {
	archDir := archivesDir(tenantID, userID)
	entries, err := s.agfsClient.ListDir(archDir)
	if err != nil {
		return 0, nil // No archives yet — start from 0.
	}
	maxIdx := -1
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if idx, perr := strconv.Atoi(e.Name); perr == nil && idx > maxIdx {
			maxIdx = idx
		}
	}
	return maxIdx + 1, nil
}

// NextArchiveIndex returns the next archive index for a tenant+user.
func (s *FSStoreService) NextArchiveIndex(tenantID, userID string) (int, error) {
	if !s.initialized {
		return 0, fmt.Errorf("FSStoreService not initialized")
	}

	archDir := archivesDir(tenantID, userID)
	entries, err := s.agfsClient.ListDir(archDir)
	if err != nil {
		// No archives yet.
		return 0, nil
	}

	maxIdx := -1
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if idx, perr := strconv.Atoi(e.Name); perr == nil && idx > maxIdx {
			maxIdx = idx
		}
	}
	return maxIdx + 1, nil
}

// --- Search with Trace ---

// SearchMemoriesWithTrace searches with full retrieval trace for visualization.
// In AGFS mode, this wraps SearchMemories and provides a basic trace.
func (s *FSStoreService) SearchMemoriesWithTrace(
	tenantID, userID, query string,
	maxResults int,
) (*SearchTrace, error) {
	if !s.initialized {
		return nil, fmt.Errorf("FSStoreService not initialized")
	}

	hits, err := s.SearchMemories(tenantID, userID, query, maxResults)
	if err != nil {
		return nil, err
	}

	// Build a basic trace from search results.
	steps := make([]TraceStep, 0, len(hits))
	for _, h := range hits {
		steps = append(steps, TraceStep{
			Path:     h.Path,
			NodeType: "memory",
			Score:    h.Score,
			Matched:  true,
		})
	}

	return &SearchTrace{
		Query:    query,
		Keywords: strings.Fields(strings.ToLower(query)),
		Steps:    steps,
		Hits:     hits,
	}, nil
}
