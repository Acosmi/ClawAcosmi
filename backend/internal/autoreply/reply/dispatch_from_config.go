package reply

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/session"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// TS 对照: auto-reply/reply/dispatch-from-config.ts (459L)
// 补全覆盖率从 32% → ≥80%。

// audioPlaceholderRe 音频占位符匹配。
var audioPlaceholderRe = regexp.MustCompile(`(?i)^<media:audio>(\s*\([^)]*\))?$`)
var audioHeaderRe = regexp.MustCompile(`(?i)^\[Audio\b`)

// ---------- 类型定义 ----------

// DispatchFromConfigResult 配置分发结果。
// TS 对照: dispatch-from-config.ts L77-80
type DispatchFromConfigResult struct {
	QueuedFinal bool
	Counts      map[ReplyDispatchKind]int
}

// TtsApplier TTS 附加处理器（DI 注入）。
// 参数: payload, cfg, channel, kind（"tool"|"block"|"final"）, inboundAudio, sessionTtsAuto.
// 返回附加了 TTS 的 payload。
type TtsApplier func(
	payload autoreply.ReplyPayload,
	cfg *types.OpenAcosmiConfig,
	channel string,
	kind string,
	inboundAudio bool,
	ttsAuto string,
) (autoreply.ReplyPayload, error)

// HookRunnerFunc 消息接收钩子运行器（DI 注入，fire-and-forget）。
type HookRunnerFunc func(ctx *autoreply.MsgContext, channel, accountID, conversationID string)

// ReplyResolverFunc 回复解析器。
// TS 对照: getReplyFromConfig (核心回复获取函数)。
type ReplyResolverFunc func(
	ctx context.Context,
	msgCtx *autoreply.MsgContext,
	cfg *types.OpenAcosmiConfig,
	opts *ReplyResolverOptions,
) ([]autoreply.ReplyPayload, error)

// ReplyResolverOptions 回复解析器选项。
type ReplyResolverOptions struct {
	OnToolResult func(payload autoreply.ReplyPayload) error
	OnBlockReply func(payload autoreply.ReplyPayload, abortCtx context.Context) error
}

// FastAbortFunc 快速中止检测器（DI 注入）。
type FastAbortFunc func(ctx *autoreply.MsgContext, cfg *types.OpenAcosmiConfig) (*FastAbortResult, error)

// DuplicateChecker 去重检测器。
type DuplicateChecker func(ctx *autoreply.MsgContext) bool

// DiagnosticsChecker 诊断是否启用检测器。
type DiagnosticsChecker func(cfg *types.OpenAcosmiConfig) bool

// DispatchFromConfigParams 配置分发参数。
type DispatchFromConfigParams struct {
	Ctx        *autoreply.MsgContext
	Cfg        *types.OpenAcosmiConfig
	Dispatcher *ReplyDispatcher
	Router     *ReplyRouter // 回复路由器（DI 注入）

	// DI 接口
	ReplyResolver      ReplyResolverFunc
	OnDuplicateCheck   DuplicateChecker
	TtsApplier         TtsApplier
	HookRunner         HookRunnerFunc
	FastAbortChecker   FastAbortFunc
	DiagnosticsEnabled bool

	// Session TTS auto 解析（DI 注入，替代直接 session store 访问）
	SessionTtsAutoResolver func(ctx *autoreply.MsgContext, cfg *types.OpenAcosmiConfig) string
}

// ---------- 音频检测 ----------

// IsInboundAudioContext 判断入站消息是否为音频上下文。
// TS 对照: dispatch-from-config.ts L26-54
func IsInboundAudioContext(ctx *autoreply.MsgContext) bool {
	if ctx == nil {
		return false
	}

	// 检查 MediaType
	if ctx.MediaType != "" {
		mt := normalizeMediaType(ctx.MediaType)
		if mt == "audio" || strings.HasPrefix(mt, "audio/") {
			return true
		}
	}

	// 检查 body
	body := resolveBodyForAudioCheck(ctx)
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return false
	}
	if audioPlaceholderRe.MatchString(trimmed) {
		return true
	}
	return audioHeaderRe.MatchString(trimmed)
}

func normalizeMediaType(value string) string {
	parts := strings.SplitN(value, ";", 2)
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

func resolveBodyForAudioCheck(ctx *autoreply.MsgContext) string {
	if ctx.BodyForCommands != "" {
		return ctx.BodyForCommands
	}
	if ctx.CommandBody != "" {
		return ctx.CommandBody
	}
	if ctx.RawBody != "" {
		return ctx.RawBody
	}
	return ctx.Body
}

// ---------- Session TTS Auto 解析 ----------

// ResolveSessionTtsAuto 从 session store 解析 TTS auto 模式。
// TS 对照: dispatch-from-config.ts resolveSessionTtsAuto (L56-75)
func ResolveSessionTtsAuto(
	ctx *autoreply.MsgContext,
	cfg *types.OpenAcosmiConfig,
	storeLoader func(storePath string) (map[string]*session.SessionEntry, error),
	storePathResolver func(cfg *types.OpenAcosmiConfig, agentID string) string,
	agentIDResolver func(sessionKey string, cfg *types.OpenAcosmiConfig) string,
) string {
	if ctx == nil {
		return ""
	}

	// 解析目标 sessionKey
	targetKey := ""
	if ctx.CommandSource == "native" {
		targetKey = strings.TrimSpace(ctx.CommandTargetSessionKey)
	}
	if targetKey == "" {
		targetKey = strings.TrimSpace(ctx.SessionKey)
	}
	if targetKey == "" {
		return ""
	}

	// 解析 agentID 和 storePath
	agentID := ""
	if agentIDResolver != nil {
		agentID = agentIDResolver(targetKey, cfg)
	}
	storePath := ""
	if storePathResolver != nil {
		storePath = storePathResolver(cfg, agentID)
	}
	if storePath == "" || storeLoader == nil {
		return ""
	}

	// 加载 session store
	store, err := storeLoader(storePath)
	if err != nil {
		return ""
	}

	// 查找 entry（先 lowercase，再原始 key）
	entry := store[strings.ToLower(targetKey)]
	if entry == nil {
		entry = store[targetKey]
	}
	if entry == nil {
		return ""
	}

	return entry.TtsAuto
}

// ---------- 诊断记录 ----------

// recordProcessed 记录消息处理结果诊断日志。
// TS 对照: dispatch-from-config.ts recordProcessed (L98-118)
func recordProcessed(
	diagnosticsEnabled bool,
	channel, chatID, messageID, sessionKey string,
	startTime time.Time,
	outcome string, // "completed" | "skipped" | "error"
	reason, errMsg string,
) {
	if !diagnosticsEnabled {
		return
	}
	duration := time.Since(startTime)
	attrs := []slog.Attr{
		slog.String("channel", channel),
		slog.String("chatId", chatID),
		slog.String("messageId", messageID),
		slog.String("sessionKey", sessionKey),
		slog.Int64("durationMs", duration.Milliseconds()),
		slog.String("outcome", outcome),
	}
	if reason != "" {
		attrs = append(attrs, slog.String("reason", reason))
	}
	if errMsg != "" {
		attrs = append(attrs, slog.String("error", errMsg))
	}
	slog.LogAttrs(context.Background(), slog.LevelInfo, "dispatch: message processed", attrs...)
}

// markProcessing 标记会话为处理中。
// TS 对照: dispatch-from-config.ts markProcessing (L120-130)
func markProcessing(diagnosticsEnabled bool, sessionKey, channel string) {
	if !diagnosticsEnabled || sessionKey == "" {
		return
	}
	slog.Info("dispatch: session state change",
		"sessionKey", sessionKey,
		"channel", channel,
		"state", "processing",
		"reason", "message_start",
	)
}

// markIdle 标记会话为空闲。
// TS 对照: dispatch-from-config.ts markIdle (L132-141)
func markIdle(diagnosticsEnabled bool, sessionKey, reason string) {
	if !diagnosticsEnabled || sessionKey == "" {
		return
	}
	slog.Info("dispatch: session state change",
		"sessionKey", sessionKey,
		"state", "idle",
		"reason", reason,
	)
}

// ---------- 主分发函数 ----------

// DispatchReplyFromConfig 从配置分发回复。
// TS 对照: dispatch-from-config.ts dispatchReplyFromConfig (L82-458)
func DispatchReplyFromConfig(ctx context.Context, params DispatchFromConfigParams) (*DispatchFromConfigResult, error) {
	msgCtx := params.Ctx
	cfg := params.Cfg
	dispatcher := params.Dispatcher

	// 解析诊断上下文
	diagnostics := params.DiagnosticsEnabled
	channel := strings.ToLower(resolveChannel(msgCtx))
	chatID := resolveChatID(msgCtx)
	messageID := resolveMessageID(msgCtx)
	sessionKey := msgCtx.SessionKey
	var startTime time.Time
	if diagnostics {
		startTime = time.Now()
	}

	doRecordProcessed := func(outcome, reason, errMsg string) {
		recordProcessed(diagnostics, channel, chatID, messageID, sessionKey, startTime, outcome, reason, errMsg)
	}

	// 1. 去重检查
	if params.OnDuplicateCheck != nil && params.OnDuplicateCheck(msgCtx) {
		doRecordProcessed("skipped", "duplicate", "")
		return &DispatchFromConfigResult{
			QueuedFinal: false,
			Counts:      dispatcher.GetQueuedCounts(),
		}, nil
	}

	// 2. 音频检测 + TTS auto 解析
	inboundAudio := IsInboundAudioContext(msgCtx)
	sessionTtsAuto := ""
	if params.SessionTtsAutoResolver != nil {
		sessionTtsAuto = params.SessionTtsAutoResolver(msgCtx, cfg)
	}

	// 3. Hook runner: message_received（fire-and-forget）
	if params.HookRunner != nil {
		channelID := strings.ToLower(dispatchFirstNonEmpty(
			msgCtx.OriginatingChannel, msgCtx.Surface, msgCtx.Provider,
		))
		conversationID := dispatchFirstNonEmpty(msgCtx.OriginatingTo, msgCtx.To, msgCtx.From)
		go params.HookRunner(msgCtx, channelID, msgCtx.AccountID, conversationID)
	}

	// 4. 跨 channel 路由检测
	originatingChannel := msgCtx.OriginatingChannel
	originatingTo := msgCtx.OriginatingTo
	currentSurface := strings.ToLower(dispatchFirstNonEmpty(msgCtx.Surface, msgCtx.Provider))

	shouldRouteToOriginating := false
	if params.Router != nil && originatingChannel != "" && originatingTo != "" {
		shouldRouteToOriginating = params.Router.IsRoutableChannel(originatingChannel) &&
			originatingChannel != currentSurface
	}

	// TTS channel: 路由到 originating 时使用 originating channel
	ttsChannel := currentSurface
	if shouldRouteToOriginating {
		ttsChannel = originatingChannel
	}

	// sendPayloadAsync 辅助函数：跨 channel 路由发送。
	// TS 对照: dispatch-from-config.ts sendPayloadAsync (L220-247)
	sendPayloadAsync := func(payload autoreply.ReplyPayload, abortCtx context.Context) error {
		if originatingChannel == "" || originatingTo == "" {
			return nil
		}
		if abortCtx != nil {
			select {
			case <-abortCtx.Done():
				return abortCtx.Err()
			default:
			}
		}
		result := params.Router.RouteReply(RouteReplyParams{
			Payload:    payload,
			Channel:    originatingChannel,
			To:         originatingTo,
			SessionKey: msgCtx.SessionKey,
			AccountID:  msgCtx.AccountID,
			ThreadID:   msgCtx.MessageThreadID,
			AbortSignal: func() context.Context {
				if abortCtx != nil {
					return abortCtx
				}
				return ctx
			}(),
		})
		if !result.OK {
			slog.Debug("dispatch-from-config: route-reply failed",
				"error", result.Error,
			)
		}
		return nil
	}

	// 应用 TTS 到 payload 的辅助函数。
	applyTts := func(payload autoreply.ReplyPayload, kind string) autoreply.ReplyPayload {
		if params.TtsApplier == nil {
			return payload
		}
		result, err := params.TtsApplier(payload, cfg, ttsChannel, kind, inboundAudio, sessionTtsAuto)
		if err != nil {
			slog.Debug("dispatch-from-config: TTS apply failed",
				"kind", kind,
				"error", err.Error(),
			)
			return payload
		}
		return result
	}

	markProcessing(diagnostics, sessionKey, channel)

	// 5. 快速中止检测
	if params.FastAbortChecker != nil {
		fastAbort, err := params.FastAbortChecker(msgCtx, cfg)
		if err != nil {
			doRecordProcessed("error", "", err.Error())
			markIdle(diagnostics, sessionKey, "message_error")
			return nil, err
		}
		if fastAbort != nil && fastAbort.Aborted {
			payload := autoreply.ReplyPayload{
				Text: FormatAbortReplyText(fastAbort.StoppedCount),
			}
			queuedFinal := false
			routedFinalCount := 0
			if shouldRouteToOriginating {
				result := params.Router.RouteReply(RouteReplyParams{
					Payload:     payload,
					Channel:     originatingChannel,
					To:          originatingTo,
					SessionKey:  msgCtx.SessionKey,
					AccountID:   msgCtx.AccountID,
					ThreadID:    msgCtx.MessageThreadID,
					AbortSignal: ctx,
				})
				queuedFinal = result.OK
				if result.OK {
					routedFinalCount++
				}
			} else {
				queuedFinal = dispatcher.SendFinalReply(payload)
			}
			dispatcher.WaitForIdle()
			counts := dispatcher.GetQueuedCounts()
			counts[DispatchFinal] += routedFinalCount
			doRecordProcessed("completed", "fast_abort", "")
			markIdle(diagnostics, sessionKey, "message_completed")
			return &DispatchFromConfigResult{QueuedFinal: queuedFinal, Counts: counts}, nil
		}
	}

	// 6. block text 累积（用于 streaming 结束后生成 TTS-only payload）。
	accumulatedBlockText := ""
	blockCount := 0

	shouldSendToolSummaries := msgCtx.ChatType != "group" && msgCtx.CommandSource != "native"

	// 7. 获取回复
	var replies []autoreply.ReplyPayload
	if params.ReplyResolver != nil {
		var resolverOpts ReplyResolverOptions

		// onToolResult 回调
		if shouldSendToolSummaries {
			resolverOpts.OnToolResult = func(payload autoreply.ReplyPayload) error {
				ttsPayload := applyTts(payload, "tool")
				if shouldRouteToOriginating {
					return sendPayloadAsync(ttsPayload, ctx)
				}
				dispatcher.SendToolResult(ttsPayload)
				return nil
			}
		}

		// onBlockReply 回调
		// TS 对照: dispatch-from-config.ts L321-346
		resolverOpts.OnBlockReply = func(payload autoreply.ReplyPayload, abortCtx context.Context) error {
			if payload.Text != "" {
				if accumulatedBlockText != "" {
					accumulatedBlockText += "\n"
				}
				accumulatedBlockText += payload.Text
				blockCount++
			}
			ttsPayload := applyTts(payload, "block")
			if shouldRouteToOriginating {
				return sendPayloadAsync(ttsPayload, abortCtx)
			}
			dispatcher.SendBlockReply(ttsPayload)
			return nil
		}

		var err error
		replies, err = params.ReplyResolver(ctx, msgCtx, cfg, &resolverOpts)
		if err != nil {
			doRecordProcessed("error", "", err.Error())
			markIdle(diagnostics, sessionKey, "message_error")
			return nil, err
		}
	}

	// 8. 分发最终回复
	queuedFinal := false
	routedFinalCount := 0
	for _, reply := range replies {
		ttsReply := applyTts(reply, "final")
		if shouldRouteToOriginating {
			result := params.Router.RouteReply(RouteReplyParams{
				Payload:     ttsReply,
				Channel:     originatingChannel,
				To:          originatingTo,
				SessionKey:  msgCtx.SessionKey,
				AccountID:   msgCtx.AccountID,
				ThreadID:    msgCtx.MessageThreadID,
				AbortSignal: ctx,
			})
			queuedFinal = result.OK || queuedFinal
			if result.OK {
				routedFinalCount++
			}
		} else {
			queuedFinal = dispatcher.SendFinalReply(ttsReply) || queuedFinal
		}
	}

	// 9. 块流式完成后 TTS-only 生成
	// TS 对照: dispatch-from-config.ts L389-443
	if params.TtsApplier != nil && len(replies) == 0 && blockCount > 0 && strings.TrimSpace(accumulatedBlockText) != "" {
		syntheticPayload := autoreply.ReplyPayload{Text: accumulatedBlockText}
		ttsSynthetic := applyTts(syntheticPayload, "final")
		if ttsSynthetic.MediaURL != "" {
			// TTS-only payload: 不含文本（避免重复块内容），仅包含音频
			ttsOnlyPayload := autoreply.ReplyPayload{
				MediaURL:     ttsSynthetic.MediaURL,
				AudioAsVoice: ttsSynthetic.AudioAsVoice,
			}
			if shouldRouteToOriginating {
				result := params.Router.RouteReply(RouteReplyParams{
					Payload:     ttsOnlyPayload,
					Channel:     originatingChannel,
					To:          originatingTo,
					SessionKey:  msgCtx.SessionKey,
					AccountID:   msgCtx.AccountID,
					ThreadID:    msgCtx.MessageThreadID,
					AbortSignal: ctx,
				})
				queuedFinal = result.OK || queuedFinal
				if result.OK {
					routedFinalCount++
				}
			} else {
				if dispatcher.SendFinalReply(ttsOnlyPayload) {
					queuedFinal = true
				}
			}
		}
	}

	// 10. 等待空闲 + 汇总
	dispatcher.WaitForIdle()
	counts := dispatcher.GetQueuedCounts()
	counts[DispatchFinal] += routedFinalCount
	doRecordProcessed("completed", "", "")
	markIdle(diagnostics, sessionKey, "message_completed")
	return &DispatchFromConfigResult{QueuedFinal: queuedFinal, Counts: counts}, nil
}

// ---------- 内部辅助 ----------

func resolveChannel(ctx *autoreply.MsgContext) string {
	if ctx.Surface != "" {
		return ctx.Surface
	}
	if ctx.Provider != "" {
		return ctx.Provider
	}
	return "unknown"
}

func resolveChatID(ctx *autoreply.MsgContext) string {
	if ctx.To != "" {
		return ctx.To
	}
	return ctx.From
}

func resolveMessageID(ctx *autoreply.MsgContext) string {
	if ctx.MessageSid != "" {
		return ctx.MessageSid
	}
	if ctx.MessageSidFirst != "" {
		return ctx.MessageSidFirst
	}
	return ctx.MessageSidLast
}

func dispatchFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// resolveChannelForDispatch 解析消息分发的 channel。
// TS 对照: const channel = String(ctx.Surface ?? ctx.Provider ?? "unknown").toLowerCase()
func resolveChannelForDispatch(ctx *autoreply.MsgContext) string {
	return strings.ToLower(resolveChannel(ctx))
}

// ---------- 已弃用兼容 ----------

// 以下类型已被新的 DI 回调替代，但保留供外部引用。

// resolveSessionTtsAutoCompat 兼容封装（简化签名）。
// 调用方应在 DispatchFromConfigParams.SessionTtsAutoResolver 中注入完整实现。
func resolveSessionTtsAutoCompat(ctx *autoreply.MsgContext, cfg *types.OpenAcosmiConfig) string {
	_ = ctx
	_ = cfg
	return "" // 无 DI 注入时返回空
}

// formatDiagnosticMessage 格式化诊断消息（内部用）。
func formatDiagnosticMessage(template string, args ...any) string {
	return fmt.Sprintf(template, args...)
}
