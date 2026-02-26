package reply

import (
	"strings"
	"sync"
	"time"
)

// TS 对照: auto-reply/reply/typing.ts (197L)

// TypingController 打字指示符控制器。
// 管理打字指示符的启停、TTL 超时和密封机制。
type TypingController struct {
	mu sync.Mutex

	onReplyStart func() error
	onCleanup    func()
	intervalMs   int
	ttlMs        int
	silentToken  string
	logFn        func(string)

	started      bool
	active       bool
	runComplete  bool
	dispatchIdle bool
	sealed       bool

	typingTicker *time.Ticker
	ttlTimer     *time.Timer
	stopCh       chan struct{}
}

// TypingControllerParams 打字控制器创建参数。
type TypingControllerParams struct {
	OnReplyStart          func() error
	OnCleanup             func()
	TypingIntervalSeconds int
	TypingTtlMs           int
	SilentToken           string
	Log                   func(string)
}

const (
	defaultTypingIntervalSeconds = 6
	defaultTypingTtlMs           = 2 * 60 * 1000 // 2 minutes
	defaultSilentToken           = "NO_REPLY"
)

// NewTypingController 创建打字控制器。
// TS 对照: typing.ts L14-196
func NewTypingController(params TypingControllerParams) *TypingController {
	intervalSec := params.TypingIntervalSeconds
	if intervalSec <= 0 {
		intervalSec = defaultTypingIntervalSeconds
	}
	ttlMs := params.TypingTtlMs
	if ttlMs <= 0 {
		ttlMs = defaultTypingTtlMs
	}
	silentToken := params.SilentToken
	if silentToken == "" {
		silentToken = defaultSilentToken
	}
	return &TypingController{
		onReplyStart: params.OnReplyStart,
		onCleanup:    params.OnCleanup,
		intervalMs:   intervalSec * 1000,
		ttlMs:        ttlMs,
		silentToken:  silentToken,
		logFn:        params.Log,
		stopCh:       make(chan struct{}),
	}
}

// OnReplyStart 确保打字已启动。
func (tc *TypingController) OnReplyStart() error {
	return tc.ensureStart()
}

// StartTypingLoop 启动打字循环。
func (tc *TypingController) StartTypingLoop() error {
	tc.mu.Lock()
	if tc.sealed || tc.runComplete {
		tc.mu.Unlock()
		return nil
	}
	tc.mu.Unlock()

	tc.RefreshTypingTtl()

	if tc.onReplyStart == nil || tc.intervalMs <= 0 {
		return nil
	}

	tc.mu.Lock()
	if tc.typingTicker != nil {
		tc.mu.Unlock()
		return nil
	}
	tc.mu.Unlock()

	if err := tc.ensureStart(); err != nil {
		return err
	}

	tc.mu.Lock()
	tc.typingTicker = time.NewTicker(time.Duration(tc.intervalMs) * time.Millisecond)
	ticker := tc.typingTicker
	tc.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				tc.mu.Lock()
				sealed := tc.sealed
				tc.mu.Unlock()
				if sealed {
					return
				}
				tc.triggerTyping()
			case <-tc.stopCh:
				return
			}
		}
	}()

	return nil
}

// StartTypingOnText 在收到非空非静默文本时启动打字。
func (tc *TypingController) StartTypingOnText(text string) error {
	tc.mu.Lock()
	if tc.sealed {
		tc.mu.Unlock()
		return nil
	}
	tc.mu.Unlock()

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if isSilentReply(trimmed, tc.silentToken) {
		return nil
	}
	tc.RefreshTypingTtl()
	return tc.StartTypingLoop()
}

// RefreshTypingTtl 刷新打字 TTL。
func (tc *TypingController) RefreshTypingTtl() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.sealed || tc.intervalMs <= 0 || tc.ttlMs <= 0 {
		return
	}
	if tc.ttlTimer != nil {
		tc.ttlTimer.Stop()
	}
	tc.ttlTimer = time.AfterFunc(time.Duration(tc.ttlMs)*time.Millisecond, func() {
		tc.mu.Lock()
		hasTicker := tc.typingTicker != nil
		tc.mu.Unlock()
		if !hasTicker {
			return
		}
		if tc.logFn != nil {
			tc.logFn("typing TTL reached; stopping typing indicator")
		}
		tc.Cleanup()
	})
}

// IsActive 是否正在打字。
func (tc *TypingController) IsActive() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.active && !tc.sealed
}

// MarkRunComplete 标记模型运行完成。
func (tc *TypingController) MarkRunComplete() {
	tc.mu.Lock()
	tc.runComplete = true
	tc.mu.Unlock()
	tc.maybeStopOnIdle()
}

// MarkDispatchIdle 标记分发器空闲。
func (tc *TypingController) MarkDispatchIdle() {
	tc.mu.Lock()
	tc.dispatchIdle = true
	tc.mu.Unlock()
	tc.maybeStopOnIdle()
}

// Cleanup 清理打字指示符。
func (tc *TypingController) Cleanup() {
	tc.mu.Lock()
	if tc.sealed {
		tc.mu.Unlock()
		return
	}
	wasActive := tc.active
	if tc.ttlTimer != nil {
		tc.ttlTimer.Stop()
		tc.ttlTimer = nil
	}
	if tc.typingTicker != nil {
		tc.typingTicker.Stop()
		tc.typingTicker = nil
	}
	tc.started = false
	tc.active = false
	tc.runComplete = false
	tc.dispatchIdle = false
	tc.sealed = true
	tc.mu.Unlock()

	// 关闭 stopCh 通知 goroutine 退出
	select {
	case <-tc.stopCh:
		// already closed
	default:
		close(tc.stopCh)
	}

	if wasActive && tc.onCleanup != nil {
		tc.onCleanup()
	}
}

func (tc *TypingController) ensureStart() error {
	tc.mu.Lock()
	if tc.sealed || tc.runComplete {
		tc.mu.Unlock()
		return nil
	}
	if !tc.active {
		tc.active = true
	}
	if tc.started {
		tc.mu.Unlock()
		return nil
	}
	tc.started = true
	tc.mu.Unlock()
	return tc.triggerTyping()
}

func (tc *TypingController) triggerTyping() error {
	tc.mu.Lock()
	if tc.sealed {
		tc.mu.Unlock()
		return nil
	}
	tc.mu.Unlock()
	if tc.onReplyStart != nil {
		return tc.onReplyStart()
	}
	return nil
}

func (tc *TypingController) maybeStopOnIdle() {
	tc.mu.Lock()
	active := tc.active
	runDone := tc.runComplete
	dispIdle := tc.dispatchIdle
	tc.mu.Unlock()

	if active && runDone && dispIdle {
		tc.Cleanup()
	}
}

func isSilentReply(text, token string) bool {
	if token == "" {
		return false
	}
	return strings.Contains(text, token)
}
