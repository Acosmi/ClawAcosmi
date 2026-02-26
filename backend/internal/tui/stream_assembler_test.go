package tui

import "testing"

func TestStreamAssemblerIngestDelta(t *testing.T) {
	a := NewTuiStreamAssembler()

	// 第一次 delta — 有新内容
	msg := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
		},
	}
	got := a.IngestDelta("run-1", msg, false)
	if got != "hello" {
		t.Errorf("first delta: got %q, want %q", got, "hello")
	}

	// 重复相同内容 — 应返回空
	got = a.IngestDelta("run-1", msg, false)
	if got != "" {
		t.Errorf("duplicate delta: got %q, want empty", got)
	}

	// 追加内容
	msg2 := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "hello world"},
		},
	}
	got = a.IngestDelta("run-1", msg2, false)
	if got != "hello world" {
		t.Errorf("updated delta: got %q, want %q", got, "hello world")
	}

	// 空消息 — 返回空
	got = a.IngestDelta("run-2", nil, false)
	if got != "" {
		t.Errorf("nil delta: got %q, want empty", got)
	}
}

func TestStreamAssemblerIngestDeltaWithThinking(t *testing.T) {
	a := NewTuiStreamAssembler()
	msg := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "thinking", "thinking": "deep thought"},
			map[string]interface{}{"type": "text", "text": "answer"},
		},
	}

	// showThinking=false → 仅显示 content
	got := a.IngestDelta("run-1", msg, false)
	if got != "answer" {
		t.Errorf("no-thinking: got %q, want %q", got, "answer")
	}

	// showThinking=true → 显示 thinking + content（新 run）
	got = a.IngestDelta("run-2", msg, true)
	want := "[thinking]\ndeep thought\n\nanswer"
	if got != want {
		t.Errorf("with-thinking: got %q, want %q", got, want)
	}
}

func TestStreamAssemblerFinalize(t *testing.T) {
	a := NewTuiStreamAssembler()

	// 先 ingest 一些 delta
	msg1 := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "partial"},
		},
	}
	a.IngestDelta("run-1", msg1, false)

	// finalize 时传入最终消息
	msgFinal := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "final text"},
		},
	}
	got := a.Finalize("run-1", msgFinal, false)
	if got != "final text" {
		t.Errorf("finalize: got %q, want %q", got, "final text")
	}

	// finalize 后 run 状态应被清理
	got = a.IngestDelta("run-1", msgFinal, false)
	if got != "final text" {
		// 新的 IngestDelta 应产生新状态
		t.Errorf("after finalize re-ingest: got %q, want %q", got, "final text")
	}
}

func TestStreamAssemblerFinalizeEmpty(t *testing.T) {
	a := NewTuiStreamAssembler()
	// finalize 空消息 — 应返回 "(no output)"
	got := a.Finalize("run-empty", nil, false)
	if got != "(no output)" {
		t.Errorf("finalize empty: got %q, want %q", got, "(no output)")
	}
}

func TestStreamAssemblerDrop(t *testing.T) {
	a := NewTuiStreamAssembler()
	msg := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
		},
	}
	a.IngestDelta("run-1", msg, false)

	a.Drop("run-1")

	// drop 后再 ingest 相同内容应产生输出（新状态）
	got := a.IngestDelta("run-1", msg, false)
	if got != "hello" {
		t.Errorf("after drop re-ingest: got %q, want %q", got, "hello")
	}

	// drop 不存在的 run — 不应 panic
	a.Drop("nonexistent")
}

func TestStreamAssemblerReset(t *testing.T) {
	a := NewTuiStreamAssembler()
	msg := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
		},
	}
	a.IngestDelta("run-1", msg, false)
	a.IngestDelta("run-2", msg, false)

	a.Reset()

	// reset 后 ingest 应产生输出
	got := a.IngestDelta("run-1", msg, false)
	if got != "hello" {
		t.Errorf("after reset: got %q, want %q", got, "hello")
	}
}

func TestStreamAssemblerMultipleRuns(t *testing.T) {
	a := NewTuiStreamAssembler()

	msg1 := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "run1 text"},
		},
	}
	msg2 := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "run2 text"},
		},
	}

	got1 := a.IngestDelta("run-1", msg1, false)
	got2 := a.IngestDelta("run-2", msg2, false)

	if got1 != "run1 text" {
		t.Errorf("run-1 got %q, want %q", got1, "run1 text")
	}
	if got2 != "run2 text" {
		t.Errorf("run-2 got %q, want %q", got2, "run2 text")
	}

	// finalize run-1 不影响 run-2
	a.Finalize("run-1", msg1, false)

	got2Again := a.IngestDelta("run-2", msg2, false)
	if got2Again != "" {
		t.Errorf("run-2 after run-1 finalize: got %q, want empty (unchanged)", got2Again)
	}
}
