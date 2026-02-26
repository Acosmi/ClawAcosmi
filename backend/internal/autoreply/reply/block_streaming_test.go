package reply

import (
	"sync"
	"testing"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- Coalescer 测试 ----------

func TestCoalescer_EmptyText(t *testing.T) {
	var flushed []autoreply.ReplyPayload
	c := NewBlockReplyCoalescer(BlockStreamingCoalescing{
		MinChars: 10, MaxChars: 100, IdleMs: 50,
	}, func(p autoreply.ReplyPayload) {
		flushed = append(flushed, p)
	}, nil)
	c.Enqueue(autoreply.ReplyPayload{Text: ""})
	c.Flush(true)
	c.Stop()
	if len(flushed) != 0 {
		t.Errorf("empty text should not produce flush, got %d", len(flushed))
	}
}

func TestCoalescer_MaxCharsFlush(t *testing.T) {
	var flushed []autoreply.ReplyPayload
	c := NewBlockReplyCoalescer(BlockStreamingCoalescing{
		MinChars: 5, MaxChars: 20, IdleMs: 5000, Joiner: " ",
	}, func(p autoreply.ReplyPayload) {
		flushed = append(flushed, p)
	}, nil)
	// 20-char threshold: 10 + joiner(1) + 10 = 21 > 20 → overflow split
	c.Enqueue(autoreply.ReplyPayload{Text: "0123456789"})
	if len(flushed) != 0 {
		t.Fatal("should not flush before maxChars")
	}
	c.Enqueue(autoreply.ReplyPayload{Text: "0123456789"})
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flush at maxChars overflow, got %d", len(flushed))
	}
	c.Flush(true) // flush remaining
	c.Stop()
}

func TestCoalescer_IdleFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed []autoreply.ReplyPayload
	c := NewBlockReplyCoalescer(BlockStreamingCoalescing{
		MinChars: 5, MaxChars: 1000, IdleMs: 50, Joiner: " ",
	}, func(p autoreply.ReplyPayload) {
		mu.Lock()
		flushed = append(flushed, p)
		mu.Unlock()
	}, nil)
	c.Enqueue(autoreply.ReplyPayload{Text: "hello"})
	time.Sleep(120 * time.Millisecond)
	mu.Lock()
	n := len(flushed)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("expected idle flush after 50ms, got %d flushes", n)
	}
	c.Stop()
}

func TestCoalescer_FlushOnEnqueue(t *testing.T) {
	var flushed []autoreply.ReplyPayload
	c := NewBlockReplyCoalescer(BlockStreamingCoalescing{
		MinChars: 100, MaxChars: 200, FlushOnEnqueue: true, Joiner: " ",
	}, func(p autoreply.ReplyPayload) {
		flushed = append(flushed, p)
	}, nil)
	c.Enqueue(autoreply.ReplyPayload{Text: "a"})
	c.Enqueue(autoreply.ReplyPayload{Text: "b"})
	if len(flushed) != 2 {
		t.Fatalf("expected 2 flushes in flushOnEnqueue mode, got %d", len(flushed))
	}
	c.Stop()
}

func TestCoalescer_ForceFlush(t *testing.T) {
	var flushed []autoreply.ReplyPayload
	c := NewBlockReplyCoalescer(BlockStreamingCoalescing{
		MinChars: 100, MaxChars: 200, Joiner: " ",
	}, func(p autoreply.ReplyPayload) {
		flushed = append(flushed, p)
	}, nil)
	c.Enqueue(autoreply.ReplyPayload{Text: "short"})
	c.Flush(false) // not enough → no flush (schedules idle)
	if len(flushed) != 0 {
		t.Fatal("force=false below minChars should not flush")
	}
	c.Flush(true) // force → flush
	if len(flushed) != 1 {
		t.Fatal("force=true should flush")
	}
	c.Stop()
}

func TestCoalescer_ContextSwitch(t *testing.T) {
	var flushed []autoreply.ReplyPayload
	c := NewBlockReplyCoalescer(BlockStreamingCoalescing{
		MinChars: 1, MaxChars: 200, Joiner: " ",
	}, func(p autoreply.ReplyPayload) {
		flushed = append(flushed, p)
	}, nil)
	c.Enqueue(autoreply.ReplyPayload{Text: "hello", ReplyToID: "msg1"})
	c.Enqueue(autoreply.ReplyPayload{Text: "world", ReplyToID: "msg2"}) // context switch → flush "hello"
	c.Flush(true)
	c.Stop()
	if len(flushed) != 2 {
		t.Fatalf("context switch should produce 2 flushes, got %d", len(flushed))
	}
	if flushed[0].ReplyToID != "msg1" {
		t.Errorf("first flush replyToId = %q, want msg1", flushed[0].ReplyToID)
	}
	if flushed[1].ReplyToID != "msg2" {
		t.Errorf("second flush replyToId = %q, want msg2", flushed[1].ReplyToID)
	}
}

// ---------- Config 测试 ----------

func TestResolveBlockStreamingChunking_Defaults(t *testing.T) {
	chunking := ResolveBlockStreamingChunking(nil)
	if chunking.MinChars != DefaultBlockStreamMin {
		t.Errorf("MinChars = %d, want %d", chunking.MinChars, DefaultBlockStreamMin)
	}
	if chunking.MaxChars != DefaultBlockStreamMax {
		t.Errorf("MaxChars = %d, want %d", chunking.MaxChars, DefaultBlockStreamMax)
	}
}

func TestResolveBlockStreamingChunking_Custom(t *testing.T) {
	chunking := ResolveBlockStreamingChunking(&types.BlockStreamingCoalesceConfig{
		MinChars: 200,
		MaxChars: 500,
		IdleMs:   2000,
	})
	if chunking.MinChars != 200 {
		t.Errorf("MinChars = %d, want 200", chunking.MinChars)
	}
	if chunking.MaxChars != 500 {
		t.Errorf("MaxChars = %d, want 500", chunking.MaxChars)
	}
}

func TestResolveBlockStreamingCoalescing_Joiner(t *testing.T) {
	// sentence → " "
	c1 := ResolveBlockStreamingCoalescing(BlockStreamingChunking{
		BreakPreference: "sentence", MinChars: 100, MaxChars: 200,
	}, nil)
	if c1.Joiner != " " {
		t.Errorf("sentence joiner = %q, want %q", c1.Joiner, " ")
	}

	// paragraph → "\n\n"
	c2 := ResolveBlockStreamingCoalescing(BlockStreamingChunking{
		BreakPreference: "paragraph", MinChars: 100, MaxChars: 200,
	}, nil)
	if c2.Joiner != "\n\n" {
		t.Errorf("paragraph joiner = %q, want %q", c2.Joiner, "\n\n")
	}
}

// ---------- Pipeline 测试 ----------

func TestPipeline_BasicSend(t *testing.T) {
	var sent []autoreply.ReplyPayload
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {
			sent = append(sent, payload)
		},
	})

	p.Enqueue(autoreply.ReplyPayload{Text: "hello"})
	p.Stop()

	if len(sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(sent))
	}
	if sent[0].Text != "hello" {
		t.Errorf("unexpected text: %s", sent[0].Text)
	}
}

func TestPipeline_Dedup(t *testing.T) {
	var sent []autoreply.ReplyPayload
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		EnableDedup: true,
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {
			sent = append(sent, payload)
		},
	})

	p.Enqueue(autoreply.ReplyPayload{Text: "dup"})
	p.Enqueue(autoreply.ReplyPayload{Text: "dup"})
	p.Enqueue(autoreply.ReplyPayload{Text: "unique"})
	p.Stop()

	if len(sent) != 2 {
		t.Fatalf("expected 2 sends (dedup), got %d", len(sent))
	}
}

func TestPipeline_MediaBypassesCoalescer(t *testing.T) {
	var sent []autoreply.ReplyPayload
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		Coalescing: &BlockStreamingCoalescing{
			MinChars: 100, MaxChars: 200, IdleMs: 5000, Joiner: " ",
		},
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {
			sent = append(sent, payload)
		},
	})

	p.Enqueue(autoreply.ReplyPayload{MediaURL: "https://img.jpg"})

	if len(sent) != 1 {
		t.Fatalf("media should bypass coalescer, got %d sends", len(sent))
	}
	p.Stop()
}

func TestPipeline_Abort(t *testing.T) {
	var sent []autoreply.ReplyPayload
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {
			sent = append(sent, payload)
		},
	})

	p.Enqueue(autoreply.ReplyPayload{Text: "before"})
	p.Abort()
	p.Enqueue(autoreply.ReplyPayload{Text: "after"})
	p.Stop()

	if len(sent) != 1 {
		t.Fatalf("abort should prevent further sends, got %d", len(sent))
	}
	if !p.IsAborted() {
		t.Error("expected IsAborted=true")
	}
}

func TestPipeline_Timeout(t *testing.T) {
	var sent []autoreply.ReplyPayload
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		TimeoutMs: 50,
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {
			sent = append(sent, payload)
		},
	})

	p.Enqueue(autoreply.ReplyPayload{Text: "before"})
	time.Sleep(120 * time.Millisecond) // wait for timeout
	p.Enqueue(autoreply.ReplyPayload{Text: "after"})
	p.Stop()

	if len(sent) != 1 {
		t.Fatalf("timeout should abort pipeline, got %d sends", len(sent))
	}
	if !p.IsAborted() {
		t.Error("expected IsAborted=true after timeout")
	}
}

func TestPipeline_DidStream(t *testing.T) {
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {},
	})

	if p.DidStream() {
		t.Error("should not have streamed before enqueue")
	}
	p.Enqueue(autoreply.ReplyPayload{Text: "x"})
	if !p.DidStream() {
		t.Error("should have streamed after enqueue")
	}
	p.Stop()
}

func TestPipeline_HasSentPayload(t *testing.T) {
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {},
	})

	payload := autoreply.ReplyPayload{Text: "check"}
	if p.HasSentPayload(payload) {
		t.Error("should not have sent before enqueue")
	}
	p.Enqueue(payload)
	if !p.HasSentPayload(payload) {
		t.Error("should have sent after enqueue")
	}
	p.Stop()
}

func TestPipeline_WithCoalescer(t *testing.T) {
	var mu sync.Mutex
	var sent []autoreply.ReplyPayload
	p := NewBlockReplyPipeline(BlockReplyPipelineConfig{
		Coalescing: &BlockStreamingCoalescing{
			MinChars: 5, MaxChars: 100, IdleMs: 50, Joiner: " ",
		},
		Send: func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext) {
			mu.Lock()
			sent = append(sent, payload)
			mu.Unlock()
		},
	})

	p.Enqueue(autoreply.ReplyPayload{Text: "hello"})
	p.Enqueue(autoreply.ReplyPayload{Text: "world"})
	time.Sleep(120 * time.Millisecond) // wait for idle flush
	p.Stop()

	mu.Lock()
	n := len(sent)
	mu.Unlock()
	if n < 1 {
		t.Fatalf("expected at least 1 coalesced send, got %d", n)
	}
}

// ---------- Dock 联动测试 ----------

func TestResolveBlockStreamingChunkingWithDock_Discord(t *testing.T) {
	// 注入 discord-like dock defaults: minChars=1500, idleMs=1000
	origProvider := BlockStreamingCoalesceDefaultsProvider
	BlockStreamingCoalesceDefaultsProvider = func(channelKey string) (int, int) {
		if channelKey == "discord" {
			return 1500, 1000
		}
		return 0, 0
	}
	defer func() { BlockStreamingCoalesceDefaultsProvider = origProvider }()

	chunking := ResolveBlockStreamingChunkingWithDock(nil, "discord")
	if chunking.MinChars != 1500 {
		t.Errorf("Discord MinChars = %d, want 1500", chunking.MinChars)
	}
}

func TestResolveBlockStreamingChunkingWithDock_NoStreaming(t *testing.T) {
	origProvider := BlockStreamingCoalesceDefaultsProvider
	BlockStreamingCoalesceDefaultsProvider = func(channelKey string) (int, int) {
		return 0, 0 // 无 streaming defaults
	}
	defer func() { BlockStreamingCoalesceDefaultsProvider = origProvider }()

	chunking := ResolveBlockStreamingChunkingWithDock(nil, "whatsapp")
	if chunking.MinChars != DefaultBlockStreamMin {
		t.Errorf("WhatsApp MinChars = %d, want %d", chunking.MinChars, DefaultBlockStreamMin)
	}
}

func TestResolveBlockStreamingChunkingWithDock_ConfigOverride(t *testing.T) {
	origProvider := BlockStreamingCoalesceDefaultsProvider
	BlockStreamingCoalesceDefaultsProvider = func(channelKey string) (int, int) {
		return 1500, 1000 // dock defaults
	}
	defer func() { BlockStreamingCoalesceDefaultsProvider = origProvider }()

	// config 级 minChars=200 应覆盖 dock 的 1500
	chunking := ResolveBlockStreamingChunkingWithDock(&types.BlockStreamingCoalesceConfig{
		MinChars: 200,
	}, "discord")
	if chunking.MinChars != 200 {
		t.Errorf("config override MinChars = %d, want 200", chunking.MinChars)
	}
}
