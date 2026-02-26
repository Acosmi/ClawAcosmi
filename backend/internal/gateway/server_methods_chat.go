package gateway

// server_methods_chat.go — chat.send, chat.abort, chat.history, chat.inject
// 对应 TS src/gateway/server-methods/chat.ts
//
// chat.send 是核心聊天管线入口：
//   消息 → 附件解析 → session 解析 → agent command 分发
//
// 当前实现策略:
//   - chat.history → 从 SessionStore 读取 transcript
//   - chat.abort   → 通过 ChatRunState 标记中断
//   - chat.send    → 参数解析 + session resolve + agent command 分发
//   - chat.inject  → transcript 追加 assistant 消息

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/agents/scope"
	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/internal/infra"
	"github.com/anthropic/open-acosmi/internal/media"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ChatHandlers 返回 chat.* 方法处理器映射。
func ChatHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"chat.history": handleChatHistory,
		"chat.abort":   handleChatAbort,
		"chat.send":    handleChatSend,
		"chat.inject":  handleChatInject,
	}
}

// ---------- chat.history ----------
// 对应 TS chat.ts L30-L100
// 返回指定 session 的消息历史。

func handleChatHistory(ctx *MethodHandlerContext) {
	sessionKey, _ := ctx.Params["sessionId"].(string)
	if sessionKey == "" {
		sessionKey, _ = ctx.Params["sessionKey"].(string)
	}

	// 解析 limit (默认 50)
	limit := 50
	if v, ok := ctx.Params["limit"]; ok {
		if f, ok := v.(float64); ok && f > 0 {
			limit = int(f)
		}
	}

	store := ctx.Context.SessionStore
	if store == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "session store not available"))
		return
	}

	// 获取 session
	if sessionKey == "" {
		// 使用主 session
		cfg := resolveConfigFromContext(ctx)
		if cfg != nil {
			sessionKey = scope.ResolveDefaultAgentId(cfg) + ":main"
		} else {
			sessionKey = "default:main"
		}
	}

	session := store.LoadSessionEntry(sessionKey)
	if session == nil {
		ctx.Respond(true, map[string]interface{}{
			"sessionKey": sessionKey,
			"messages":   []interface{}{},
			"total":      0,
		}, nil)
		return
	}

	// 从 transcript JSONL 文件读取消息
	storePath := ctx.Context.StorePath
	var messages []map[string]interface{}
	if session.SessionId != "" {
		rawMessages := ReadTranscriptMessages(session.SessionId, storePath, session.SessionFile)
		sanitized := StripEnvelopeFromMessages(rawMessages)

		// 按 limit 和字节限制裁剪
		hardMax := 1000
		defaultLimit := 200
		requested := limit
		if requested <= 0 {
			requested = defaultLimit
		}
		max := requested
		if max > hardMax {
			max = hardMax
		}
		if len(sanitized) > max {
			sanitized = sanitized[len(sanitized)-max:]
		}

		// 按 JSON 大小上限裁剪 (5MB)
		const maxChatHistoryBytes = 5 * 1024 * 1024
		messages = CapArrayByJSONBytes(sanitized, maxChatHistoryBytes)
	}
	if messages == nil {
		messages = []map[string]interface{}{}
	}

	ctx.Respond(true, map[string]interface{}{
		"sessionKey": sessionKey,
		"sessionId":  session.SessionId,
		"messages":   messages,
		"total":      len(messages),
		"title":      session.Label,
		"limit":      limit,
	}, nil)
}

// ---------- chat.abort ----------
// 对应 TS chat.ts L102-L133
// 中断指定 session 的运行中聊天。

func handleChatAbort(ctx *MethodHandlerContext) {
	sessionKey, _ := ctx.Params["sessionId"].(string)
	if sessionKey == "" {
		sessionKey, _ = ctx.Params["sessionKey"].(string)
	}
	runId, _ := ctx.Params["runId"].(string)

	chatState := ctx.Context.ChatState
	if chatState == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "chat state not available"))
		return
	}

	// 标记为 aborted
	if runId != "" {
		chatState.AbortedRuns.Store(runId, time.Now().UnixMilli())
	}
	if sessionKey != "" && chatState.Registry != nil {
		entry := chatState.Registry.Shift(sessionKey)
		if entry != nil {
			slog.Info("chat.abort: aborted run", "sessionKey", sessionKey, "runId", runId)
		}
	}

	// 广播 abort 事件
	if bc := ctx.Context.Broadcaster; bc != nil {
		bc.Broadcast("chat.abort", map[string]interface{}{
			"sessionKey": sessionKey,
			"runId":      runId,
			"ts":         time.Now().UnixMilli(),
		}, nil)
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":      true,
		"aborted": true,
	}, nil)
}

// ---------- chat.send ----------
// 对应 TS chat.ts L135-L695
// 核心聊天发送管线。
//
// 完整实现需依赖:
//   - dispatchInboundMessage (autoreply/reply/)
//   - session transcript read/write
//   - agent execution pipeline
// 当前为框架实现: 参数解析 + session resolve + broadcast 骨架。

func handleChatSend(ctx *MethodHandlerContext) {
	text, _ := ctx.Params["text"].(string)
	if text == "" {
		text, _ = ctx.Params["message"].(string) // 兼容前端 chat.ts 发送的 "message" 字段
	}
	sessionKey, _ := ctx.Params["sessionId"].(string)
	if sessionKey == "" {
		sessionKey, _ = ctx.Params["sessionKey"].(string)
	}
	agentId, _ := ctx.Params["agentId"].(string)
	idempotencyKey, _ := ctx.Params["idempotencyKey"].(string)

	// 解析 attachments
	var attachments []map[string]interface{}
	if v, ok := ctx.Params["attachments"]; ok {
		if arr, ok := v.([]interface{}); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					attachments = append(attachments, m)
				}
			}
		}
	}

	// 解析 session / agent
	cfg := resolveConfigFromContext(ctx)
	if agentId == "" && cfg != nil {
		agentId = scope.ResolveDefaultAgentId(cfg)
	}
	if sessionKey == "" {
		sessionKey = agentId + ":main"
	}

	// 幂等检查
	if idempotencyKey != "" && ctx.Context.IdempotencyCache != nil {
		check := ctx.Context.IdempotencyCache.CheckOrRegister(idempotencyKey)
		if check.IsDuplicate {
			if check.State == IdempotencyCompleted {
				ctx.Respond(true, check.CachedResult, nil)
				return
			}
			// InFlight — 正在处理中
			ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "duplicate request in flight"))
			return
		}
	}

	chatState := ctx.Context.ChatState
	if chatState == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "chat state not available"))
		return
	}

	// 生成 runId — 优先使用客户端的 idempotencyKey，确保事件 runId 与 UI 的 chatRunId 匹配
	runId := idempotencyKey
	if runId == "" {
		runId = fmt.Sprintf("run_%d", time.Now().UnixNano())
	}

	// 注册运行条目
	chatState.Registry.Add(sessionKey, ChatRunEntry{
		SessionKey:  sessionKey,
		ClientRunID: runId,
	})

	slog.Info("chat.send: dispatching",
		"sessionKey", sessionKey,
		"agentId", agentId,
		"text", truncateStr(text, 80),
		"attachments", len(attachments),
		"runId", runId,
	)

	// 广播 chat 开始事件
	if bc := ctx.Context.Broadcaster; bc != nil {
		bc.Broadcast("chat.delta", map[string]interface{}{
			"sessionKey": sessionKey,
			"runId":      runId,
			"agentId":    agentId,
			"type":       "start",
			"ts":         time.Now().UnixMilli(),
		}, nil)
	}

	// 立即返回 ack（非阻塞）
	ctx.Respond(true, map[string]interface{}{
		"runId":  runId,
		"status": "started",
		"ts":     time.Now().UnixMilli(), // F5: ACK 时间戳
	}, nil)

	// 在 goroutine 中异步运行 autoreply 管线
	// TS 对照: chat.ts L520-614 dispatchInboundMessage 异步流
	pipelineCtx, pipelineCancel := context.WithCancel(context.Background())
	broadcaster := ctx.Context.Broadcaster
	storePath := ctx.Context.StorePath
	dispatcher := ctx.Context.PipelineDispatcher

	go func() {
		defer pipelineCancel()
		defer func() {
			// 清理运行条目
			chatState.Registry.Remove(sessionKey, runId, sessionKey)
		}()

		// 订阅全局事件总线 → 广播 agent 工具事件到 WebSocket
		if broadcaster != nil {
			unsubAgentEvents := infra.OnAgentEvent(func(evt infra.AgentEventPayload) {
				if evt.RunID != runId {
					return
				}
				broadcaster.Broadcast("agent", evt, &BroadcastOptions{DropIfSlow: true})
			})
			defer unsubAgentEvents()
		}

		// ---- 确保 session 存在 & 解析 sessionId ----
		var resolvedSessionId string
		{
			store := ctx.Context.SessionStore
			if store != nil {
				entry := store.LoadSessionEntry(sessionKey)
				if entry == nil {
					// 首次对话 — 自动创建 session
					newId := fmt.Sprintf("session_%d", time.Now().UnixNano())
					entry = &SessionEntry{
						SessionKey: sessionKey,
						SessionId:  newId,
						Label:      sessionKey,
					}
					store.Save(entry)
					slog.Info("chat.send: auto-created session", "sessionKey", sessionKey, "sessionId", newId)
				}
				resolvedSessionId = entry.SessionId
			}
		}
		if resolvedSessionId == "" {
			resolvedSessionId = runId // 最后兜底
		}

		// ---- 持久化用户消息到 transcript ----
		AppendUserTranscriptMessage(AppendTranscriptParams{
			Message:         text,
			SessionID:       resolvedSessionId,
			StorePath:       storePath,
			CreateIfMissing: true,
		})

		// 处理附件：音频→STT 转录，文档→DocConv 转换
		enhancedText := processAttachmentsForChat(pipelineCtx, text, attachments, ctx.Context.ConfigLoader)

		// 构建 MsgContext
		msgCtx := &autoreply.MsgContext{
			Body:               enhancedText,
			BodyForAgent:       enhancedText,
			BodyForCommands:    enhancedText,
			RawBody:            text,
			CommandBody:        enhancedText,
			SessionKey:         sessionKey,
			Provider:           "webchat",
			Surface:            "webchat",
			OriginatingChannel: "webchat",
			ChatType:           "direct",
			CommandAuthorized:  true,
			MessageSid:         runId,
		}

		// 调用管线
		result := DispatchInboundMessage(pipelineCtx, DispatchInboundParams{
			MsgCtx:     msgCtx,
			SessionKey: sessionKey,
			AgentID:    agentId,
			StorePath:  storePath,
			RunID:      runId,
			Ctx:        pipelineCtx,
			Dispatcher: dispatcher,
		})

		if result.Error != nil {
			slog.Error("chat.send: pipeline error",
				"error", result.Error,
				"runId", runId,
				"sessionKey", sessionKey,
			)
			// 广播错误
			if broadcaster != nil {
				broadcaster.Broadcast("chat", map[string]interface{}{
					"runId":        runId,
					"sessionKey":   sessionKey,
					"state":        "error",
					"errorMessage": result.Error.Error(),
				}, nil)
			}
			return
		}

		// 合并回复
		combinedReply := CombineReplyPayloads(result.Replies)

		// 写入 transcript 并广播 final
		var message map[string]interface{}
		if combinedReply != "" {
			// 获取 session 信息
			var sessionId string
			store := ctx.Context.SessionStore
			if store != nil {
				if entry := store.LoadSessionEntry(sessionKey); entry != nil {
					sessionId = entry.SessionId
				}
			}
			if sessionId == "" {
				sessionId = runId
			}

			appended := AppendAssistantTranscriptMessage(AppendTranscriptParams{
				Message:         combinedReply,
				SessionID:       sessionId,
				StorePath:       storePath,
				CreateIfMissing: true,
			})
			if appended.OK {
				message = appended.Message
			} else {
				slog.Warn("chat.send: transcript append failed", "error", appended.Error)
				now := time.Now().UnixMilli()
				message = map[string]interface{}{
					"role": "assistant",
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": combinedReply},
					},
					"timestamp":  now,
					"stopReason": "stop",
					"usage":      map[string]interface{}{"input": 0, "output": 0, "totalTokens": 0},
				}
			}
		}

		// 广播 final
		if broadcaster != nil {
			broadcaster.Broadcast("chat", map[string]interface{}{
				"runId":      runId,
				"sessionKey": sessionKey,
				"state":      "final",
				"message":    message,
			}, nil)
		}

		slog.Info("chat.send: complete",
			"runId", runId,
			"sessionKey", sessionKey,
			"replyLength", len(combinedReply),
		)
	}()
}

// ---------- chat.inject ----------
// 对应 TS chat.ts (最后部分)
// 将 assistant 消息注入 transcript，不触发 agent。

func handleChatInject(ctx *MethodHandlerContext) {
	sessionKey, _ := ctx.Params["sessionId"].(string)
	if sessionKey == "" {
		sessionKey, _ = ctx.Params["sessionKey"].(string)
	}
	text, _ := ctx.Params["text"].(string)
	role, _ := ctx.Params["role"].(string)
	if role == "" {
		role = "assistant"
	}

	if text == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "text required"))
		return
	}

	store := ctx.Context.SessionStore
	if store == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "session store not available"))
		return
	}

	if sessionKey == "" {
		cfg := resolveConfigFromContext(ctx)
		if cfg != nil {
			sessionKey = scope.ResolveDefaultAgentId(cfg) + ":main"
		} else {
			sessionKey = "default:main"
		}
	}

	slog.Info("chat.inject", "sessionKey", sessionKey, "role", role, "textLen", len(text))

	// 解析 label
	label, _ := ctx.Params["label"].(string)

	// 加载 session 获取 transcript 路径
	session := store.LoadSessionEntry(sessionKey)
	var sessionId, storePath, sessionFile string
	if session != nil {
		sessionId = session.SessionId
		sessionFile = session.SessionFile
	}
	storePath = ctx.Context.StorePath

	if sessionId == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "session not found"))
		return
	}

	// 追加到 session transcript
	appended := AppendAssistantTranscriptMessage(AppendTranscriptParams{
		Message:         text,
		Label:           label,
		SessionID:       sessionId,
		StorePath:       storePath,
		SessionFile:     sessionFile,
		CreateIfMissing: true,
	})

	if !appended.OK || appended.MessageID == "" {
		errMsg := "unknown error"
		if appended.Error != "" {
			errMsg = appended.Error
		}
		ctx.Respond(false, nil, NewErrorShape(ErrCodeServiceUnavailable, "failed to write transcript: "+errMsg))
		return
	}

	// 广播到 webchat 实现即时 UI 更新
	if bc := ctx.Context.Broadcaster; bc != nil {
		chatPayload := map[string]interface{}{
			"runId":      fmt.Sprintf("inject-%s", appended.MessageID),
			"sessionKey": sessionKey,
			"seq":        0,
			"state":      "final",
			"message":    appended.Message,
		}
		bc.Broadcast("chat", chatPayload, nil)
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":        true,
		"messageId": appended.MessageID,
	}, nil)
}

// ---------- 辅助函数 ----------

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// processAttachmentsForChat 处理 chat.send 附件：音频→STT，文档→DocConv。
// 将转录/转换结果追加到消息文本中返回增强后的文本。
func processAttachmentsForChat(ctx context.Context, text string, attachments []map[string]interface{}, cfgLoader interface {
	LoadConfig() (*types.OpenAcosmiConfig, error)
}) string {
	if len(attachments) == 0 || cfgLoader == nil {
		return text
	}

	cfg, err := cfgLoader.LoadConfig()
	if err != nil || cfg == nil {
		return text
	}

	var parts []string
	if text != "" {
		parts = append(parts, text)
	}

	for _, att := range attachments {
		attType, _ := att["type"].(string)
		contentB64, _ := att["content"].(string)
		mimeType, _ := att["mimeType"].(string)
		fileName, _ := att["fileName"].(string)

		if contentB64 == "" {
			continue
		}

		// H-05: 限制 base64 大小（解码前检查，25 MB decoded ≈ 34 MB base64）
		const maxBase64Len = 34 * 1024 * 1024
		if len(contentB64) > maxBase64Len {
			parts = append(parts, "[附件: 数据过大]")
			continue
		}

		switch attType {
		case "audio":
			if cfg.STT == nil || cfg.STT.Provider == "" {
				parts = append(parts, "[语音附件: STT 未配置]")
				continue
			}
			data, decErr := base64.StdEncoding.DecodeString(contentB64)
			if decErr != nil {
				parts = append(parts, "[语音附件: 解码失败]")
				continue
			}
			provider, provErr := media.NewSTTProvider(cfg.STT)
			if provErr != nil {
				parts = append(parts, "[语音附件: STT 初始化失败]")
				continue
			}
			if mimeType == "" {
				mimeType = "audio/webm"
			}
			sttCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			transcript, sttErr := provider.Transcribe(sttCtx, data, mimeType)
			cancel()
			if sttErr != nil {
				slog.Error("chat.send: STT failed", "error", sttErr)
				parts = append(parts, "[语音转录失败]")
			} else {
				parts = append(parts, fmt.Sprintf("[语音转录]: %s", transcript))
			}

		case "document":
			if cfg.DocConv == nil || cfg.DocConv.Provider == "" {
				if fileName != "" {
					parts = append(parts, fmt.Sprintf("[文件: %s]", fileName))
				}
				continue
			}
			if fileName == "" {
				fileName = "untitled"
			}
			if !media.IsSupportedFormat(fileName) {
				parts = append(parts, fmt.Sprintf("[文件: %s, 格式不支持转换]", fileName))
				continue
			}
			data, decErr := base64.StdEncoding.DecodeString(contentB64)
			if decErr != nil {
				parts = append(parts, fmt.Sprintf("[文件: %s, 解码失败]", fileName))
				continue
			}
			converter, convErr := media.NewDocConverter(cfg.DocConv)
			if convErr != nil {
				parts = append(parts, fmt.Sprintf("[文件: %s, 转换器初始化失败]", fileName))
				continue
			}
			convCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			markdown, convErr2 := converter.Convert(convCtx, data, mimeType, fileName)
			cancel()
			if convErr2 != nil {
				slog.Error("chat.send: DocConv failed", "file", fileName, "error", convErr2)
				parts = append(parts, fmt.Sprintf("[文件: %s, 转换失败]", fileName))
			} else {
				parts = append(parts, fmt.Sprintf("[文件: %s]\n%s", fileName, markdown))
			}
		}
	}

	if len(parts) <= 1 {
		return text
	}
	return strings.Join(parts, "\n")
}
