package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// Telegram Bot 核心 — 继承自 src/telegram/bot.ts (499L)
// 替代 grammy Bot 实例，使用原生 HTTP API

// ---------------------------------------------------------------------------
// TelegramBotOptions — 完整创建选项，对齐 TS TelegramBotOptions
// ---------------------------------------------------------------------------

// UpdateOffsetCallbacks 持久化回调（对齐 TS opts.updateOffset）
type UpdateOffsetCallbacks struct {
	// LastUpdateID 启动时的初始 offset（nil 表示从头开始）
	LastUpdateID *int
	// OnUpdateID 每次 offset 推进时的持久化回调
	OnUpdateID func(updateID int)
}

// TelegramBotOptions Bot 创建选项 — 对齐 TS TelegramBotOptions
type TelegramBotOptions struct {
	Token          string
	AccountID      string
	Config         *types.OpenAcosmiConfig
	RequireMention *bool
	AllowFrom      []string
	GroupAllowFrom []string
	MediaMaxMB     *int
	ReplyToMode    string
	UpdateOffset   *UpdateOffsetCallbacks
}

// ---------------------------------------------------------------------------
// TelegramBot — Bot 实例
// ---------------------------------------------------------------------------

// TelegramBot Bot 实例
type TelegramBot struct {
	Token       string
	AccountID   string
	Config      *types.OpenAcosmiConfig
	Options     TelegramBotOptions
	Client      *http.Client
	BotID       int64
	BotUsername string
	Dedupe      *TelegramUpdateDedupe
	Handlers    *TelegramHandlerContext
	Seq         *sequentializer

	cancelFunc context.CancelFunc
	stopOnce   sync.Once
	stopped    chan struct{}
}

// CreateTelegramBot 创建 Telegram Bot 实例
func CreateTelegramBot(ctx context.Context, opts TelegramBotOptions) (*TelegramBot, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	cfg := opts.Config
	if cfg == nil {
		cfg = &types.OpenAcosmiConfig{}
	}

	account := ResolveTelegramAccount(cfg, opts.AccountID)
	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}

	bot := &TelegramBot{
		Token:     opts.Token,
		AccountID: account.AccountID,
		Config:    cfg,
		Options:   opts,
		Client:    client,
		Dedupe:    NewTelegramUpdateDedupe(),
		Seq:       newSequentializer(),
		stopped:   make(chan struct{}),
	}

	// getMe 获取 bot 信息
	if err := bot.fetchBotInfo(ctx); err != nil {
		return nil, fmt.Errorf("getMe: %w", err)
	}

	slog.Info("telegram bot created",
		"botId", bot.BotID,
		"username", bot.BotUsername,
		"account", bot.AccountID,
	)

	return bot, nil
}

func (b *TelegramBot) fetchBotInfo(ctx context.Context) error {
	body, status, err := doProbeRequest(ctx, b.Client, fmt.Sprintf("%s/bot%s/getMe", TelegramAPIBaseURL, b.Token), 10000)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("getMe: HTTP %d", status)
	}

	var resp struct {
		OK     bool `json:"ok"`
		Result *struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("getMe decode: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("getMe: %s", resp.Description)
	}
	if resp.Result != nil {
		b.BotID = resp.Result.ID
		b.BotUsername = resp.Result.Username
	}
	return nil
}

// ShouldSkipUpdate 检查更新是否应跳过（去重）
func (b *TelegramBot) ShouldSkipUpdate(ctx *TelegramUpdateKeyContext) bool {
	key := BuildTelegramUpdateKey(ctx)
	if key == "" {
		return false
	}
	return b.Dedupe.IsDuplicate(key)
}

// ---------------------------------------------------------------------------
// Start — 启动 Bot（长轮询或 webhook）
// 从 config 解析所有运行时参数，对齐 TS createTelegramBot 的 config resolution。
// ---------------------------------------------------------------------------

func (b *TelegramBot) Start(ctx context.Context) error {
	cfg := b.Config
	opts := b.Options
	account := ResolveTelegramAccount(cfg, b.AccountID)
	telegramCfg := account.Config

	// --- config resolution (对齐 TS bot.ts L226-L263) ---

	// historyLimit
	defaultHistoryLimit := 100
	historyLimit := resolveIntDefault(telegramCfg.HistoryLimit, resolveGroupChatHistoryLimit(cfg), &defaultHistoryLimit)
	if historyLimit < 0 {
		historyLimit = 0
	}

	// textLimit
	defaultTextLimit := 4096
	textLimit := resolveIntDefault(telegramCfg.TextChunkLimit, &defaultTextLimit)

	// dmPolicy
	dmPolicy := string(telegramCfg.DmPolicy)
	if dmPolicy == "" {
		dmPolicy = "pairing"
	}

	// allowFrom: opts override > telegramCfg
	allowFromRaw := opts.AllowFrom
	if len(allowFromRaw) == 0 {
		allowFromRaw = interfaceSliceToStringSlice(telegramCfg.AllowFrom)
	}
	allowFrom := NormalizeAllowFrom(allowFromRaw)

	// groupAllowFrom: opts override > telegramCfg.GroupAllowFrom > telegramCfg.AllowFrom > opts.AllowFrom
	groupAllowFromRaw := opts.GroupAllowFrom
	if len(groupAllowFromRaw) == 0 {
		groupAllowFromRaw = interfaceSliceToStringSlice(telegramCfg.GroupAllowFrom)
	}
	if len(groupAllowFromRaw) == 0 && len(telegramCfg.AllowFrom) > 0 {
		groupAllowFromRaw = interfaceSliceToStringSlice(telegramCfg.AllowFrom)
	}
	if len(groupAllowFromRaw) == 0 && len(opts.AllowFrom) > 0 {
		groupAllowFromRaw = opts.AllowFrom
	}
	groupAllowFrom := NormalizeAllowFrom(groupAllowFromRaw)

	// replyToMode
	replyToMode := opts.ReplyToMode
	if replyToMode == "" {
		replyToMode = string(telegramCfg.ReplyToMode)
	}
	if replyToMode == "" {
		replyToMode = "first"
	}

	// ackReactionScope
	ackReactionScope := ""
	if cfg.Messages != nil && cfg.Messages.AckReactionScope != "" {
		ackReactionScope = string(cfg.Messages.AckReactionScope)
	}
	if ackReactionScope == "" {
		ackReactionScope = "group-mentions"
	}

	// mediaMaxBytes
	mediaMaxMB := 5 // default 5 MB (对齐 TS mediaMaxMb ?? 5)
	if opts.MediaMaxMB != nil {
		mediaMaxMB = *opts.MediaMaxMB
	} else if telegramCfg.MediaMaxMB != nil {
		mediaMaxMB = *telegramCfg.MediaMaxMB
	}
	mediaMaxBytes := int64(mediaMaxMB) * 1024 * 1024

	// streamMode
	streamMode := ResolveTelegramStreamMode(telegramCfg.StreamMode)

	// --- create a cancellable context for the polling loop ---
	pollCtx, cancel := context.WithCancel(ctx)
	b.cancelFunc = cancel

	// --- register handlers ---
	handlers := RegisterTelegramHandlers(pollCtx, RegisterTelegramHandlerParams{
		Client:         b.Client,
		Token:          b.Token,
		Config:         cfg,
		TelegramCfg:    telegramCfg,
		AccountID:      b.AccountID,
		BotID:          b.BotID,
		BotUsername:    b.BotUsername,
		MediaMaxBytes:  mediaMaxBytes,
		AllowFrom:      allowFrom,
		GroupAllowFrom: groupAllowFrom,
		TextLimit:      textLimit,
		ReplyToMode:    replyToMode,
		ProcessMessage: CreateTelegramMessageProcessor(TelegramMessageProcessorDeps{
			BotID:          b.BotID,
			BotUsername:    b.BotUsername,
			Token:          b.Token,
			AccountID:      b.AccountID,
			AllowFrom:      allowFrom,
			GroupAllowFrom: groupAllowFrom,
			TextLimit:      textLimit,
			StreamMode:     string(streamMode),
			ReplyToMode:    replyToMode,
			Config:         cfg,
			TelegramCfg:    telegramCfg,
			Client:         b.Client,
			// DY-012: 传递缺失的配置参数和解析回调
			HistoryLimit:               historyLimit,
			DMPolicy:                   dmPolicy,
			AckReactionScope:           ackReactionScope,
			GroupHistories:             make(map[string][]TelegramHistoryEntry),
			ResolveBotTopicsEnabled:    ResolveBotTopicsEnabledFunc(b.Client, b.Token),
			ResolveGroupActivation:     ResolveGroupActivationFunc(cfg, b.AccountID, nil),
			ResolveGroupRequireMention: ResolveGroupRequireMentionFunc(cfg, b.AccountID, opts.RequireMention),
			ResolveTelegramGroupConfig: ResolveTelegramGroupConfigFunc(telegramCfg),
		}),
	})
	b.Handlers = handlers

	slog.Info("telegram bot starting",
		"account", b.AccountID,
		"historyLimit", historyLimit,
		"textLimit", textLimit,
		"dmPolicy", dmPolicy,
		"replyToMode", replyToMode,
		"ackReactionScope", ackReactionScope,
		"mediaMaxMB", mediaMaxMB,
		"streamMode", streamMode,
	)

	// --- start long polling ---
	return MonitorTelegramProvider(pollCtx, MonitorConfig{
		Token:     b.Token,
		AccountID: b.AccountID,
		Config:    cfg,
		Handlers:  handlers,
	})
}

// Stop 停止 Bot — 取消轮询上下文并等待优雅关闭
func (b *TelegramBot) Stop() {
	b.stopOnce.Do(func() {
		slog.Info("telegram bot stopping", "account", b.AccountID)
		if b.cancelFunc != nil {
			b.cancelFunc()
		}
		close(b.stopped)
		slog.Info("telegram bot stopped", "account", b.AccountID)
	})
}

// Stopped 返回一个在 Bot 停止后关闭的 channel
func (b *TelegramBot) Stopped() <-chan struct{} {
	return b.stopped
}

// ---------------------------------------------------------------------------
// HandleUpdateWithRecovery — 带 panic recovery + sequentialization 的 update 处理入口
// 由 monitor 调用，替代直接调用 handlers.HandleUpdate。
// ---------------------------------------------------------------------------

// HandleUpdateWithRecovery 以 panic-safe + 串行化方式处理一个 Telegram 更新。
func (b *TelegramBot) HandleUpdateWithRecovery(ctx context.Context, update *TelegramUpdate) {
	if update == nil || b.Handlers == nil {
		return
	}

	// raw update debug logging
	logRawUpdate(update)

	seqKey := getTelegramSequentialKey(update, b.BotUsername)

	b.Seq.run(seqKey, func() {
		recoverMiddleware("HandleUpdate", func() {
			b.Handlers.HandleUpdate(ctx, update)
		})
	})
}

// ---------------------------------------------------------------------------
// getTelegramSequentialKey — 对齐 TS getTelegramSequentialKey
// 返回一个字符串 key，保证相同 key 的更新串行处理。
// ---------------------------------------------------------------------------

func getTelegramSequentialKey(update *TelegramUpdate, botUsername string) string {
	if update == nil {
		return "telegram:unknown"
	}

	// Handle reaction updates
	if reaction := update.MessageReaction; reaction != nil {
		if reaction.Chat.ID != 0 {
			return "telegram:" + strconv.FormatInt(reaction.Chat.ID, 10) + ":reaction"
		}
		return "telegram:unknown"
	}

	// Resolve the message from multiple update fields
	msg := update.Message
	if msg == nil {
		msg = update.EditedMessage
	}
	if msg == nil && update.CallbackQuery != nil {
		msg = update.CallbackQuery.Message
	}

	chatID := int64(0)
	if msg != nil {
		chatID = msg.Chat.ID
	}

	// Check if this is a control command
	rawText := ""
	if msg != nil {
		rawText = msg.Text
		if rawText == "" {
			rawText = msg.Caption
		}
	}
	if rawText != "" && isControlCommand(rawText, botUsername) {
		if chatID != 0 {
			return "telegram:" + strconv.FormatInt(chatID, 10) + ":control"
		}
		return "telegram:control"
	}

	// Forum topic threading
	if msg != nil && chatID != 0 {
		isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"
		if isGroup {
			threadID := ResolveTelegramForumThreadID(msg.Chat.IsForum, msg.MessageThreadID)
			if threadID != nil {
				return "telegram:" + strconv.FormatInt(chatID, 10) + ":topic:" + strconv.Itoa(*threadID)
			}
		} else if msg.MessageThreadID != nil {
			// DM with thread
			return "telegram:" + strconv.FormatInt(chatID, 10) + ":topic:" + strconv.Itoa(*msg.MessageThreadID)
		}
		return "telegram:" + strconv.FormatInt(chatID, 10)
	}

	if chatID != 0 {
		return "telegram:" + strconv.FormatInt(chatID, 10)
	}

	return "telegram:unknown"
}

// isControlCommand 检查消息文本是否为控制命令（以 / 开头的命令消息）。
// 使用 autoreply.IsControlCommandMessage 作为核心检测。
func isControlCommand(text, botUsername string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed[0] != '/' {
		return false
	}
	// 移除 @bot 后缀后检测
	cleaned := trimmed
	if botUsername != "" {
		cleaned = strings.Replace(trimmed, "@"+botUsername, "", 1)
	}
	return autoreply.IsControlCommandMessage(cleaned)
}

// ---------------------------------------------------------------------------
// sequentializer — 按 key 串行化执行
// 对齐 TS @grammyjs/runner sequentialize()
// ---------------------------------------------------------------------------

type sequentializer struct {
	mu    sync.Mutex
	lanes map[string]chan func()
}

func newSequentializer() *sequentializer {
	return &sequentializer{
		lanes: make(map[string]chan func()),
	}
}

// run 将 fn 提交到 key 对应的串行化通道。
// 相同 key 的回调保证 FIFO 串行执行；不同 key 并发。
func (s *sequentializer) run(key string, fn func()) {
	s.mu.Lock()
	ch, ok := s.lanes[key]
	if !ok {
		ch = make(chan func(), 256)
		s.lanes[key] = ch
		go s.drain(key, ch)
	}
	s.mu.Unlock()

	ch <- fn
}

func (s *sequentializer) drain(key string, ch chan func()) {
	const idleTimeout = 5 * time.Minute
	for {
		select {
		case fn, ok := <-ch:
			if !ok {
				// channel 被关闭
				goto cleanup
			}
			fn()
		case <-time.After(idleTimeout):
			// 空闲超时，退出 goroutine 避免泄漏
			goto cleanup
		}
	}
cleanup:
	s.mu.Lock()
	// 仅当 channel 为空且仍是同一个 channel 时清理 (防止 race)
	if current, ok := s.lanes[key]; ok && current == ch && len(ch) == 0 {
		delete(s.lanes, key)
	}
	s.mu.Unlock()
}

// ---------------------------------------------------------------------------
// recoverMiddleware — panic recovery wrapper
// 对齐 TS bot.catch(err => ...)
// ---------------------------------------------------------------------------

// recoverMiddleware 在一个新 goroutine 中安全地执行 fn，捕获 panic 并记录日志。
// label 用于标识调用来源。
func recoverMiddleware(label string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			slog.Error("telegram bot panic recovered",
				"label", label,
				"panic", fmt.Sprintf("%v", r),
				"stack", string(stack),
			)
		}
	}()
	fn()
}

// ---------------------------------------------------------------------------
// logRawUpdate — raw update debug logging
// 对齐 TS rawUpdateLogger / stringifyUpdate
// ---------------------------------------------------------------------------

const (
	maxRawUpdateChars  = 8000
	maxRawUpdateString = 500
	maxRawUpdateArray  = 20
)

func logRawUpdate(update *TelegramUpdate) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	raw, err := marshalTruncated(update)
	if err != nil {
		slog.Debug("telegram update log failed", "err", err)
		return
	}

	preview := raw
	if len(preview) > maxRawUpdateChars {
		preview = preview[:maxRawUpdateChars] + "..."
	}

	slog.Debug("telegram update", "raw", preview)
}

// marshalTruncated 序列化更新并截断过长的字符串字段和数组。
func marshalTruncated(v interface{}) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	// 对于小 payload 无需截断
	if len(raw) <= maxRawUpdateChars {
		return string(raw), nil
	}

	// 重新解析为 generic structure 进行截断
	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return string(raw), nil
	}

	truncated := truncateValue(generic)
	result, err := json.Marshal(truncated)
	if err != nil {
		return string(raw), nil
	}
	return string(result), nil
}

func truncateValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		if len(val) > maxRawUpdateString {
			return val[:maxRawUpdateString] + "..."
		}
		return val
	case []interface{}:
		if len(val) > maxRawUpdateArray {
			result := make([]interface{}, maxRawUpdateArray+1)
			for i := 0; i < maxRawUpdateArray; i++ {
				result[i] = truncateValue(val[i])
			}
			result[maxRawUpdateArray] = fmt.Sprintf("...(%d more)", len(val)-maxRawUpdateArray)
			return result
		}
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = truncateValue(item)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, item := range val {
			result[k] = truncateValue(item)
		}
		return result
	default:
		return val
	}
}

// ---------------------------------------------------------------------------
// Config resolution helpers
// ---------------------------------------------------------------------------

// resolveIntDefault 返回第一个非 nil 的 *int 值，否则返回 fallback。
func resolveIntDefault(ptrs ...*int) int {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return 0
}

// resolveGroupChatHistoryLimit 从全局 config 解析群聊历史限制
func resolveGroupChatHistoryLimit(cfg *types.OpenAcosmiConfig) *int {
	if cfg == nil || cfg.Messages == nil || cfg.Messages.GroupChat == nil {
		return nil
	}
	return cfg.Messages.GroupChat.HistoryLimit
}

// interfaceSliceToStringSlice 将 []interface{} 转换为 []string
func interfaceSliceToStringSlice(src []interface{}) []string {
	if len(src) == 0 {
		return nil
	}
	result := make([]string, 0, len(src))
	for _, v := range src {
		switch val := v.(type) {
		case string:
			result = append(result, val)
		case float64:
			result = append(result, strconv.FormatFloat(val, 'f', -1, 64))
		case int:
			result = append(result, strconv.Itoa(val))
		case int64:
			result = append(result, strconv.FormatInt(val, 10))
		default:
			result = append(result, fmt.Sprintf("%v", v))
		}
	}
	return result
}
