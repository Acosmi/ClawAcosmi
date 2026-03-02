package media

// ============================================================================
// media/draft_store.go — 内容草稿持久化存储
// 实现 DraftStore 接口 + FileDraftStore（JSON 文件存储）。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P0-5
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------- 接口 ----------

// DraftStore 草稿存储接口。
type DraftStore interface {
	Save(draft *ContentDraft) error
	Get(id string) (*ContentDraft, error)
	List(platform string) ([]*ContentDraft, error)
	UpdateStatus(id string, status DraftStatus) error
	Delete(id string) error
}

// ---------- FileDraftStore ----------

// FileDraftStore JSON 文件驱动的草稿存储。
// 每个草稿存储为 `{baseDir}/{id}.json`。
type FileDraftStore struct {
	mu      sync.Mutex
	baseDir string // e.g., "_media/drafts"
}

// NewFileDraftStore 创建文件草稿存储。
// baseDir 不存在时自动创建。
func NewFileDraftStore(baseDir string) (*FileDraftStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create draft dir: %w", err)
	}
	return &FileDraftStore{baseDir: baseDir}, nil
}

// Save 保存草稿。若 ID 为空则自动生成。
func (s *FileDraftStore) Save(draft *ContentDraft) error {
	if draft == nil {
		return fmt.Errorf("draft is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if draft.ID == "" {
		draft.ID = uuid.New().String()
	}
	if err := validateID(draft.ID); err != nil {
		return err
	}
	now := time.Now().UTC()
	if draft.CreatedAt.IsZero() {
		draft.CreatedAt = now
	}
	draft.UpdatedAt = now
	if draft.Status == "" {
		draft.Status = DraftStatusDraft
	}

	return s.writeFile(draft)
}

// Get 按 ID 获取草稿。
func (s *FileDraftStore) Get(id string) (*ContentDraft, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readFile(id)
}

// List 列出草稿，可按 platform 过滤（空字符串返回全部）。
func (s *FileDraftStore) List(platform string) ([]*ContentDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read draft dir: %w", err)
	}

	var drafts []*ContentDraft
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5] // strip .json
		draft, err := s.readFile(id)
		if err != nil {
			slog.Warn("draft store: skipping corrupted file",
				"file", entry.Name(), "error", err)
			continue
		}
		if platform != "" && string(draft.Platform) != platform {
			continue
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

// UpdateStatus 更新草稿状态。
func (s *FileDraftStore) UpdateStatus(id string, status DraftStatus) error {
	if err := validateID(id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	draft, err := s.readFile(id)
	if err != nil {
		return err
	}
	draft.Status = status
	draft.UpdatedAt = time.Now().UTC()
	return s.writeFile(draft)
}

// Delete 删除指定草稿。
func (s *FileDraftStore) Delete(id string) error {
	if err := validateID(id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("draft %s not found", id)
		}
		return fmt.Errorf("delete draft %s: %w", id, err)
	}
	return nil
}

// ---------- internal I/O ----------

func (s *FileDraftStore) filePath(id string) string {
	return filepath.Join(s.baseDir, id+".json")
}

// validateID rejects IDs that could cause path traversal.
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("draft id is empty")
	}
	if strings.ContainsAny(id, "/\\.") {
		return fmt.Errorf("draft id contains invalid characters")
	}
	return nil
}

func (s *FileDraftStore) readFile(id string) (*ContentDraft, error) {
	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("draft %s not found", id)
		}
		return nil, fmt.Errorf("read draft %s: %w", id, err)
	}
	var draft ContentDraft
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("parse draft %s: %w", id, err)
	}
	return &draft, nil
}

func (s *FileDraftStore) writeFile(draft *ContentDraft) error {
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal draft %s: %w", draft.ID, err)
	}
	if err := os.WriteFile(s.filePath(draft.ID), data, 0o600); err != nil {
		return fmt.Errorf("write draft %s: %w", draft.ID, err)
	}
	return nil
}
