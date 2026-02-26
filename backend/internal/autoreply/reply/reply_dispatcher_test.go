package reply

import (
	"sync"
	"testing"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

func TestReplyDispatcher_SendFinalReply(t *testing.T) {
	var mu sync.Mutex
	var delivered []autoreply.ReplyPayload

	d := CreateReplyDispatcher(ReplyDispatcherOptions{
		Deliver: func(payload autoreply.ReplyPayload, kind ReplyDispatchKind) error {
			mu.Lock()
			delivered = append(delivered, payload)
			mu.Unlock()
			return nil
		},
	})
	defer d.Close()

	d.SendFinalReply(autoreply.ReplyPayload{Text: "Hello"})
	d.WaitForIdle()

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(delivered))
	}
	if delivered[0].Text != "Hello" {
		t.Errorf("got %q, want %q", delivered[0].Text, "Hello")
	}
}

func TestReplyDispatcher_SkipsEmptyReply(t *testing.T) {
	deliverCount := 0
	skipCalled := false

	d := CreateReplyDispatcher(ReplyDispatcherOptions{
		Deliver: func(payload autoreply.ReplyPayload, kind ReplyDispatchKind) error {
			deliverCount++
			return nil
		},
		OnSkip: func(payload autoreply.ReplyPayload, kind ReplyDispatchKind, reason NormalizeReplySkipReason) {
			skipCalled = true
		},
	})
	defer d.Close()

	// 空文本 + 无媒体 → 应跳过
	queued := d.SendFinalReply(autoreply.ReplyPayload{Text: "  "})
	if queued {
		t.Error("empty reply should not be queued")
	}
	if !skipCalled {
		t.Error("OnSkip should have been called")
	}
	if deliverCount != 0 {
		t.Error("Deliver should not have been called for empty reply")
	}
}

func TestReplyDispatcher_QueuedCounts(t *testing.T) {
	d := CreateReplyDispatcher(ReplyDispatcherOptions{
		Deliver: func(payload autoreply.ReplyPayload, kind ReplyDispatchKind) error {
			return nil
		},
	})

	d.SendToolResult(autoreply.ReplyPayload{Text: "tool1"})
	d.SendToolResult(autoreply.ReplyPayload{Text: "tool2"})
	d.SendBlockReply(autoreply.ReplyPayload{Text: "block1"})
	d.SendFinalReply(autoreply.ReplyPayload{Text: "final1"})

	d.WaitForIdle()
	d.Close()

	counts := d.GetQueuedCounts()
	if counts[DispatchTool] != 2 {
		t.Errorf("tool count: got %d, want 2", counts[DispatchTool])
	}
	if counts[DispatchBlock] != 1 {
		t.Errorf("block count: got %d, want 1", counts[DispatchBlock])
	}
	if counts[DispatchFinal] != 1 {
		t.Errorf("final count: got %d, want 1", counts[DispatchFinal])
	}
}

func TestReplyDispatcher_ResponsePrefix(t *testing.T) {
	var mu sync.Mutex
	var delivered []autoreply.ReplyPayload

	d := CreateReplyDispatcher(ReplyDispatcherOptions{
		Deliver: func(payload autoreply.ReplyPayload, kind ReplyDispatchKind) error {
			mu.Lock()
			delivered = append(delivered, payload)
			mu.Unlock()
			return nil
		},
		ResponsePrefix: "[AI] ",
	})

	d.SendFinalReply(autoreply.ReplyPayload{Text: "Hello"})
	d.WaitForIdle()
	d.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(delivered))
	}
	if delivered[0].Text != "[AI] Hello" {
		t.Errorf("got %q, want %q", delivered[0].Text, "[AI] Hello")
	}
}

func TestReplyDispatcher_WaitForIdle(t *testing.T) {
	count := 0
	var mu sync.Mutex

	d := CreateReplyDispatcher(ReplyDispatcherOptions{
		Deliver: func(payload autoreply.ReplyPayload, kind ReplyDispatchKind) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		},
	})

	for i := 0; i < 5; i++ {
		d.SendBlockReply(autoreply.ReplyPayload{Text: "msg"})
	}
	d.WaitForIdle()
	d.Close()

	mu.Lock()
	defer mu.Unlock()
	if count != 5 {
		t.Errorf("expected 5 deliveries, got %d", count)
	}
}

func TestGetHumanDelay_Off(t *testing.T) {
	delay := getHumanDelay(nil)
	if delay != 0 {
		t.Errorf("nil config should return 0, got %d", delay)
	}
	delay = getHumanDelay(&HumanDelayConfig{Mode: "off"})
	if delay != 0 {
		t.Errorf("off mode should return 0, got %d", delay)
	}
}

func TestGetHumanDelay_On(t *testing.T) {
	delay := getHumanDelay(&HumanDelayConfig{Mode: "on"})
	if delay < defaultHumanDelayMinMs || delay > defaultHumanDelayMaxMs {
		t.Errorf("on mode delay %d out of range [%d, %d]", delay, defaultHumanDelayMinMs, defaultHumanDelayMaxMs)
	}
}

func TestGetHumanDelay_Custom(t *testing.T) {
	delay := getHumanDelay(&HumanDelayConfig{Mode: "custom", MinMs: 100, MaxMs: 200})
	if delay < 100 || delay > 200 {
		t.Errorf("custom delay %d out of range [100, 200]", delay)
	}
}

func TestGetHumanDelay_CustomMinEqualsMax(t *testing.T) {
	delay := getHumanDelay(&HumanDelayConfig{Mode: "custom", MinMs: 500, MaxMs: 500})
	if delay != 500 {
		t.Errorf("min==max should return min, got %d", delay)
	}
}
