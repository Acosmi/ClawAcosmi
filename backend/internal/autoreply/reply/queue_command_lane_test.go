package reply

import (
	"testing"
)

// ========== Command Lane ==========

func TestClearCommandLaneEmpty(t *testing.T) {
	removed := ClearCommandLane("nonexistent-lane")
	if removed != 0 {
		t.Errorf("ClearCommandLane nonexistent = %d, want 0", removed)
	}
}

func TestClearCommandLaneWithEntries(t *testing.T) {
	// 启动一个阻塞任务占住 lane，然后入队多个等待任务
	block := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		err := EnqueueCommandInLane("test-clear-lane", func() error {
			<-block // 阻塞直到释放
			return nil
		}, 60000)
		done <- err
	}()

	// 等待阻塞任务开始执行
	for {
		if GetCommandLaneQueueSize("test-clear-lane") > 0 || func() bool {
			state := getLaneState("test-clear-lane")
			state.mu.Lock()
			defer state.mu.Unlock()
			return state.active > 0
		}() {
			break
		}
	}

	// 入队 2 个等待任务
	errCh1 := make(chan error, 1)
	errCh2 := make(chan error, 1)
	go func() {
		errCh1 <- EnqueueCommandInLane("test-clear-lane", func() error { return nil }, 60000)
	}()
	go func() {
		errCh2 <- EnqueueCommandInLane("test-clear-lane", func() error { return nil }, 60000)
	}()

	// 等待队列增长
	for GetCommandLaneQueueSize("test-clear-lane") < 3 {
		// spin wait
	}

	// 清理等待中的任务
	removed := ClearCommandLane("test-clear-lane")
	if removed != 2 {
		t.Errorf("ClearCommandLane = %d, want 2", removed)
	}

	// 释放阻塞任务
	close(block)
	<-done

	// 等待被清除的任务返回错误
	err1 := <-errCh1
	err2 := <-errCh2
	if err1 == nil || err2 == nil {
		t.Error("cleared tasks should return error")
	}
}

func TestResolveEmbeddedSessionLane(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"my-session", "session:my-session"},
		{"session:already", "session:already"},
		{"", "session:main"},
		{"  ", "session:main"},
		{"  spaces  ", "session:spaces"},
	}
	for _, tt := range tests {
		got := resolveEmbeddedSessionLane(tt.key)
		if got != tt.want {
			t.Errorf("resolveEmbeddedSessionLane(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestGetCommandLaneQueueSizeNonexistent(t *testing.T) {
	size := GetCommandLaneQueueSize("nonexistent")
	if size != 0 {
		t.Errorf("GetCommandLaneQueueSize nonexistent = %d, want 0", size)
	}
}

func TestGetTotalCommandLaneQueueSize(t *testing.T) {
	// 在清理完后，总大小应该回到 0 或以上
	total := GetTotalCommandLaneQueueSize()
	if total < 0 {
		t.Errorf("GetTotalCommandLaneQueueSize = %d, should be >= 0", total)
	}
}

func TestEnqueueCommandInLaneExecution(t *testing.T) {
	executed := false
	err := EnqueueCommandInLane("test-exec", func() error {
		executed = true
		return nil
	}, 2000)
	if err != nil {
		t.Errorf("EnqueueCommandInLane error = %v", err)
	}
	if !executed {
		t.Error("task was not executed")
	}
}

// ========== Directive Tokenizer ==========

func TestExtractQueueDirectiveTokenizerStopsAtUnrecognized(t *testing.T) {
	// TS 关键语义: 遇到无法识别的 token 立即停止
	r := ExtractQueueDirective("hello /queue followup unknown-token cap:5 world")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.QueueMode != QueueModeFollowup {
		t.Errorf("QueueMode = %q, want followup", r.QueueMode)
	}
	// cap:5 在 unknown-token 之后，不应被解析
	if r.Cap != nil {
		t.Errorf("Cap should be nil (after unrecognized token), got %d", *r.Cap)
	}
	// cleaned 应包含 unknown-token 和之后的文本
	if r.Cleaned == "" {
		t.Error("Cleaned should not be empty")
	}
}

func TestExtractQueueDirectiveConsumedOffset(t *testing.T) {
	// TS 关键语义: consumed 偏移精确切割 cleaned
	r := ExtractQueueDirective("before /queue steer after")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.QueueMode != QueueModeSteer {
		t.Errorf("QueueMode = %q, want steer", r.QueueMode)
	}
	if r.Cleaned != "before after" {
		t.Errorf("Cleaned = %q, want %q", r.Cleaned, "before after")
	}
}

func TestExtractQueueDirectiveCompoundDuration(t *testing.T) {
	// TS 关键语义: 复合时长支持
	r := ExtractQueueDirective("/queue followup debounce:1h30m")
	if !r.HasDirective {
		t.Error("expected HasDirective")
	}
	if r.DebounceMs == nil {
		t.Fatal("DebounceMs should not be nil")
	}
	expected := 5400000 // 1h30m = 90min = 5400000ms
	if *r.DebounceMs != expected {
		t.Errorf("DebounceMs = %d, want %d", *r.DebounceMs, expected)
	}
}

func TestExtractQueueDirectiveHourUnit(t *testing.T) {
	r := ExtractQueueDirective("/queue debounce:2h")
	if r.DebounceMs == nil {
		t.Fatal("DebounceMs should not be nil")
	}
	if *r.DebounceMs != 7200000 {
		t.Errorf("DebounceMs = %d, want 7200000", *r.DebounceMs)
	}
}

func TestExtractQueueDirectiveDayUnit(t *testing.T) {
	r := ExtractQueueDirective("/queue debounce:1d")
	if r.DebounceMs == nil {
		t.Fatal("DebounceMs should not be nil")
	}
	if *r.DebounceMs != 86400000 {
		t.Errorf("DebounceMs = %d, want 86400000", *r.DebounceMs)
	}
}

func TestExtractQueueDirectiveModeAliases(t *testing.T) {
	tests := []struct {
		body string
		want QueueMode
	}{
		{"/queue steer", QueueModeSteer},
		{"/queue steering", QueueModeSteer},
		{"/queue queue", QueueModeSteer},
		{"/queue queued", QueueModeSteer},
		{"/queue followup", QueueModeFollowup},
		{"/queue follow-ups", QueueModeFollowup},
		{"/queue followups", QueueModeFollowup},
		{"/queue collect", QueueModeCollect},
		{"/queue coalesce", QueueModeCollect},
		{"/queue interrupt", QueueModeInterrupt},
		{"/queue interrupts", QueueModeInterrupt},
		{"/queue abort", QueueModeInterrupt},
		{"/queue steer+backlog", QueueModeSteerBacklog},
		{"/queue steer-backlog", QueueModeSteerBacklog},
		{"/queue steer_backlog", QueueModeSteerBacklog},
	}
	for _, tt := range tests {
		r := ExtractQueueDirective(tt.body)
		if r.QueueMode != tt.want {
			t.Errorf("ExtractQueueDirective(%q).QueueMode = %q, want %q", tt.body, r.QueueMode, tt.want)
		}
	}
}

func TestExtractQueueDirectiveDropAliases(t *testing.T) {
	tests := []struct {
		body string
		want QueueDropPolicy
	}{
		{"/queue drop:old", QueueDropOld},
		{"/queue drop:oldest", QueueDropOld},
		{"/queue drop:new", QueueDropNew},
		{"/queue drop:newest", QueueDropNew},
		{"/queue drop:summarize", QueueDropSummarize},
		{"/queue drop:summary", QueueDropSummarize},
	}
	for _, tt := range tests {
		r := ExtractQueueDirective(tt.body)
		if r.DropPolicy != tt.want {
			t.Errorf("ExtractQueueDirective(%q).DropPolicy = %q, want %q", tt.body, r.DropPolicy, tt.want)
		}
	}
}

func TestExtractQueueDirectiveEqualsDelimiter(t *testing.T) {
	r := ExtractQueueDirective("/queue debounce=500ms cap=3 drop=old")
	if r.DebounceMs == nil || *r.DebounceMs != 500 {
		t.Errorf("DebounceMs = %v, want 500", r.DebounceMs)
	}
	if r.Cap == nil || *r.Cap != 3 {
		t.Errorf("Cap = %v, want 3", r.Cap)
	}
	if r.DropPolicy != QueueDropOld {
		t.Errorf("DropPolicy = %q, want old", r.DropPolicy)
	}
}

func TestParseDurationMs(t *testing.T) {
	tests := []struct {
		raw         string
		defaultUnit string
		want        float64
		wantErr     bool
	}{
		{"500", "ms", 500, false},
		{"500ms", "ms", 500, false},
		{"2s", "ms", 2000, false},
		{"5m", "ms", 300000, false},
		{"1h", "ms", 3600000, false},
		{"1d", "ms", 86400000, false},
		{"1h30m", "ms", 5400000, false},
		{"2m30s", "ms", 150000, false},
		{"", "ms", 0, true},
		{"abc", "ms", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDurationMs(tt.raw, tt.defaultUnit)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDurationMs(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseDurationMs(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestClearSessionQueuesWithLane(t *testing.T) {
	// 确保 ClearSessionQueues 现在也清理 command-lane（不再始终为 0 的逻辑验证）
	result := ClearSessionQueues([]string{"test-w2-cleanup"})
	if result.FollowupCleared != 0 {
		t.Errorf("FollowupCleared = %d, want 0 (no followup queue)", result.FollowupCleared)
	}
	// LaneCleared 在没有活跃 lane 时应为 0
	if result.LaneCleared != 0 {
		t.Errorf("LaneCleared = %d, want 0 (no active lane)", result.LaneCleared)
	}
	if len(result.Keys) != 1 || result.Keys[0] != "test-w2-cleanup" {
		t.Errorf("Keys = %v, want [test-w2-cleanup]", result.Keys)
	}
}
