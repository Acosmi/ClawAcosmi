package line

// TS 对照: src/line/bot-handlers.ts (346L)
// LINE Webhook 事件处理器 — DM/群组策略、pairing 门控、消息/回调分发

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// ---------- 配对存储接口（轻量 stub，可被上层替换） ----------

// PairingStore pairing 操作接口。
type PairingStore interface {
	// ReadChannelAllowFrom 从 store 读取频道 allowFrom 列表。
	ReadChannelAllowFrom(channel string) ([]string, error)
	// UpsertChannelPairingRequest 插入或查询配对请求，返回 (code, created)。
	UpsertChannelPairingRequest(channel, senderID string) (code string, created bool, err error)
	// ResolvePairingIDLabel 解析展示给用户的 ID 标签（如 "lineUserId"）。
	ResolvePairingIDLabel(channel string) string
	// BuildPairingReply 构建配对回复消息。
	BuildPairingReply(channel, idLine, code string) string
}

// noopPairingStore 空实现：所有配对门控均通过，不发送配对消息。
type noopPairingStore struct{}

func (noopPairingStore) ReadChannelAllowFrom(_ string) ([]string, error) { return nil, nil }
func (noopPairingStore) UpsertChannelPairingRequest(_, _ string) (string, bool, error) {
	return "", false, nil
}
func (noopPairingStore) ResolvePairingIDLabel(_ string) string { return "lineUserId" }
func (noopPairingStore) BuildPairingReply(_, idLine, code string) string {
	return fmt.Sprintf("%s\nPairing code: %s", idLine, code)
}

// ---------- HandlerContext ----------

// LineInboundContext 上层 processMessage 所需的入站上下文。
// 与 TS 端 LineInboundContext 对应。
type LineInboundContext struct {
	// 标准消息字段
	Body    string
	From    string
	Channel string
	IsGroup bool
	GroupID string
	RoomID  string
	UserID  string
	// LINE 特有
	ReplyToken string
	AccountID  string
	// 扩展
	MediaPaths []string
}

// LineHandlerContext LINE 处理器上下文。
// TS: LineHandlerContext
type LineHandlerContext struct {
	Account       ResolvedLineAccount
	Config        LineConfig
	MediaMaxBytes int64
	PairingStore  PairingStore
	Client        *Client
	// ProcessMessage 处理入站消息的回调（由 monitor 层注入）。
	ProcessMessage func(ctx context.Context, inbound LineInboundContext) error
	// LogVerbose 可选 verbose 日志函数。
	LogVerbose func(msg string)
	// OnError 错误回调。
	OnError func(err error)
}

func (h *LineHandlerContext) logVerbose(msg string) {
	if h.LogVerbose != nil {
		h.LogVerbose(msg)
	}
}

func (h *LineHandlerContext) onError(err error) {
	if h.OnError != nil {
		h.OnError(err)
	} else {
		log.Printf("[line] error: %v", err)
	}
}

func (h *LineHandlerContext) pairingStore() PairingStore {
	if h.PairingStore != nil {
		return h.PairingStore
	}
	return noopPairingStore{}
}

// ---------- sourceInfo ----------

type lineSourceInfo struct {
	UserID  string
	GroupID string
	RoomID  string
	IsGroup bool
}

func getSourceInfo(src EventSource) lineSourceInfo {
	userID := src.UserID
	groupID := ""
	roomID := ""
	isGroup := false

	switch src.Type {
	case "group":
		groupID = src.GroupID
		isGroup = true
	case "room":
		roomID = src.RoomID
		isGroup = true
	}

	return lineSourceInfo{
		UserID:  userID,
		GroupID: groupID,
		RoomID:  roomID,
		IsGroup: isGroup,
	}
}

// ---------- group config resolution ----------

func resolveLineGroupConfig(cfg LineConfig, groupID, roomID string) *LineGroupConfig {
	groups := cfg.Groups
	if groups == nil {
		return nil
	}
	if groupID != "" {
		if g, ok := groups[groupID]; ok {
			return &g
		}
		if g, ok := groups["group:"+groupID]; ok {
			return &g
		}
		if g, ok := groups["*"]; ok {
			return &g
		}
	}
	if roomID != "" {
		if g, ok := groups[roomID]; ok {
			return &g
		}
		if g, ok := groups["room:"+roomID]; ok {
			return &g
		}
		if g, ok := groups["*"]; ok {
			return &g
		}
	}
	if g, ok := groups["*"]; ok {
		return &g
	}
	return nil
}

// ---------- pairing reply ----------

func sendLinePairingReply(
	ctx context.Context,
	senderID string,
	replyToken string,
	hctx *LineHandlerContext,
) {
	store := hctx.pairingStore()
	code, created, err := store.UpsertChannelPairingRequest("line", senderID)
	if err != nil || !created {
		return
	}

	hctx.logVerbose(fmt.Sprintf("line pairing request sender=%s", senderID))

	idLabel := store.ResolvePairingIDLabel("line")
	text := store.BuildPairingReply(
		"line",
		fmt.Sprintf("Your %s: %s", idLabel, senderID),
		code,
	)

	// 优先用 reply token
	if replyToken != "" && hctx.Client != nil {
		if err := hctx.Client.ReplyText(ctx, replyToken, text); err != nil {
			hctx.logVerbose(fmt.Sprintf("line pairing reply failed for %s: %v", senderID, err))
		} else {
			return
		}
	}

	// fallback: push
	if hctx.Client != nil {
		to := "line:" + senderID
		if _, err := hctx.Client.PushText(ctx, to, text); err != nil {
			hctx.logVerbose(fmt.Sprintf("line pairing push failed for %s: %v", senderID, err))
		}
	}
}

// ---------- shouldProcessLineEvent ----------

// ShouldProcessLineEvent 决定是否处理此事件。
// TS: shouldProcessLineEvent()
func ShouldProcessLineEvent(
	ctx context.Context,
	src EventSource,
	replyToken string,
	hctx *LineHandlerContext,
) (bool, error) {
	store := hctx.pairingStore()
	info := getSourceInfo(src)
	senderID := info.UserID

	storeAllowFrom, _ := store.ReadChannelAllowFrom("line")
	effectiveDmAllow := NormalizeAllowFromWithStore(hctx.Account.Config.AllowFrom, storeAllowFrom)

	groupCfg := resolveLineGroupConfig(hctx.Config, info.GroupID, info.RoomID)
	var groupAllowOverride []string
	if groupCfg != nil && groupCfg.AllowFrom != nil {
		groupAllowOverride = groupCfg.AllowFrom
	}

	var fallbackGroupAllowFrom []string
	if len(hctx.Account.Config.AllowFrom) > 0 {
		fallbackGroupAllowFrom = hctx.Account.Config.AllowFrom
	}

	groupAllowFrom := FirstDefined(groupAllowOverride, hctx.Account.Config.GroupAllowFrom, fallbackGroupAllowFrom)
	effectiveGroupAllow := NormalizeAllowFromWithStore(groupAllowFrom, storeAllowFrom)

	dmPolicy := hctx.Account.Config.DMPolicy
	if dmPolicy == "" {
		dmPolicy = "pairing"
	}
	groupPolicy := hctx.Account.Config.GroupPolicy
	if groupPolicy == "" {
		groupPolicy = "allowlist"
	}

	if info.IsGroup {
		if groupCfg != nil && !groupCfg.Enabled && groupCfg.Enabled != (groupCfg.AllowFrom != nil) {
			// explicit enabled=false
			if !groupCfg.Enabled {
				hctx.logVerbose(fmt.Sprintf("Blocked line group %s (group disabled)", coalesce(info.GroupID, info.RoomID, "unknown")))
				return false, nil
			}
		}
		if groupAllowOverride != nil {
			if senderID == "" {
				hctx.logVerbose("Blocked line group message (group allowFrom override, no sender ID)")
				return false, nil
			}
			if !IsSenderAllowedLine(effectiveGroupAllow, senderID) {
				hctx.logVerbose(fmt.Sprintf("Blocked line group sender %s (group allowFrom override)", senderID))
				return false, nil
			}
		}
		switch groupPolicy {
		case "disabled":
			hctx.logVerbose("Blocked line group message (groupPolicy: disabled)")
			return false, nil
		case "allowlist":
			if senderID == "" {
				hctx.logVerbose("Blocked line group message (no sender ID, groupPolicy: allowlist)")
				return false, nil
			}
			if !effectiveGroupAllow.HasEntries {
				hctx.logVerbose("Blocked line group message (groupPolicy: allowlist, no groupAllowFrom)")
				return false, nil
			}
			if !IsSenderAllowedLine(effectiveGroupAllow, senderID) {
				hctx.logVerbose(fmt.Sprintf("Blocked line group message from %s (groupPolicy: allowlist)", senderID))
				return false, nil
			}
		}
		return true, nil
	}

	// DM 策略
	if dmPolicy == "disabled" {
		hctx.logVerbose("Blocked line sender (dmPolicy: disabled)")
		return false, nil
	}

	dmAllowed := dmPolicy == "open" || IsSenderAllowedLine(effectiveDmAllow, senderID)
	if !dmAllowed {
		if dmPolicy == "pairing" {
			if senderID == "" {
				hctx.logVerbose("Blocked line sender (dmPolicy: pairing, no sender ID)")
				return false, nil
			}
			sendLinePairingReply(ctx, senderID, replyToken, hctx)
		} else {
			if senderID == "" {
				senderID = "unknown"
			}
			hctx.logVerbose(fmt.Sprintf("Blocked line sender %s (dmPolicy: %s)", senderID, dmPolicy))
		}
		return false, nil
	}

	return true, nil
}

// ---------- event handlers ----------

// HandleMessageEvent 处理消息事件。
// TS: handleMessageEvent()
func HandleMessageEvent(ctx context.Context, event WebhookEvent, hctx *LineHandlerContext) error {
	if event.Message == nil {
		return nil
	}

	ok, err := ShouldProcessLineEvent(ctx, event.Source, event.ReplyToken, hctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	info := getSourceInfo(event.Source)

	// 下载媒体
	var mediaPaths []string
	if event.Message.Type == "image" || event.Message.Type == "video" || event.Message.Type == "audio" {
		if hctx.Client != nil {
			maxBytes := hctx.MediaMaxBytes
			if maxBytes <= 0 {
				maxBytes = 10 * 1024 * 1024
			}
			result, dlErr := DownloadLineMedia(event.Message.ID, hctx.Account.ChannelAccessToken, maxBytes)
			if dlErr != nil {
				errMsg := dlErr.Error()
				if containsAll(errMsg, "exceeds", "limit") {
					hctx.logVerbose(fmt.Sprintf("line: media exceeds size limit for message %s", event.Message.ID))
				} else {
					hctx.onError(fmt.Errorf("line: failed to download media: %w", dlErr))
				}
			} else {
				mediaPaths = append(mediaPaths, result.Path)
			}
		}
	}

	// 构建入站上下文
	inbound := LineInboundContext{
		Body:       event.Message.Text,
		From:       info.UserID,
		Channel:    "line",
		IsGroup:    info.IsGroup,
		GroupID:    info.GroupID,
		RoomID:     info.RoomID,
		UserID:     info.UserID,
		ReplyToken: event.ReplyToken,
		AccountID:  hctx.Account.AccountID,
		MediaPaths: mediaPaths,
	}

	if inbound.Body == "" && len(mediaPaths) == 0 {
		hctx.logVerbose("line: skipping empty message")
		return nil
	}

	if hctx.ProcessMessage == nil {
		return nil
	}
	return hctx.ProcessMessage(ctx, inbound)
}

// HandleFollowEvent 处理关注事件。
// TS: handleFollowEvent()
func HandleFollowEvent(_ context.Context, event WebhookEvent, hctx *LineHandlerContext) {
	userID := ""
	if event.Source.Type == "user" {
		userID = event.Source.UserID
	}
	if userID == "" {
		userID = "unknown"
	}
	hctx.logVerbose(fmt.Sprintf("line: user %s followed", userID))
}

// HandleUnfollowEvent 处理取消关注事件。
// TS: handleUnfollowEvent()
func HandleUnfollowEvent(_ context.Context, event WebhookEvent, hctx *LineHandlerContext) {
	userID := ""
	if event.Source.Type == "user" {
		userID = event.Source.UserID
	}
	if userID == "" {
		userID = "unknown"
	}
	hctx.logVerbose(fmt.Sprintf("line: user %s unfollowed", userID))
}

// HandleJoinEvent 处理 bot 入群事件。
// TS: handleJoinEvent()
func HandleJoinEvent(_ context.Context, event WebhookEvent, hctx *LineHandlerContext) {
	if event.Source.GroupID != "" {
		hctx.logVerbose(fmt.Sprintf("line: bot joined group %s", event.Source.GroupID))
	} else {
		hctx.logVerbose(fmt.Sprintf("line: bot joined room %s", event.Source.RoomID))
	}
}

// HandleLeaveEvent 处理 bot 离群事件。
// TS: handleLeaveEvent()
func HandleLeaveEvent(_ context.Context, event WebhookEvent, hctx *LineHandlerContext) {
	if event.Source.GroupID != "" {
		hctx.logVerbose(fmt.Sprintf("line: bot left group %s", event.Source.GroupID))
	} else {
		hctx.logVerbose(fmt.Sprintf("line: bot left room %s", event.Source.RoomID))
	}
}

// HandlePostbackEvent 处理 postback 事件。
// TS: handlePostbackEvent()
func HandlePostbackEvent(ctx context.Context, event WebhookEvent, hctx *LineHandlerContext) error {
	if event.Postback == nil {
		return nil
	}

	hctx.logVerbose(fmt.Sprintf("line: received postback: %s", event.Postback.Data))

	ok, err := ShouldProcessLineEvent(ctx, event.Source, event.ReplyToken, hctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	info := getSourceInfo(event.Source)
	inbound := LineInboundContext{
		Body:       event.Postback.Data,
		From:       info.UserID,
		Channel:    "line",
		IsGroup:    info.IsGroup,
		GroupID:    info.GroupID,
		RoomID:     info.RoomID,
		UserID:     info.UserID,
		ReplyToken: event.ReplyToken,
		AccountID:  hctx.Account.AccountID,
	}

	if hctx.ProcessMessage == nil {
		return nil
	}
	return hctx.ProcessMessage(ctx, inbound)
}

// HandleLineWebhookEvents 遍历处理全部 webhook 事件。
// TS: handleLineWebhookEvents()
func HandleLineWebhookEvents(ctx context.Context, events []WebhookEvent, hctx *LineHandlerContext) {
	for _, event := range events {
		var handlerErr error
		switch event.Type {
		case "message":
			handlerErr = HandleMessageEvent(ctx, event, hctx)
		case "follow":
			HandleFollowEvent(ctx, event, hctx)
		case "unfollow":
			HandleUnfollowEvent(ctx, event, hctx)
		case "join":
			HandleJoinEvent(ctx, event, hctx)
		case "leave":
			HandleLeaveEvent(ctx, event, hctx)
		case "postback":
			handlerErr = HandlePostbackEvent(ctx, event, hctx)
		default:
			hctx.logVerbose(fmt.Sprintf("line: unhandled event type: %s", event.Type))
		}
		if handlerErr != nil {
			hctx.onError(fmt.Errorf("line: event handler failed: %w", handlerErr))
		}
	}
}

// ---------- helpers ----------

// coalesce 返回第一个非空字符串。
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// containsAll 检查字符串是否包含所有子串。
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
