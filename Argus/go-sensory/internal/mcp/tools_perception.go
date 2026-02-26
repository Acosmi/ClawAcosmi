package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"Argus-compound/go-sensory/internal/agent"
	"Argus-compound/go-sensory/internal/capture"
	"Argus-compound/go-sensory/internal/imaging"
	"Argus-compound/go-sensory/internal/vlm"
)

// ──────────────────────────────────────────────────────────────
// Shared dependencies for perception tools
// ──────────────────────────────────────────────────────────────

// PerceptionDeps bundles the shared dependencies that all perception
// tools need.  This avoids passing 4+ constructor args per tool.
type PerceptionDeps struct {
	Capturer capture.Capturer
	VLM      vlm.Provider
	Scaler   *imaging.Scaler
	Parser   *agent.UIParser
}

// ──────────────────────────────────────────────────────────────
// RegisterPerceptionTools batch-registers all perception tools.
// ──────────────────────────────────────────────────────────────

// RegisterPerceptionTools adds all perception MCP tools to the registry.
func RegisterPerceptionTools(r *Registry, deps PerceptionDeps) {
	r.Register(&CaptureScreenTool{deps: deps})
	r.Register(&DescribeSceneTool{deps: deps})
	r.Register(&LocateElementTool{deps: deps})
	r.Register(&ReadTextTool{deps: deps})
	r.Register(&DetectDialogTool{deps: deps})
	r.Register(&WatchForChangeTool{deps: deps})
}

// waitForFrame polls the capturer for up to 5 seconds waiting for the first
// frame to become available.  This handles the common MCP startup race where a
// tool call arrives before the screen capture backend has produced its first
// frame.
func waitForFrame(ctx context.Context, capturer capture.Capturer) *capture.Frame {
	// Fast path: frame already available.
	if f := capturer.LatestFrame(); f != nil {
		return f
	}

	log.Println("[MCP] Waiting for first frame...")
	const maxWait = 5 * time.Second
	const poll = 200 * time.Millisecond

	deadline := time.After(maxWait)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deadline:
			return nil
		case <-ticker.C:
			if f := capturer.LatestFrame(); f != nil {
				log.Println("[MCP] First frame available")
				return f
			}
		}
	}
}

// ══════════════════════════════════════════════════════════════
// Tool 1: capture_screen
// ══════════════════════════════════════════════════════════════

// CaptureScreenTool captures the current screen as a JPEG image.
type CaptureScreenTool struct{ deps PerceptionDeps }

func (t *CaptureScreenTool) Name() string { return "capture_screen" }
func (t *CaptureScreenTool) Description() string {
	return "截屏当前屏幕，返回 base64 编码的 JPEG 图片"
}
func (t *CaptureScreenTool) Category() ToolCategory { return CategoryPerception }
func (t *CaptureScreenTool) Risk() RiskLevel        { return RiskLow }
func (t *CaptureScreenTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"quality": {
				Type:        "string",
				Description: "Image quality: 'vlm' (downscaled for AI) or 'display' (full resolution)",
				Enum:        []string{"vlm", "display"},
				Default:     "vlm",
			},
		},
	}
}

func (t *CaptureScreenTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Quality string `json:"quality"`
	}
	p.Quality = "vlm" // default
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}

	frame := waitForFrame(ctx, t.deps.Capturer)
	if frame == nil {
		return &ToolResult{IsError: true, Error: "no frame available (timeout after 5s — is screen recording permission granted?)"}, nil
	}

	var jpegBytes []byte
	var err error
	switch p.Quality {
	case "display":
		jpegBytes, err = t.deps.Scaler.ForDisplay(frame)
	default:
		jpegBytes, err = t.deps.Scaler.ForVLM(frame)
	}
	if err != nil {
		return nil, fmt.Errorf("encoding frame: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(jpegBytes)
	return &ToolResult{
		Content: map[string]any{
			"image_b64": b64,
			"width":     frame.Width,
			"height":    frame.Height,
			"quality":   p.Quality,
			"size_kb":   len(jpegBytes) / 1024,
		},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 2: describe_scene
// ══════════════════════════════════════════════════════════════

// DescribeSceneTool uses VLM to describe what's visible on screen.
type DescribeSceneTool struct{ deps PerceptionDeps }

func (t *DescribeSceneTool) Name() string           { return "describe_scene" }
func (t *DescribeSceneTool) Description() string    { return "用 VLM 分析并描述当前屏幕内容" }
func (t *DescribeSceneTool) Category() ToolCategory { return CategoryPerception }
func (t *DescribeSceneTool) Risk() RiskLevel        { return RiskLow }
func (t *DescribeSceneTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"focus": {
				Type:        "string",
				Description: "Optional focus area: what specific aspect to describe",
			},
		},
	}
}

func (t *DescribeSceneTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Focus string `json:"focus"`
	}
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}

	frame := waitForFrame(ctx, t.deps.Capturer)
	if frame == nil {
		return &ToolResult{IsError: true, Error: "no frame available (timeout after 5s — is screen recording permission granted?)"}, nil
	}

	jpegBytes, err := t.deps.Scaler.ForVLM(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding frame: %w", err)
	}

	prompt := "Describe what you see on this screen in detail. Include: active application, visible UI elements, any text content, and overall layout."
	if p.Focus != "" {
		prompt = fmt.Sprintf("Focus on: %s\n\n%s", p.Focus, prompt)
	}

	resp, err := callVLMWithImage(ctx, t.deps.VLM, jpegBytes, prompt)
	if err != nil {
		return nil, err
	}

	return &ToolResult{
		Content: map[string]any{
			"description": resp,
			"focus":       p.Focus,
		},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 3: locate_element
// ══════════════════════════════════════════════════════════════

// LocateElementTool finds a UI element matching a description using
// Set-of-Marks (SoM) grounding.
type LocateElementTool struct{ deps PerceptionDeps }

func (t *LocateElementTool) Name() string { return "locate_element" }
func (t *LocateElementTool) Description() string {
	return "通过描述定位屏幕上的 UI 元素（SoM 标注法）"
}
func (t *LocateElementTool) Category() ToolCategory { return CategoryPerception }
func (t *LocateElementTool) Risk() RiskLevel        { return RiskLow }
func (t *LocateElementTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"description": {
				Type:        "string",
				Description: "Natural language description of the element to find",
			},
		},
		Required: []string{"description"},
	}
}

func (t *LocateElementTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Description == "" {
		return &ToolResult{IsError: true, Error: "description is required"}, nil
	}

	if t.deps.Parser == nil {
		return &ToolResult{IsError: true, Error: "UIParser not configured"}, nil
	}

	frame := waitForFrame(ctx, t.deps.Capturer)
	if frame == nil {
		return &ToolResult{IsError: true, Error: "no frame available (timeout after 5s — is screen recording permission granted?)"}, nil
	}

	jpegBytes, err := t.deps.Scaler.ForDisplay(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding frame: %w", err)
	}

	// Use SoM grounding pipeline: detect → annotate → classify
	elem, err := t.deps.Parser.GroundWithSoM(ctx, jpegBytes, p.Description)
	if err != nil {
		return &ToolResult{IsError: true, Error: fmt.Sprintf("element not found: %v", err)}, nil
	}

	return &ToolResult{
		Content: elem.ToMap(),
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 4: read_text
// ══════════════════════════════════════════════════════════════

// ReadTextTool extracts text from a specified region or the full screen.
type ReadTextTool struct{ deps PerceptionDeps }

func (t *ReadTextTool) Name() string { return "read_text" }
func (t *ReadTextTool) Description() string {
	return "从屏幕截图中提取可见文本（OCR via VLM）"
}
func (t *ReadTextTool) Category() ToolCategory { return CategoryPerception }
func (t *ReadTextTool) Risk() RiskLevel        { return RiskLow }
func (t *ReadTextTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"region": {
				Type:        "string",
				Description: "Optional region hint: 'full' (default), 'top', 'bottom', 'center'",
				Enum:        []string{"full", "top", "bottom", "center"},
				Default:     "full",
			},
		},
	}
}

func (t *ReadTextTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Region string `json:"region"`
	}
	p.Region = "full"
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}

	frame := waitForFrame(ctx, t.deps.Capturer)
	if frame == nil {
		return &ToolResult{IsError: true, Error: "no frame available (timeout after 5s — is screen recording permission granted?)"}, nil
	}

	jpegBytes, err := t.deps.Scaler.ForVLM(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding frame: %w", err)
	}

	regionHint := ""
	if p.Region != "full" {
		regionHint = fmt.Sprintf(" Focus on the %s portion of the screen.", p.Region)
	}

	prompt := fmt.Sprintf("Extract ALL visible text from this screenshot.%s Return the text exactly as it appears, preserving line breaks. Do not interpret or summarize.", regionHint)

	text, err := callVLMWithImage(ctx, t.deps.VLM, jpegBytes, prompt)
	if err != nil {
		return nil, err
	}

	return &ToolResult{
		Content: map[string]any{
			"text":   strings.TrimSpace(text),
			"region": p.Region,
		},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 5: detect_dialog
// ══════════════════════════════════════════════════════════════

// DetectDialogTool detects system dialogs, alerts, or modal popups.
type DetectDialogTool struct{ deps PerceptionDeps }

func (t *DetectDialogTool) Name() string { return "detect_dialog" }
func (t *DetectDialogTool) Description() string {
	return "检测屏幕上的系统对话框/弹窗/权限请求"
}
func (t *DetectDialogTool) Category() ToolCategory { return CategoryPerception }
func (t *DetectDialogTool) Risk() RiskLevel        { return RiskLow }
func (t *DetectDialogTool) InputSchema() ToolSchema {
	return ToolSchema{Type: "object"}
}

func (t *DetectDialogTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	frame := waitForFrame(ctx, t.deps.Capturer)
	if frame == nil {
		return &ToolResult{IsError: true, Error: "no frame available (timeout after 5s — is screen recording permission granted?)"}, nil
	}

	jpegBytes, err := t.deps.Scaler.ForVLM(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding frame: %w", err)
	}

	prompt := `Analyze this screenshot for dialogs, alerts, or modal popups.
Respond in JSON:
{
  "has_dialog": true/false,
  "dialogs": [
    {
      "type": "alert|confirm|permission|error|info|file_picker|other",
      "title": "<dialog title>",
      "message": "<dialog message>",
      "buttons": ["<button labels>"],
      "is_system": true/false
    }
  ]
}`

	resp, err := callVLMWithImage(ctx, t.deps.VLM, jpegBytes, prompt)
	if err != nil {
		return nil, err
	}

	// Parse VLM response (may have markdown fences)
	text := strings.TrimSpace(resp)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			text = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		// Fallback: return raw text
		return &ToolResult{
			Content: map[string]any{
				"has_dialog": false,
				"raw":        resp,
			},
		}, nil
	}

	return &ToolResult{Content: result}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 6: watch_for_change
// ══════════════════════════════════════════════════════════════

// WatchForChangeTool polls the screen for changes over a time window.
type WatchForChangeTool struct{ deps PerceptionDeps }

func (t *WatchForChangeTool) Name() string { return "watch_for_change" }
func (t *WatchForChangeTool) Description() string {
	return "监控屏幕变化，等待特定事件出现"
}
func (t *WatchForChangeTool) Category() ToolCategory { return CategoryPerception }
func (t *WatchForChangeTool) Risk() RiskLevel        { return RiskLow }
func (t *WatchForChangeTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"watch_for": {
				Type:        "string",
				Description: "Description of the change to watch for",
			},
			"timeout_seconds": {
				Type:        "integer",
				Description: "Maximum time to wait in seconds (default 10, max 30)",
				Default:     10,
			},
			"interval_ms": {
				Type:        "integer",
				Description: "Polling interval in milliseconds (default 1000, min 500)",
				Default:     1000,
			},
		},
		Required: []string{"watch_for"},
	}
}

func (t *WatchForChangeTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		WatchFor       string `json:"watch_for"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		IntervalMs     int    `json:"interval_ms"`
	}
	p.TimeoutSeconds = 10
	p.IntervalMs = 1000
	if len(params) > 0 {
		json.Unmarshal(params, &p)
	}
	if p.WatchFor == "" {
		return &ToolResult{IsError: true, Error: "watch_for is required"}, nil
	}

	// Guard bounds
	if p.TimeoutSeconds <= 0 || p.TimeoutSeconds > 30 {
		p.TimeoutSeconds = 10
	}
	if p.IntervalMs < 500 {
		p.IntervalMs = 500
	}

	timeout := time.Duration(p.TimeoutSeconds) * time.Second
	interval := time.Duration(p.IntervalMs) * time.Millisecond

	watchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startTime := time.Now()
	iterations := 0

	for {
		select {
		case <-watchCtx.Done():
			return &ToolResult{
				Content: map[string]any{
					"detected":   false,
					"watch_for":  p.WatchFor,
					"elapsed_ms": time.Since(startTime).Milliseconds(),
					"iterations": iterations,
					"reason":     "timeout",
				},
			}, nil

		case <-ticker.C:
			iterations++
			frame := t.deps.Capturer.LatestFrame()
			if frame == nil {
				continue
			}

			jpegBytes, err := t.deps.Scaler.ForVLM(frame)
			if err != nil {
				log.Printf("[MCP:watch_for_change] encoding error: %v", err)
				continue
			}

			prompt := fmt.Sprintf(`Look at this screenshot. Has the following change occurred?
Change to detect: %s

Respond with ONLY a JSON object:
{"detected": true/false, "description": "<what you see>", "confidence": 0.0-1.0}`, p.WatchFor)

			resp, err := callVLMWithImage(ctx, t.deps.VLM, jpegBytes, prompt)
			if err != nil {
				log.Printf("[MCP:watch_for_change] VLM error: %v", err)
				continue
			}

			// Parse response
			text := strings.TrimSpace(resp)
			if strings.HasPrefix(text, "```") {
				lines := strings.Split(text, "\n")
				if len(lines) >= 3 {
					text = strings.Join(lines[1:len(lines)-1], "\n")
				}
			}

			var result struct {
				Detected    bool    `json:"detected"`
				Description string  `json:"description"`
				Confidence  float64 `json:"confidence"`
			}
			if err := json.Unmarshal([]byte(text), &result); err == nil && result.Detected {
				return &ToolResult{
					Content: map[string]any{
						"detected":    true,
						"watch_for":   p.WatchFor,
						"description": result.Description,
						"confidence":  result.Confidence,
						"elapsed_ms":  time.Since(startTime).Milliseconds(),
						"iterations":  iterations,
					},
				}, nil
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Shared VLM helper
// ──────────────────────────────────────────────────────────────

// callVLMWithImage sends a JPEG + text prompt to the VLM and returns the
// text response.  Shared utility across all perception tools.
func callVLMWithImage(ctx context.Context, provider vlm.Provider, jpegData []byte, prompt string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("VLM provider not configured; set VLM_API_KEY, GEMINI_API_KEY, or OLLAMA_ENDPOINT environment variable")
	}
	b64 := base64.StdEncoding.EncodeToString(jpegData)
	req := vlm.ChatRequest{
		Messages: []vlm.Message{
			{
				Role: "user",
				Content: []vlm.ContentPart{
					{Type: "text", Text: prompt},
					{Type: "image_url", ImageURL: &vlm.ImageURL{
						URL:    "data:image/jpeg;base64," + b64,
						Detail: "high",
					}},
				},
			},
		},
		MaxTokens: 2048,
	}

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("VLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("VLM returned no choices")
	}
	return resp.Choices[0].Message.GetTextContent(), nil
}
