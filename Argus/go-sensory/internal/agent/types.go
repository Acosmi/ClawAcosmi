// Package agent implements the ReAct (Reason + Act) decision loop
// and UI element parsing, all running in-process with zero HTTP overhead.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/input"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Consumer-side minimal interfaces (§4.4: 接口定义在消费侧)
//
// These narrow interfaces decouple agent from the full capture/input/vlm APIs.
// capture.Capturer, input.InputController, and vlm.Provider satisfy these
// implicitly — no explicit assertion needed.
// ──────────────────────────────────────────────────────────────

// FrameSource provides screen frames for observation.
// Consumer-side subset of capture.Capturer.
type FrameSource interface {
	LatestFrame() *capture.Frame
}

// ActionExecutor performs input actions on the host OS.
// Consumer-side subset of input.InputController.
type ActionExecutor interface {
	Click(x, y int, button input.MouseButton) error
	DoubleClick(x, y int) error
	Type(text string) error
	Hotkey(keys ...input.Key) error
	Scroll(x, y int, deltaX, deltaY int) error
}

// VLMChatter sends chat completion requests to a VLM provider.
// Consumer-side subset of vlm.Provider.
type VLMChatter interface {
	ChatCompletion(ctx context.Context, req vlm.ChatRequest) (*vlm.ChatResponse, error)
}

// ──────────────────────────────────────────────────────────────
// Action Types
// ──────────────────────────────────────────────────────────────

// ActionType enumerates supported agent actions.
type ActionType string

const (
	ActionClick       ActionType = "click"
	ActionDoubleClick ActionType = "double_click"
	ActionRightClick  ActionType = "right_click"
	ActionType_       ActionType = "type" // ActionType_ avoids name collision with the type itself
	ActionHotkey      ActionType = "hotkey"
	ActionScroll      ActionType = "scroll"
	ActionWait        ActionType = "wait"
	ActionDone        ActionType = "done"
	ActionFail        ActionType = "fail"
)

// Action represents an action the agent decides to perform.
type Action struct {
	Type      ActionType     `json:"action"`
	Params    map[string]any `json:"params,omitempty"`
	Reasoning string         `json:"reasoning,omitempty"`
}

// Observation captures what the agent sees on screen.
type Observation struct {
	ScreenshotJPEG   []byte      `json:"-"`
	Description      string      `json:"description,omitempty"`
	DetectedElements []UIElement `json:"detected_elements,omitempty"`
	Timestamp        float64     `json:"timestamp"`
}

// Step records one iteration of the ReAct loop.
type Step struct {
	StepNo       int         `json:"step_no"`
	Observation  Observation `json:"-"` // too large to serialize
	Thought      string      `json:"thought"`
	Action       Action      `json:"action"`
	Success      bool        `json:"success"`
	Verification string      `json:"verification,omitempty"`
	DurationMs   float64     `json:"duration_ms"`
}

// TaskResult holds the final outcome of executing a ReAct task.
type TaskResult struct {
	Goal            string  `json:"goal"`
	Success         bool    `json:"success"`
	Steps           []Step  `json:"steps"`
	TotalDurationMs float64 `json:"total_duration_ms"`
	Error           string  `json:"error,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// UI Element Types (for UIParser / SoM)
// ──────────────────────────────────────────────────────────────

// ElementType classifies a UI element.
type ElementType string

const (
	ElemButton    ElementType = "button"
	ElemTextField ElementType = "text_field"
	ElemCheckbox  ElementType = "checkbox"
	ElemRadio     ElementType = "radio"
	ElemDropdown  ElementType = "dropdown"
	ElemLink      ElementType = "link"
	ElemIcon      ElementType = "icon"
	ElemMenuItem  ElementType = "menu_item"
	ElemTab       ElementType = "tab"
	ElemSlider    ElementType = "slider"
	ElemImage     ElementType = "image"
	ElemText      ElementType = "text"
	ElemWindow    ElementType = "window"
	ElemDialog    ElementType = "dialog"
	ElemToolbar   ElementType = "toolbar"
	ElemUnknown   ElementType = "unknown"
)

// validElementTypes for quick lookup.
var validElementTypes = map[ElementType]bool{
	ElemButton: true, ElemTextField: true, ElemCheckbox: true,
	ElemRadio: true, ElemDropdown: true, ElemLink: true,
	ElemIcon: true, ElemMenuItem: true, ElemTab: true,
	ElemSlider: true, ElemImage: true, ElemText: true,
	ElemWindow: true, ElemDialog: true, ElemToolbar: true,
	ElemUnknown: true,
}

// ParseElementType safely converts a string to ElementType, defaulting to unknown.
func ParseElementType(s string) ElementType {
	et := ElementType(s)
	if validElementTypes[et] {
		return et
	}
	return ElemUnknown
}

// UIElement represents a detected UI element on screen.
type UIElement struct {
	ID           int         `json:"id"`
	Type         ElementType `json:"type"`
	Label        string      `json:"label"`
	X1           int         `json:"x1"`
	Y1           int         `json:"y1"`
	X2           int         `json:"x2"`
	Y2           int         `json:"y2"`
	Confidence   float64     `json:"confidence"`
	Interactable bool        `json:"interactable"`
}

// Center returns the center point of the element's bounding box.
func (e UIElement) Center() (int, int) {
	return (e.X1 + e.X2) / 2, (e.Y1 + e.Y2) / 2
}

// Width returns the element's width.
func (e UIElement) Width() int { return e.X2 - e.X1 }

// Height returns the element's height.
func (e UIElement) Height() int { return e.Y2 - e.Y1 }

// ToMap converts for JSON serialization.
func (e UIElement) ToMap() map[string]any {
	cx, cy := e.Center()
	return map[string]any{
		"id":           e.ID,
		"type":         string(e.Type),
		"label":        e.Label,
		"bbox":         map[string]int{"x1": e.X1, "y1": e.Y1, "x2": e.X2, "y2": e.Y2},
		"center":       map[string]int{"x": cx, "y": cy},
		"interactable": e.Interactable,
		"confidence":   e.Confidence,
	}
}

// ──────────────────────────────────────────────────────────────
// SoM Color Palette
// ──────────────────────────────────────────────────────────────

// SoMColors defines the numbered-label color palette (hex strings).
var SoMColors = []string{
	"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7",
	"#DDA0DD", "#98D8C8", "#F7DC6F", "#BB8FCE", "#85C1E9",
	"#F0B27A", "#82E0AA", "#F1948A", "#AED6F1", "#D7BDE2",
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// NowMs returns current time in milliseconds as float64.
func NowMs() float64 {
	return float64(time.Now().UnixMilli()) / 1000.0
}

// FormatStepHistory returns a text summary of the last N steps.
func FormatStepHistory(steps []Step, maxSteps int) string {
	if len(steps) == 0 {
		return ""
	}
	start := 0
	if len(steps) > maxSteps {
		start = len(steps) - maxSteps
	}
	var result string
	for _, s := range steps[start:] {
		mark := "✓"
		if !s.Success {
			mark = "✗"
		}
		params, _ := json.Marshal(s.Action.Params)
		result += fmt.Sprintf("  %s Step %d: %s(%s)\n", mark, s.StepNo, s.Action.Type, string(params))
	}
	return result
}
