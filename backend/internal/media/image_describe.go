package media

// image_describe.go — 图片理解 Fallback 接口 + 工厂（Phase E 新增）
// 当主模型不支持多模态时，通过独立视觉 API 将图片转为文字描述。
// 遵循 STTProvider / DocConverter 完全相同的模式。

import (
	"context"
	"fmt"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ImageDescriber 图片理解 Provider 接口
type ImageDescriber interface {
	// Describe 将图片数据转为文字描述
	// imageData: 原始图片二进制数据
	// mimeType: 图片 MIME 类型 (image/png, image/jpeg 等)
	// Returns: 文字描述, 错误
	Describe(ctx context.Context, imageData []byte, mimeType string) (string, error)

	// Name 返回 Provider 名称
	Name() string

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}

// NewImageDescriber 根据配置创建 ImageDescriber（工厂方法）
func NewImageDescriber(cfg *types.ImageUnderstandingConfig) (ImageDescriber, error) {
	if cfg == nil || cfg.Provider == "" {
		return nil, fmt.Errorf("image: provider not configured")
	}

	// 创建局部副本，避免修改调用方的入参指针
	local := *cfg

	switch local.Provider {
	case "qwen-vl":
		if local.BaseURL == "" {
			local.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		if local.Model == "" {
			local.Model = "qwen-vl-plus"
		}
		return NewOpenAIImageDescriber(&local), nil

	case "openai":
		if local.BaseURL == "" {
			local.BaseURL = "https://api.openai.com/v1"
		}
		if local.Model == "" {
			local.Model = "gpt-4o-mini"
		}
		return NewOpenAIImageDescriber(&local), nil

	case "ollama":
		if local.BaseURL == "" {
			local.BaseURL = "http://localhost:11434/v1"
		}
		if local.Model == "" {
			local.Model = "llava"
		}
		return NewOpenAIImageDescriber(&local), nil

	case "google":
		if local.BaseURL == "" {
			local.BaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		}
		if local.Model == "" {
			local.Model = "gemini-2.0-flash"
		}
		return NewOpenAIImageDescriber(&local), nil

	case "anthropic":
		if local.BaseURL == "" {
			local.BaseURL = "https://api.anthropic.com/v1"
		}
		if local.Model == "" {
			local.Model = "claude-3-haiku-20240307"
		}
		return NewAnthropicImageDescriber(&local), nil

	default:
		return nil, fmt.Errorf("image: unknown provider %q", local.Provider)
	}
}

// DefaultImageModels 返回指定 provider 的默认视觉模型列表
func DefaultImageModels(provider string) []string {
	switch provider {
	case "qwen-vl":
		return []string{"qwen-vl-plus", "qwen-vl-max", "qwen2.5-vl-72b-instruct"}
	case "openai":
		return []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo"}
	case "ollama":
		return []string{"llava", "llava:13b", "bakllava", "moondream", "llama3.2-vision"}
	case "google":
		return []string{"gemini-2.0-flash", "gemini-1.5-flash", "gemini-1.5-pro"}
	case "anthropic":
		return []string{"claude-3-haiku-20240307", "claude-3-5-sonnet-20241022", "claude-3-opus-20240229"}
	default:
		return nil
	}
}
