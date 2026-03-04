package media

// ============================================================================
// media/publish_tool_test.go — 平台发布工具单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-5
// ============================================================================

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ---------- mock Publisher ----------

type mockPublisher struct {
	publishFunc func(ctx context.Context, draft *ContentDraft) (*PublishResult, error)
}

func (m *mockPublisher) Publish(ctx context.Context, draft *ContentDraft) (*PublishResult, error) {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, draft)
	}
	return &PublishResult{
		Platform: draft.Platform,
		PostID:   "mock_post_001",
		URL:      "https://example.com/mock",
		Status:   "published",
	}, nil
}

// ---------- tests ----------

func TestPublishTool_Approve(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Approve Test",
		Body:     "Body",
		Platform: PlatformWeChat,
		Status:   DraftStatusDraft,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := CreateMediaPublishTool(store, nil, nil)
	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "approve",
		"draft_id": draft.ID,
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "approved") {
		t.Errorf("expected 'approved' in result: %s", result.Content[0].Text)
	}

	// Verify status updated.
	got, err := store.Get(draft.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != DraftStatusApproved {
		t.Errorf("status: got %q, want %q", got.Status, DraftStatusApproved)
	}
}

func TestPublishTool_Approve_AlreadyPublished(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Published Draft",
		Body:     "Body",
		Platform: PlatformWeChat,
		Status:   DraftStatusPublished,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := CreateMediaPublishTool(store, nil, nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "approve",
		"draft_id": draft.ID,
	})
	if err == nil {
		t.Fatal("expected error for already published draft")
	}
}

func TestPublishTool_Publish_Approved(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Publish Test",
		Body:     "Body content",
		Platform: PlatformWeChat,
		Status:   DraftStatusApproved,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	publishers := map[Platform]MediaPublisher{
		PlatformWeChat: &mockPublisher{},
	}

	tool := CreateMediaPublishTool(store, publishers, nil)
	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "publish",
		"draft_id": draft.ID,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "published") {
		t.Errorf("expected 'published' in result: %s", result.Content[0].Text)
	}

	// Verify status updated to published.
	got, err := store.Get(draft.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != DraftStatusPublished {
		t.Errorf("status: got %q, want %q", got.Status, DraftStatusPublished)
	}
}

func TestPublishTool_Publish_NotApproved(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Not Approved",
		Body:     "Body",
		Platform: PlatformWeChat,
		Status:   DraftStatusDraft,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := CreateMediaPublishTool(store, nil, nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "publish",
		"draft_id": draft.ID,
	})
	if err == nil {
		t.Fatal("expected error for unapproved draft")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublishTool_Publish_UnsupportedPlatform(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "XHS Draft",
		Body:     "Body",
		Platform: PlatformXiaohongshu,
		Status:   DraftStatusApproved,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Only wechat publisher registered.
	publishers := map[Platform]MediaPublisher{
		PlatformWeChat: &mockPublisher{},
	}

	tool := CreateMediaPublishTool(store, publishers, nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "publish",
		"draft_id": draft.ID,
	})
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublishTool_Publish_Website(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Website Draft",
		Body:     "Body for website",
		Platform: PlatformWebsite,
		Status:   DraftStatusApproved,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	publishers := map[Platform]MediaPublisher{
		PlatformWebsite: &mockPublisher{},
	}

	tool := CreateMediaPublishTool(store, publishers, nil)
	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "publish",
		"draft_id": draft.ID,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "published") {
		t.Errorf("expected 'published' in result: %s", result.Content[0].Text)
	}
}

func TestPublishTool_Publish_Error(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Fail Draft",
		Body:     "Body",
		Platform: PlatformWeChat,
		Status:   DraftStatusApproved,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	publishers := map[Platform]MediaPublisher{
		PlatformWeChat: &mockPublisher{
			publishFunc: func(ctx context.Context, d *ContentDraft) (*PublishResult, error) {
				return nil, fmt.Errorf("API rate limit exceeded")
			},
		},
	}

	tool := CreateMediaPublishTool(store, publishers, nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "publish",
		"draft_id": draft.ID,
	})
	if err == nil {
		t.Fatal("expected error from publisher")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublishTool_Publish_WritesHistory(t *testing.T) {
	store := newTestStore(t)
	historyStore := newTestHistoryStore(t)

	draft := &ContentDraft{
		Title:    "History Test",
		Body:     "Body with history",
		Platform: PlatformWeChat,
		Status:   DraftStatusApproved,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	publishers := map[Platform]MediaPublisher{
		PlatformWeChat: &mockPublisher{},
	}

	tool := CreateMediaPublishTool(store, publishers, historyStore)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "publish",
		"draft_id": draft.ID,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Verify history was written.
	records, err := historyStore.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(records))
	}
	if records[0].DraftID != draft.ID {
		t.Errorf("DraftID: got %q, want %q", records[0].DraftID, draft.ID)
	}
	if records[0].Title != "History Test" {
		t.Errorf("Title: got %q, want %q", records[0].Title, "History Test")
	}
	if records[0].Platform != PlatformWeChat {
		t.Errorf("Platform: got %q, want %q", records[0].Platform, PlatformWeChat)
	}
	if records[0].PostID != "mock_post_001" {
		t.Errorf("PostID: got %q, want %q", records[0].PostID, "mock_post_001")
	}
}

func TestPublishTool_Status(t *testing.T) {
	store := newTestStore(t)
	draft := &ContentDraft{
		Title:    "Status Check",
		Body:     "Body",
		Platform: PlatformWeChat,
		Status:   DraftStatusApproved,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := CreateMediaPublishTool(store, nil, nil)
	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "status",
		"draft_id": draft.ID,
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "approved") {
		t.Errorf("expected status in result: %s", result.Content[0].Text)
	}
}

func TestPublishTool_NilStore(t *testing.T) {
	tool := CreateMediaPublishTool(nil, nil, nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "approve",
		"draft_id": "some-id",
	})
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestPublishTool_UnknownAction(t *testing.T) {
	store := newTestStore(t)
	tool := CreateMediaPublishTool(store, nil, nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":   "unknown",
		"draft_id": "some-id",
	})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}
