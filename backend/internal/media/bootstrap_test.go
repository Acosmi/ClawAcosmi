package media

// ============================================================================
// media/bootstrap_test.go — 媒体子系统引导模块单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-6
// ============================================================================

import (
	"testing"
)

func TestNewMediaSubsystem_Default(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:      dir,
		EnablePublish:  false,
		EnableInteract: false,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	// 默认启用 trending_topics + content_compose。
	if len(sub.Tools) != 2 {
		t.Errorf("tool count: got %d, want 2", len(sub.Tools))
	}

	names := sub.ToolNames()
	expectNames := map[string]bool{
		ToolTrendingTopics: true,
		ToolContentCompose: true,
	}
	for _, n := range names {
		if !expectNames[n] {
			t.Errorf("unexpected tool: %s", n)
		}
		delete(expectNames, n)
	}
	for n := range expectNames {
		t.Errorf("missing tool: %s", n)
	}
}

func TestNewMediaSubsystem_AllEnabled(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:      dir,
		EnablePublish:  true,
		EnableInteract: true,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	// 全部启用: trending + compose + publish + interact = 4。
	if len(sub.Tools) != 4 {
		t.Errorf("tool count: got %d, want 4", len(sub.Tools))
	}

	// Verify GetTool works.
	if sub.GetTool(ToolMediaPublish) == nil {
		t.Error("media_publish tool not found")
	}
	if sub.GetTool(ToolSocialInteract) == nil {
		t.Error("social_interact tool not found")
	}
}

func TestNewMediaSubsystem_RegisterPublisher(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:     dir,
		EnablePublish: true,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	// Initially no publishers.
	if len(sub.Publishers) != 0 {
		t.Errorf("publishers: got %d, want 0", len(sub.Publishers))
	}

	// Register a mock publisher.
	sub.RegisterPublisher(PlatformWeChat, &mockPublisher{})
	if len(sub.Publishers) != 1 {
		t.Errorf("publishers after register: got %d, want 1",
			len(sub.Publishers))
	}
}

func TestNewMediaSubsystem_RegisterInteractor_Replace(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:      dir,
		EnableInteract: true,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	// Initially has tool but with nil interactor.
	tool := sub.GetTool(ToolSocialInteract)
	if tool == nil {
		t.Fatal("social_interact tool should exist")
	}

	// Register real interactor — should replace the tool.
	sub.RegisterInteractor(&mockInteractor{})
	tool2 := sub.GetTool(ToolSocialInteract)
	if tool2 == nil {
		t.Fatal("social_interact tool should still exist after register")
	}
	if tool2 == tool {
		t.Error("tool should have been replaced (different pointer)")
	}
}

func TestNewMediaSubsystem_RegisterInteractor_Add(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:      dir,
		EnableInteract: false, // not initially enabled
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	// Initially no social_interact tool.
	if sub.GetTool(ToolSocialInteract) != nil {
		t.Fatal("social_interact tool should not exist when disabled")
	}

	// Register interactor — should add the tool.
	sub.RegisterInteractor(&mockInteractor{})
	if sub.GetTool(ToolSocialInteract) == nil {
		t.Fatal("social_interact tool should exist after RegisterInteractor")
	}
}

func TestNewMediaSubsystem_PublishHistory(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:     dir,
		EnablePublish: true,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	if sub.PublishHistory == nil {
		t.Fatal("PublishHistory should not be nil")
	}

	// Verify store works.
	record := &PublishRecord{
		DraftID:  "draft-001",
		Title:    "Test",
		Platform: PlatformWeChat,
		Status:   "published",
	}
	if err := sub.PublishHistory.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}
	records, err := sub.PublishHistory.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestNewMediaSubsystem_PublishHistory_Disabled(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace:     dir,
		EnablePublish: false,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	if sub.PublishHistory != nil {
		t.Error("PublishHistory should be nil when EnablePublish=false")
	}
}

func TestNewMediaSubsystem_DraftStore(t *testing.T) {
	dir := t.TempDir()

	sub, err := NewMediaSubsystem(MediaSubsystemConfig{
		Workspace: dir,
	})
	if err != nil {
		t.Fatalf("NewMediaSubsystem: %v", err)
	}

	if sub.DraftStore == nil {
		t.Error("DraftStore should not be nil")
	}

	// Verify store works.
	draft := &ContentDraft{
		Title:    "Test",
		Body:     "Body",
		Platform: PlatformWeChat,
	}
	if err := sub.DraftStore.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := sub.DraftStore.Get(draft.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Test" {
		t.Errorf("title: got %q, want %q", got.Title, "Test")
	}
}
