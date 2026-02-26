package reply

import (
	"strings"
	"testing"
	"time"
)

// ========== NormalizeQueueMode ==========

func TestNormalizeQueueMode(t *testing.T) {
	tests := []struct {
		raw  string
		want QueueMode
	}{
		{"", ""},
		{"steer", QueueModeSteer},
		{"steering", QueueModeSteer},
		{"queue", QueueModeSteer},
		{"queued", QueueModeSteer},
		{"followup", QueueModeFollowup},
		{"follow-ups", QueueModeFollowup},
		{"followups", QueueModeFollowup},
		{"collect", QueueModeCollect},
		{"coalesce", QueueModeCollect},
		{"interrupt", QueueModeInterrupt},
		{"interrupts", QueueModeInterrupt},
		{"abort", QueueModeInterrupt},
		{"steer+backlog", QueueModeSteerBacklog},
		{"steer-backlog", QueueModeSteerBacklog},
		{"steer_backlog", QueueModeSteerBacklog},
		{"unknown", ""},
		{"  STEER  ", QueueModeSteer},
	}
	for _, tt := range tests {
		got := NormalizeQueueMode(tt.raw)
		if got != tt.want {
			t.Errorf("NormalizeQueueMode(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestNormalizeQueueDropPolicy(t *testing.T) {
	tests := []struct {
		raw  string
		want QueueDropPolicy
	}{
		{"", ""},
		{"old", QueueDropOld},
		{"oldest", QueueDropOld},
		{"new", QueueDropNew},
		{"newest", QueueDropNew},
		{"summarize", QueueDropSummarize},
		{"summary", QueueDropSummarize},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := NormalizeQueueDropPolicy(tt.raw)
		if got != tt.want {
			t.Errorf("NormalizeQueueDropPolicy(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

// ========== ElideQueueText ==========

func TestElideQueueText(t *testing.T) {
	short := "hello"
	if got := ElideQueueText(short, 10); got != short {
		t.Errorf("ElideQueueText short = %q, want %q", got, short)
	}
	long := "hello world, this is a test"
	got := ElideQueueText(long, 10)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("ElideQueueText long should end with …, got %q", got)
	}
	if len([]rune(got)) > 10 {
		t.Errorf("ElideQueueText result too long: %d runes", len([]rune(got)))
	}
}

// ========== GetFollowupQueue + ClearFollowupQueue ==========

func TestGetFollowupQueueCreateAndReuse(t *testing.T) {
	// 清理全局状态
	defer ClearFollowupQueue("test-q1")

	debounce := 500
	cap := 10
	settings := QueueSettings{
		Mode:       QueueModeFollowup,
		DebounceMs: &debounce,
		Cap:        &cap,
		DropPolicy: QueueDropSummarize,
	}

	q1 := GetFollowupQueue("test-q1", settings)
	if q1 == nil {
		t.Fatal("expected non-nil queue")
	}
	if q1.Mode != QueueModeFollowup {
		t.Errorf("Mode = %q, want followup", q1.Mode)
	}
	if q1.DebounceMs != 500 {
		t.Errorf("DebounceMs = %d, want 500", q1.DebounceMs)
	}
	if q1.Cap != 10 {
		t.Errorf("Cap = %d, want 10", q1.Cap)
	}

	// 复用时更新 settings
	newDebounce := 200
	settings2 := QueueSettings{
		Mode:       QueueModeCollect,
		DebounceMs: &newDebounce,
	}
	q2 := GetFollowupQueue("test-q1", settings2)
	if q1 != q2 {
		t.Error("expected same queue instance")
	}
	if q2.Mode != QueueModeCollect {
		t.Errorf("updated Mode = %q, want collect", q2.Mode)
	}
	if q2.DebounceMs != 200 {
		t.Errorf("updated DebounceMs = %d, want 200", q2.DebounceMs)
	}
}

func TestClearFollowupQueue(t *testing.T) {
	debounce := 100
	cap := 5
	settings := QueueSettings{Mode: QueueModeFollowup, DebounceMs: &debounce, Cap: &cap}
	q := GetFollowupQueue("test-clear", settings)
	q.Items = append(q.Items, &FollowupRun{Prompt: "msg1"})
	q.Items = append(q.Items, &FollowupRun{Prompt: "msg2"})
	q.DroppedCount = 3

	cleared := ClearFollowupQueue("test-clear")
	if cleared != 5 { // 2 items + 3 dropped
		t.Errorf("cleared = %d, want 5", cleared)
	}

	// 确认已删除
	if depth := GetFollowupQueueDepth("test-clear"); depth != 0 {
		t.Errorf("depth after clear = %d, want 0", depth)
	}
}

// ========== EnqueueFollowupRun ==========

func TestEnqueueFollowupRunDedup(t *testing.T) {
	defer ClearFollowupQueue("test-dedup")

	debounce := 0
	cap := 10
	settings := QueueSettings{Mode: QueueModeFollowup, DebounceMs: &debounce, Cap: &cap}

	run1 := &FollowupRun{
		Prompt:    "hello",
		MessageID: "msg-123",
		Run:       FollowupRunParams{SessionID: "s1"},
	}

	// 首次入队：成功
	ok := EnqueueFollowupRun("test-dedup", run1, settings, QueueDedupeMessageID)
	if !ok {
		t.Error("first enqueue should succeed")
	}

	// 重复入队（同 messageID）：被去重
	run2 := &FollowupRun{
		Prompt:    "different",
		MessageID: "msg-123",
		Run:       FollowupRunParams{SessionID: "s1"},
	}
	ok = EnqueueFollowupRun("test-dedup", run2, settings, QueueDedupeMessageID)
	if ok {
		t.Error("duplicate messageID should be rejected")
	}

	// 不同 messageID：成功
	run3 := &FollowupRun{
		Prompt:    "world",
		MessageID: "msg-456",
		Run:       FollowupRunParams{SessionID: "s1"},
	}
	ok = EnqueueFollowupRun("test-dedup", run3, settings, QueueDedupeMessageID)
	if !ok {
		t.Error("different messageID should succeed")
	}

	if depth := GetFollowupQueueDepth("test-dedup"); depth != 2 {
		t.Errorf("depth = %d, want 2", depth)
	}
}

func TestEnqueueFollowupRunPromptDedup(t *testing.T) {
	defer ClearFollowupQueue("test-prompt-dedup")

	debounce := 0
	cap := 10
	settings := QueueSettings{Mode: QueueModeFollowup, DebounceMs: &debounce, Cap: &cap}

	run1 := &FollowupRun{Prompt: "hello", Run: FollowupRunParams{SessionID: "s1"}}
	ok := EnqueueFollowupRun("test-prompt-dedup", run1, settings, QueueDedupePrompt)
	if !ok {
		t.Error("first enqueue should succeed")
	}

	// 同 prompt：被去重
	run2 := &FollowupRun{Prompt: "hello", Run: FollowupRunParams{SessionID: "s1"}}
	ok = EnqueueFollowupRun("test-prompt-dedup", run2, settings, QueueDedupePrompt)
	if ok {
		t.Error("duplicate prompt should be rejected")
	}
}

// ========== ApplyQueueDropPolicy ==========

func TestApplyQueueDropPolicyNew(t *testing.T) {
	state := &FollowupQueueState{
		Cap:        2,
		DropPolicy: QueueDropNew,
		Items:      []*FollowupRun{{Prompt: "a"}, {Prompt: "b"}},
	}
	ok := ApplyQueueDropPolicy(state, func(r *FollowupRun) string { return r.Prompt })
	if ok {
		t.Error("dropPolicy=new should reject when at cap")
	}
}

func TestApplyQueueDropPolicySummarize(t *testing.T) {
	state := &FollowupQueueState{
		Cap:        2,
		DropPolicy: QueueDropSummarize,
		Items:      []*FollowupRun{{Prompt: "old1"}, {Prompt: "old2"}},
	}
	ok := ApplyQueueDropPolicy(state, func(r *FollowupRun) string { return r.Prompt })
	if !ok {
		t.Error("dropPolicy=summarize should accept")
	}
	if state.DroppedCount != 1 {
		t.Errorf("DroppedCount = %d, want 1", state.DroppedCount)
	}
	if len(state.SummaryLines) != 1 {
		t.Errorf("SummaryLines len = %d, want 1", len(state.SummaryLines))
	}
}

func TestApplyQueueDropPolicyOld(t *testing.T) {
	state := &FollowupQueueState{
		Cap:        2,
		DropPolicy: QueueDropOld,
		Items:      []*FollowupRun{{Prompt: "old1"}, {Prompt: "old2"}},
	}
	ok := ApplyQueueDropPolicy(state, func(r *FollowupRun) string { return r.Prompt })
	if !ok {
		t.Error("dropPolicy=old should accept")
	}
	if len(state.Items) != 1 {
		t.Errorf("Items len = %d, want 1", len(state.Items))
	}
	if state.Items[0].Prompt != "old2" {
		t.Errorf("remaining item = %q, want old2", state.Items[0].Prompt)
	}
}

// ========== HasCrossChannelItems ==========

func TestHasCrossChannelItemsSameChannel(t *testing.T) {
	items := []*FollowupRun{
		{OriginatingChannel: "telegram", OriginatingTo: "chat1"},
		{OriginatingChannel: "telegram", OriginatingTo: "chat1"},
	}
	result := HasCrossChannelItems(items, func(r *FollowupRun) (string, bool) {
		if r.OriginatingChannel == "" || r.OriginatingTo == "" {
			return "", true
		}
		return r.OriginatingChannel + "|" + r.OriginatingTo, false
	})
	if result {
		t.Error("same channel items should not be cross-channel")
	}
}

func TestHasCrossChannelItemsDifferentChannels(t *testing.T) {
	items := []*FollowupRun{
		{OriginatingChannel: "telegram", OriginatingTo: "chat1"},
		{OriginatingChannel: "discord", OriginatingTo: "chat2"},
	}
	result := HasCrossChannelItems(items, func(r *FollowupRun) (string, bool) {
		if r.OriginatingChannel == "" || r.OriginatingTo == "" {
			return "", true
		}
		return r.OriginatingChannel + "|" + r.OriginatingTo, false
	})
	if !result {
		t.Error("different channel items should be cross-channel")
	}
}

// ========== BuildQueueSummaryPrompt ==========

func TestBuildQueueSummaryPromptNoOverflow(t *testing.T) {
	state := &QueueSummaryState{DropPolicy: QueueDropSummarize, DroppedCount: 0}
	got := BuildQueueSummaryPrompt(state, "message", "")
	if got != "" {
		t.Errorf("no overflow should return empty, got %q", got)
	}
}

func TestBuildQueueSummaryPromptWithOverflow(t *testing.T) {
	state := &QueueSummaryState{
		DropPolicy:   QueueDropSummarize,
		DroppedCount: 2,
		SummaryLines: []string{"line 1", "line 2"},
	}
	got := BuildQueueSummaryPrompt(state, "message", "")
	if !strings.Contains(got, "Dropped 2 messages") {
		t.Errorf("should contain drop count, got %q", got)
	}
	if !strings.Contains(got, "- line 1") {
		t.Errorf("should contain summary lines, got %q", got)
	}
	// 消耗后应清零
	if state.DroppedCount != 0 {
		t.Errorf("DroppedCount should be 0 after prompt, got %d", state.DroppedCount)
	}
}

// ========== ResolveQueueSettings ==========

func TestResolveQueueSettingsDefaults(t *testing.T) {
	settings := ResolveQueueSettings(ResolveQueueSettingsParams{})
	if settings.Mode != QueueModeCollect {
		t.Errorf("default Mode = %q, want collect", settings.Mode)
	}
	if settings.DebounceMs == nil || *settings.DebounceMs != DefaultQueueDebounceMs {
		t.Errorf("default DebounceMs = %v, want %d", settings.DebounceMs, DefaultQueueDebounceMs)
	}
	if settings.Cap == nil || *settings.Cap != DefaultQueueCap {
		t.Errorf("default Cap = %v, want %d", settings.Cap, DefaultQueueCap)
	}
	if settings.DropPolicy != DefaultQueueDrop {
		t.Errorf("default DropPolicy = %q, want %q", settings.DropPolicy, DefaultQueueDrop)
	}
}

func TestResolveQueueSettingsInlinePriority(t *testing.T) {
	debounce := 200
	cap := 5
	settings := ResolveQueueSettings(ResolveQueueSettingsParams{
		InlineMode: QueueModeSteer,
		InlineOptions: &QueueOptions{
			DebounceMs: &debounce,
			Cap:        &cap,
			DropPolicy: QueueDropOld,
		},
	})
	if settings.Mode != QueueModeSteer {
		t.Errorf("inline Mode = %q, want steer", settings.Mode)
	}
	if settings.DebounceMs == nil || *settings.DebounceMs != 200 {
		t.Errorf("inline DebounceMs = %v, want 200", settings.DebounceMs)
	}
	if settings.Cap == nil || *settings.Cap != 5 {
		t.Errorf("inline Cap = %v, want 5", settings.Cap)
	}
	if settings.DropPolicy != QueueDropOld {
		t.Errorf("inline DropPolicy = %q, want old", settings.DropPolicy)
	}
}

// ========== ClearSessionQueues ==========

func TestClearSessionQueues(t *testing.T) {
	// 准备队列
	debounce := 0
	cap := 10
	settings := QueueSettings{Mode: QueueModeFollowup, DebounceMs: &debounce, Cap: &cap}
	q := GetFollowupQueue("test-cleanup-1", settings)
	q.Items = append(q.Items, &FollowupRun{Prompt: "msg"})

	result := ClearSessionQueues([]string{"test-cleanup-1", "", "test-cleanup-1"}) // 含空 + 去重
	if result.FollowupCleared != 1 {
		t.Errorf("FollowupCleared = %d, want 1", result.FollowupCleared)
	}
	if len(result.Keys) != 1 {
		t.Errorf("Keys len = %d, want 1", len(result.Keys))
	}
}

// ========== FollowupRun 字段完整性 ==========

func TestFollowupRunFields(t *testing.T) {
	run := FollowupRun{
		Prompt:               "test",
		MessageID:            "msg-1",
		SummaryLine:          "summary",
		EnqueuedAt:           time.Now().UnixMilli(),
		OriginatingChannel:   "telegram",
		OriginatingTo:        "chat-1",
		OriginatingAccountID: "acc-1",
		OriginatingThreadID:  "thread-1",
		OriginatingChatType:  "group",
		Run: FollowupRunParams{
			SessionID: "session-1",
			AgentID:   "agent-1",
		},
	}
	if run.MessageID != "msg-1" {
		t.Errorf("MessageID = %q, want msg-1", run.MessageID)
	}
	if run.SummaryLine != "summary" {
		t.Errorf("SummaryLine = %q, want summary", run.SummaryLine)
	}
	if run.EnqueuedAt <= 0 {
		t.Error("EnqueuedAt should be positive")
	}
}

// ========== PluginDebounce ==========

func TestResolveQueueSettingsPluginDebounce(t *testing.T) {
	// 注入 plugin debounce provider
	origProvider := PluginDebounceProvider
	pluginDebounce := 2500
	PluginDebounceProvider = func(channelKey string) *int {
		if channelKey == "custom-plugin" {
			return &pluginDebounce
		}
		return nil
	}
	defer func() { PluginDebounceProvider = origProvider }()

	// plugin debounce 应生效（无 inline/session/channel-specific 设置时）
	settings := ResolveQueueSettings(ResolveQueueSettingsParams{
		Channel: "custom-plugin",
	})
	if settings.DebounceMs == nil || *settings.DebounceMs != 2500 {
		t.Errorf("plugin debounce = %v, want 2500", settings.DebounceMs)
	}

	// debounceMsByChannel 应覆盖 plugin debounce（优先级更高）
	channelDebounce := 800
	settings2 := ResolveQueueSettings(ResolveQueueSettingsParams{
		Channel: "custom-plugin",
		InlineOptions: &QueueOptions{
			DebounceMs: &channelDebounce,
		},
	})
	if settings2.DebounceMs == nil || *settings2.DebounceMs != 800 {
		t.Errorf("inline should override plugin debounce, got %v, want 800", settings2.DebounceMs)
	}

	// 未知频道应 fallback 到默认值
	settings3 := ResolveQueueSettings(ResolveQueueSettingsParams{
		Channel: "unknown-channel",
	})
	if settings3.DebounceMs == nil || *settings3.DebounceMs != DefaultQueueDebounceMs {
		t.Errorf("unknown channel debounce = %v, want %d", settings3.DebounceMs, DefaultQueueDebounceMs)
	}
}
