package reply

// TS 对照: auto-reply/reply/typing-mode.ts (143L)

// TypingMode 打字指示符模式。
type TypingMode string

const (
	TypingModeInstant  TypingMode = "instant"
	TypingModeMessage  TypingMode = "message"
	TypingModeThinking TypingMode = "thinking"
	TypingModeNever    TypingMode = "never"
)

// DefaultGroupTypingMode 默认群聊打字模式。
const DefaultGroupTypingMode = TypingModeMessage

// TypingModeContext 打字模式解析上下文。
type TypingModeContext struct {
	Configured   TypingMode
	IsGroupChat  bool
	WasMentioned bool
	IsHeartbeat  bool
}

// ResolveTypingMode 解析打字指示符模式。
// TS 对照: typing-mode.ts L14-30
func ResolveTypingMode(ctx TypingModeContext) TypingMode {
	if ctx.IsHeartbeat {
		return TypingModeNever
	}
	if ctx.Configured != "" {
		return ctx.Configured
	}
	if !ctx.IsGroupChat || ctx.WasMentioned {
		return TypingModeInstant
	}
	return DefaultGroupTypingMode
}

// TypingSignaler 打字信号器。
// 根据 TypingMode 决定在不同事件时是否触发打字指示符。
type TypingSignaler struct {
	typing   *TypingController
	Mode     TypingMode
	disabled bool

	ShouldStartImmediately    bool
	ShouldStartOnMessageStart bool
	ShouldStartOnText         bool
	ShouldStartOnReasoning    bool

	hasRenderableText bool
}

// NewTypingSignaler 创建打字信号器。
// TS 对照: typing-mode.ts L45-141
func NewTypingSignaler(typing *TypingController, mode TypingMode, isHeartbeat bool) *TypingSignaler {
	return &TypingSignaler{
		typing:                    typing,
		Mode:                      mode,
		disabled:                  isHeartbeat || mode == TypingModeNever,
		ShouldStartImmediately:    mode == TypingModeInstant,
		ShouldStartOnMessageStart: mode == TypingModeMessage,
		ShouldStartOnText:         mode == TypingModeMessage || mode == TypingModeInstant,
		ShouldStartOnReasoning:    mode == TypingModeThinking,
	}
}

// SignalRunStart 运行开始信号。
func (s *TypingSignaler) SignalRunStart() error {
	if s.disabled || !s.ShouldStartImmediately {
		return nil
	}
	return s.typing.StartTypingLoop()
}

// SignalMessageStart 消息开始信号。
func (s *TypingSignaler) SignalMessageStart() error {
	if s.disabled || !s.ShouldStartOnMessageStart {
		return nil
	}
	if !s.hasRenderableText {
		return nil
	}
	return s.typing.StartTypingLoop()
}

// SignalTextDelta 文本增量信号。
func (s *TypingSignaler) SignalTextDelta(text string) error {
	if s.disabled {
		return nil
	}
	renderable := isRenderableText(text, defaultSilentToken)
	if renderable {
		s.hasRenderableText = true
	}
	if s.ShouldStartOnText {
		return s.typing.StartTypingOnText(text)
	}
	if s.ShouldStartOnReasoning {
		if !s.typing.IsActive() {
			if err := s.typing.StartTypingLoop(); err != nil {
				return err
			}
		}
		s.typing.RefreshTypingTtl()
	}
	return nil
}

// SignalReasoningDelta 推理增量信号。
func (s *TypingSignaler) SignalReasoningDelta() error {
	if s.disabled || !s.ShouldStartOnReasoning {
		return nil
	}
	if !s.hasRenderableText {
		return nil
	}
	if err := s.typing.StartTypingLoop(); err != nil {
		return err
	}
	s.typing.RefreshTypingTtl()
	return nil
}

// SignalToolStart 工具开始信号。
func (s *TypingSignaler) SignalToolStart() error {
	if s.disabled {
		return nil
	}
	if !s.typing.IsActive() {
		if err := s.typing.StartTypingLoop(); err != nil {
			return err
		}
		s.typing.RefreshTypingTtl()
		return nil
	}
	s.typing.RefreshTypingTtl()
	return nil
}

func isRenderableText(text, silentToken string) bool {
	trimmed := trimWhitespace(text)
	if trimmed == "" {
		return false
	}
	return !isSilentReply(trimmed, silentToken)
}
