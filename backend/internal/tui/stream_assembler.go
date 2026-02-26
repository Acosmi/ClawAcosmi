// stream_assembler.go — TUI 流式文本组装器
//
// 对齐 TS: src/tui/tui-stream-assembler.ts(79L)
// 纯内存状态管理器，维护每个 run 的增量文本组装。
// 无隐藏依赖。
package tui

// runStreamState 单个 run 的流式状态。
type runStreamState struct {
	thinkingText string
	contentText  string
	displayText  string
}

// TuiStreamAssembler 流式文本组装器。
// 维护 map[runId]*runStreamState，支持增量 delta 和最终化。
type TuiStreamAssembler struct {
	runs map[string]*runStreamState
}

// NewTuiStreamAssembler 创建流式文本组装器。
func NewTuiStreamAssembler() *TuiStreamAssembler {
	return &TuiStreamAssembler{
		runs: make(map[string]*runStreamState),
	}
}

// getOrCreateRun 获取或创建 run 状态。
func (a *TuiStreamAssembler) getOrCreateRun(runID string) *runStreamState {
	state, ok := a.runs[runID]
	if !ok {
		state = &runStreamState{}
		a.runs[runID] = state
	}
	return state
}

// updateRunState 更新 run 的文本状态。
// TS 参考: tui-stream-assembler.ts L30-48
func (a *TuiStreamAssembler) updateRunState(state *runStreamState, message interface{}, showThinking bool) {
	thinkingText := ExtractThinkingFromMessage(message)
	contentText := ExtractContentFromMessage(message)

	if thinkingText != "" {
		state.thinkingText = thinkingText
	}
	if contentText != "" {
		state.contentText = contentText
	}

	state.displayText = ComposeThinkingAndContent(
		state.thinkingText,
		state.contentText,
		showThinking,
	)
}

// IngestDelta 增量组装。
// 返回更新后的 displayText；如果无变化或为空返回空串。
// TS 参考: tui-stream-assembler.ts L50-60
func (a *TuiStreamAssembler) IngestDelta(runID string, message interface{}, showThinking bool) string {
	state := a.getOrCreateRun(runID)
	previousDisplayText := state.displayText
	a.updateRunState(state, message, showThinking)

	if state.displayText == "" || state.displayText == previousDisplayText {
		return ""
	}
	return state.displayText
}

// Finalize 最终化 run 文本。
// 返回最终文本并删除 run 状态。
// TS 参考: tui-stream-assembler.ts L62-73
func (a *TuiStreamAssembler) Finalize(runID string, message interface{}, showThinking bool) string {
	state := a.getOrCreateRun(runID)
	a.updateRunState(state, message, showThinking)
	finalComposed := state.displayText

	finalText := ResolveFinalAssistantText(finalComposed, state.displayText)
	delete(a.runs, runID)
	return finalText
}

// Drop 丢弃 run 状态。
// TS 参考: tui-stream-assembler.ts L75-77
func (a *TuiStreamAssembler) Drop(runID string) {
	delete(a.runs, runID)
}

// Reset 清空所有 run 状态。
func (a *TuiStreamAssembler) Reset() {
	a.runs = make(map[string]*runStreamState)
}
