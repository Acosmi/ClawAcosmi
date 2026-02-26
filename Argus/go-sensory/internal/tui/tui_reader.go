// Package tui provides terminal UI (TUI) recognition, interaction, and
// verification capabilities.  It enables go-sensory to understand what
// is happening in a terminal session (via OCR + VLM analysis) and
// interact with it through simulated keyboard input — all gated by
// the Approval Gateway for privacy-first control.
package tui

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"log"
	"strings"
	"time"

	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Terminal prompt types
// ──────────────────────────────────────────────────────────────

// PromptType classifies the kind of terminal prompt detected.
type PromptType string

const (
	PromptYesNo    PromptType = "yes_no"   // e.g. "Proceed? [Y/n]"
	PromptChoice   PromptType = "choice"   // e.g. "Select (1-5):"
	PromptDiff     PromptType = "diff"     // e.g. git diff, patch review
	PromptInput    PromptType = "input"    // free-form text input
	PromptPassword PromptType = "password" // password prompt (masked)
	PromptIdle     PromptType = "idle"     // shell ready, no prompt
	PromptRunning  PromptType = "running"  // command in progress
	PromptUnknown  PromptType = "unknown"
)

// ──────────────────────────────────────────────────────────────
// Terminal state
// ──────────────────────────────────────────────────────────────

// TerminalState describes the current state of a terminal session
// as understood by VLM analysis.
type TerminalState struct {
	// Raw text extracted via OCR.
	RawText string `json:"raw_text"`

	// VLM-generated semantic description of the terminal.
	Description string `json:"description"`

	// Detected prompt type.
	Prompt PromptType `json:"prompt_type"`

	// Prompt text if a prompt was detected.
	PromptText string `json:"prompt_text,omitempty"`

	// Available options (for choice/yes_no prompts).
	Options []string `json:"options,omitempty"`

	// Current working directory (if detectable).
	CWD string `json:"cwd,omitempty"`

	// Running process name (if detectable).
	RunningProcess string `json:"running_process,omitempty"`

	// Whether the terminal appears to be waiting for input.
	WaitingForInput bool `json:"waiting_for_input"`

	// Confidence score (0-1) from the VLM analysis.
	Confidence float64 `json:"confidence"`

	// Timestamp of the analysis.
	Timestamp time.Time `json:"timestamp"`
}

// ──────────────────────────────────────────────────────────────
// TUI Reader — terminal state recognition
// ──────────────────────────────────────────────────────────────

// Reader captures and analyzes terminal screen state using the
// dual-track approach: OCR for raw text extraction + VLM for
// semantic understanding.
type Reader struct {
	capturer capture.Capturer
	vlm      vlm.Provider
}

// NewReader creates a TUI reader with the given dependencies.
func NewReader(capturer capture.Capturer, vlmProvider vlm.Provider) *Reader {
	return &Reader{
		capturer: capturer,
		vlm:      vlmProvider,
	}
}

// ReadState captures the current terminal screen and analyzes it.
// Returns a TerminalState with prompt classification and context.
func (r *Reader) ReadState(ctx context.Context) (*TerminalState, error) {
	// 1. Capture current frame
	frame := r.capturer.LatestFrame()
	if frame == nil {
		return nil, fmt.Errorf("no frame available from capturer")
	}

	// 2. Encode frame as base64 PNG for VLM
	b64, err := frameToBase64PNG(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding frame: %w", err)
	}

	// 3. Send to VLM for analysis
	state, err := r.analyzeTerminal(ctx, b64)
	if err != nil {
		return nil, fmt.Errorf("VLM analysis: %w", err)
	}

	state.Timestamp = time.Now()
	return state, nil
}

// WaitForPrompt polls the terminal until a prompt is detected or
// the context expires.  This is useful after executing a command
// to wait for the output to stabilize.
func (r *Reader) WaitForPrompt(ctx context.Context, interval time.Duration) (*TerminalState, error) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			state, err := r.ReadState(ctx)
			if err != nil {
				log.Printf("[TUI] WaitForPrompt poll error: %v", err)
				continue
			}
			if state.WaitingForInput || state.Prompt == PromptIdle {
				return state, nil
			}
		}
	}
}

// analyzeTerminal sends a screenshot to the VLM with a structured
// prompt asking it to classify the terminal state.
func (r *Reader) analyzeTerminal(ctx context.Context, screenshotB64 string) (*TerminalState, error) {
	prompt := `Analyze this terminal/TUI screenshot. Respond in JSON only:
{
  "raw_text": "<visible text in terminal>",
  "description": "<1-2 sentence description of what's happening>",
  "prompt_type": "yes_no|choice|diff|input|password|idle|running|unknown",
  "prompt_text": "<the prompt text if any>",
  "options": ["<available options if choice/yes_no>"],
  "cwd": "<current directory if visible>",
  "running_process": "<process name if running>",
  "waiting_for_input": true/false,
  "confidence": 0.0-1.0
}`

	req := vlm.ChatRequest{
		Messages: []vlm.Message{
			{
				Role: "user",
				Content: []vlm.ContentPart{
					{Type: "text", Text: prompt},
					{Type: "image_url", ImageURL: &vlm.ImageURL{
						URL:    "data:image/png;base64," + screenshotB64,
						Detail: "high",
					}},
				},
			},
		},
		MaxTokens: 1024,
	}

	resp, err := r.vlm.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("VLM returned no choices")
	}

	content := resp.Choices[0].Message.GetTextContent()
	return parseTerminalState(content)
}

// parseTerminalState extracts a TerminalState from VLM JSON output.
// Handles cases where the VLM wraps JSON in markdown code fences.
func parseTerminalState(raw string) (*TerminalState, error) {
	// Strip markdown code fences if present
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		// Remove first and last lines (the fences)
		if len(lines) >= 3 {
			text = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var state TerminalState
	if err := parseJSON(text, &state); err != nil {
		return nil, fmt.Errorf("parsing VLM response: %w (raw: %.200s)", err, raw)
	}
	return &state, nil
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// frameToBase64PNG converts a capture.Frame (BGRA raw pixels) to
// a base64-encoded PNG string.
func frameToBase64PNG(f *capture.Frame) (string, error) {
	if f == nil || len(f.Pixels) == 0 {
		return "", fmt.Errorf("empty frame")
	}

	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			srcIdx := y*f.Stride + x*f.Channels
			if srcIdx+3 >= len(f.Pixels) {
				break
			}
			dstIdx := y*img.Stride + x*4
			// BGRA → RGBA
			img.Pix[dstIdx+0] = f.Pixels[srcIdx+2] // R
			img.Pix[dstIdx+1] = f.Pixels[srcIdx+1] // G
			img.Pix[dstIdx+2] = f.Pixels[srcIdx+0] // B
			img.Pix[dstIdx+3] = f.Pixels[srcIdx+3] // A
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
