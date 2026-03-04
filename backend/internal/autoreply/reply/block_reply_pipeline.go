package reply

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// ---------- Block Reply Pipeline ----------
// 对齐 TS auto-reply/reply/block-reply-pipeline.ts
// 管理 block reply 的去重、合并、超时、abort、buffer、串行发送。

// BlockReplyPipelineConfig 配置。
type BlockReplyPipelineConfig struct {
	Coalescing  *BlockStreamingCoalescing
	Send        func(payload autoreply.ReplyPayload, ctx *autoreply.BlockReplyContext)
	TimeoutMs   int  // 超时中止（0=无超时）
	EnableDedup bool // 启用 payload 去重
	Buffer      *BlockReplyBuffer
}

// BlockReplyBuffer 缓冲区接口（对齐 TS BlockReplyBuffer）。
// TS 对照: block-reply-pipeline.ts L16-20
type BlockReplyBuffer struct {
	ShouldBuffer func(payload autoreply.ReplyPayload) bool
	OnEnqueue    func(payload autoreply.ReplyPayload)
	Finalize     func(payload autoreply.ReplyPayload) autoreply.ReplyPayload
}

// BlockReplyPipeline 管线核心。
type BlockReplyPipeline struct {
	mu               sync.Mutex
	cfg              BlockReplyPipelineConfig
	coalescer        *BlockReplyCoalescer
	sentKeys         map[string]bool // 已发送的 payload key
	seenKeys         map[string]bool // 已见过的 payload key
	pendingKeys      map[string]bool // 正在发送中的 payload key
	bufferedKeys     map[string]bool // 在 coalescer 中的 key
	buffPayloadKeys  map[string]bool // 在 buffer 中的 payload key
	bufferedPayloads []autoreply.ReplyPayload
	aborted          bool
	didStream        bool
	stopped          bool
	timeoutCtx       *time.Timer
}

// NewBlockReplyPipeline 创建 block reply 管线。
func NewBlockReplyPipeline(cfg BlockReplyPipelineConfig) *BlockReplyPipeline {
	p := &BlockReplyPipeline{
		cfg:             cfg,
		sentKeys:        make(map[string]bool),
		seenKeys:        make(map[string]bool),
		pendingKeys:     make(map[string]bool),
		bufferedKeys:    make(map[string]bool),
		buffPayloadKeys: make(map[string]bool),
	}

	// 创建 coalescer（如果提供了 coalescing 配置）
	if cfg.Coalescing != nil {
		p.coalescer = NewBlockReplyCoalescer(*cfg.Coalescing, func(payload autoreply.ReplyPayload) {
			// coalescer flush → 清除 bufferedKeys 并发送
			p.mu.Lock()
			p.bufferedKeys = make(map[string]bool)
			p.mu.Unlock()
			p.sendPayload(payload, false)
		}, func() bool {
			p.mu.Lock()
			defer p.mu.Unlock()
			return p.aborted
		})
	}

	// 设置超时
	if cfg.TimeoutMs > 0 {
		p.timeoutCtx = time.AfterFunc(time.Duration(cfg.TimeoutMs)*time.Millisecond, func() {
			p.Abort()
		})
	}

	return p
}

// Enqueue 添加 payload 到管线。
// TS 对照: block-reply-pipeline.ts L195-218
func (p *BlockReplyPipeline) Enqueue(payload autoreply.ReplyPayload) {
	p.mu.Lock()
	if p.aborted || p.stopped {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	// 缓冲区检查（audioAsVoice 等）
	if p.bufferPayload(payload) {
		return
	}

	// 媒体 payload → flush coalescer + 直接发送
	hasMedia := payload.MediaURL != "" || len(payload.MediaURLs) > 0
	if hasMedia {
		if p.coalescer != nil {
			p.coalescer.Flush(true)
		}
		p.sendPayload(payload, false)
		return
	}

	// 文本 payload → 走 coalescer 或直接发送
	if p.coalescer != nil {
		payloadKey := payloadKey(payload)
		p.mu.Lock()
		if p.seenKeys[payloadKey] || p.pendingKeys[payloadKey] || p.bufferedKeys[payloadKey] {
			p.mu.Unlock()
			return
		}
		p.bufferedKeys[payloadKey] = true
		p.mu.Unlock()
		p.coalescer.Enqueue(payload)
		return
	}

	p.sendPayload(payload, false)
}

// Flush 刷新管线（包括 coalescer 缓冲区和 buffer）。
func (p *BlockReplyPipeline) Flush(force bool) {
	if p.coalescer != nil {
		p.coalescer.Flush(force)
	}
	p.flushBuffered()
}

// Stop 停止管线。
func (p *BlockReplyPipeline) Stop() {
	p.mu.Lock()
	p.stopped = true
	if p.timeoutCtx != nil {
		p.timeoutCtx.Stop()
	}
	p.mu.Unlock()

	if p.coalescer != nil {
		p.coalescer.Stop()
	}
}

// Abort 中止管线（超时或错误）。
func (p *BlockReplyPipeline) Abort() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.aborted = true
	if p.timeoutCtx != nil {
		p.timeoutCtx.Stop()
	}
}

// DidStream 是否已发送过至少一个 payload。
func (p *BlockReplyPipeline) DidStream() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.didStream
}

// IsAborted 是否已中止。
func (p *BlockReplyPipeline) IsAborted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.aborted
}

// HasBuffered 是否有待发送的缓冲内容。
func (p *BlockReplyPipeline) HasBuffered() bool {
	hasCB := false
	if p.coalescer != nil {
		hasCB = p.coalescer.HasBuffered()
	}
	p.mu.Lock()
	hasBP := len(p.bufferedPayloads) > 0
	p.mu.Unlock()
	return hasCB || hasBP
}

// HasSentPayload 检查某个 payload 是否已被发送。
func (p *BlockReplyPipeline) HasSentPayload(payload autoreply.ReplyPayload) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sentKeys[payloadKey(payload)]
}

// --- Buffer 方法 ---
// TS 对照: block-reply-pipeline.ts L163-181

func (p *BlockReplyPipeline) bufferPayload(payload autoreply.ReplyPayload) bool {
	buf := p.cfg.Buffer
	if buf == nil {
		return false
	}
	if buf.OnEnqueue != nil {
		buf.OnEnqueue(payload)
	}
	if !buf.ShouldBuffer(payload) {
		return false
	}
	key := payloadKey(payload)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seenKeys[key] || p.sentKeys[key] || p.pendingKeys[key] || p.buffPayloadKeys[key] {
		return true
	}
	p.seenKeys[key] = true
	p.buffPayloadKeys[key] = true
	p.bufferedPayloads = append(p.bufferedPayloads, payload)
	return true
}

// flushBuffered 发送所有 buffer 中的 payload。
// TS 对照: block-reply-pipeline.ts L183-193
func (p *BlockReplyPipeline) flushBuffered() {
	p.mu.Lock()
	if len(p.bufferedPayloads) == 0 {
		p.mu.Unlock()
		return
	}
	payloads := make([]autoreply.ReplyPayload, len(p.bufferedPayloads))
	copy(payloads, p.bufferedPayloads)
	p.bufferedPayloads = p.bufferedPayloads[:0]
	p.buffPayloadKeys = make(map[string]bool)
	p.mu.Unlock()

	for _, payload := range payloads {
		if p.cfg.Buffer != nil && p.cfg.Buffer.Finalize != nil {
			payload = p.cfg.Buffer.Finalize(payload)
		}
		p.sendPayload(payload, true) // skipSeen=true（buffer 内已去重）
	}
}

// --- 内部方法 ---

func (p *BlockReplyPipeline) sendPayload(payload autoreply.ReplyPayload, skipSeen bool) {
	p.mu.Lock()
	if p.aborted || p.stopped {
		p.mu.Unlock()
		return
	}

	key := payloadKey(payload)
	if !skipSeen {
		if p.seenKeys[key] {
			p.mu.Unlock()
			return
		}
		p.seenKeys[key] = true
	}
	if p.sentKeys[key] || p.pendingKeys[key] {
		p.mu.Unlock()
		return
	}
	p.pendingKeys[key] = true
	p.mu.Unlock()

	// 同步发送（Go 不需要 promise chain）
	ctx := NewBlockReplyContext()
	p.cfg.Send(payload, ctx)

	p.mu.Lock()
	delete(p.pendingKeys, key)
	p.sentKeys[key] = true
	p.didStream = true
	p.mu.Unlock()
}

// payloadKey 生成 payload 的去重 key。
// TS 对照: block-reply-pipeline.ts L37-49
func payloadKey(payload autoreply.ReplyPayload) string {
	text := payload.Text
	// 构建 mediaList
	var mediaList []string
	if len(payload.MediaURLs) > 0 {
		mediaList = payload.MediaURLs
	} else if payload.MediaURL != "" {
		mediaList = []string{payload.MediaURL}
	}
	replyToID := payload.ReplyToID
	data, _ := json.Marshal(struct {
		Text      string   `json:"text,omitempty"`
		MediaList []string `json:"mediaList,omitempty"`
		ReplyToID string   `json:"replyToId,omitempty"`
	}{
		Text:      text,
		MediaList: mediaList,
		ReplyToID: replyToID,
	})
	return string(data)
}

// CreateAudioAsVoiceBuffer 创建 audioAsVoice 缓冲区。
// TS 对照: block-reply-pipeline.ts L22-35
func CreateAudioAsVoiceBuffer(isAudioPayload func(autoreply.ReplyPayload) bool) *BlockReplyBuffer {
	seenAudioAsVoice := false
	return &BlockReplyBuffer{
		OnEnqueue: func(payload autoreply.ReplyPayload) {
			if payload.AudioAsVoice {
				seenAudioAsVoice = true
			}
		},
		ShouldBuffer: func(payload autoreply.ReplyPayload) bool {
			return isAudioPayload(payload)
		},
		Finalize: func(payload autoreply.ReplyPayload) autoreply.ReplyPayload {
			if seenAudioAsVoice {
				payload.AudioAsVoice = true
			}
			return payload
		},
	}
}
