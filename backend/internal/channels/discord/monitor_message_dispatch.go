package discord

// Discord 入站消息分发 — 继承自 src/discord/monitor/process.ts (450L)
// Phase 9 实现：agent 路由 + MsgContext 构建 + session 记录 + auto-reply 分发。
// DY-030 fix: debounce 改造为收集模式 — 合并窗口内所有消息而非仅保留最后一条。
// DY-028 fix: 补充 ack reaction / reply context / thread / forum parent 管线步骤。

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// ---------------------------------------------------------------------------
// Debounce mechanism — DY-030 flush-based collection mode
// TS ref: createInboundDebouncer (message-handler.ts L91-132)
// 多条快速连发消息在 debounce 窗口内被收集，最终由最后一个 goroutine
// 将它们合并为 synthetic message 后统一 dispatch。
// ---------------------------------------------------------------------------

const discordDispatchDebounceMs = 300

// debounceMessage 收集窗口内的单条消息。
type debounceMessage struct {
	msg *DiscordInboundMessage
	raw *discordgo.MessageCreate
}

// dispatchDebounceEntry 收集器 — 存储 debounce 窗口内的所有消息。
// DY-030 fix: 从仅保留最后一条改为收集所有消息。
type dispatchDebounceEntry struct {
	mu       sync.Mutex
	seq      uint64
	messages []*debounceMessage // 收集窗口内的所有消息
}

// dispatchDebounceMap is a package-level sync.Map keyed by "channelID:userID".
// Values are *dispatchDebounceEntry.
var dispatchDebounceMap sync.Map

// getDebounceEntry returns (or lazily creates) the debounce entry for a
// channel+user pair.
func getDebounceEntry(channelID, userID string) *dispatchDebounceEntry {
	key := channelID + ":" + userID
	val, _ := dispatchDebounceMap.LoadOrStore(key, &dispatchDebounceEntry{})
	return val.(*dispatchDebounceEntry)
}

// shouldDebounceMessage 判断消息是否应该参与 debounce。
// 有附件的消息不做 debounce（直接 dispatch），纯文本消息才 debounce。
// DY-030 fix: 对齐 TS createInboundDebouncer 中的过滤逻辑。
func shouldDebounceMessage(msg *DiscordInboundMessage) bool {
	if len(msg.Attachments) > 0 {
		return false
	}
	if strings.TrimSpace(msg.Text) == "" {
		return false
	}
	return true
}

// DispatchDiscordInbound 分发预处理后的入站消息到 agent 管线。
// DY-030 fix: 改造为收集模式 — debounce 窗口内的消息被收集到 entry.messages，
// 最后一个 goroutine 负责取出所有消息并合并后 dispatch。
// 有附件的消息不参与 debounce，立即 dispatch。
func DispatchDiscordInbound(ctx context.Context, monCtx *DiscordMonitorContext, msg *DiscordInboundMessage, raw *discordgo.MessageCreate) {
	// 有附件的消息或空文本消息不参与 debounce，立即 dispatch
	if !shouldDebounceMessage(msg) {
		dispatchDiscordInboundCore(ctx, monCtx, msg, raw)
		return
	}

	entry := getDebounceEntry(msg.ChannelID, msg.SenderID)

	// 将消息加入收集列表并递增 seq
	entry.mu.Lock()
	entry.messages = append(entry.messages, &debounceMessage{msg: msg, raw: raw})
	entry.seq++
	mySeq := entry.seq
	entry.mu.Unlock()

	// Wait for the debounce window.
	time.Sleep(time.Duration(discordDispatchDebounceMs) * time.Millisecond)

	// After the sleep, check whether a newer message has arrived.
	entry.mu.Lock()
	currentSeq := entry.seq
	if currentSeq != mySeq {
		// A newer message arrived during debounce window — let the latest goroutine handle it.
		entry.mu.Unlock()
		monCtx.Logger.Debug("dispatch debounced (superseded by newer message)",
			"sender", msg.SenderID,
			"channel", msg.ChannelID,
			"messageID", msg.MessageID,
		)
		return
	}

	// 我是最后一个 goroutine — 取出所有收集到的消息并清空列表
	collected := entry.messages
	entry.messages = nil
	entry.mu.Unlock()

	if len(collected) == 0 {
		return
	}

	// 合并消息
	mergedMsg, mergedRaw := mergeDebounceMessages(collected)
	if mergedMsg == nil {
		return
	}

	if len(collected) > 1 {
		monCtx.Logger.Debug("dispatch debounce merged messages",
			"sender", msg.SenderID,
			"channel", msg.ChannelID,
			"count", len(collected),
			"mergedIDs", mergedMsg.MessageIDs,
		)
	}

	dispatchDiscordInboundCore(ctx, monCtx, mergedMsg, mergedRaw)
}

// mergeDebounceMessages 合并 debounce 窗口内收集到的多条消息。
// DY-030 fix: 对齐 TS message-handler.ts L91-132 createInboundDebouncer 的 flush 逻辑。
// 单条消息直接返回；多条消息合并文本（join "\n"），构建 synthetic message。
func mergeDebounceMessages(entries []*debounceMessage) (*DiscordInboundMessage, *discordgo.MessageCreate) {
	if len(entries) == 0 {
		return nil, nil
	}
	if len(entries) == 1 {
		return entries[0].msg, entries[0].raw
	}

	last := entries[len(entries)-1]

	// 合并所有消息文本
	var texts []string
	var allIDs []string
	for _, e := range entries {
		if e.msg.Text != "" {
			texts = append(texts, e.msg.Text)
		}
		allIDs = append(allIDs, e.msg.MessageID)
	}

	// 复制最后一条消息作为基础
	merged := *last.msg
	merged.Text = strings.Join(texts, "\n")
	merged.Attachments = nil // 合并消息清空附件

	// 设置 batch IDs
	merged.MessageIDs = allIDs
	merged.MessageIDFirst = allIDs[0]
	merged.MessageIDLast = allIDs[len(allIDs)-1]

	return &merged, last.raw
}

// DispatchDiscordInboundImmediate dispatches without debouncing. Useful for
// callers that have already applied their own coalescing logic.
func DispatchDiscordInboundImmediate(ctx context.Context, monCtx *DiscordMonitorContext, msg *DiscordInboundMessage, raw *discordgo.MessageCreate) {
	dispatchDiscordInboundCore(ctx, monCtx, msg, raw)
}

// dispatchDiscordInboundCore is the actual dispatch logic, separated from the
// debounce wrapper.
// DY-028 fix: 补充 ack reaction / reply context / thread / forum parent 管线步骤。
func dispatchDiscordInboundCore(ctx context.Context, monCtx *DiscordMonitorContext, msg *DiscordInboundMessage, raw *discordgo.MessageCreate) {
	logger := monCtx.Logger.With(
		"action", "dispatch",
		"sender", msg.SenderID,
		"channel", msg.ChannelID,
	)

	if monCtx.Deps == nil {
		logger.Warn("deps not available, skipping dispatch")
		return
	}

	// 1. Agent 路由
	peerKind := "channel"
	if msg.IsDM {
		peerKind = "direct"
	}
	if monCtx.Deps.ResolveAgentRoute == nil {
		logger.Warn("ResolveAgentRoute not available")
		return
	}
	route, err := monCtx.Deps.ResolveAgentRoute(DiscordAgentRouteParams{
		Channel:   "discord",
		AccountID: monCtx.AccountID,
		PeerKind:  peerKind,
		PeerID:    resolveDiscordPeerID(msg),
	})
	if err != nil || route == nil {
		logger.Warn("route resolution failed", "error", err)
		return
	}

	// 2. typing 指示
	_ = SendTypingIndicator(ctx, monCtx.Session, msg.ChannelID)

	// 2b. Ack reaction（确认收到消息的表情反应）
	// DY-028 fix: 对齐 TS message-handler.process.ts ack reaction 步骤。
	// 只在消息包含 mention 或 DM 时 ack。
	if monCtx.Deps.AddReaction != nil {
		if msg.WasMentioned || msg.IsDM {
			_ = monCtx.Deps.AddReaction(ctx, msg.ChannelID, msg.MessageID)
		}
	}

	// 3. 构建 MsgContext
	transcript := buildDiscordTranscript(msg, raw)

	chatType := "channel"
	if msg.IsDM {
		chatType = "direct"
	}

	msgCtx := &autoreply.MsgContext{
		Provider:   "discord",
		AccountID:  monCtx.AccountID,
		SessionKey: route.SessionKey,
		Body:       msg.Text,
		RawBody:    raw.Content,
		Transcript: transcript,
		ChatType:   chatType,
		ChannelID:  msg.ChannelID,
		SenderID:   msg.SenderID,
		SenderName: msg.SenderName,
	}

	if msg.WasMentioned {
		msgCtx.WasMentioned = "true"
	}

	// 3b. Reply context — 引用消息信息
	// DY-028 fix: 对齐 TS process.ts reply context 步骤。
	if msg.ReplyToMessageID != "" {
		// 使用 MessageThreadID 字段传递 reply-to 信息（MsgContext 中最接近的字段）
		if msgCtx.MessageThreadID == "" {
			msgCtx.MessageThreadID = msg.ReplyToMessageID
		}
	}

	// 3c. Thread session key — 线程消息使用 ThreadID
	// DY-028 fix: 对齐 TS process.ts thread session key 步骤。
	if msg.ThreadID != "" {
		msgCtx.MessageThreadID = msg.ThreadID
	}

	// 3d. Forum parent slug
	// DY-028 fix: 对齐 TS process.ts forum parent 步骤。
	if msg.ForumParentSlug != "" {
		msgCtx.GroupChannel = msg.ForumParentSlug
	}

	// 3e. Batch message IDs（debounce 合并后的批量 ID）
	if len(msg.MessageIDs) > 0 {
		msgCtx.MessageSidFirst = msg.MessageIDFirst
		msgCtx.MessageSidLast = msg.MessageIDLast
	}

	// 4. 记录入站会话
	if monCtx.Deps.RecordInboundSession != nil {
		storePath := ""
		if monCtx.Deps.ResolveStorePath != nil {
			storePath = monCtx.Deps.ResolveStorePath(route.AgentID)
		}
		var lastRoute *DiscordLastRouteUpdate
		if msg.IsDM {
			lastRoute = &DiscordLastRouteUpdate{
				SessionKey: route.SessionKey,
				Channel:    "discord",
				To:         msg.SenderID,
				AccountID:  monCtx.AccountID,
			}
		}
		if err := monCtx.Deps.RecordInboundSession(DiscordRecordSessionParams{
			StorePath:       storePath,
			SessionKey:      route.SessionKey,
			Ctx:             msgCtx,
			UpdateLastRoute: lastRoute,
		}); err != nil {
			logger.Error("record session failed", "error", err)
		}
	}

	// 5. 记录入站活动
	if monCtx.Deps.RecordChannelActivity != nil {
		monCtx.Deps.RecordChannelActivity("discord", monCtx.AccountID, "inbound")
	}

	// 6. 分发到 auto-reply 管线
	if monCtx.Deps.DispatchInboundMessage == nil {
		logger.Warn("DispatchInboundMessage not available")
		return
	}
	result, err := monCtx.Deps.DispatchInboundMessage(ctx, DiscordDispatchParams{
		Ctx: msgCtx,
	})
	if err != nil {
		logger.Error("dispatch failed", "error", err)
		return
	}

	// 7. 移除 ack reaction
	// DY-028 fix: 对齐 TS message-handler.process.ts — removeAckReactionAfterReply。
	if monCtx.Deps.RemoveReaction != nil {
		if msg.WasMentioned || msg.IsDM {
			_ = monCtx.Deps.RemoveReaction(ctx, msg.ChannelID, msg.MessageID)
		}
	}

	logger.Debug("dispatch completed",
		"queuedFinal", result != nil && result.QueuedFinal,
		"sessionKey", route.SessionKey,
	)
}

// resolveDiscordPeerID 解析 peer ID。
func resolveDiscordPeerID(msg *DiscordInboundMessage) string {
	if msg.IsDM {
		return msg.SenderID
	}
	if msg.GuildID != "" {
		return fmt.Sprintf("%s:%s", msg.GuildID, msg.ChannelID)
	}
	return msg.ChannelID
}

// buildDiscordTranscript 构建消息转录文本。
func buildDiscordTranscript(msg *DiscordInboundMessage, raw *discordgo.MessageCreate) string {
	text := msg.Text

	// 附件标注
	if len(msg.Attachments) > 0 {
		var attachDescs []string
		for _, a := range msg.Attachments {
			desc := fmt.Sprintf("[attachment: %s]", a.Filename)
			attachDescs = append(attachDescs, desc)
		}
		if text != "" {
			text = text + "\n" + strings.Join(attachDescs, "\n")
		} else {
			text = strings.Join(attachDescs, "\n")
		}
	}

	return text
}
