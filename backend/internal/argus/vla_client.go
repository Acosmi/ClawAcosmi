package argus

// vla_client.go — VLA 模型客户端统一接口
//
// VLAClient 定义视觉-语言-动作模型的统一推理接口。
// 当前实现：AnthropicVisionClient（云端 fallback，无需本地 GPU）。
// 未来扩展：ShowUIClient、OpenCUAClient（需本地 vLLM + GPU）。

import (
	"context"
	"fmt"
	"log/slog"
)

// VLAClient 视觉-语言-动作模型推理接口。
type VLAClient interface {
	// Infer 对截图进行视觉分析，返回建议动作。
	Infer(ctx context.Context, goal string, screenshot []byte, w, h int) (*VLMActionResult, error)
	// ModelID 返回模型标识。
	ModelID() string
}

// ---------- NoopVLAClient：仅截图模式 ----------

// NoopVLAClient 不执行 VLM 推理，仅返回空结果。
// 用于 vlaModel="none" 或无 API Key 时的 fallback。
type NoopVLAClient struct{}

func (c *NoopVLAClient) Infer(_ context.Context, _ string, _ []byte, _, _ int) (*VLMActionResult, error) {
	return nil, nil
}

func (c *NoopVLAClient) ModelID() string { return "none" }

// ---------- AnthropicVisionClient：云端 fallback ----------

// AnthropicVisionClient 通过 Anthropic Vision API 分析截图。
// 作为零本地部署的 VLA fallback，适合快速验证。
type AnthropicVisionClient struct {
	Endpoint string // API endpoint（默认使用系统级 Anthropic 配置）
	Model    string // 模型名（默认 claude-opus-4-6）
	APIKey   string // 可选，为空时使用系统级配置
}

func (c *AnthropicVisionClient) Infer(ctx context.Context, goal string, screenshot []byte, w, h int) (*VLMActionResult, error) {
	if len(screenshot) == 0 {
		return nil, fmt.Errorf("empty screenshot")
	}

	// TODO: 实际调用 Anthropic Vision API
	// 当前返回占位结果，待接入真实 API
	slog.Debug("anthropic vision infer (stub)",
		"goal", goal,
		"imageSize", len(screenshot),
		"resolution", fmt.Sprintf("%dx%d", w, h),
	)

	return &VLMActionResult{
		Action:    "DONE",
		Reasoning: "stub: Anthropic Vision API not yet connected",
	}, nil
}

func (c *AnthropicVisionClient) ModelID() string {
	if c.Model != "" {
		return c.Model
	}
	return "anthropic-vision"
}

// ---------- 工厂函数 ----------

// NewVLAClient 根据模型名创建对应的 VLA 客户端。
func NewVLAClient(model, endpoint, apiKey string) VLAClient {
	switch model {
	case "anthropic":
		return &AnthropicVisionClient{
			Endpoint: endpoint,
			Model:    "claude-opus-4-6",
			APIKey:   apiKey,
		}
	case "none", "":
		return &NoopVLAClient{}
	default:
		// showui-2b / opencua-7b 等本地模型（未来实现）
		slog.Warn("unknown VLA model, falling back to noop",
			"model", model,
		)
		return &NoopVLAClient{}
	}
}
