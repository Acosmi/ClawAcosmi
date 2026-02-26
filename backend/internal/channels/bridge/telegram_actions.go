package bridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Telegram action 路由 — 继承自 src/agents/tools/telegram-actions.ts (325L)

// TelegramButton inline keyboard 按钮
type TelegramButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// TelegramBridgeSendOpts 发送选项（bridge 层面）
type TelegramBridgeSendOpts struct {
	MediaURL         string
	Buttons          [][]TelegramButton
	ReplyToMessageID *int
	MessageThreadID  *int
	QuoteText        string
	AsVoice          bool
	Silent           bool
}

// TelegramActionDeps Telegram bridge action 依赖接口。
// 调用方注入 channel API 实现以避免循环导入。
type TelegramActionDeps interface {
	// SendMessage 发送消息（文本或媒体）。
	SendMessage(ctx context.Context, to, text string, opts TelegramBridgeSendOpts) (messageID, chatID string, err error)
	// EditMessage 编辑消息文本。
	EditMessage(ctx context.Context, chatID string, messageID int, text string, buttons [][]TelegramButton) error
	// DeleteMessage 删除消息。
	DeleteMessage(ctx context.Context, chatID string, messageID int) error
	// ReactMessage 发送/移除反应 emoji。
	ReactMessage(ctx context.Context, chatID string, messageID int, emoji string, remove bool) error
	// SendSticker 发送贴纸。
	SendSticker(ctx context.Context, to, fileID string) (msgID, chatID string, err error)
	// SearchStickers 搜索贴纸缓存。
	SearchStickers(query string, limit int) []StickerResult
	// GetStickerCacheStats 获取贴纸缓存状态。
	GetStickerCacheStats() map[string]interface{}
	// CallAPI 直接调用 Telegram Bot API 方法（用于 forward/copy/poll/pin/admin 等）。
	CallAPI(ctx context.Context, method string, params map[string]interface{}) (interface{}, error)
}

// StickerResult 贴纸搜索结果
type StickerResult struct {
	FileID      string `json:"fileId"`
	Emoji       string `json:"emoji"`
	Description string `json:"description"`
	SetName     string `json:"setName"`
}

// InlineButtonsScopeChecker 可选接口，检查 inline buttons 是否允许发送到目标。
// 对齐 TS: resolveTelegramInlineButtonsScope() + resolveTelegramTargetChatType() 检查。
// 实现方在 deps 层注入；bridge 层通过 interface assertion 调用。
type InlineButtonsScopeChecker interface {
	CheckInlineButtonsAllowed(to string) error
}

// ReactionLevelChecker 可选接口，检查 agent reactions 是否允许。
// 对齐 TS: resolveTelegramReactionLevel() 检查。
type ReactionLevelChecker interface {
	CheckReactionAllowed() error
}

// ReadTelegramButtons 从参数中解析 inline keyboard 按钮矩阵。
// 对齐 TS: readTelegramButtons 严格验证非数组行/非对象按钮/缺失字段/callback_data 长度。
func ReadTelegramButtons(params map[string]interface{}) ([][]TelegramButton, error) {
	raw, ok := params["buttons"]
	if !ok || raw == nil {
		return nil, nil
	}
	rows, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("buttons must be an array of button rows")
	}
	var result [][]TelegramButton
	for ri, row := range rows {
		rowArr, ok := row.([]interface{})
		if !ok {
			return nil, fmt.Errorf("buttons[%d] must be an array", ri)
		}
		var buttonRow []TelegramButton
		for bi, btn := range rowArr {
			m, ok := btn.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("buttons[%d][%d] must be an object", ri, bi)
			}
			text, _ := m["text"].(string)
			text = strings.TrimSpace(text)
			cb, _ := m["callback_data"].(string)
			cb = strings.TrimSpace(cb)
			if text == "" || cb == "" {
				return nil, fmt.Errorf("buttons[%d][%d] requires text and callback_data", ri, bi)
			}
			if len(cb) > 64 {
				return nil, fmt.Errorf("buttons[%d][%d] callback_data too long (max 64 chars)", ri, bi)
			}
			buttonRow = append(buttonRow, TelegramButton{Text: text, CallbackData: cb})
		}
		if len(buttonRow) > 0 {
			result = append(result, buttonRow)
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// HandleTelegramAction Telegram action 分发路由
func HandleTelegramAction(ctx context.Context, params map[string]interface{}, actionGate ActionGate, deps TelegramActionDeps) (ToolResult, error) {
	action, err := ReadStringParam(params, "action", true)
	if err != nil {
		return ErrorResult(err), err
	}

	switch action {
	// ── Messaging ──────────────────────────────────────────────────────
	case "sendMessage":
		if !actionGate("sendMessage") {
			return ToolResult{}, fmt.Errorf("Telegram sendMessage is disabled")
		}
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), err
		}
		mediaURL, _ := ReadStringParam(params, "mediaUrl", false)
		content, _ := ReadStringParam(params, "content", false)
		if content == "" && mediaURL == "" {
			return ToolResult{}, fmt.Errorf("content is required when no mediaUrl is provided")
		}
		buttons, err := ReadTelegramButtons(params)
		if err != nil {
			return ErrorResult(err), err
		}
		// 对齐 TS: 检查 inline buttons scope（off/dm/group/all）
		if buttons != nil {
			if checker, ok := deps.(InlineButtonsScopeChecker); ok {
				if err := checker.CheckInlineButtonsAllowed(to); err != nil {
					return ErrorResult(err), err
				}
			}
		}
		var replyToMsgID *int
		if v, ok := ReadIntParam(params, "replyToMessageId"); ok {
			replyToMsgID = &v
		}
		var threadID *int
		if v, ok := ReadIntParam(params, "messageThreadId"); ok {
			threadID = &v
		}
		quoteText, _ := ReadStringParam(params, "quoteText", false)
		asVoice := ReadBoolParam(params, "asVoice", false)
		silent := ReadBoolParam(params, "silent", false)

		msgID, chatID, err := deps.SendMessage(ctx, to, content, TelegramBridgeSendOpts{
			MediaURL:         mediaURL,
			Buttons:          buttons,
			ReplyToMessageID: replyToMsgID,
			MessageThreadID:  threadID,
			QuoteText:        quoteText,
			AsVoice:          asVoice,
			Silent:           silent,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "messageId": msgID, "chatId": chatID}), nil

	case "editMessage":
		if !actionGate("editMessage") {
			return ToolResult{}, fmt.Errorf("Telegram editMessage is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		content, err := ReadStringParam(params, "content", true)
		if err != nil {
			return ErrorResult(err), err
		}
		buttons, err := ReadTelegramButtons(params)
		if err != nil {
			return ErrorResult(err), err
		}
		// 对齐 TS: editMessage 也检查 inline buttons scope
		if buttons != nil {
			if checker, ok := deps.(InlineButtonsScopeChecker); ok {
				if err := checker.CheckInlineButtonsAllowed(chatID); err != nil {
					return ErrorResult(err), err
				}
			}
		}
		if err := deps.EditMessage(ctx, chatID, int(msgIDRaw), content, buttons); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "deleteMessage":
		if !actionGate("deleteMessage") {
			return ToolResult{}, fmt.Errorf("Telegram deleteMessage is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.DeleteMessage(ctx, chatID, int(msgIDRaw)); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "deleted": true}), nil

	case "forwardMessage":
		if !actionGate("forwardMessage") {
			return ToolResult{}, fmt.Errorf("Telegram forwardMessage is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		fromChatID, err := ReadStringParam(params, "fromChatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		apiParams := map[string]interface{}{
			"chat_id":      chatID,
			"from_chat_id": fromChatID,
			"message_id":   int(msgIDRaw),
		}
		result, err := deps.CallAPI(ctx, "forwardMessage", apiParams)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	case "copyMessage":
		if !actionGate("copyMessage") {
			return ToolResult{}, fmt.Errorf("Telegram copyMessage is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		fromChatID, err := ReadStringParam(params, "fromChatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		apiParams := map[string]interface{}{
			"chat_id":      chatID,
			"from_chat_id": fromChatID,
			"message_id":   int(msgIDRaw),
		}
		result, err := deps.CallAPI(ctx, "copyMessage", apiParams)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	case "readMessages":
		if !actionGate("readMessages") {
			return ToolResult{}, fmt.Errorf("Telegram readMessages is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		apiParams := map[string]interface{}{
			"chat_id": chatID,
		}
		if limit, ok := ReadIntParam(params, "limit"); ok {
			apiParams["limit"] = limit
		}
		if offset, ok := ReadIntParam(params, "offset"); ok {
			apiParams["offset"] = offset
		}
		result, err := deps.CallAPI(ctx, "getUpdates", apiParams)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	case "searchMessages":
		// Telegram Bot API 无原生搜索，透传到 CallAPI 让 deps 层处理
		if !actionGate("searchMessages") {
			return ToolResult{}, fmt.Errorf("Telegram searchMessages is disabled")
		}
		return ToolResult{}, fmt.Errorf("Telegram Bot API does not support message search natively")

	// ── Reactions ──────────────────────────────────────────────────────
	case "react", "reactions":
		// 对齐 TS: 先检查 reactionLevel 再检查 actionGate
		if checker, ok := deps.(ReactionLevelChecker); ok {
			if err := checker.CheckReactionAllowed(); err != nil {
				return ErrorResult(err), err
			}
		}
		if !actionGate("reactions") {
			return ToolResult{}, fmt.Errorf("Telegram reactions are disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		reaction := ReadReactionParams(params)
		if err := deps.ReactMessage(ctx, chatID, int(msgIDRaw), reaction.Emoji, reaction.Remove); err != nil {
			return ErrorResult(err), err
		}
		if !reaction.Remove && !reaction.IsEmpty {
			return OkResult(map[string]interface{}{"ok": true, "added": reaction.Emoji}), nil
		}
		return OkResult(map[string]interface{}{"ok": true, "removed": true}), nil

	// ── Poll ───────────────────────────────────────────────────────────
	case "poll":
		if !actionGate("poll") {
			return ToolResult{}, fmt.Errorf("Telegram poll is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		question, err := ReadStringParam(params, "question", true)
		if err != nil {
			return ErrorResult(err), err
		}
		options := ReadStringArrayParam(params, "options")
		if len(options) < 2 {
			return ToolResult{}, fmt.Errorf("poll requires at least 2 options")
		}
		apiParams := map[string]interface{}{
			"chat_id":  chatID,
			"question": question,
			"options":  options,
		}
		if isAnon, ok := params["isAnonymous"].(bool); ok {
			apiParams["is_anonymous"] = isAnon
		}
		if allowMulti, ok := params["allowsMultipleAnswers"].(bool); ok {
			apiParams["allows_multiple_answers"] = allowMulti
		}
		result, err := deps.CallAPI(ctx, "sendPoll", apiParams)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	// ── Pin ────────────────────────────────────────────────────────────
	case "pinMessage":
		if !actionGate("pin") {
			return ToolResult{}, fmt.Errorf("Telegram pin is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		_, err = deps.CallAPI(ctx, "pinChatMessage", map[string]interface{}{
			"chat_id":    chatID,
			"message_id": int(msgIDRaw),
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "unpinMessage":
		if !actionGate("pin") {
			return ToolResult{}, fmt.Errorf("Telegram pin is disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgIDRaw, err := ReadNumberParamRequired(params, "messageId")
		if err != nil {
			return ErrorResult(err), err
		}
		_, err = deps.CallAPI(ctx, "unpinChatMessage", map[string]interface{}{
			"chat_id":    chatID,
			"message_id": int(msgIDRaw),
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "listPins":
		if !actionGate("pin") {
			return ToolResult{}, fmt.Errorf("Telegram pin is disabled")
		}
		// Telegram 没有 listPins API，需要通过 getChat 获取 pinned_message
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		result, err := deps.CallAPI(ctx, "getChat", map[string]interface{}{
			"chat_id": chatID,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	// ── Admin ──────────────────────────────────────────────────────────
	case "chatInfo":
		if !actionGate("admin") {
			return ToolResult{}, fmt.Errorf("Telegram admin actions are disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		result, err := deps.CallAPI(ctx, "getChat", map[string]interface{}{
			"chat_id": chatID,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	case "memberInfo":
		if !actionGate("admin") {
			return ToolResult{}, fmt.Errorf("Telegram admin actions are disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userIDInt, convErr := strconv.ParseInt(userID, 10, 64)
		if convErr != nil {
			return ToolResult{}, fmt.Errorf("userId must be numeric: %w", convErr)
		}
		result, err := deps.CallAPI(ctx, "getChatMember", map[string]interface{}{
			"chat_id": chatID,
			"user_id": userIDInt,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": result}), nil

	case "setChatTitle":
		if !actionGate("admin") {
			return ToolResult{}, fmt.Errorf("Telegram admin actions are disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		title, err := ReadStringParam(params, "title", true)
		if err != nil {
			return ErrorResult(err), err
		}
		_, err = deps.CallAPI(ctx, "setChatTitle", map[string]interface{}{
			"chat_id": chatID,
			"title":   title,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "setChatDescription":
		if !actionGate("admin") {
			return ToolResult{}, fmt.Errorf("Telegram admin actions are disabled")
		}
		chatID, err := ReadStringParam(params, "chatId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		desc, _ := ReadStringParam(params, "description", false)
		_, err = deps.CallAPI(ctx, "setChatDescription", map[string]interface{}{
			"chat_id":     chatID,
			"description": desc,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	// ── Sticker ────────────────────────────────────────────────────────
	case "sendSticker":
		if !actionGate("sticker") {
			return ToolResult{}, fmt.Errorf("Telegram sticker actions are disabled")
		}
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), err
		}
		fileID, err := ReadStringParam(params, "fileId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgID, chatID, err := deps.SendSticker(ctx, to, fileID)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "messageId": msgID, "chatId": chatID}), nil

	case "searchSticker":
		if !actionGate("sticker") {
			return ToolResult{}, fmt.Errorf("Telegram sticker actions are disabled")
		}
		query, err := ReadStringParam(params, "query", true)
		if err != nil {
			return ErrorResult(err), err
		}
		limit := 5
		if v, ok := ReadIntParam(params, "limit"); ok && v > 0 {
			limit = v
		}
		results := deps.SearchStickers(query, limit)
		stickers := make([]map[string]interface{}, 0, len(results))
		for _, s := range results {
			stickers = append(stickers, map[string]interface{}{
				"fileId":      s.FileID,
				"emoji":       s.Emoji,
				"description": s.Description,
				"setName":     s.SetName,
			})
		}
		return OkResult(map[string]interface{}{"ok": true, "count": len(stickers), "stickers": stickers}), nil

	case "stickerCacheStats":
		stats := deps.GetStickerCacheStats()
		stats["ok"] = true
		return OkResult(stats), nil

	default:
		return ToolResult{}, fmt.Errorf("unknown telegram action: %s", action)
	}
}
