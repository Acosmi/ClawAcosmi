package reply

import (
	"math/rand"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/reply-dispatcher.ts (194L)

const (
	defaultHumanDelayMinMs = 800
	defaultHumanDelayMaxMs = 2500
)

// HumanDelayConfig 人类模拟延迟配置。
type HumanDelayConfig struct {
	Mode  string // "off" | "on" | "custom"
	MinMs int
	MaxMs int
}

// getHumanDelay 生成人类模拟延迟（毫秒）。
// TS 对照: reply-dispatcher.ts L25-39
func getHumanDelay(config *HumanDelayConfig) int {
	if config == nil || config.Mode == "" || config.Mode == "off" {
		return 0
	}
	minMs := defaultHumanDelayMinMs
	maxMs := defaultHumanDelayMaxMs
	if config.Mode == "custom" {
		if config.MinMs > 0 {
			minMs = config.MinMs
		}
		if config.MaxMs > 0 {
			maxMs = config.MaxMs
		}
	}
	if maxMs <= minMs {
		return minMs
	}
	return minMs + rand.Intn(maxMs-minMs+1)
}

// ReplyDispatchDeliverer 回复投递函数。
type ReplyDispatchDeliverer func(payload autoreply.ReplyPayload, kind ReplyDispatchKind) error

// ReplyDispatchErrorHandler 回复投递错误处理器。
type ReplyDispatchErrorHandler func(err error, kind ReplyDispatchKind)

// ReplyDispatchSkipHandler 回复跳过处理器。
type ReplyDispatchSkipHandler func(payload autoreply.ReplyPayload, kind ReplyDispatchKind, reason NormalizeReplySkipReason)

// ReplyDispatcherOptions 回复分发器选项。
// TS 对照: reply-dispatcher.ts L41-56
type ReplyDispatcherOptions struct {
	Deliver                       ReplyDispatchDeliverer
	ResponsePrefix                string
	ResponsePrefixContext         *ResponsePrefixContext
	ResponsePrefixContextProvider func() *ResponsePrefixContext
	OnHeartbeatStrip              func()
	OnIdle                        func()
	OnError                       ReplyDispatchErrorHandler
	OnSkip                        ReplyDispatchSkipHandler
	HumanDelay                    *HumanDelayConfig
}

// ReplyDispatcher 回复分发器接口。
// TS 对照: reply-dispatcher.ts L71-77
type ReplyDispatcher struct {
	mu             sync.Mutex
	pending        int
	sentFirstBlock bool
	queuedCounts   map[ReplyDispatchKind]int
	sendCh         chan sendItem
	doneCh         chan struct{}
	options        ReplyDispatcherOptions
}

type sendItem struct {
	kind    ReplyDispatchKind
	payload autoreply.ReplyPayload
	delay   bool
}

// CreateReplyDispatcher 创建回复分发器。
// TS 对照: reply-dispatcher.ts L101-164
func CreateReplyDispatcher(opts ReplyDispatcherOptions) *ReplyDispatcher {
	d := &ReplyDispatcher{
		queuedCounts: map[ReplyDispatchKind]int{
			DispatchTool:  0,
			DispatchBlock: 0,
			DispatchFinal: 0,
		},
		sendCh:  make(chan sendItem, 64),
		doneCh:  make(chan struct{}),
		options: opts,
	}

	// 启动串行发送 goroutine
	go d.processLoop()

	return d
}

// processLoop 串行处理发送队列。
func (d *ReplyDispatcher) processLoop() {
	for item := range d.sendCh {
		// 人类模拟延迟（仅第一个 block 之后的 block）
		if item.delay {
			delayMs := getHumanDelay(d.options.HumanDelay)
			if delayMs > 0 {
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
			}
		}

		err := d.options.Deliver(item.payload, item.kind)
		if err != nil && d.options.OnError != nil {
			d.options.OnError(err, item.kind)
		}

		d.mu.Lock()
		d.pending--
		idle := d.pending == 0
		d.mu.Unlock()

		if idle && d.options.OnIdle != nil {
			d.options.OnIdle()
		}
	}
	close(d.doneCh)
}

// enqueue 入队发送项。
func (d *ReplyDispatcher) enqueue(kind ReplyDispatchKind, payload autoreply.ReplyPayload) bool {
	// 获取前缀上下文
	var prefixCtx *ResponsePrefixContext
	if d.options.ResponsePrefixContextProvider != nil {
		prefixCtx = d.options.ResponsePrefixContextProvider()
	} else {
		prefixCtx = d.options.ResponsePrefixContext
	}

	normalized := NormalizeReplyPayload(payload, &NormalizeReplyOptions{
		ResponsePrefix:        d.options.ResponsePrefix,
		ResponsePrefixContext: prefixCtx,
		OnHeartbeatStrip:      d.options.OnHeartbeatStrip,
		OnSkip: func(reason NormalizeReplySkipReason) {
			if d.options.OnSkip != nil {
				d.options.OnSkip(payload, kind, reason)
			}
		},
	})
	if normalized == nil {
		return false
	}

	d.mu.Lock()
	d.queuedCounts[kind]++
	d.pending++
	shouldDelay := kind == DispatchBlock && d.sentFirstBlock
	if kind == DispatchBlock {
		d.sentFirstBlock = true
	}
	d.mu.Unlock()

	d.sendCh <- sendItem{
		kind:    kind,
		payload: *normalized,
		delay:   shouldDelay,
	}
	return true
}

// SendToolResult 发送工具结果。
func (d *ReplyDispatcher) SendToolResult(payload autoreply.ReplyPayload) bool {
	return d.enqueue(DispatchTool, payload)
}

// SendBlockReply 发送块回复。
func (d *ReplyDispatcher) SendBlockReply(payload autoreply.ReplyPayload) bool {
	return d.enqueue(DispatchBlock, payload)
}

// SendFinalReply 发送最终回复。
func (d *ReplyDispatcher) SendFinalReply(payload autoreply.ReplyPayload) bool {
	return d.enqueue(DispatchFinal, payload)
}

// WaitForIdle 等待所有回复发送完毕。
func (d *ReplyDispatcher) WaitForIdle() {
	d.mu.Lock()
	pending := d.pending
	d.mu.Unlock()
	if pending == 0 {
		return
	}
	// 简单轮询等待
	for {
		time.Sleep(10 * time.Millisecond)
		d.mu.Lock()
		pending = d.pending
		d.mu.Unlock()
		if pending == 0 {
			return
		}
	}
}

// GetQueuedCounts 获取各类型排队计数。
func (d *ReplyDispatcher) GetQueuedCounts() map[ReplyDispatchKind]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make(map[ReplyDispatchKind]int)
	for k, v := range d.queuedCounts {
		result[k] = v
	}
	return result
}

// Close 关闭分发器。
func (d *ReplyDispatcher) Close() {
	close(d.sendCh)
	<-d.doneCh
}
