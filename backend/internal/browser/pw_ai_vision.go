// pw_ai_vision.go — AI vision utilities for browser automation.
//
// Provides screenshot annotation and Vision API prompt construction.
// Aligns with TS screenshotWithLabelsViaPlaywright + snapshotAiViaPlaywright.
//
// Design: These utilities are consumed by the AIBrowseLoop and Agent Runner
// pipeline to build multimodal prompts for LLM Vision analysis.
package browser

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// VisionPromptConfig configures how the AI vision prompt is built.
type VisionPromptConfig struct {
	IncludeScreenshot bool   // include base64 screenshot in prompt
	IncludeARIA       bool   // include ARIA snapshot text
	MaxSnapshotLines  int    // truncate ARIA snapshot to N lines; default 200
	Language          string // "en" | "zh"; affects prompt language
}

// DefaultVisionPromptConfig returns sensible defaults.
func DefaultVisionPromptConfig() VisionPromptConfig {
	return VisionPromptConfig{
		IncludeScreenshot: true,
		IncludeARIA:       true,
		MaxSnapshotLines:  200,
		Language:          "en",
	}
}

// VisionPrompt is a constructed multimodal prompt for LLM Vision.
type VisionPrompt struct {
	SystemPrompt string `json:"systemPrompt"`
	UserPrompt   string `json:"userPrompt"`
	ImageBase64  string `json:"imageBase64,omitempty"` // PNG screenshot
}

// BuildVisionPrompt constructs a multimodal prompt from browser state.
func BuildVisionPrompt(goal string, state AIBrowseState, cfg VisionPromptConfig) *VisionPrompt {
	prompt := &VisionPrompt{}

	// System prompt establishes the AI agent role.
	if cfg.Language == "zh" {
		prompt.SystemPrompt = visionSystemPromptZh
	} else {
		prompt.SystemPrompt = visionSystemPromptEn
	}

	// Build user prompt with browser state context.
	var parts []string

	parts = append(parts, fmt.Sprintf("**Goal**: %s", goal))
	parts = append(parts, fmt.Sprintf("**Step**: %d", state.StepNumber))

	if state.CurrentURL != "" {
		parts = append(parts, fmt.Sprintf("**Current URL**: %s", state.CurrentURL))
	}

	// Include ARIA snapshot.
	if cfg.IncludeARIA && state.AriaSnapshot != nil {
		if snapshotText, ok := state.AriaSnapshot["snapshot"].(string); ok {
			lines := strings.Split(snapshotText, "\n")
			if cfg.MaxSnapshotLines > 0 && len(lines) > cfg.MaxSnapshotLines {
				lines = lines[:cfg.MaxSnapshotLines]
				lines = append(lines, "... (truncated)")
			}
			parts = append(parts, "**Page Structure (ARIA)**:")
			parts = append(parts, "```")
			parts = append(parts, strings.Join(lines, "\n"))
			parts = append(parts, "```")
		}
	}

	parts = append(parts, "")
	if cfg.Language == "zh" {
		parts = append(parts, visionActionInstructionZh)
	} else {
		parts = append(parts, visionActionInstructionEn)
	}

	prompt.UserPrompt = strings.Join(parts, "\n")

	// Attach screenshot.
	if cfg.IncludeScreenshot && len(state.Screenshot) > 0 {
		prompt.ImageBase64 = base64.StdEncoding.EncodeToString(state.Screenshot)
	}

	return prompt
}

// AnnotateSnapshotForAI adds ref annotations to an ARIA snapshot text
// for AI consumption. Each interactive element gets a [ref=eN] suffix.
func AnnotateSnapshotForAI(ariaText string, refs map[string]RoleRef) string {
	if len(refs) == 0 {
		return ariaText
	}

	lines := strings.Split(ariaText, "\n")
	var result []string

	for _, line := range lines {
		annotated := line
		for ref, info := range refs {
			// Match role + name pattern.
			pattern := fmt.Sprintf(`%s "%s"`, info.Role, info.Name)
			if strings.Contains(line, pattern) {
				annotated = line + fmt.Sprintf(" [ref=%s]", ref)
				break
			}
		}
		result = append(result, annotated)
	}

	return strings.Join(result, "\n")
}

// --- Prompt templates ---

const visionSystemPromptEn = `You are a browser automation AI agent. You analyze screenshots and page structure to determine the next action needed to achieve a goal.

You MUST respond with a JSON object containing:
- "type": one of "click", "fill", "navigate", "scroll", "wait", "done"
- "ref": element reference (e.g. "e1") for click/fill actions
- "value": text for fill, URL for navigate
- "reasoning": brief explanation of why this action is needed

Rules:
- Use element refs from the ARIA snapshot (e.g. "e1", "e2")
- If the goal is achieved, respond with type "done"
- If stuck, try scrolling or navigating
- Never take more actions than needed`

const visionSystemPromptZh = `你是一个浏览器自动化 AI 代理。你分析截图和页面结构来决定实现目标所需的下一步操作。

你必须以 JSON 对象格式回复，包含：
- "type": "click" | "fill" | "navigate" | "scroll" | "wait" | "done" 之一
- "ref": 元素引用（如 "e1"）用于 click/fill 操作
- "value": fill 的文本或 navigate 的 URL
- "reasoning": 简要说明为什么需要这个操作

规则：
- 使用 ARIA 快照中的元素引用（如 "e1", "e2"）
- 如果目标已达成，回复 type "done"
- 如果卡住了，尝试滚动或导航
- 不要执行多余的操作`

const visionActionInstructionEn = `Based on the above page state, what is the next action to achieve the goal? Respond with a JSON object.`

const visionActionInstructionZh = `基于以上页面状态，实现目标的下一步操作是什么？请以 JSON 对象格式回复。`
