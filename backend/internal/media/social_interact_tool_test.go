package media

// ============================================================================
// media/social_interact_tool_test.go — 社交互动工具单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P3-3
// ============================================================================

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------- mock SocialInteractor ----------

type mockInteractor struct {
	comments []InteractionItem
	dms      []InteractionItem
	replyErr error
}

func (m *mockInteractor) ListComments(
	_ context.Context, noteID string,
) ([]InteractionItem, error) {
	return m.comments, nil
}

func (m *mockInteractor) ReplyComment(
	_ context.Context, noteID, commentID, reply string,
) error {
	return m.replyErr
}

func (m *mockInteractor) ListDMs(
	_ context.Context,
) ([]InteractionItem, error) {
	return m.dms, nil
}

func (m *mockInteractor) ReplyDM(
	_ context.Context, userID, message string,
) error {
	return m.replyErr
}

// ---------- tests ----------

func TestSocialInteract_ListComments(t *testing.T) {
	mock := &mockInteractor{
		comments: []InteractionItem{
			{
				Type:       InteractionComment,
				Platform:   PlatformXiaohongshu,
				NoteID:     "note1",
				AuthorName: "alice",
				Content:    "great post!",
				Timestamp:  time.Now(),
			},
		},
	}

	tool := CreateSocialInteractTool(mock)
	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":  "list_comments",
		"note_id": "note1",
	})
	if err != nil {
		t.Fatalf("list_comments: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "note1") {
		t.Errorf("expected note_id in result: %s", result.Content[0].Text)
	}
}

func TestSocialInteract_ReplyComment(t *testing.T) {
	mock := &mockInteractor{}
	tool := CreateSocialInteractTool(mock)

	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":     "reply_comment",
		"note_id":    "note1",
		"comment_id": "c1",
		"message":    "thanks!",
	})
	if err != nil {
		t.Fatalf("reply_comment: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "replied") {
		t.Errorf("expected 'replied' in result: %s", result.Content[0].Text)
	}
}

func TestSocialInteract_ListDMs(t *testing.T) {
	mock := &mockInteractor{
		dms: []InteractionItem{
			{
				Type:       InteractionDM,
				Platform:   PlatformXiaohongshu,
				AuthorName: "bob",
				Content:    "hi!",
				Timestamp:  time.Now(),
			},
		},
	}

	tool := CreateSocialInteractTool(mock)
	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action": "list_dms",
	})
	if err != nil {
		t.Fatalf("list_dms: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, `"count": 1`) {
		t.Errorf("expected count=1 in result: %s", result.Content[0].Text)
	}
}

func TestSocialInteract_ReplyDM(t *testing.T) {
	mock := &mockInteractor{}
	tool := CreateSocialInteractTool(mock)

	result, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":  "reply_dm",
		"user_id": "user1",
		"message": "hello!",
	})
	if err != nil {
		t.Fatalf("reply_dm: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "replied") {
		t.Errorf("expected 'replied' in result: %s", result.Content[0].Text)
	}
}

func TestSocialInteract_NilInteractor(t *testing.T) {
	tool := CreateSocialInteractTool(nil)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":  "list_comments",
		"note_id": "note1",
	})
	if err == nil {
		t.Fatal("expected error for nil interactor")
	}
}

func TestSocialInteract_UnknownAction(t *testing.T) {
	tool := CreateSocialInteractTool(&mockInteractor{})
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action": "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestSocialInteract_ReplyError(t *testing.T) {
	mock := &mockInteractor{
		replyErr: fmt.Errorf("rate limited"),
	}
	tool := CreateSocialInteractTool(mock)
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":     "reply_comment",
		"note_id":    "note1",
		"comment_id": "c1",
		"message":    "test",
	})
	if err == nil {
		t.Fatal("expected error from reply")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSocialInteract_MissingParams(t *testing.T) {
	tool := CreateSocialInteractTool(&mockInteractor{})

	// list_comments without note_id
	_, err := tool.ToolExecute(context.Background(), "test", map[string]any{
		"action": "list_comments",
	})
	if err == nil {
		t.Fatal("expected error for missing note_id")
	}

	// reply_dm without message
	_, err = tool.ToolExecute(context.Background(), "test", map[string]any{
		"action":  "reply_dm",
		"user_id": "u1",
	})
	if err == nil {
		t.Fatal("expected error for missing message")
	}
}
