package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Telegram 草稿流 — 继承自 src/telegram/draft-stream.ts (140L)
// 用于向 Telegram 客户端推送「正在输入」的实时预览草稿。

const (
	telegramDraftMaxChars = 4096
	defaultThrottleMs     = 300
)

// TelegramDraftStream 草稿流控制器
type TelegramDraftStream struct {
	mu          sync.Mutex
	client      *http.Client
	token       string
	chatID      int64
	draftID     int
	draftMsgID  int // 降级模式下追踪的已发送草稿消息 ID
	maxChars    int
	throttleMs  int
	threadParam map[string]int

	lastSentText string
	lastSentAt   time.Time
	pendingText  string
	inFlight     bool
	stopped      bool
	timer        *time.Timer
	log          func(string)
	warn         func(string)
}

// DraftStreamConfig 草稿流配置
type DraftStreamConfig struct {
	Client     *http.Client
	Token      string
	ChatID     int64
	DraftID    int
	MaxChars   int
	Thread     *TelegramThreadSpec
	ThrottleMs int
	Log        func(string)
	Warn       func(string)
}

// NewTelegramDraftStream 创建草稿流
func NewTelegramDraftStream(cfg DraftStreamConfig) *TelegramDraftStream {
	maxChars := telegramDraftMaxChars
	if cfg.MaxChars > 0 && cfg.MaxChars < maxChars {
		maxChars = cfg.MaxChars
	}
	throttleMs := defaultThrottleMs
	if cfg.ThrottleMs > 50 {
		throttleMs = cfg.ThrottleMs
	}
	draftID := cfg.DraftID
	if draftID == 0 {
		draftID = 1
	}
	if draftID < 0 {
		draftID = -draftID
	}

	logFn := cfg.Log
	if logFn == nil {
		logFn = func(s string) { slog.Debug(s) }
	}
	warnFn := cfg.Warn
	if warnFn == nil {
		warnFn = func(s string) { slog.Warn(s) }
	}

	ds := &TelegramDraftStream{
		client:      cfg.Client,
		token:       cfg.Token,
		chatID:      cfg.ChatID,
		draftID:     draftID,
		maxChars:    maxChars,
		throttleMs:  throttleMs,
		threadParam: BuildTelegramThreadParams(cfg.Thread),
		log:         logFn,
		warn:        warnFn,
	}

	logFn(fmt.Sprintf("telegram draft stream ready (draftId=%d, maxChars=%d, throttleMs=%d)", draftID, maxChars, throttleMs))
	return ds
}

// Update 更新草稿文本
func (ds *TelegramDraftStream) Update(text string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.stopped {
		return
	}
	ds.pendingText = text
	if ds.inFlight {
		ds.schedule()
		return
	}
	if ds.timer == nil && time.Since(ds.lastSentAt) >= time.Duration(ds.throttleMs)*time.Millisecond {
		go ds.flush()
		return
	}
	ds.schedule()
}

// Flush 立即发送草稿
func (ds *TelegramDraftStream) Flush() {
	ds.flush()
}

// Stop 停止草稿流并清理草稿消息。
// DY-021 审计修复: 降级方案使用 sendMessage 创建了真实可见消息，
// Stop 时需要删除该消息以避免聊天中残留草稿。
func (ds *TelegramDraftStream) Stop() {
	ds.mu.Lock()
	ds.stopped = true
	ds.pendingText = ""
	if ds.timer != nil {
		ds.timer.Stop()
		ds.timer = nil
	}
	msgID := ds.draftMsgID
	ds.draftMsgID = 0
	ds.mu.Unlock()

	// 清理草稿消息（异步，不阻塞调用方）
	if msgID != 0 {
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := callTelegramAPI(cleanupCtx, ds.client, ds.token, "deleteMessage", map[string]interface{}{
				"chat_id":    ds.chatID,
				"message_id": msgID,
			})
			if err != nil {
				ds.warn(fmt.Sprintf("telegram draft stream cleanup failed: %v", err))
			}
		}()
	}
}

func (ds *TelegramDraftStream) flush() {
	ds.mu.Lock()
	if ds.timer != nil {
		ds.timer.Stop()
		ds.timer = nil
	}
	if ds.stopped {
		ds.mu.Unlock()
		return
	}
	if ds.inFlight {
		// 对齐 TS L73: inFlight 时重新排队，确保 pending 更新不丢失
		ds.schedule()
		ds.mu.Unlock()
		return
	}
	text := ds.pendingText
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		ds.pendingText = ""
		ds.mu.Unlock()
		return
	}
	ds.pendingText = ""
	ds.inFlight = true
	ds.mu.Unlock()

	ds.sendDraft(trimmed)

	ds.mu.Lock()
	ds.inFlight = false
	hasPending := ds.pendingText != ""
	ds.mu.Unlock()

	if hasPending {
		ds.mu.Lock()
		ds.schedule()
		ds.mu.Unlock()
	}
}

func (ds *TelegramDraftStream) sendDraft(text string) {
	if len([]rune(text)) > ds.maxChars {
		ds.mu.Lock()
		ds.stopped = true
		ds.mu.Unlock()
		ds.warn(fmt.Sprintf("telegram draft stream stopped (draft length %d > %d)", len([]rune(text)), ds.maxChars))
		return
	}
	if text == ds.lastSentText {
		return
	}
	ds.lastSentText = text
	ds.lastSentAt = time.Now()

	// DY-021: 降级策略 — TS 使用非公开 sendMessageDraft API（仅客户端可见的草稿指示器）。
	// Go 使用 sendMessage + editMessageText 实现可见的实时预览（已知行为差异）。
	// 这是有意的降级方案：非公开 API 在 Bot API 中不可用，此方案为最佳替代。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if ds.draftMsgID == 0 {
		// 首次: 发送新消息作为草稿
		params := map[string]interface{}{
			"chat_id": ds.chatID,
			"text":    text + " ✏️",
		}
		for k, v := range ds.threadParam {
			params[k] = v
		}
		result, err := callTelegramAPI(ctx, ds.client, ds.token, "sendMessage", params)
		if err != nil {
			ds.mu.Lock()
			ds.stopped = true
			ds.mu.Unlock()
			ds.warn(fmt.Sprintf("telegram draft stream send failed: %v", err))
			return
		}
		if result != nil {
			ds.draftMsgID = result.MessageID
		}
	} else {
		// 后续: 编辑已有的草稿消息
		params := map[string]interface{}{
			"chat_id":    ds.chatID,
			"message_id": ds.draftMsgID,
			"text":       text + " ✏️",
		}
		_, err := callTelegramAPI(ctx, ds.client, ds.token, "editMessageText", params)
		if err != nil {
			ds.mu.Lock()
			ds.stopped = true
			ds.mu.Unlock()
			ds.warn(fmt.Sprintf("telegram draft stream edit failed: %v", err))
		}
	}
}

func (ds *TelegramDraftStream) schedule() {
	if ds.timer != nil {
		return
	}
	delay := time.Duration(ds.throttleMs)*time.Millisecond - time.Since(ds.lastSentAt)
	if delay < 0 {
		delay = 0
	}
	delay = time.Duration(math.Max(0, float64(delay)))
	ds.timer = time.AfterFunc(delay, func() {
		ds.flush()
	})
}
