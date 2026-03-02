package media

import (
	"context"
	"strings"
	"testing"
)

// ---------- ContentComposeTool tests (P1-2) ----------

func newTestStore(t *testing.T) DraftStore {
	t.Helper()
	store, err := NewFileDraftStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileDraftStore: %v", err)
	}
	return store
}

func TestCreateContentComposeTool_Draft(t *testing.T) {
	store := newTestStore(t)
	tool := CreateContentComposeTool(store)
	if tool.ToolName != ToolContentCompose {
		t.Fatalf("name: got %q, want %q", tool.ToolName, ToolContentCompose)
	}

	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "draft",
		"platform": "wechat",
		"title":    "测试标题",
		"body":     "这是一段测试内容。",
		"style":    "casual",
		"tags":     []any{"go", "test"},
	})
	if err != nil {
		t.Fatalf("Execute draft: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}

	// Verify draft was saved.
	drafts, err := store.List("wechat")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}
	if drafts[0].Title != "测试标题" {
		t.Errorf("title: got %q, want %q", drafts[0].Title, "测试标题")
	}
}

func TestCreateContentComposeTool_Preview(t *testing.T) {
	store := newTestStore(t)

	draft := &ContentDraft{
		ID:       "preview-test-id",
		Title:    "Preview Title",
		Body:     "Preview body.",
		Platform: PlatformWeChat,
		Style:    StyleInformative,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := CreateContentComposeTool(store)
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "preview",
		"draft_id": "preview-test-id",
	})
	if err != nil {
		t.Fatalf("Execute preview: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "Preview Title") {
		t.Errorf("expected result to contain title, got: %s", result.Content[0].Text)
	}
}

func TestCreateContentComposeTool_Revise(t *testing.T) {
	store := newTestStore(t)

	draft := &ContentDraft{
		ID:       "revise-test-id",
		Title:    "Original Title",
		Body:     "Original body.",
		Platform: PlatformWebsite,
		Style:    StyleProfessional,
		Status:   DraftStatusPendingReview,
	}
	if err := store.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := CreateContentComposeTool(store)
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "revise",
		"draft_id": "revise-test-id",
		"title":    "Updated Title",
		"body":     "Updated body.",
	})
	if err != nil {
		t.Fatalf("Execute revise: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	got, err := store.Get("revise-test-id")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("title: got %q, want %q", got.Title, "Updated Title")
	}
	if got.Status != DraftStatusDraft {
		t.Errorf("status should reset to draft on revision, got %q", got.Status)
	}
}

func TestCreateContentComposeTool_List(t *testing.T) {
	store := newTestStore(t)

	for _, d := range []*ContentDraft{
		{Title: "WC1", Platform: PlatformWeChat, Style: StyleInformative},
		{Title: "WC2", Platform: PlatformWeChat, Style: StyleCasual},
		{Title: "XHS1", Platform: PlatformXiaohongshu, Style: StyleCasual, Body: "短内容"},
	} {
		if err := store.Save(d); err != nil {
			t.Fatalf("Save %q: %v", d.Title, err)
		}
	}

	tool := CreateContentComposeTool(store)

	// List all.
	result, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("Execute list all: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// List by platform.
	result, err = tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "list",
		"platform": "wechat",
	})
	if err != nil {
		t.Fatalf("Execute list wechat: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestCreateContentComposeTool_PlatformConstraints(t *testing.T) {
	store := newTestStore(t)
	tool := CreateContentComposeTool(store)

	// Xiaohongshu title > 20 characters should fail.
	_, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "draft",
		"platform": "xiaohongshu",
		"title":    "这是一个超过二十个字符的标题这是一个超过二十个字符的标题",
		"body":     "短内容",
	})
	if err == nil {
		t.Fatal("expected error for title exceeding xiaohongshu limit")
	}
	if !strings.Contains(err.Error(), "title too long") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Xiaohongshu body > 1000 characters should fail.
	longBody := strings.Repeat("中", 1001)
	_, err = tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "draft",
		"platform": "xiaohongshu",
		"title":    "短标题",
		"body":     longBody,
	})
	if err == nil {
		t.Fatal("expected error for body exceeding xiaohongshu limit")
	}
	if !strings.Contains(err.Error(), "body too long") {
		t.Errorf("unexpected error message: %v", err)
	}

	// WeChat title ≤ 64 should pass.
	wechatTitle := strings.Repeat("字", 64)
	_, err = tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "draft",
		"platform": "wechat",
		"title":    wechatTitle,
		"body":     "正文content",
	})
	if err != nil {
		t.Fatalf("expected success for wechat title at limit: %v", err)
	}

	// WeChat title > 64 should fail.
	wechatTitleTooLong := strings.Repeat("字", 65)
	_, err = tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action":   "draft",
		"platform": "wechat",
		"title":    wechatTitleTooLong,
		"body":     "正文",
	})
	if err == nil {
		t.Fatal("expected error for wechat title exceeding 64 characters")
	}
}

func TestCreateContentComposeTool_NilStore(t *testing.T) {
	tool := CreateContentComposeTool(nil)

	_, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "draft",
		"title":  "Test",
		"body":   "Test body",
	})
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestCreateContentComposeTool_UnknownAction(t *testing.T) {
	store := newTestStore(t)
	tool := CreateContentComposeTool(store)

	_, err := tool.ToolExecute(context.Background(), "test-call", map[string]any{
		"action": "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}
