package media

// ============================================================================
// media/publish_history.go — 发布历史持久化存储
// 记录每次发布操作的结果，支持查询历史和状态跟踪。
// 存储模式与 draft_store.go 一致（每条记录一个 JSON 文件）。
//
// Tracking doc: docs/claude/tracking/tracking-media-subagent-upgrade.md §P0-4
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------- 接口 ----------

// PublishHistoryStore 发布历史持久化接口。
type PublishHistoryStore interface {
	Save(record *PublishRecord) error
	Get(id string) (*PublishRecord, error)
	List(opts *PublishListOptions) ([]*PublishRecord, error)
}

// PublishListOptions 列表查询选项。nil 等价于无限制。
type PublishListOptions struct {
	Limit  int // 最大返回数量，0 表示不限制
	Offset int // 跳过前 N 条记录
}

// ---------- 数据结构 ----------

// PublishRecord 发布历史记录。
type PublishRecord struct {
	ID          string    `json:"id"`
	DraftID     string    `json:"draft_id"`
	Title       string    `json:"title"`
	Platform    Platform  `json:"platform"`
	PostID      string    `json:"post_id,omitempty"`
	URL         string    `json:"url,omitempty"`
	Status      string    `json:"status"`
	PublishedAt time.Time `json:"published_at"`
}

// ---------- FilePublishHistoryStore ----------

// FilePublishHistoryStore JSON 文件驱动的发布历史存储。
// 每条记录存储为 `{baseDir}/{id}.json`。
type FilePublishHistoryStore struct {
	mu      sync.Mutex
	baseDir string // e.g., "_media/publish_history"
}

// NewFilePublishHistoryStore 创建文件发布历史存储。
func NewFilePublishHistoryStore(baseDir string) (*FilePublishHistoryStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create publish history dir: %w", err)
	}
	return &FilePublishHistoryStore{baseDir: baseDir}, nil
}

// Save 保存发布记录。若 ID 为空则自动生成。
func (s *FilePublishHistoryStore) Save(record *PublishRecord) error {
	if record == nil {
		return fmt.Errorf("publish record is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if err := validateID(record.ID); err != nil {
		return err
	}
	if record.PublishedAt.IsZero() {
		record.PublishedAt = time.Now().UTC()
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal publish record %s: %w", record.ID, err)
	}
	path := filepath.Join(s.baseDir, record.ID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write publish record %s: %w", record.ID, err)
	}
	return nil
}

// Get 按 ID 获取发布记录。
func (s *FilePublishHistoryStore) Get(id string) (*PublishRecord, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("publish record %s not found", id)
		}
		return nil, fmt.Errorf("read publish record %s: %w", id, err)
	}
	var record PublishRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse publish record %s: %w", id, err)
	}
	return &record, nil
}

// List 列出发布记录，按发布时间倒序。
// opts 为 nil 时返回全部记录。
func (s *FilePublishHistoryStore) List(opts *PublishListOptions) ([]*PublishRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read publish history dir: %w", err)
	}

	var records []*PublishRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		path := filepath.Join(s.baseDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("publish history: skipping unreadable file",
				"file", entry.Name(), "error", err)
			continue
		}
		var record PublishRecord
		if err := json.Unmarshal(data, &record); err != nil {
			slog.Warn("publish history: skipping corrupted file",
				"file", entry.Name(), "error", err)
			continue
		}
		if record.ID == "" {
			record.ID = id
		}
		records = append(records, &record)
	}

	// 按发布时间倒序排列
	sort.Slice(records, func(i, j int) bool {
		return records[i].PublishedAt.After(records[j].PublishedAt)
	})

	// 应用分页
	if opts != nil {
		if opts.Offset > 0 {
			if opts.Offset >= len(records) {
				return nil, nil
			}
			records = records[opts.Offset:]
		}
		if opts.Limit > 0 && opts.Limit < len(records) {
			records = records[:opts.Limit]
		}
	}

	return records, nil
}
