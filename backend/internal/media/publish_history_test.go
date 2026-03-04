package media

// ============================================================================
// media/publish_history_test.go — 发布历史持久化单元测试
//
// Tracking doc: docs/claude/tracking/tracking-media-subagent-upgrade.md §P0-5
// ============================================================================

import (
	"testing"
	"time"
)

func newTestHistoryStore(t *testing.T) *FilePublishHistoryStore {
	t.Helper()
	store, err := NewFilePublishHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilePublishHistoryStore: %v", err)
	}
	return store
}

func TestPublishHistory_SaveAndGet(t *testing.T) {
	store := newTestHistoryStore(t)

	record := &PublishRecord{
		DraftID:  "draft-001",
		Title:    "Test Article",
		Platform: PlatformWeChat,
		PostID:   "post-123",
		URL:      "https://mp.weixin.qq.com/s/abc",
		Status:   "published",
	}

	if err := store.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if record.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if record.PublishedAt.IsZero() {
		t.Fatal("expected auto-set PublishedAt")
	}

	got, err := store.Get(record.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DraftID != "draft-001" {
		t.Errorf("DraftID: got %q, want %q", got.DraftID, "draft-001")
	}
	if got.Title != "Test Article" {
		t.Errorf("Title: got %q, want %q", got.Title, "Test Article")
	}
	if got.Platform != PlatformWeChat {
		t.Errorf("Platform: got %q, want %q", got.Platform, PlatformWeChat)
	}
	if got.PostID != "post-123" {
		t.Errorf("PostID: got %q, want %q", got.PostID, "post-123")
	}
}

func TestPublishHistory_List_Empty(t *testing.T) {
	store := newTestHistoryStore(t)

	records, err := store.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestPublishHistory_List_Ordered(t *testing.T) {
	store := newTestHistoryStore(t)

	// 保存 3 条记录，时间递增
	for i, title := range []string{"First", "Second", "Third"} {
		record := &PublishRecord{
			DraftID:     "draft-" + title,
			Title:       title,
			Platform:    PlatformWeChat,
			Status:      "published",
			PublishedAt: time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		if err := store.Save(record); err != nil {
			t.Fatalf("Save %s: %v", title, err)
		}
	}

	records, err := store.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// 验证倒序（最新在前）
	if records[0].Title != "Third" {
		t.Errorf("first record: got %q, want %q", records[0].Title, "Third")
	}
	if records[2].Title != "First" {
		t.Errorf("last record: got %q, want %q", records[2].Title, "First")
	}
}

func TestPublishHistory_List_Pagination(t *testing.T) {
	store := newTestHistoryStore(t)

	// 保存 5 条记录
	for i, title := range []string{"A", "B", "C", "D", "E"} {
		record := &PublishRecord{
			DraftID:     "draft-" + title,
			Title:       title,
			Platform:    PlatformWeChat,
			Status:      "published",
			PublishedAt: time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		if err := store.Save(record); err != nil {
			t.Fatalf("Save %s: %v", title, err)
		}
	}

	// 全量（nil opts）
	all, err := store.List(nil)
	if err != nil {
		t.Fatalf("List(nil): %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5, got %d", len(all))
	}
	// 倒序: E, D, C, B, A
	if all[0].Title != "E" || all[4].Title != "A" {
		t.Errorf("order: first=%q last=%q", all[0].Title, all[4].Title)
	}

	// Limit=2
	page1, err := store.List(&PublishListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List(limit=2): %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("limit=2: got %d", len(page1))
	}
	if page1[0].Title != "E" || page1[1].Title != "D" {
		t.Errorf("limit=2: got %q, %q", page1[0].Title, page1[1].Title)
	}

	// Offset=2, Limit=2
	page2, err := store.List(&PublishListOptions{Offset: 2, Limit: 2})
	if err != nil {
		t.Fatalf("List(offset=2,limit=2): %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("offset=2,limit=2: got %d", len(page2))
	}
	if page2[0].Title != "C" || page2[1].Title != "B" {
		t.Errorf("offset=2,limit=2: got %q, %q", page2[0].Title, page2[1].Title)
	}

	// Offset=4, Limit=10 — 只剩 1 条
	tail, err := store.List(&PublishListOptions{Offset: 4, Limit: 10})
	if err != nil {
		t.Fatalf("List(offset=4,limit=10): %v", err)
	}
	if len(tail) != 1 {
		t.Fatalf("offset=4,limit=10: got %d", len(tail))
	}
	if tail[0].Title != "A" {
		t.Errorf("tail: got %q", tail[0].Title)
	}

	// Offset 超出范围
	empty, err := store.List(&PublishListOptions{Offset: 100})
	if err != nil {
		t.Fatalf("List(offset=100): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("offset=100: got %d, want 0", len(empty))
	}
}

func TestPublishHistory_Get_NotFound(t *testing.T) {
	store := newTestHistoryStore(t)

	_, err := store.Get("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent record")
	}
}

func TestPublishHistory_Save_NilRecord(t *testing.T) {
	store := newTestHistoryStore(t)

	err := store.Save(nil)
	if err == nil {
		t.Fatal("expected error for nil record")
	}
}

func TestPublishHistory_InvalidID(t *testing.T) {
	store := newTestHistoryStore(t)

	// Path traversal attempt
	record := &PublishRecord{
		ID:       "../escape",
		DraftID:  "draft-001",
		Title:    "Bad ID",
		Platform: PlatformWeChat,
		Status:   "published",
	}
	if err := store.Save(record); err == nil {
		t.Fatal("expected error for path traversal ID")
	}

	// Slash in ID
	record.ID = "a/b"
	if err := store.Save(record); err == nil {
		t.Fatal("expected error for slash in ID")
	}
}
