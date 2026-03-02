package media

import (
	"testing"
	"time"
)

// ---------- DraftStore tests (P0-5) ----------

func TestFileDraftStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	draft := &ContentDraft{
		Title:    "Test Title",
		Body:     "Test body content",
		Platform: PlatformWeChat,
		Style:    StyleInformative,
		Tags:     []string{"go", "test"},
	}

	// Save should auto-generate ID and timestamps.
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if draft.ID == "" {
		t.Fatal("expected non-empty ID after Save")
	}
	if draft.Status != DraftStatusDraft {
		t.Fatalf("expected status %q, got %q", DraftStatusDraft, draft.Status)
	}
	if draft.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	// Get should return the same draft.
	got, err := store.Get(draft.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != draft.Title {
		t.Errorf("title: got %q, want %q", got.Title, draft.Title)
	}
	if got.Body != draft.Body {
		t.Errorf("body: got %q, want %q", got.Body, draft.Body)
	}
	if got.Platform != draft.Platform {
		t.Errorf("platform: got %q, want %q", got.Platform, draft.Platform)
	}
}

func TestFileDraftStore_SaveExistingID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	draft := &ContentDraft{
		ID:       "fixed-id-123",
		Title:    "Original",
		Body:     "Original body",
		Platform: PlatformXiaohongshu,
		Style:    StyleCasual,
	}

	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if draft.ID != "fixed-id-123" {
		t.Fatalf("expected ID to remain fixed-id-123, got %q", draft.ID)
	}

	// Overwrite with same ID.
	draft.Title = "Updated"
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}

	got, err := store.Get("fixed-id-123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("title: got %q, want %q", got.Title, "Updated")
	}
}

func TestFileDraftStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent draft")
	}
}

func TestFileDraftStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	// Save drafts for different platforms.
	drafts := []*ContentDraft{
		{Title: "WeChat 1", Platform: PlatformWeChat, Style: StyleInformative},
		{Title: "WeChat 2", Platform: PlatformWeChat, Style: StyleProfessional},
		{Title: "XHS 1", Platform: PlatformXiaohongshu, Style: StyleCasual},
		{Title: "Web 1", Platform: PlatformWebsite, Style: StyleInformative},
	}
	for _, d := range drafts {
		if err := store.Save(d); err != nil {
			t.Fatalf("Save %q: %v", d.Title, err)
		}
	}

	// List all.
	all, err := store.List("")
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("List all: got %d, want 4", len(all))
	}

	// List by platform.
	wechat, err := store.List("wechat")
	if err != nil {
		t.Fatalf("List wechat: %v", err)
	}
	if len(wechat) != 2 {
		t.Errorf("List wechat: got %d, want 2", len(wechat))
	}

	xhs, err := store.List("xiaohongshu")
	if err != nil {
		t.Fatalf("List xiaohongshu: %v", err)
	}
	if len(xhs) != 1 {
		t.Errorf("List xiaohongshu: got %d, want 1", len(xhs))
	}
}

func TestFileDraftStore_UpdateStatus(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	draft := &ContentDraft{
		Title:    "Status Test",
		Platform: PlatformWeChat,
		Style:    StyleInformative,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	beforeUpdate := draft.UpdatedAt

	// Small delay to ensure UpdatedAt changes.
	time.Sleep(10 * time.Millisecond)

	if err := store.UpdateStatus(draft.ID, DraftStatusPendingReview); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := store.Get(draft.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != DraftStatusPendingReview {
		t.Errorf("status: got %q, want %q", got.Status, DraftStatusPendingReview)
	}
	if !got.UpdatedAt.After(beforeUpdate) {
		t.Error("expected UpdatedAt to advance after status update")
	}
}

func TestFileDraftStore_NilDraft(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	if err := store.Save(nil); err == nil {
		t.Fatal("expected error for nil draft")
	}
}

func TestFileDraftStore_EmptyID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	_, err = store.Get("")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}

	err = store.UpdateStatus("", DraftStatusApproved)
	if err == nil {
		t.Fatal("expected error for empty ID in UpdateStatus")
	}
}

func TestFileDraftStore_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileDraftStore(dir)
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}

	// IDs with path separators or dots must be rejected.
	badIDs := []string{"../etc/passwd", "foo/bar", "a\\b", ".."}
	for _, id := range badIDs {
		_, err := store.Get(id)
		if err == nil {
			t.Errorf("expected error for ID %q", id)
		}

		draft := &ContentDraft{
			ID:       id,
			Title:    "Malicious",
			Platform: PlatformWeChat,
			Style:    StyleCasual,
		}
		if err := store.Save(draft); err == nil {
			t.Errorf("expected error saving draft with ID %q", id)
		}
	}
}
