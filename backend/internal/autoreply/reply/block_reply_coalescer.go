package reply

import (
	"strings"
	"sync"
	"time"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// ---------- Block Reply Coalescer ----------
// 对齐 TS auto-reply/reply/block-reply-coalescer.ts
// 将 ReplyPayload 合并到缓冲区，达到 minChars/maxChars 或 idle timeout 时 flush。
// 跟踪 replyToId / audioAsVoice 上下文，上下文切换时自动 flush。

// BlockStreamingCoalescing 合并配置。
type BlockStreamingCoalescing struct {
	MinChars       int
	MaxChars       int
	IdleMs         int
	Joiner         string // 默认 " "（句子）/ "\n\n"（段落）/ "\n"（换行）
	FlushOnEnqueue bool   // true 则每次 enqueue 都立即 flush（无缓冲模式）
}

// BlockReplyCoalescer 文本合并缓冲器。
// TS 对照: block-reply-coalescer.ts createBlockReplyCoalescer
type BlockReplyCoalescer struct {
	mu                 sync.Mutex
	cfg                BlockStreamingCoalescing
	minChars           int
	maxChars           int
	bufferText         string
	bufferReplyToID    string // 当前缓冲区的 replyToId 上下文
	bufferAudioAsVoice bool   // 当前缓冲区的 audioAsVoice 上下文
	onFlush            func(payload autoreply.ReplyPayload)
	shouldAbort        func() bool
	idleStop           func()
	stopped            bool
}

// NewBlockReplyCoalescer 创建合并缓冲器。
// TS 对照: block-reply-coalescer.ts L11-15
func NewBlockReplyCoalescer(cfg BlockStreamingCoalescing, onFlush func(payload autoreply.ReplyPayload), shouldAbort func() bool) *BlockReplyCoalescer {
	minChars := cfg.MinChars
	if minChars < 1 {
		minChars = 1
	}
	maxChars := cfg.MaxChars
	if maxChars < minChars {
		maxChars = minChars
	}
	return &BlockReplyCoalescer{
		cfg:         cfg,
		minChars:    minChars,
		maxChars:    maxChars,
		onFlush:     onFlush,
		shouldAbort: shouldAbort,
	}
}

// Enqueue 添加 payload 到缓冲区。
// TS 对照: block-reply-coalescer.ts L74-139
func (c *BlockReplyCoalescer) Enqueue(payload autoreply.ReplyPayload) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	if c.shouldAbort != nil && c.shouldAbort() {
		c.resetBufferLocked()
		return
	}

	// 媒体 payload → flush 缓冲区 + 直接转发
	// TS 对照: L78-85
	hasMedia := payload.MediaURL != "" || len(payload.MediaURLs) > 0
	if hasMedia {
		c.flushLocked(true)
		if c.onFlush != nil {
			c.onFlush(payload)
		}
		return
	}

	text := strings.TrimSpace(payload.Text)
	if text == "" {
		return
	}

	joiner := c.cfg.Joiner

	// FlushOnEnqueue 模式：每次 enqueue 都立即 flush
	// TS 对照: L92-101
	if c.cfg.FlushOnEnqueue {
		if c.bufferText != "" {
			c.flushLocked(true)
		}
		c.bufferReplyToID = payload.ReplyToID
		c.bufferAudioAsVoice = payload.AudioAsVoice
		c.bufferText = payload.Text // 使用原始文本（非 trimmed）
		c.flushLocked(true)
		return
	}

	// 上下文切换检测 — replyToId 或 audioAsVoice 变化时 flush
	// TS 对照: L103-108
	if c.bufferText != "" &&
		(c.bufferReplyToID != payload.ReplyToID || c.bufferAudioAsVoice != payload.AudioAsVoice) {
		c.flushLocked(true)
	}

	// 首次填充 → 记录上下文
	// TS 对照: L110-113
	if c.bufferText == "" {
		c.bufferReplyToID = payload.ReplyToID
		c.bufferAudioAsVoice = payload.AudioAsVoice
	}

	// maxChars 溢出拆分逻辑
	// TS 对照: L115-131
	nextText := payload.Text
	if c.bufferText != "" {
		nextText = c.bufferText + joiner + payload.Text
	}
	if len(nextText) > c.maxChars {
		if c.bufferText != "" {
			// 先 flush 已有缓冲
			c.flushLocked(true)
			c.bufferReplyToID = payload.ReplyToID
			c.bufferAudioAsVoice = payload.AudioAsVoice
			// 如果单条文本也超出 maxChars → 直接发送
			if len(payload.Text) >= c.maxChars {
				if c.onFlush != nil {
					c.onFlush(payload)
				}
				return
			}
			c.bufferText = payload.Text
			c.scheduleIdleFlushLocked()
			return
		}
		// 缓冲区为空但单条文本超出 → 直接发送
		if c.onFlush != nil {
			c.onFlush(payload)
		}
		return
	}

	c.bufferText = nextText

	// 达到 maxChars → 立即 flush
	if len(c.bufferText) >= c.maxChars {
		c.flushLocked(true)
		return
	}

	c.scheduleIdleFlushLocked()
}

// Flush 刷新缓冲区（force=true 忽略 minChars 要求）。
func (c *BlockReplyCoalescer) Flush(force bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushLocked(force)
}

// HasBuffered 是否有待 flush 的内容。
func (c *BlockReplyCoalescer) HasBuffered() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bufferText != ""
}

// Stop 停止接受新内容。
// TS 对照: block-reply-coalescer.ts L145
func (c *BlockReplyCoalescer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	c.stopIdleTimerLocked()
}

// --- 内部方法 ---

func (c *BlockReplyCoalescer) flushLocked(force bool) {
	c.stopIdleTimerLocked()

	if c.shouldAbort != nil && c.shouldAbort() {
		c.resetBufferLocked()
		return
	}

	if c.bufferText == "" {
		return
	}

	if !force && !c.cfg.FlushOnEnqueue && len(c.bufferText) < c.minChars {
		c.scheduleIdleFlushLocked()
		return
	}

	payload := autoreply.ReplyPayload{
		Text:         c.bufferText,
		ReplyToID:    c.bufferReplyToID,
		AudioAsVoice: c.bufferAudioAsVoice,
	}
	c.resetBufferLocked()

	if c.onFlush != nil {
		c.onFlush(payload)
	}
}

func (c *BlockReplyCoalescer) resetBufferLocked() {
	c.bufferText = ""
	c.bufferReplyToID = ""
	c.bufferAudioAsVoice = false
}

func (c *BlockReplyCoalescer) scheduleIdleFlushLocked() {
	c.stopIdleTimerLocked()

	if c.cfg.IdleMs <= 0 {
		return
	}

	timer := time.AfterFunc(time.Duration(c.cfg.IdleMs)*time.Millisecond, func() {
		c.Flush(false) // idle timeout → 非 force（受 minChars 约束）
	})
	c.idleStop = func() { timer.Stop() }
}

func (c *BlockReplyCoalescer) stopIdleTimerLocked() {
	if c.idleStop != nil {
		c.idleStop()
		c.idleStop = nil
	}
}

// --- BlockReplyContext ---
// 对齐 TS types 中的 BlockReplyContext

// BlockReplyContext block reply 上下文信息（传递给 channel adapter）。
func NewBlockReplyContext() *autoreply.BlockReplyContext {
	return &autoreply.BlockReplyContext{}
}
