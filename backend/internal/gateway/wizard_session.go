package gateway

// wizard_session.go — Setup Wizard 会话引擎
// TS 对照: src/wizard/session.ts (265L) + prompts.ts (53L)
//
// Go 实现：用 goroutine + channel 替代 TS 的 Deferred/Promise 模式。
// WizardSession 启动一个 runner goroutine，通过 stepCh / answerCh 与 RPC handler 交互。

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// ---------- 类型定义（对齐 TS wizard/session.ts + protocol/schema/wizard.ts） ----------

// WizardStepType 向导步骤类型。
type WizardStepType string

const (
	WizardStepNote        WizardStepType = "note"
	WizardStepSelect      WizardStepType = "select"
	WizardStepText        WizardStepType = "text"
	WizardStepConfirm     WizardStepType = "confirm"
	WizardStepMultiSelect WizardStepType = "multiselect"
	WizardStepProgress    WizardStepType = "progress"
	WizardStepAction      WizardStepType = "action"
)

// WizardStepOption 步骤选项。
type WizardStepOption struct {
	Value interface{} `json:"value"`
	Label string      `json:"label"`
	Hint  string      `json:"hint,omitempty"`
}

// WizardStep 向导步骤定义。
type WizardStep struct {
	ID           string             `json:"id"`
	Type         WizardStepType     `json:"type"`
	Title        string             `json:"title,omitempty"`
	Message      string             `json:"message,omitempty"`
	Options      []WizardStepOption `json:"options,omitempty"`
	InitialValue interface{}        `json:"initialValue,omitempty"`
	Placeholder  string             `json:"placeholder,omitempty"`
	Sensitive    bool               `json:"sensitive,omitempty"`
	Executor     string             `json:"executor,omitempty"` // "gateway" | "client"
}

// WizardSessionStatus 会话状态。
type WizardSessionStatus string

const (
	WizardStatusRunning   WizardSessionStatus = "running"
	WizardStatusDone      WizardSessionStatus = "done"
	WizardStatusCancelled WizardSessionStatus = "cancelled"
	WizardStatusError     WizardSessionStatus = "error"
)

// WizardNextResult wizard.next 返回值。
type WizardNextResult struct {
	Done   bool                `json:"done"`
	Step   *WizardStep         `json:"step,omitempty"`
	Status WizardSessionStatus `json:"status"`
	Error  string              `json:"error,omitempty"`
}

// WizardStartResult wizard.start 返回值。
type WizardStartResult struct {
	SessionID string              `json:"sessionId"`
	Done      bool                `json:"done"`
	Step      *WizardStep         `json:"step,omitempty"`
	Status    WizardSessionStatus `json:"status,omitempty"`
	Error     string              `json:"error,omitempty"`
}

// WizardStatusResult wizard.status 返回值。
type WizardStatusResult struct {
	Status WizardSessionStatus `json:"status"`
	Error  string              `json:"error,omitempty"`
}

// ---------- WizardPrompter 接口（对齐 TS prompts.ts） ----------

// WizardPrompter 向导提示器接口。
// runner 函数通过此接口向前端发送步骤并等待回答。
type WizardPrompter interface {
	Intro(title string) error
	Outro(message string) error
	Note(message, title string) error
	Select(message string, options []WizardStepOption, initialValue interface{}) (interface{}, error)
	MultiSelect(message string, options []WizardStepOption, initialValues []interface{}) ([]interface{}, error)
	Text(message, placeholder string, initialValue string, sensitive bool) (string, error)
	Confirm(message string, initialValue bool) (bool, error)
}

// WizardCancelledError 向导取消错误。
type WizardCancelledError struct{}

func (e *WizardCancelledError) Error() string { return "wizard cancelled" }

// ---------- WizardSession 核心会话（对齐 TS WizardSession class） ----------

// WizardRunnerFunc 向导 runner 函数签名。
type WizardRunnerFunc func(prompter WizardPrompter) error

// WizardSession 管理一次 wizard 会话的步骤/回答交互。
// TS 用 Deferred<T> + Promise 实现协程式交互，Go 用 channel 对实现同等效果。
type WizardSession struct {
	mu          sync.Mutex
	currentStep *WizardStep // 当前待回答步骤
	status      WizardSessionStatus
	errMsg      string

	// stepCh: runner goroutine 推送步骤 → Next() 读取
	stepCh chan *WizardStep
	// answerChs: Next() 写入回答 → runner goroutine 中的 awaitAnswer 读取
	answerChs map[string]chan interface{}
	// doneCh: runner goroutine 结束信号
	doneCh chan struct{}
}

// NewWizardSession 创建并启动一个新的 wizard 会话。
func NewWizardSession(runner WizardRunnerFunc) *WizardSession {
	s := &WizardSession{
		status:    WizardStatusRunning,
		stepCh:    make(chan *WizardStep, 1),
		answerChs: make(map[string]chan interface{}),
		doneCh:    make(chan struct{}),
	}
	prompter := &wizardSessionPrompter{session: s}
	go s.run(runner, prompter)
	return s
}

// Next 获取下一个步骤。阻塞直到 runner 推送步骤或完成。
func (s *WizardSession) Next() WizardNextResult {
	s.mu.Lock()
	// 如果有待处理的步骤，直接返回
	if s.currentStep != nil {
		step := s.currentStep
		s.mu.Unlock()
		return WizardNextResult{Done: false, Step: step, Status: s.status}
	}
	// 如果已结束
	if s.status != WizardStatusRunning {
		status := s.status
		errMsg := s.errMsg
		s.mu.Unlock()
		return WizardNextResult{Done: true, Status: status, Error: errMsg}
	}
	s.mu.Unlock()

	// 等待 runner 推送步骤或结束
	select {
	case step, ok := <-s.stepCh:
		if !ok || step == nil {
			s.mu.Lock()
			defer s.mu.Unlock()
			return WizardNextResult{Done: true, Status: s.status, Error: s.errMsg}
		}
		return WizardNextResult{Done: false, Step: step, Status: s.status}
	case <-s.doneCh:
		s.mu.Lock()
		defer s.mu.Unlock()
		return WizardNextResult{Done: true, Status: s.status, Error: s.errMsg}
	}
}

// Answer 回答当前步骤。
func (s *WizardSession) Answer(stepID string, value interface{}) error {
	s.mu.Lock()
	ch, ok := s.answerChs[stepID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("wizard: no pending step with id %s", stepID)
	}
	delete(s.answerChs, stepID)
	s.currentStep = nil
	s.mu.Unlock()

	ch <- value
	return nil
}

// Cancel 取消会话。
func (s *WizardSession) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != WizardStatusRunning {
		return
	}
	s.status = WizardStatusCancelled
	s.errMsg = "cancelled"
	s.currentStep = nil

	// 向所有等待 answer 的 channel 发送 nil 使 runner 解除阻塞
	for id, ch := range s.answerChs {
		close(ch)
		delete(s.answerChs, id)
	}
}

// GetStatus 获取会话状态。
func (s *WizardSession) GetStatus() WizardSessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// GetError 获取错误信息。
func (s *WizardSession) GetError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errMsg
}

// run 在 goroutine 中执行 runner 函数。
func (s *WizardSession) run(runner WizardRunnerFunc, prompter WizardPrompter) {
	defer func() {
		// 确保 stepCh 关闭
		select {
		case <-s.doneCh:
		default:
			close(s.doneCh)
		}
	}()

	err := runner(prompter)
	s.mu.Lock()
	if err != nil {
		if _, ok := err.(*WizardCancelledError); ok {
			s.status = WizardStatusCancelled
			s.errMsg = err.Error()
		} else {
			s.status = WizardStatusError
			s.errMsg = err.Error()
		}
	} else {
		if s.status == WizardStatusRunning {
			s.status = WizardStatusDone
		}
	}
	s.mu.Unlock()
}

// awaitAnswer 供 prompter 内部调用：推送步骤并等待回答。
func (s *WizardSession) awaitAnswer(step *WizardStep) (interface{}, error) {
	s.mu.Lock()
	if s.status != WizardStatusRunning {
		s.mu.Unlock()
		return nil, &WizardCancelledError{}
	}

	// 设置当前步骤
	s.currentStep = step
	answerCh := make(chan interface{}, 1)
	s.answerChs[step.ID] = answerCh
	s.mu.Unlock()

	// 通知 Next() 有新步骤
	select {
	case s.stepCh <- step:
	default:
		// stepCh buffer 已满，丢弃旧的（不会发生，单步串行）
	}

	// 等待回答
	val, ok := <-answerCh
	if !ok {
		return nil, &WizardCancelledError{}
	}
	return val, nil
}

// ---------- wizardSessionPrompter 实现（对齐 TS WizardSessionPrompter） ----------

type wizardSessionPrompter struct {
	session *WizardSession
}

func (p *wizardSessionPrompter) Intro(title string) error {
	_, err := p.prompt(WizardStep{
		Type:     WizardStepNote,
		Title:    title,
		Message:  "",
		Executor: "client",
	})
	return err
}

func (p *wizardSessionPrompter) Outro(message string) error {
	_, err := p.prompt(WizardStep{
		Type:     WizardStepNote,
		Title:    "Done",
		Message:  message,
		Executor: "client",
	})
	return err
}

func (p *wizardSessionPrompter) Note(message, title string) error {
	_, err := p.prompt(WizardStep{
		Type:     WizardStepNote,
		Title:    title,
		Message:  message,
		Executor: "client",
	})
	return err
}

func (p *wizardSessionPrompter) Select(message string, options []WizardStepOption, initialValue interface{}) (interface{}, error) {
	return p.prompt(WizardStep{
		Type:         WizardStepSelect,
		Message:      message,
		Options:      options,
		InitialValue: initialValue,
		Executor:     "client",
	})
}

func (p *wizardSessionPrompter) Text(message, placeholder string, initialValue string, sensitive bool) (string, error) {
	val, err := p.prompt(WizardStep{
		Type:         WizardStepText,
		Message:      message,
		Placeholder:  placeholder,
		InitialValue: initialValue,
		Sensitive:    sensitive,
		Executor:     "client",
	})
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", val), nil
}

func (p *wizardSessionPrompter) Confirm(message string, initialValue bool) (bool, error) {
	val, err := p.prompt(WizardStep{
		Type:         WizardStepConfirm,
		Message:      message,
		InitialValue: initialValue,
		Executor:     "client",
	})
	if err != nil {
		return false, err
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, nil
}

func (p *wizardSessionPrompter) MultiSelect(message string, options []WizardStepOption, initialValues []interface{}) ([]interface{}, error) {
	val, err := p.prompt(WizardStep{
		Type:         WizardStepMultiSelect,
		Message:      message,
		Options:      options,
		InitialValue: initialValues,
		Executor:     "client",
	})
	if err != nil {
		return nil, err
	}
	if arr, ok := val.([]interface{}); ok {
		return arr, nil
	}
	// 兼容前端返回单值情况
	if val != nil {
		return []interface{}{val}, nil
	}
	return nil, nil
}

func (p *wizardSessionPrompter) prompt(step WizardStep) (interface{}, error) {
	step.ID = uuid.New().String()
	return p.session.awaitAnswer(&step)
}

// ---------- WizardSessionTracker 会话管理器（对齐 TS server-wizard-sessions.ts） ----------

// WizardSessionTracker 管理所有活跃 wizard 会话。
// 并发安全（sync.RWMutex）。
type WizardSessionTracker struct {
	mu       sync.RWMutex
	sessions map[string]*WizardSession
}

// NewWizardSessionTracker 创建空的会话管理器。
func NewWizardSessionTracker() *WizardSessionTracker {
	return &WizardSessionTracker{
		sessions: make(map[string]*WizardSession),
	}
}

// Set 注册新会话。
func (t *WizardSessionTracker) Set(id string, session *WizardSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessions[id] = session
}

// Get 获取会话。
func (t *WizardSessionTracker) Get(id string) *WizardSession {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessions[id]
}

// Delete 删除会话。
func (t *WizardSessionTracker) Delete(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sessions, id)
}

// FindRunning 查找正在运行的会话 ID（全局唯一）。
// 修复: 内联 status 检查，避免在持有 t.mu 时调用 session.GetStatus() 导致跨锁死锁。
func (t *WizardSessionTracker) FindRunning() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for id, session := range t.sessions {
		session.mu.Lock()
		running := session.status == WizardStatusRunning
		session.mu.Unlock()
		if running {
			return id
		}
	}
	return ""
}

// Purge 清除已完成的会话（非 running 状态才删除）。
// 修复: 内联 status 检查，避免在持有 t.mu 时调用 session.GetStatus() 导致跨锁死锁。
func (t *WizardSessionTracker) Purge(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	session, ok := t.sessions[id]
	if !ok {
		return
	}
	session.mu.Lock()
	running := session.status == WizardStatusRunning
	session.mu.Unlock()
	if running {
		return
	}
	delete(t.sessions, id)
}
