package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Telegram Bot Handler 注册 — 继承自 src/telegram/bot-handlers.ts (929L)
// 核心功能：
// - 消息处理器注册（文本、媒体、转发）
// - 媒体组聚合（同一 media_group_id 的消息合并）
// - 文本分片聚合（长消息自动分割后重组）
// - 回调查询处理（model selection inline buttons）
// - 消息反应处理
// - 编辑消息处理

// TelegramUpdate 完整 Telegram 更新
type TelegramUpdate struct {
	UpdateID        int              `json:"update_id"`
	Message         *TelegramMessage `json:"message,omitempty"`
	EditedMessage   *TelegramMessage `json:"edited_message,omitempty"`
	CallbackQuery   *CallbackQuery   `json:"callback_query,omitempty"`
	MessageReaction *MessageReaction `json:"message_reaction,omitempty"`
	MyChatMember    json.RawMessage  `json:"my_chat_member,omitempty"`
	ChatMember      json.RawMessage  `json:"chat_member,omitempty"`
}

// CallbackQuery 回调查询
type CallbackQuery struct {
	ID      string           `json:"id"`
	From    *TelegramUser    `json:"from,omitempty"`
	Message *TelegramMessage `json:"message,omitempty"`
	Data    string           `json:"data"`
}

// MessageReaction 消息反应
type MessageReaction struct {
	Chat      TelegramChat    `json:"chat"`
	MessageID int             `json:"message_id"`
	User      *TelegramUser   `json:"user,omitempty"`
	OldEmoji  []ReactionEmoji `json:"old_reaction,omitempty"`
	NewEmoji  []ReactionEmoji `json:"new_reaction,omitempty"`
}

// ReactionEmoji 反应 emoji
type ReactionEmoji struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

// RegisterTelegramHandlerParams 处理器注册参数
type RegisterTelegramHandlerParams struct {
	Client         *http.Client
	Token          string
	Config         *types.OpenAcosmiConfig
	TelegramCfg    types.TelegramAccountConfig
	AccountID      string
	BotID          int64
	BotUsername    string
	MediaMaxBytes  int64
	AllowFrom      NormalizedAllowFrom
	GroupAllowFrom NormalizedAllowFrom
	TextLimit      int
	ReplyToMode    string
	ProcessMessage TelegramMessageProcessor
	Deps           *TelegramMonitorDeps
}

// 文本分片聚合常量 (TS L60-64)
const (
	textFragmentStartThreshold = 4000  // 开始缓冲的字符数阈值
	textFragmentMaxGapMs       = 1500  // 分片间最大时间间隔 (ms)
	textFragmentMaxIDGap       = 1     // 分片间最大 message_id 间隔
	textFragmentMaxParts       = 12    // 最大分片数
	textFragmentMaxTotalChars  = 50000 // 最大总字符数
)

// TelegramHandlerContext 处理器上下文（保持 handler 状态）
type TelegramHandlerContext struct {
	params             RegisterTelegramHandlerParams
	dedupe             *TelegramUpdateDedupe
	registeredCommands *TelegramRegisteredCommands
	mediaGroups        sync.Map // map[string]*mediaGroupPending
	textFragments      sync.Map // map[string]*textFragmentPending
	debouncers         sync.Map // map[string]*inboundDebouncePending
	debounceMs         int
}

// mediaGroupPending 待聚合的媒体组
type mediaGroupPending struct {
	mu       sync.Mutex
	messages []*TelegramMessage
	allMedia []TelegramMediaRef
	timer    *time.Timer
}

// textFragmentEntry 文本分片条目
type textFragmentEntry struct {
	msg        *TelegramMessage
	receivedAt time.Time
}

// textFragmentPending 待聚合的文本分片
type textFragmentPending struct {
	mu      sync.Mutex
	key     string
	entries []textFragmentEntry
	timer   *time.Timer
}

// inboundDebouncePending 入站防抖缓冲
type inboundDebouncePending struct {
	mu       sync.Mutex
	entries  []*TelegramMessage
	allMedia []TelegramMediaRef
	timer    *time.Timer
}

// RegisterTelegramHandlers 注册所有 Telegram Bot 消息处理器。
// 返回 handler context 供 monitor/webhook 分发使用。
func RegisterTelegramHandlers(ctx context.Context, params RegisterTelegramHandlerParams) *TelegramHandlerContext {
	slog.Info("telegram handlers registered",
		"account", params.AccountID,
		"botId", params.BotID,
		"botUsername", params.BotUsername,
	)

	debounceMs := autoreply.ResolveInboundDebounceMs(0, autoreply.DefaultDebounceMs)

	hctx := &TelegramHandlerContext{
		params:     params,
		dedupe:     NewTelegramUpdateDedupe(),
		debounceMs: debounceMs,
	}

	// 注册原生命令（动态发现 native/custom/plugin 命令）
	registeredCmds, regErr := RegisterTelegramNativeCommands(ctx, RegisterTelegramNativeCommandsParams{
		Client:         params.Client,
		Token:          params.Token,
		AccountID:      params.AccountID,
		Config:         params.Config,
		TelegramCfg:    params.TelegramCfg,
		AllowFrom:      params.AllowFrom,
		GroupAllowFrom: params.GroupAllowFrom,
		ReplyToMode:    params.ReplyToMode,
		TextLimit:      params.TextLimit,
		Deps:           params.Deps,
	})
	if regErr != nil {
		slog.Warn("telegram: native commands registration failed", "err", regErr)
	}
	hctx.registeredCommands = registeredCmds

	return hctx
}

// HandleUpdate 处理单个 Telegram 更新
func (h *TelegramHandlerContext) HandleUpdate(ctx context.Context, update *TelegramUpdate) {
	if update == nil {
		return
	}

	// 去重检查
	keyCtx := &TelegramUpdateKeyContext{
		UpdateID: update.UpdateID,
		Message:  update.Message,
	}
	if update.EditedMessage != nil {
		keyCtx.EditedMsg = update.EditedMessage
	}
	if update.CallbackQuery != nil {
		keyCtx.CallbackID = update.CallbackQuery.ID
		keyCtx.CallbackMsg = update.CallbackQuery.Message
	}
	key := BuildTelegramUpdateKey(keyCtx)
	if h.dedupe.IsDuplicate(key) {
		slog.Debug("telegram: duplicate update skipped", "updateId", update.UpdateID)
		return
	}

	// 路由到具体处理器
	switch {
	case update.Message != nil:
		h.handleMessage(ctx, update.Message)
	case update.EditedMessage != nil:
		h.handleEditedMessage(ctx, update.EditedMessage)
	case update.CallbackQuery != nil:
		h.handleCallbackQuery(ctx, update.CallbackQuery)
	case update.MessageReaction != nil:
		h.handleReaction(ctx, update.MessageReaction)
	}
}

// handleMessage 处理普通消息（含媒体组聚合 + 文本分片聚合 + 入站防抖）
func (h *TelegramHandlerContext) handleMessage(ctx context.Context, msg *TelegramMessage) {
	if msg == nil {
		return
	}

	// 检查是否为命令
	if isCommand(msg) {
		h.handleCommandMessage(ctx, msg)
		return
	}

	// 媒体组聚合
	if mgID := msg.MediaGroupID; mgID != "" {
		h.addToMediaGroup(ctx, mgID, msg)
		return
	}

	// 文本分片聚合 (TS L776-836)
	// Telegram 将超过 4096 字符的消息自动分割成多条
	text := msg.Text
	isCommandLike := strings.HasPrefix(strings.TrimSpace(text), "/")
	if text != "" && !isCommandLike {
		if h.tryTextFragmentAggregation(ctx, msg) {
			return // 消息已被缓冲，等待后续分片
		}
	}

	// 入站防抖 (TS L86-138)
	// 同一用户快速连发的文本消息合并处理
	if h.tryInboundDebounce(ctx, msg) {
		return // 消息已被防抖缓冲
	}

	// 普通消息处理
	h.processMessage(ctx, msg, nil)
}

// handleCommandMessage 处理命令消息（动态发现的 native/custom/plugin 命令）
func (h *TelegramHandlerContext) handleCommandMessage(ctx context.Context, msg *TelegramMessage) {
	cmd := extractCommand(msg)
	if cmd == "" {
		return
	}

	threadSpec := ResolveTelegramThreadSpec(
		msg.Chat.Type == "group" || msg.Chat.Type == "supergroup",
		msg.Chat.IsForum,
		msg.MessageThreadID,
	)
	err := HandleTelegramCommand(ctx, HandleTelegramCommandParams{
		Client:             h.params.Client,
		Token:              h.params.Token,
		ChatID:             msg.Chat.ID,
		Command:            cmd,
		Thread:             &threadSpec,
		Config:             h.params.Config,
		Deps:               h.params.Deps,
		AccountID:          h.params.AccountID,
		AllowFrom:          h.params.AllowFrom,
		GroupAllowFrom:     h.params.GroupAllowFrom,
		ReplyToMode:        h.params.ReplyToMode,
		TextLimit:          h.params.TextLimit,
		Msg:                msg,
		RegisteredCommands: h.registeredCommands,
	})
	if err != nil {
		slog.Warn("telegram: command handler failed", "cmd", cmd, "err", err)
	}
}

// handleEditedMessage 处理编辑消息
func (h *TelegramHandlerContext) handleEditedMessage(_ context.Context, msg *TelegramMessage) {
	slog.Debug("telegram: edited message received",
		"chatId", msg.Chat.ID,
		"messageId", msg.MessageID,
	)
	// 编辑消息当前仅记录日志，不重新触发 agent
}

// handleCallbackQuery 处理回调查询（model buttons 等）
func (h *TelegramHandlerContext) handleCallbackQuery(ctx context.Context, query *CallbackQuery) {
	if query == nil || query.Data == "" {
		return
	}

	slog.Debug("telegram: callback query",
		"queryId", query.ID,
		"data", query.Data,
	)

	// 应答回调（避免 Telegram 客户端旋转 loading）
	_, _ = callTelegramAPI(ctx, h.params.Client, h.params.Token, "answerCallbackQuery",
		map[string]interface{}{"callback_query_id": query.ID})

	// 处理 model selection 回调
	if isModelSelectionCallback(query.Data) {
		h.handleModelCallback(ctx, query)
	}
}

// handleModelCallback 处理模型选择回调
func (h *TelegramHandlerContext) handleModelCallback(ctx context.Context, query *CallbackQuery) {
	modelRef := extractModelFromCallback(query.Data)
	if modelRef == "" || h.params.Deps == nil || h.params.Deps.SwitchModel == nil {
		return
	}

	chatID := int64(0)
	if query.Message != nil {
		chatID = query.Message.Chat.ID
	}

	sessionKey := ""
	if chatID != 0 {
		sessionKey = resolveSessionKeyForChat(h.params.AccountID, chatID)
	}

	storePath := ""
	if h.params.Deps.ResolveStorePath != nil {
		storePath = h.params.Deps.ResolveStorePath("")
	}

	if err := h.params.Deps.SwitchModel(ctx, sessionKey, storePath, modelRef); err != nil {
		slog.Warn("telegram: model switch failed", "err", err, "model", modelRef)
		return
	}

	// 更新消息提示
	if query.Message != nil {
		_, _ = callTelegramAPI(ctx, h.params.Client, h.params.Token, "editMessageText",
			map[string]interface{}{
				"chat_id":    chatID,
				"message_id": query.Message.MessageID,
				"text":       "✅ Model switched to: " + modelRef,
			})
	}
}

// handleReaction 处理消息反应
func (h *TelegramHandlerContext) handleReaction(_ context.Context, reaction *MessageReaction) {
	if h.params.Deps == nil || h.params.Deps.EnqueueSystemEvent == nil {
		return
	}

	var addedEmoji string
	if len(reaction.NewEmoji) > len(reaction.OldEmoji) {
		for _, ne := range reaction.NewEmoji {
			found := false
			for _, oe := range reaction.OldEmoji {
				if ne.Emoji == oe.Emoji {
					found = true
					break
				}
			}
			if !found {
				addedEmoji = ne.Emoji
				break
			}
		}
	}

	senderID := ""
	if reaction.User != nil {
		senderID = formatUserID(reaction.User.ID)
	}

	h.params.Deps.EnqueueSystemEvent("reaction", map[string]interface{}{
		"channel":   "telegram",
		"chatId":    reaction.Chat.ID,
		"messageId": reaction.MessageID,
		"senderId":  senderID,
		"emoji":     addedEmoji,
	})
}

// addToMediaGroup 添加消息到媒体组并设置聚合定时器
func (h *TelegramHandlerContext) addToMediaGroup(ctx context.Context, groupID string, msg *TelegramMessage) {
	val, _ := h.mediaGroups.LoadOrStore(groupID, &mediaGroupPending{})
	pending := val.(*mediaGroupPending)

	media := resolveMediaFromMessage(msg, h.params.MediaMaxBytes, h.params.Token)

	pending.mu.Lock()
	pending.messages = append(pending.messages, msg)
	if media != nil {
		pending.allMedia = append(pending.allMedia, *media)
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	pending.timer = time.AfterFunc(time.Duration(MediaGroupTimeoutMs)*time.Millisecond, func() {
		h.flushMediaGroup(ctx, groupID)
	})
	pending.mu.Unlock()
}

// flushMediaGroup 聚合完成后处理媒体组
func (h *TelegramHandlerContext) flushMediaGroup(ctx context.Context, groupID string) {
	val, ok := h.mediaGroups.LoadAndDelete(groupID)
	if !ok {
		return
	}
	pending := val.(*mediaGroupPending)
	pending.mu.Lock()
	defer pending.mu.Unlock()

	if len(pending.messages) == 0 {
		return
	}

	// DY-013: 按 message_id 排序，确保处理顺序一致（对齐 TS 媒体组排序逻辑）
	sort.Slice(pending.messages, func(i, j int) bool {
		return pending.messages[i].MessageID < pending.messages[j].MessageID
	})

	// 审计修复: 优先选有 caption/text 的消息作为主消息（对齐 TS captionMsg ?? messages[0]）
	primary := pending.messages[0]
	for _, m := range pending.messages {
		if m.Caption != "" || m.Text != "" {
			primary = m
			break
		}
	}
	h.processMessage(ctx, primary, pending.allMedia)
}

// --- 文本分片聚合 ---

// tryTextFragmentAggregation 尝试文本分片聚合。
// 返回 true 表示消息已被缓冲等待更多分片，调用方应跳过正常处理。
func (h *TelegramHandlerContext) tryTextFragmentAggregation(ctx context.Context, msg *TelegramMessage) bool {
	text := msg.Text
	if text == "" {
		return false
	}

	chatID := msg.Chat.ID
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"
	threadSpec := ResolveTelegramThreadSpec(isGroup, msg.Chat.IsForum, msg.MessageThreadID)
	resolvedThreadID := "main"
	if threadSpec.Scope == "forum" && threadSpec.ID != nil {
		resolvedThreadID = strconv.Itoa(*threadSpec.ID)
	}
	senderID := "unknown"
	if msg.From != nil {
		senderID = strconv.FormatInt(msg.From.ID, 10)
	}
	key := fmt.Sprintf("text:%d:%s:%s", chatID, resolvedThreadID, senderID)
	now := time.Now()

	// 检查是否有已存在的分片缓冲
	if val, ok := h.textFragments.Load(key); ok {
		pending := val.(*textFragmentPending)
		pending.mu.Lock()

		if len(pending.entries) > 0 {
			last := pending.entries[len(pending.entries)-1]
			idGap := msg.MessageID - last.msg.MessageID
			timeGapMs := now.Sub(last.receivedAt).Milliseconds()

			canAppend := idGap > 0 &&
				idGap <= textFragmentMaxIDGap &&
				timeGapMs >= 0 &&
				timeGapMs <= textFragmentMaxGapMs

			if canAppend {
				totalChars := 0
				for _, e := range pending.entries {
					totalChars += len([]rune(e.msg.Text))
				}
				nextTotal := totalChars + len([]rune(text))

				if len(pending.entries)+1 <= textFragmentMaxParts && nextTotal <= textFragmentMaxTotalChars {
					pending.entries = append(pending.entries, textFragmentEntry{msg: msg, receivedAt: now})
					if pending.timer != nil {
						pending.timer.Stop()
					}
					pending.timer = time.AfterFunc(time.Duration(textFragmentMaxGapMs)*time.Millisecond, func() {
						h.flushTextFragments(ctx, key)
					})
					pending.mu.Unlock()
					return true
				}
			}

			// 不可追加：先刷新旧缓冲
			if pending.timer != nil {
				pending.timer.Stop()
			}
			pending.mu.Unlock()
			h.flushTextFragments(ctx, key)
		} else {
			pending.mu.Unlock()
		}
	}

	// 判断是否应开始新的分片缓冲
	if len([]rune(text)) >= textFragmentStartThreshold {
		pending := &textFragmentPending{
			key:     key,
			entries: []textFragmentEntry{{msg: msg, receivedAt: now}},
		}
		pending.timer = time.AfterFunc(time.Duration(textFragmentMaxGapMs)*time.Millisecond, func() {
			h.flushTextFragments(ctx, key)
		})
		h.textFragments.Store(key, pending)
		return true
	}

	return false
}

// flushTextFragments 刷新文本分片缓冲
func (h *TelegramHandlerContext) flushTextFragments(ctx context.Context, key string) {
	val, ok := h.textFragments.LoadAndDelete(key)
	if !ok {
		return
	}
	pending := val.(*textFragmentPending)
	pending.mu.Lock()
	if pending.timer != nil {
		pending.timer.Stop()
	}
	entries := pending.entries
	pending.mu.Unlock()

	if len(entries) == 0 {
		return
	}

	// 合并所有分片文本
	var parts []string
	for _, e := range entries {
		if e.msg.Text != "" {
			parts = append(parts, e.msg.Text)
		}
	}
	combinedText := strings.Join(parts, "")
	if strings.TrimSpace(combinedText) == "" {
		return
	}

	// 使用第一条消息作为基础，最后一条的 ID 作为 override
	firstMsg := entries[0].msg
	lastMsg := entries[len(entries)-1].msg

	// 创建合成消息
	syntheticMsg := *firstMsg
	syntheticMsg.Text = combinedText
	syntheticMsg.Caption = ""
	syntheticMsg.Entities = nil
	syntheticMsg.CaptionEntities = nil
	if lastMsg.Date > firstMsg.Date {
		syntheticMsg.Date = lastMsg.Date
	}

	// 读取 storeAllowFrom
	var storeAllowFrom []string
	if h.params.Deps != nil && h.params.Deps.ReadAllowFromStore != nil {
		if af, err := h.params.Deps.ReadAllowFromStore("telegram"); err == nil {
			storeAllowFrom = af
		}
	}

	if h.params.ProcessMessage != nil {
		opts := &TelegramMessageContextOptions{
			MessageIDOverride: strconv.Itoa(lastMsg.MessageID),
		}
		if err := h.params.ProcessMessage(&syntheticMsg, nil, storeAllowFrom, opts); err != nil {
			slog.Warn("telegram: text fragment processing failed",
				"chatId", firstMsg.Chat.ID, "err", err)
		}
	}
}

// --- 入站防抖 ---

// tryInboundDebounce 尝试入站防抖。
// 返回 true 表示消息已被防抖缓冲。
func (h *TelegramHandlerContext) tryInboundDebounce(ctx context.Context, msg *TelegramMessage) bool {
	if h.debounceMs <= 0 {
		return false
	}

	// 有媒体的消息不防抖
	if hasMediaContent(msg) {
		return false
	}

	// 文本为空不防抖
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	if strings.TrimSpace(text) == "" {
		return false
	}

	// 控制命令不防抖
	if autoreply.HasControlCommand(text) {
		return false
	}

	// 构建防抖 key
	chatID := msg.Chat.ID
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"
	threadSpec := ResolveTelegramThreadSpec(isGroup, msg.Chat.IsForum, msg.MessageThreadID)
	resolvedThreadID := "main"
	if threadSpec.Scope == "forum" && threadSpec.ID != nil {
		resolvedThreadID = strconv.Itoa(*threadSpec.ID)
	}
	senderID := "unknown"
	if msg.From != nil {
		senderID = strconv.FormatInt(msg.From.ID, 10)
	}
	debounceKey := fmt.Sprintf("telegram:%s:%d:%s:%s",
		h.params.AccountID, chatID, resolvedThreadID, senderID)

	val, loaded := h.debouncers.LoadOrStore(debounceKey, &inboundDebouncePending{
		entries: []*TelegramMessage{msg},
	})
	pending := val.(*inboundDebouncePending)

	if loaded {
		// 已有缓冲：追加并重置定时器
		pending.mu.Lock()
		pending.entries = append(pending.entries, msg)
		if pending.timer != nil {
			pending.timer.Stop()
		}
		pending.timer = time.AfterFunc(time.Duration(h.debounceMs)*time.Millisecond, func() {
			h.flushDebounce(ctx, debounceKey)
		})
		pending.mu.Unlock()
		return true
	}

	// 新缓冲：设置定时器
	pending.mu.Lock()
	pending.timer = time.AfterFunc(time.Duration(h.debounceMs)*time.Millisecond, func() {
		h.flushDebounce(ctx, debounceKey)
	})
	pending.mu.Unlock()
	return true
}

// flushDebounce 刷新防抖缓冲
func (h *TelegramHandlerContext) flushDebounce(ctx context.Context, key string) {
	val, ok := h.debouncers.LoadAndDelete(key)
	if !ok {
		return
	}
	pending := val.(*inboundDebouncePending)
	pending.mu.Lock()
	if pending.timer != nil {
		pending.timer.Stop()
	}
	entries := pending.entries
	pending.mu.Unlock()

	if len(entries) == 0 {
		return
	}

	// 单条消息直接处理
	if len(entries) == 1 {
		h.processMessage(ctx, entries[0], nil)
		return
	}

	// 多条消息合并
	var textParts []string
	for _, e := range entries {
		text := e.Text
		if text == "" {
			text = e.Caption
		}
		if text != "" {
			textParts = append(textParts, text)
		}
	}
	combinedText := strings.Join(textParts, "\n")
	if strings.TrimSpace(combinedText) == "" {
		return
	}

	firstMsg := entries[0]
	lastMsg := entries[len(entries)-1]

	// 创建合成消息
	syntheticMsg := *firstMsg
	syntheticMsg.Text = combinedText
	syntheticMsg.Caption = ""
	syntheticMsg.Entities = nil
	syntheticMsg.CaptionEntities = nil
	if lastMsg.Date > firstMsg.Date {
		syntheticMsg.Date = lastMsg.Date
	}

	// 读取 storeAllowFrom
	var storeAllowFrom []string
	if h.params.Deps != nil && h.params.Deps.ReadAllowFromStore != nil {
		if af, err := h.params.Deps.ReadAllowFromStore("telegram"); err == nil {
			storeAllowFrom = af
		}
	}

	if h.params.ProcessMessage != nil {
		opts := &TelegramMessageContextOptions{
			MessageIDOverride: strconv.Itoa(lastMsg.MessageID),
		}
		if err := h.params.ProcessMessage(&syntheticMsg, nil, storeAllowFrom, opts); err != nil {
			slog.Warn("telegram: debounce processing failed",
				"chatId", firstMsg.Chat.ID, "err", err)
		}
	}
}

// hasMediaContent 检查消息是否包含媒体内容
func hasMediaContent(msg *TelegramMessage) bool {
	return len(msg.Photo) > 0 || msg.Video != nil || msg.Audio != nil ||
		msg.Voice != nil || msg.Document != nil || msg.Sticker != nil
}

// processMessage 处理消息（普通或聚合后的媒体组）
func (h *TelegramHandlerContext) processMessage(ctx context.Context, msg *TelegramMessage, groupMedia []TelegramMediaRef) {
	allMedia := groupMedia
	if len(allMedia) == 0 {
		if media := resolveMediaFromMessage(msg, h.params.MediaMaxBytes, h.params.Token); media != nil {
			allMedia = []TelegramMediaRef{*media}
		}
	}

	// 读取动态 allowFrom
	var storeAllowFrom []string
	if h.params.Deps != nil && h.params.Deps.ReadAllowFromStore != nil {
		if af, err := h.params.Deps.ReadAllowFromStore("telegram"); err == nil {
			storeAllowFrom = af
		}
	}

	if h.params.ProcessMessage != nil {
		if err := h.params.ProcessMessage(msg, allMedia, storeAllowFrom, nil); err != nil {
			slog.Warn("telegram: message processing failed",
				"chatId", msg.Chat.ID,
				"err", err,
			)
		}
	}
}

// resolveMediaFromMessage 从消息提取媒体引用
func resolveMediaFromMessage(msg *TelegramMessage, maxBytes int64, token string) *TelegramMediaRef {
	return ResolveMedia(msg, maxBytes, token)
}

// --- 辅助函数 ---

func isCommand(msg *TelegramMessage) bool {
	if msg.Text == "" {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type == "bot_command" && e.Offset == 0 {
			return true
		}
	}
	return false
}

func extractCommand(msg *TelegramMessage) string {
	for _, e := range msg.Entities {
		if e.Type == "bot_command" && e.Offset == 0 {
			runes := []rune(msg.Text)
			end := e.Offset + e.Length
			if end > len(runes) {
				end = len(runes)
			}
			return string(runes[e.Offset:end])
		}
	}
	return ""
}

func isModelSelectionCallback(data string) bool {
	return len(data) > 6 && data[:6] == "model:"
}

func extractModelFromCallback(data string) string {
	if len(data) > 6 && data[:6] == "model:" {
		return data[6:]
	}
	return ""
}

func resolveSessionKeyForChat(accountID string, chatID int64) string {
	return "telegram:" + accountID + ":" + strconv.FormatInt(chatID, 10)
}

func formatChatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func formatUserID(id int64) string {
	return strconv.FormatInt(id, 10)
}
