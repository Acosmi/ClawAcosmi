package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"Argus-compound/go-sensory/internal/input"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Action result types
// ──────────────────────────────────────────────────────────────

// ActionResult describes the outcome of a TUI interaction after
// the Action-Verify loop.
type ActionResult struct {
	// Whether the action achieved its intended effect.
	Success bool `json:"success"`

	// Human-readable description of what happened.
	Description string `json:"description"`

	// VLM-verified state change category.
	Effect EffectType `json:"effect"`

	// The terminal state after the action.
	AfterState *TerminalState `json:"after_state,omitempty"`
}

// EffectType classifies the observed effect of an action.
type EffectType string

const (
	EffectPromptDismissed EffectType = "prompt_dismissed"
	EffectOptionSelected  EffectType = "option_selected"
	EffectTextEntered     EffectType = "text_entered"
	EffectCommandExecuted EffectType = "command_executed"
	EffectKeyProcessed    EffectType = "key_processed"
	EffectNoChange        EffectType = "no_change"
	EffectUnexpected      EffectType = "unexpected"
)

// ──────────────────────────────────────────────────────────────
// Interactor
// ──────────────────────────────────────────────────────────────

// Interactor provides terminal interaction capabilities with an
// Action-Verify loop.  Every action is:
//  1. Gated by the ApprovalGateway (Phase 0)
//  2. Executed via InputController (keyboard/mouse simulation)
//  3. Verified by Reader (screenshot + VLM comparison)
type Interactor struct {
	reader  *Reader
	input   input.InputController
	gateway *input.ApprovalGateway
	vlm     vlm.Provider
}

// NewInteractor creates a TUI interactor with the given dependencies.
func NewInteractor(reader *Reader, inputCtrl input.InputController, gateway *input.ApprovalGateway, vlmProvider vlm.Provider) *Interactor {
	return &Interactor{
		reader:  reader,
		input:   inputCtrl,
		gateway: gateway,
		vlm:     vlmProvider,
	}
}

// ──────────────────────────────────────────────────────────────
// TUI Actions — the 3 input types
// ──────────────────────────────────────────────────────────────

// Respond sends a keyboard response to a terminal prompt (y/n,
// option selection, etc.).  Equivalent to typing the input and
// pressing Enter.
//
// MCP Tool: tui_respond
func (it *Interactor) Respond(ctx context.Context, response string) (*ActionResult, error) {
	params, _ := json.Marshal(map[string]string{"input": response})

	// Gate through approval
	approved, modifiedParams, err := it.gateway.CheckAndApprove(
		ctx, "tui_respond", params, "tui_interactor", nil,
	)
	if err != nil {
		return nil, fmt.Errorf("approval check: %w", err)
	}
	if !approved {
		return &ActionResult{Success: false, Description: "操作被人类审核员拒绝", Effect: EffectNoChange}, nil
	}

	// Apply modified params if human edited them
	actual := response
	if modifiedParams != nil {
		var p struct {
			Input string `json:"input"`
		}
		if json.Unmarshal(modifiedParams, &p) == nil && p.Input != "" {
			actual = p.Input
		}
	}

	// Capture pre-state
	beforeState, _ := it.reader.ReadState(ctx)

	// Execute: type response + Enter
	if err := it.input.Type(actual); err != nil {
		return nil, fmt.Errorf("typing response: %w", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := it.input.KeyPress(input.KeyReturn); err != nil {
		return nil, fmt.Errorf("pressing Enter: %w", err)
	}

	// Verify
	return it.verifyAction(ctx, "tui_respond", beforeState, fmt.Sprintf("typed %q and pressed Enter", actual))
}

// SendKeys sends a key combination to the terminal (e.g. Ctrl+C,
// Arrow keys, Escape, Tab).
//
// MCP Tool: tui_send_keys
func (it *Interactor) SendKeys(ctx context.Context, keys ...input.Key) (*ActionResult, error) {
	keyUints := make([]uint16, len(keys))
	for i, k := range keys {
		keyUints[i] = uint16(k)
	}
	params, _ := json.Marshal(map[string][]uint16{"keys": keyUints})

	keysDesc := formatKeys(keys)

	// Gate through approval
	approved, _, err := it.gateway.CheckAndApprove(
		ctx, "tui_send_keys", params, "tui_interactor", nil,
	)
	if err != nil {
		return nil, fmt.Errorf("approval check: %w", err)
	}
	if !approved {
		return &ActionResult{Success: false, Description: "操作被人类审核员拒绝", Effect: EffectNoChange}, nil
	}

	// Capture pre-state
	beforeState, _ := it.reader.ReadState(ctx)

	// Execute: distinguish single key vs hotkey
	if len(keys) == 1 {
		if err := it.input.KeyPress(keys[0]); err != nil {
			return nil, fmt.Errorf("pressing key: %w", err)
		}
	} else {
		if err := it.input.Hotkey(keys...); err != nil {
			return nil, fmt.Errorf("pressing hotkey: %w", err)
		}
	}

	// Verify
	return it.verifyAction(ctx, "tui_send_keys", beforeState, fmt.Sprintf("pressed %s", keysDesc))
}

// RunCommand types a full command in the terminal and presses Enter.
// Only suitable when the terminal is idle (no active prompt or
// running process).
//
// MCP Tool: tui_run_command
func (it *Interactor) RunCommand(ctx context.Context, command string) (*ActionResult, error) {
	params, _ := json.Marshal(map[string]string{"command": command})

	// Gate through approval — type_text risk rules apply for sensitive keywords
	approved, modifiedParams, err := it.gateway.CheckAndApprove(
		ctx, "tui_run_command", params, "tui_interactor", nil,
	)
	if err != nil {
		return nil, fmt.Errorf("approval check: %w", err)
	}
	if !approved {
		return &ActionResult{Success: false, Description: "操作被人类审核员拒绝", Effect: EffectNoChange}, nil
	}

	// Apply modified params
	actual := command
	if modifiedParams != nil {
		var p struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(modifiedParams, &p) == nil && p.Command != "" {
			actual = p.Command
		}
	}

	// Pre-flight: check terminal is idle
	preState, err := it.reader.ReadState(ctx)
	if err != nil {
		log.Printf("[TUI] Warning: cannot read pre-state for RunCommand: %v", err)
	} else if preState.Prompt != PromptIdle && !preState.WaitingForInput {
		log.Printf("[TUI] Warning: terminal may not be idle (prompt=%s)", preState.Prompt)
	}

	// Execute: type command + Enter
	if err := it.input.Type(actual); err != nil {
		return nil, fmt.Errorf("typing command: %w", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := it.input.KeyPress(input.KeyReturn); err != nil {
		return nil, fmt.Errorf("pressing Enter: %w", err)
	}

	// Verify
	return it.verifyAction(ctx, "tui_run_command", preState, fmt.Sprintf("executed command: %s", actual))
}

// ──────────────────────────────────────────────────────────────
// Action-Verify loop
// ──────────────────────────────────────────────────────────────

// verifyAction captures the post-action screen state and uses VLM
// to compare before/after, determining if the action had the
// intended effect.
func (it *Interactor) verifyAction(ctx context.Context, action string, before *TerminalState, actionDesc string) (*ActionResult, error) {
	// Brief delay for the terminal to update
	time.Sleep(300 * time.Millisecond)

	// Capture post-state
	after, err := it.reader.ReadState(ctx)
	if err != nil {
		log.Printf("[TUI] Warning: post-action state read failed: %v", err)
		return &ActionResult{
			Success:     true, // optimistic — action was sent
			Description: fmt.Sprintf("%s (无法验证结果)", actionDesc),
			Effect:      EffectUnexpected,
		}, nil
	}

	// Use VLM to compare before/after
	effect, err := it.compareStates(ctx, before, after, action, actionDesc)
	if err != nil {
		log.Printf("[TUI] Warning: VLM comparison failed: %v", err)
		return &ActionResult{
			Success:     true,
			Description: actionDesc,
			Effect:      EffectUnexpected,
			AfterState:  after,
		}, nil
	}

	success := effect != EffectNoChange && effect != EffectUnexpected
	return &ActionResult{
		Success:     success,
		Description: fmt.Sprintf("%s → %s", actionDesc, effect),
		Effect:      effect,
		AfterState:  after,
	}, nil
}

// compareStates uses VLM to analyze the before/after terminal states
// and classify the effect of the action.
func (it *Interactor) compareStates(ctx context.Context, before, after *TerminalState, action, actionDesc string) (EffectType, error) {
	if it.vlm == nil {
		return EffectUnexpected, fmt.Errorf("no VLM provider")
	}

	prompt := fmt.Sprintf(`Compare these two terminal states and classify the action effect.

Action performed: %s (%s)

BEFORE state:
- Raw text: %s
- Prompt type: %s
- Description: %s

AFTER state:
- Raw text: %s
- Prompt type: %s
- Description: %s

Respond with exactly one of these effect types:
- prompt_dismissed: A prompt/dialog was closed
- option_selected: An option was selected from a menu/list
- text_entered: Text was successfully entered
- command_executed: A shell command was run and output appeared
- key_processed: A keyboard shortcut was processed
- no_change: Nothing visibly changed
- unexpected: Something unexpected happened

Respond with ONLY the effect type string, nothing else.`,
		action, actionDesc,
		truncate(safeStr(before, func(s *TerminalState) string { return s.RawText }), 500),
		safeStr(before, func(s *TerminalState) string { return string(s.Prompt) }),
		safeStr(before, func(s *TerminalState) string { return s.Description }),
		truncate(after.RawText, 500),
		string(after.Prompt),
		after.Description,
	)

	req := vlm.ChatRequest{
		Messages:  []vlm.Message{{Role: "user", Content: prompt}},
		MaxTokens: 32,
	}

	resp, err := it.vlm.ChatCompletion(ctx, req)
	if err != nil {
		return EffectUnexpected, err
	}

	if len(resp.Choices) == 0 {
		return EffectUnexpected, fmt.Errorf("VLM returned no choices")
	}

	effectStr := strings.TrimSpace(resp.Choices[0].Message.GetTextContent())
	return parseEffect(effectStr), nil
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// parseEffect converts a VLM response string to an EffectType.
func parseEffect(s string) EffectType {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "prompt_dismissed":
		return EffectPromptDismissed
	case "option_selected":
		return EffectOptionSelected
	case "text_entered":
		return EffectTextEntered
	case "command_executed":
		return EffectCommandExecuted
	case "key_processed":
		return EffectKeyProcessed
	case "no_change":
		return EffectNoChange
	default:
		return EffectUnexpected
	}
}

// formatKeys returns a human-readable key combination string.
func formatKeys(keys []input.Key) string {
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = keyName(k)
	}
	return strings.Join(names, "+")
}

// keyName returns a human-readable name for a key code.
func keyName(k input.Key) string {
	switch k {
	case input.KeyCommand:
		return "⌘"
	case input.KeyControl:
		return "Ctrl"
	case input.KeyOption:
		return "⌥"
	case input.KeyShift:
		return "⇧"
	case input.KeyReturn:
		return "↩"
	case input.KeyTab:
		return "⇥"
	case input.KeyEscape:
		return "⎋"
	case input.KeyDelete:
		return "⌫"
	case input.KeyArrowUp:
		return "↑"
	case input.KeyArrowDown:
		return "↓"
	case input.KeyArrowLeft:
		return "←"
	case input.KeyArrowRight:
		return "→"
	case input.KeySpace:
		return "Space"
	case input.KeyC:
		return "C"
	case input.KeyD:
		return "D"
	case input.KeyZ:
		return "Z"
	case input.KeyQ:
		return "Q"
	default:
		return fmt.Sprintf("0x%02x", uint16(k))
	}
}

// safeStr safely extracts a string from a possibly-nil TerminalState.
func safeStr(s *TerminalState, fn func(*TerminalState) string) string {
	if s == nil {
		return "<unknown>"
	}
	return fn(s)
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
