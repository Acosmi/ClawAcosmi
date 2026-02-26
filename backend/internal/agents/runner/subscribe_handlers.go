package runner

// ============================================================================
// 流式订阅层 — 事件 Handler
// 对应 TS: pi-embedded-subscribe.handlers.*.ts (700L 合计)
// ============================================================================

import (
	"fmt"
	"strings"
)

// SubscribeContext 事件处理上下文。
type SubscribeContext struct {
	Params SubscribeParams
	State  *SubscribeState
}

// NewSubscribeContext 创建订阅上下文。
func NewSubscribeContext(params SubscribeParams) *SubscribeContext {
	return &SubscribeContext{
		Params: params,
		State:  NewSubscribeState(),
	}
}

// ---------- Event 类型 ----------

// SubscribeEventType 事件类型枚举。
type SubscribeEventType string

const (
	EventMessageStart      SubscribeEventType = "message_start"
	EventMessageUpdate     SubscribeEventType = "message_update"
	EventMessageEnd        SubscribeEventType = "message_end"
	EventToolExecStart     SubscribeEventType = "tool_execution_start"
	EventToolExecUpdate    SubscribeEventType = "tool_execution_update"
	EventToolExecEnd       SubscribeEventType = "tool_execution_end"
	EventAgentSessionStart SubscribeEventType = "agent_start"
	EventAgentSessionEnd   SubscribeEventType = "agent_end"
	EventAutoCompactStart  SubscribeEventType = "auto_compaction_start"
	EventAutoCompactEnd    SubscribeEventType = "auto_compaction_end"
)

// SubscribeEvent 统一事件结构。
type SubscribeEvent struct {
	Type           SubscribeEventType
	SubType        string // "text_delta" | "text_start" | "text_end" (S4)
	Text           string // message_update: delta text
	Content        string // text_start/text_end: full content
	ReasoningText  string // reasoning 流内容 (S2)
	ReasoningLevel ReasoningLevel
	ToolName       string
	ToolID         string
	Args           map[string]interface{}
	IsError        bool
	Result         interface{}
	WillRetry      bool
	Usage          *NormalizedUsage
}

// HandleEvent 分发事件。
func (ctx *SubscribeContext) HandleEvent(evt SubscribeEvent) {
	switch evt.Type {
	case EventMessageStart:
		ctx.handleMessageStart(evt)
	case EventMessageUpdate:
		ctx.handleMessageUpdate(evt)
	case EventMessageEnd:
		ctx.handleMessageEnd(evt)
	case EventToolExecStart:
		ctx.handleToolStart(evt)
	case EventToolExecUpdate:
		ctx.handleToolUpdate(evt)
	case EventToolExecEnd:
		ctx.handleToolEnd(evt)
	case EventAgentSessionStart:
		ctx.handleAgentStart()
	case EventAgentSessionEnd:
		ctx.handleAgentEnd()
	case EventAutoCompactStart:
		ctx.handleCompactionStart()
	case EventAutoCompactEnd:
		ctx.handleCompactionEnd(evt)
	}
}

// ---------- Message Handlers ----------

func (ctx *SubscribeContext) handleMessageStart(_ SubscribeEvent) {
	s := ctx.State
	s.AssistantMessageIndex++
	s.ResetAssistantMessage(len(s.AssistantTexts))

	// S7: assistant message start 回调
	if ctx.Params.OnAssistantStart != nil {
		ctx.Params.OnAssistantStart(s.AssistantMessageIndex)
	}

	ctx.emitAgentEvent("lifecycle", map[string]interface{}{
		"phase": "message_start",
	})
}

func (ctx *SubscribeContext) handleMessageUpdate(evt SubscribeEvent) {
	s := ctx.State

	// S2: Reasoning 流分发 — 独立于正文处理
	if evt.ReasoningText != "" && ctx.Params.OnReasoning != nil {
		ctx.Params.OnReasoning(ReasoningChunk{
			Level: evt.ReasoningLevel,
			Text:  evt.ReasoningText,
		})
		return
	}

	// S4: 区分 text_delta / text_start / text_end 子事件
	subType := evt.SubType
	if subType == "" {
		subType = "text_delta"
	}
	if subType != "text_delta" && subType != "text_start" && subType != "text_end" {
		return
	}

	// 计算 chunk (TS L84-101 delta 补偿)
	delta := evt.Text
	content := evt.Content
	var chunk string

	switch subType {
	case "text_delta":
		chunk = delta
	case "text_start", "text_end":
		if delta != "" {
			chunk = delta
		} else if content != "" {
			// 某些 provider 在 text_end 重发完整 content，
			// 只追加差异部分保持单调性
			if strings.HasPrefix(content, s.DeltaBuffer) {
				chunk = content[len(s.DeltaBuffer):]
			} else if strings.HasPrefix(s.DeltaBuffer, content) {
				chunk = ""
			} else if !strings.Contains(s.DeltaBuffer, content) {
				chunk = content
			}
		}
	}

	if chunk == "" {
		return
	}

	// 累积 delta
	s.DeltaBuffer += chunk

	// 标签剥离
	stripped := StripBlockTags(chunk, &s.BlockState)

	// 更新最后流式内容
	if stripped != "" {
		s.LastStreamedAssistant += stripped
		s.EmittedAssistantUpdate = true
	}

	// Block chunking
	if ctx.Params.OnBlockReply != nil && !s.SuppressBlockChunks && stripped != "" {
		s.BlockBuffer += stripped
	}

	// text_end 时刷新 block (text_end 模式)
	if subType == "text_end" && ctx.Params.BlockReplyMode == "text_end" {
		ctx.flushBlockBuffer()
	}

	// Partial reply
	if ctx.Params.OnPartial != nil && stripped != "" {
		ctx.Params.OnPartial(stripped)
	}
}

func (ctx *SubscribeContext) handleMessageEnd(evt SubscribeEvent) {
	s := ctx.State

	// DeltaBuffer 中的内容已由 handleMessageUpdate 逐 chunk 处理，
	// 此处仅清空残余（如未闭合标签中的内容不应输出）
	s.DeltaBuffer = ""

	// 提取最终文本 + S3: 移除指令行
	finalText := strings.TrimSpace(s.LastStreamedAssistant)
	if finalText != "" {
		finalText = strings.TrimSpace(StripDirectiveLines(finalText))
	}

	// 去重: 检查是否已由消息工具发送
	if finalText != "" && len(s.MessagingTextsNorm) > 0 {
		norm := NormalizeTextForComparison(finalText)
		for _, sent := range s.MessagingTextsNorm {
			if norm == sent {
				subscribeLog.Debug("suppressing duplicate messaging tool text",
					"runId", ctx.Params.RunID,
					"len", len(finalText))
				finalText = ""
				s.SuppressBlockChunks = true
				break
			}
		}
	}

	// 添加到 assistantTexts (S9: 去重检查)
	if finalText != "" {
		dup := false
		for _, prev := range s.AssistantTexts {
			if prev == finalText {
				dup = true
				break
			}
		}
		if !dup {
			s.AssistantTexts = append(s.AssistantTexts, finalText)
		}
	}

	// 刷新 block buffer (text_end 模式)
	if ctx.Params.BlockReplyMode != "message_end" {
		ctx.flushBlockBuffer()
	}

	// 记录 usage
	if evt.Usage != nil {
		s.RecordUsage(evt.Usage)
	}

	s.BlockState = TagStripState{}
}

// ---------- Tool Handlers ----------

func (ctx *SubscribeContext) handleToolStart(evt SubscribeEvent) {
	s := ctx.State
	toolName := evt.ToolName
	toolID := evt.ToolID

	// 刷新 block buffer
	ctx.flushBlockBuffer()
	if ctx.Params.OnBlockFlush != nil {
		ctx.Params.OnBlockFlush()
	}

	// 记录工具元数据
	meta := inferToolMeta(toolName, evt.Args)
	s.ToolMetaByID[toolID] = meta

	subscribeLog.Debug("tool start",
		"runId", ctx.Params.RunID,
		"tool", toolName,
		"toolCallId", toolID)

	// 发射工具摘要
	if ctx.Params.OnToolResult != nil && !s.ToolSummaryByID[toolID] {
		s.ToolSummaryByID[toolID] = true
		summary := toolName
		if meta != "" {
			summary = fmt.Sprintf("%s (%s)", toolName, meta)
		}
		ctx.Params.OnToolResult(summary)
	}

	ctx.emitAgentEvent("tool", map[string]interface{}{
		"phase": "start", "name": toolName, "toolCallId": toolID,
	})

	// 跟踪消息工具
	ctx.trackMessagingToolStart(toolName, toolID, evt.Args)
}

func (ctx *SubscribeContext) handleToolUpdate(evt SubscribeEvent) {
	ctx.emitAgentEvent("tool", map[string]interface{}{
		"phase": "update", "name": evt.ToolName, "toolCallId": evt.ToolID,
	})
}

func (ctx *SubscribeContext) handleToolEnd(evt SubscribeEvent) {
	s := ctx.State
	toolName := evt.ToolName
	toolID := evt.ToolID
	isError := evt.IsError

	meta := s.ToolMetaByID[toolID]
	s.ToolMetas = append(s.ToolMetas, ToolMeta{ToolName: toolName, Meta: meta})
	delete(s.ToolMetaByID, toolID)
	delete(s.ToolSummaryByID, toolID)

	if isError {
		errMsg := fmt.Sprintf("%v", evt.Result)
		s.LastToolError = &ToolErrorEntry{
			ToolName: toolName, Meta: meta, Error: errMsg,
		}
	}

	// 提交/丢弃消息工具文本
	ctx.commitMessagingTool(toolID, isError)

	ctx.emitAgentEvent("tool", map[string]interface{}{
		"phase": "result", "name": toolName, "toolCallId": toolID,
		"isError": isError,
	})

	subscribeLog.Debug("tool end",
		"runId", ctx.Params.RunID,
		"tool", toolName,
		"toolCallId", toolID)
}

// ---------- Lifecycle Handlers ----------

func (ctx *SubscribeContext) handleAgentStart() {
	ctx.emitAgentEvent("lifecycle", map[string]interface{}{
		"phase": "start",
	})
}

func (ctx *SubscribeContext) handleAgentEnd() {
	// 刷新剩余 block buffer
	ctx.flushBlockBuffer()

	// 解析 compaction 等待
	s := ctx.State
	if s.PendingCompactionRetry > 0 && s.CompactionRetryDone != nil {
		close(s.CompactionRetryDone)
		s.CompactionRetryDone = nil
	}

	ctx.emitAgentEvent("lifecycle", map[string]interface{}{
		"phase": "end",
	})
}

func (ctx *SubscribeContext) handleCompactionStart() {
	s := ctx.State
	s.CompactionInFlight = true
	s.IncrementCompaction()

	ctx.emitAgentEvent("compaction", map[string]interface{}{
		"phase": "start",
	})
}

func (ctx *SubscribeContext) handleCompactionEnd(evt SubscribeEvent) {
	s := ctx.State
	s.CompactionInFlight = false

	if evt.WillRetry {
		s.PendingCompactionRetry++
		s.ResetForCompactionRetry()
	}

	ctx.emitAgentEvent("compaction", map[string]interface{}{
		"phase": "end", "willRetry": evt.WillRetry,
	})
}

// ---------- 内部辅助 ----------

func (ctx *SubscribeContext) flushBlockBuffer() {
	s := ctx.State
	if ctx.Params.OnBlockReply == nil || s.BlockBuffer == "" {
		return
	}
	text := s.BlockBuffer
	s.BlockBuffer = ""
	// S8: 去重 — 与上次发出的 block reply 相同则跳过
	if text == s.LastBlockReplyText {
		return
	}
	s.LastBlockReplyText = text
	ctx.Params.OnBlockReply(text)
}

func (ctx *SubscribeContext) emitAgentEvent(stream string, data map[string]interface{}) {
	if ctx.Params.OnAgentEvent != nil {
		ctx.Params.OnAgentEvent(stream, data)
	}
}

func (ctx *SubscribeContext) trackMessagingToolStart(toolName, toolID string, args map[string]interface{}) {
	if !isMessagingToolName(toolName) {
		return
	}
	text, _ := args["content"].(string)
	if text == "" {
		text, _ = args["message"].(string)
	}
	if text != "" {
		ctx.State.PendingMsgTexts[toolID] = text
	}
}

func (ctx *SubscribeContext) commitMessagingTool(toolID string, isError bool) {
	s := ctx.State
	text, hasTxt := s.PendingMsgTexts[toolID]
	target, hasTgt := s.PendingMsgTargets[toolID]
	delete(s.PendingMsgTexts, toolID)
	delete(s.PendingMsgTargets, toolID)

	if isError {
		return
	}
	if hasTxt && text != "" {
		s.MessagingTexts = append(s.MessagingTexts, text)
		s.MessagingTextsNorm = append(s.MessagingTextsNorm, NormalizeTextForComparison(text))
	}
	if hasTgt {
		s.MessagingTargets = append(s.MessagingTargets, target)
	}
}

func isMessagingToolName(name string) bool {
	n := strings.ToLower(name)
	return n == "discord_send" || n == "slack_send" || n == "sessions_send" ||
		strings.HasPrefix(n, "messaging_")
}

func inferToolMeta(toolName string, args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	n := strings.ToLower(toolName)
	switch {
	case n == "read":
		if p, ok := args["path"].(string); ok {
			return p
		}
	case n == "write":
		if p, ok := args["path"].(string); ok {
			return p
		}
	case n == "exec" || n == "bash":
		if cmd, ok := args["command"].(string); ok {
			if len(cmd) > 80 {
				return cmd[:80] + "…"
			}
			return cmd
		}
	}
	return ""
}
