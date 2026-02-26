package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/input"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Prompt Templates (mirrored from Python react_loop.py)
// ──────────────────────────────────────────────────────────────

const thinkPromptTpl = `You are an AI agent controlling a computer. Your goal is: %s

Current step: %d/%d
Previous actions: %s

Looking at the current screenshot, decide what action to take next.

Available actions:
- click(x, y): Click at screen coordinates
- double_click(x, y): Double-click
- right_click(x, y): Right-click
- type(text): Type text
- hotkey(keys): Press key combination (e.g., "cmd+c")
- scroll(x, y, delta_x, delta_y): Scroll at position
- wait(seconds): Wait before next action
- done(): Task is complete
- fail(reason): Task cannot be completed

Respond with ONLY a JSON object:
{"action": "click", "params": {"x": 100, "y": 200}, "reasoning": "Clicking the search button to..."}`

// SoM-mode think prompt: VLM picks a numbered element instead of raw coordinates.
const thinkSoMPromptTpl = `You are an AI agent controlling a computer. Your goal is: %s

Current step: %d/%d
Previous actions: %s

This screenshot has numbered labels [0], [1], [2], etc. on detected UI elements.
Pick the element to interact with, or choose a non-element action.

Available actions:
- click_element(id): Click the numbered UI element
- type(text): Type text into the focused field
- hotkey(keys): Press key combination (e.g., "cmd+c")
- scroll(x, y, delta_x, delta_y): Scroll at position
- wait(seconds): Wait before next action
- done(): Task is complete
- fail(reason): Task cannot be completed

Respond with ONLY a JSON object:
{"action": "click_element", "params": {"id": 3}, "reasoning": "Clicking the search button..."}`

const verifyPromptTpl = `You are verifying if an action was successful.

Goal: %s
Action taken: %s(%v)
Expected effect: %s

Compare the before and after screenshots. Did the action succeed?

Respond with ONLY a JSON object:
{"success": true, "description": "What happened after the action"}`

const describePrompt = "Describe what you see on this screen."

// ──────────────────────────────────────────────────────────────
// ReActLoop
// ──────────────────────────────────────────────────────────────

// ReActLoop implements the Observe → Think → Act → Verify cycle.
//
// CRITICAL DIFFERENCE from Python version:
//   - Python: 3+ HTTP round-trips per step (capture → VLM → action)
//   - Go:     All calls are in-process, zero network overhead
type ReActLoop struct {
	capturer  FrameSource
	inputCtrl ActionExecutor
	vlmRouter *vlm.Router
	scaler    *imaging.Scaler
	uiParser  *UIParser // nil = SoM disabled

	MaxSteps      int
	VerifyActions bool
	StepDelay     time.Duration
}

// NewReActLoop constructs the ReAct loop with all in-process dependencies.
func NewReActLoop(
	capturer FrameSource,
	inputCtrl ActionExecutor,
	vlmRouter *vlm.Router,
	scaler *imaging.Scaler,
	uiParser *UIParser,
) *ReActLoop {
	return &ReActLoop{
		capturer:      capturer,
		inputCtrl:     inputCtrl,
		vlmRouter:     vlmRouter,
		scaler:        scaler,
		uiParser:      uiParser,
		MaxSteps:      20,
		VerifyActions: true,
		StepDelay:     500 * time.Millisecond,
	}
}

// Execute runs the ReAct task loop for the given goal.
func (r *ReActLoop) Execute(ctx context.Context, goal string) (*TaskResult, error) {
	log.Printf("[ReAct] Starting task: %s", goal)
	start := time.Now()
	var steps []Step

	for stepNo := 1; stepNo <= r.MaxSteps; stepNo++ {
		stepStart := time.Now()

		// 1. OBSERVE — capture + describe screen
		obs := r.observe(ctx)

		// 2. THINK — ask VLM what action to take
		history := FormatStepHistory(steps, 5)
		thought, action := r.think(ctx, goal, stepNo, history, obs)

		// Check termination
		if action.Type == ActionDone || action.Type == ActionFail {
			step := Step{
				StepNo:      stepNo,
				Observation: obs,
				Thought:     thought,
				Action:      action,
				Success:     action.Type == ActionDone,
				DurationMs:  float64(time.Since(stepStart).Milliseconds()),
			}
			steps = append(steps, step)
			label := "completed"
			if action.Type == ActionFail {
				label = "failed"
			}
			log.Printf("[ReAct] Task %s at step %d: %s", label, stepNo, action.Reasoning)
			break
		}

		// 3. ACT — execute the action
		actSuccess := r.act(action)

		// Brief pause for UI to settle
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(r.StepDelay):
		}

		// 4. VERIFY (optional)
		verification := ""
		if r.VerifyActions && actSuccess {
			verification = r.verify(ctx, goal, action)
		}

		step := Step{
			StepNo:       stepNo,
			Observation:  obs,
			Thought:      thought,
			Action:       action,
			Success:      actSuccess,
			Verification: verification,
			DurationMs:   float64(time.Since(stepStart).Milliseconds()),
		}
		steps = append(steps, step)

		status := "OK"
		if !actSuccess {
			status = "FAIL"
		}
		log.Printf("[ReAct] Step %d: %s(%v) → %s | %.0fms",
			stepNo, action.Type, action.Params, status, step.DurationMs)
	}

	totalMs := float64(time.Since(start).Milliseconds())
	success := false
	for _, s := range steps {
		if s.Action.Type == ActionDone {
			success = true
			break
		}
	}

	result := &TaskResult{
		Goal:            goal,
		Success:         success,
		Steps:           steps,
		TotalDurationMs: totalMs,
	}
	if !success {
		result.Error = "Max steps reached or task failed"
	}

	status := "SUCCEEDED"
	if !success {
		status = "FAILED"
	}
	log.Printf("[ReAct] Task %s: %d steps, %.1fs total",
		status, len(steps), totalMs/1000)

	return result, nil
}

// ──────────────────────────────────────────────────────────────
// Internal steps
// ──────────────────────────────────────────────────────────────

// observe captures the current screen and gets a VLM description.
func (r *ReActLoop) observe(ctx context.Context) Observation {
	jpegData := r.captureJPEG()
	if jpegData == nil {
		return Observation{Timestamp: NowMs()}
	}

	// Get scene description from VLM (in-process, no HTTP)
	description := ""
	if r.vlmRouter != nil {
		desc, err := r.callVLMWithImage(ctx, jpegData, describePrompt)
		if err != nil {
			log.Printf("[ReAct] Describe failed: %v", err)
		} else {
			description = desc
		}
	}

	return Observation{
		ScreenshotJPEG: jpegData,
		Description:    description,
		Timestamp:      NowMs(),
	}
}

// think asks the VLM to decide the next action.
// Strategy: SoM mode first (AX detect → annotate → VLM picks number),
// then fallback to direct coordinate mode.
func (r *ReActLoop) think(ctx context.Context, goal string, stepNo int, history string, obs Observation) (string, Action) {
	if history == "" {
		history = "None yet"
	}

	if r.vlmRouter == nil || obs.ScreenshotJPEG == nil {
		return "No VLM available", Action{Type: ActionWait, Params: map[string]any{"seconds": 1}, Reasoning: "No VLM"}
	}

	// Try SoM mode if UIParser is available
	if r.uiParser != nil {
		result, action, ok := r.thinkSoM(ctx, goal, stepNo, history, obs)
		if ok {
			return result, action
		}
		log.Printf("[ReAct] SoM mode unavailable, falling back to direct coordinates")
	}

	// Fallback: direct coordinate mode (original logic)
	return r.thinkDirect(ctx, goal, stepNo, history, obs)
}

// thinkSoM tries SoM-based grounding. Returns (reasoning, action, true) on success.
func (r *ReActLoop) thinkSoM(ctx context.Context, goal string, stepNo int, history string, obs Observation) (string, Action, bool) {
	// 1. Detect UI elements (AX-first path via UIParser)
	elements, err := r.uiParser.DetectElements(ctx, obs.ScreenshotJPEG)
	if err != nil || len(elements) == 0 {
		return "", Action{}, false
	}

	// 2. Annotate screenshot with SoM labels
	annotatedJPEG, err := r.uiParser.AnnotateSoM(obs.ScreenshotJPEG, elements)
	if err != nil {
		log.Printf("[ReAct] SoM annotation failed: %v", err)
		return "", Action{}, false
	}

	// 3. Ask VLM to pick a numbered element
	prompt := fmt.Sprintf(thinkSoMPromptTpl, goal, stepNo, r.MaxSteps, history)
	raw, err := r.callVLMWithImage(ctx, annotatedJPEG, prompt)
	if err != nil {
		log.Printf("[ReAct] SoM VLM call failed: %v", err)
		return "", Action{}, false
	}

	// 4. Parse response
	cleaned := stripMarkdownFences(raw)
	var data struct {
		ActionStr string         `json:"action"`
		Params    map[string]any `json:"params"`
		Reasoning string         `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(cleaned), &data); err != nil {
		log.Printf("[ReAct] SoM JSON parse error: %v", err)
		return "", Action{}, false
	}

	// Handle click_element → resolve to coordinates from element bounding box
	if data.ActionStr == "click_element" {
		id := getIntParam(data.Params, "id", -1)
		if id >= 0 && id < len(elements) {
			el := elements[id]
			cx := (el.X1 + el.X2) / 2
			cy := (el.Y1 + el.Y2) / 2
			log.Printf("[ReAct] [SoM] click_element[%d] '%s' → click(%d, %d)", id, el.Label, cx, cy)
			return data.Reasoning, Action{
				Type:      ActionClick,
				Params:    map[string]any{"x": cx, "y": cy},
				Reasoning: data.Reasoning,
			}, true
		}
		log.Printf("[ReAct] SoM element id %d out of range [0, %d)", id, len(elements))
		return "", Action{}, false
	}

	// Other actions pass through normally
	actionType := ActionType(data.ActionStr)
	switch actionType {
	case ActionType_, ActionHotkey, ActionScroll, ActionWait, ActionDone, ActionFail:
		return data.Reasoning, Action{
			Type:      actionType,
			Params:    data.Params,
			Reasoning: data.Reasoning,
		}, true
	default:
		return "", Action{}, false // unknown action, fall back to direct mode
	}
}

// thinkDirect uses direct coordinate mode (original logic).
func (r *ReActLoop) thinkDirect(ctx context.Context, goal string, stepNo int, history string, obs Observation) (string, Action) {
	prompt := fmt.Sprintf(thinkPromptTpl, goal, stepNo, r.MaxSteps, history)

	raw, err := r.callVLMWithImage(ctx, obs.ScreenshotJPEG, prompt)
	if err != nil {
		msg := fmt.Sprintf("Think error: %v", err)
		return msg, Action{Type: ActionWait, Params: map[string]any{"seconds": 1}, Reasoning: msg}
	}

	// Parse JSON from VLM response (handle markdown fences)
	cleaned := stripMarkdownFences(raw)

	var data struct {
		ActionStr string         `json:"action"`
		Params    map[string]any `json:"params"`
		Reasoning string         `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(cleaned), &data); err != nil {
		msg := fmt.Sprintf("JSON parse error: %v (raw: %.200s)", err, raw)
		return msg, Action{Type: ActionWait, Params: map[string]any{"seconds": 1}, Reasoning: msg}
	}

	actionType := ActionType(data.ActionStr)
	// Validate action type
	switch actionType {
	case ActionClick, ActionDoubleClick, ActionRightClick, ActionType_,
		ActionHotkey, ActionScroll, ActionWait, ActionDone, ActionFail:
		// valid
	default:
		actionType = ActionWait
	}

	return data.Reasoning, Action{
		Type:      actionType,
		Params:    data.Params,
		Reasoning: data.Reasoning,
	}
}

// act executes the decided action via the in-process input controller.
func (r *ReActLoop) act(action Action) bool {
	var err error

	switch action.Type {
	case ActionClick:
		x, y := getIntParam(action.Params, "x", 0), getIntParam(action.Params, "y", 0)
		err = r.inputCtrl.Click(x, y, input.MouseLeft)

	case ActionDoubleClick:
		x, y := getIntParam(action.Params, "x", 0), getIntParam(action.Params, "y", 0)
		err = r.inputCtrl.DoubleClick(x, y)

	case ActionRightClick:
		// Bug fix: Python version called click(x, y, button=1) which maps to MouseRight
		x, y := getIntParam(action.Params, "x", 0), getIntParam(action.Params, "y", 0)
		err = r.inputCtrl.Click(x, y, input.MouseRight)

	case ActionType_:
		text := getStringParam(action.Params, "text", "")
		err = r.inputCtrl.Type(text)

	case ActionHotkey:
		keys := getKeysParam(action.Params)
		if len(keys) > 0 {
			err = r.inputCtrl.Hotkey(keys...)
		}

	case ActionScroll:
		x := getIntParam(action.Params, "x", 0)
		y := getIntParam(action.Params, "y", 0)
		dx := getIntParam(action.Params, "delta_x", 0)
		dy := getIntParam(action.Params, "delta_y", -3)
		err = r.inputCtrl.Scroll(x, y, dx, dy)

	case ActionWait:
		seconds := getFloatParam(action.Params, "seconds", 1)
		time.Sleep(time.Duration(seconds * float64(time.Second)))
		return true

	default:
		return true // DONE/FAIL don't need execution
	}

	if err != nil {
		log.Printf("[ReAct] Action %s failed: %v", action.Type, err)
		return false
	}
	return true
}

// verify checks if the action had the expected effect.
func (r *ReActLoop) verify(ctx context.Context, goal string, action Action) string {
	if r.vlmRouter == nil {
		return "Verification skipped (no VLM)"
	}

	jpegData := r.captureJPEG()
	if jpegData == nil {
		return "Verification skipped (no screenshot)"
	}

	prompt := fmt.Sprintf(verifyPromptTpl, goal, action.Type, action.Params, action.Reasoning)
	result, err := r.callVLMWithImage(ctx, jpegData, prompt)
	if err != nil {
		return fmt.Sprintf("Verification error: %v", err)
	}
	return result
}
