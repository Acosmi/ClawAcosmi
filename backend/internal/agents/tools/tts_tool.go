// tools/tts_tool.go — 文本转语音工具。
// TS 参考：src/agents/tools/tts-tool.ts (61L)
package tools

import (
	"context"
	"fmt"
)

// TTSProvider 文本转语音接口。
type TTSProvider interface {
	Synthesize(ctx context.Context, text string, opts TTSOpts) ([]byte, string, error)
}

// TTSOpts TTS 选项。
type TTSOpts struct {
	Voice    string  `json:"voice,omitempty"`
	Language string  `json:"language,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
}

// CreateTTSTool 创建文本转语音工具。
func CreateTTSTool(provider TTSProvider) *AgentTool {
	return &AgentTool{
		Name:        "tts",
		Label:       "Text-to-Speech",
		Description: "Convert text to speech audio.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type": "string", "description": "Text to convert to speech",
				},
				"voice": map[string]any{
					"type": "string", "description": "Voice ID or name (optional)",
				},
				"language": map[string]any{
					"type": "string", "description": "Language code (optional, e.g. 'en', 'ja')",
				},
				"speed": map[string]any{
					"type": "number", "description": "Speed multiplier (default: 1.0)",
				},
			},
			"required": []any{"text"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			text, err := ReadStringParam(args, "text", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			voice, _ := ReadStringParam(args, "voice", nil)
			language, _ := ReadStringParam(args, "language", nil)
			speed := 1.0
			if s, ok, _ := ReadNumberParam(args, "speed", nil); ok {
				speed = s
			}

			if provider == nil {
				return nil, fmt.Errorf("TTS provider not configured")
			}

			data, mimeType, err := provider.Synthesize(ctx, text, TTSOpts{
				Voice: voice, Language: language, Speed: speed,
			})
			if err != nil {
				return nil, fmt.Errorf("TTS synthesis failed: %w", err)
			}

			return JsonResult(map[string]any{
				"status":    "synthesized",
				"mimeType":  mimeType,
				"sizeBytes": len(data),
			}), nil
		},
	}
}
