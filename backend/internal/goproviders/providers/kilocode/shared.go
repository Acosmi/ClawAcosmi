// providers/kilocode/shared.go — Kilocode 共享常量与模型目录
// 对应 TS 文件: src/providers/kilocode-shared.ts
// 包含 Kilocode API 基础 URL、默认模型配置和模型目录。
package kilocode

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// KilocodeBaseURL Kilocode API 网关基础 URL。
const KilocodeBaseURL = "https://api.kilo.ai/api/gateway/"

// KilocodeDefaultModelID 默认模型 ID。
const KilocodeDefaultModelID = "anthropic/claude-opus-4.6"

// KilocodeDefaultModelRef 默认模型引用（含 kilocode/ 前缀）。
const KilocodeDefaultModelRef = "kilocode/" + KilocodeDefaultModelID

// KilocodeDefaultModelName 默认模型显示名称。
const KilocodeDefaultModelName = "Claude Opus 4.6"

// KilocodeDefaultContextWindow 默认上下文窗口大小。
const KilocodeDefaultContextWindow = 1000000

// KilocodeDefaultMaxTokens 默认最大输出 token 数。
const KilocodeDefaultMaxTokens = 128000

// KilocodeDefaultCost Kilocode 默认调用费用（全部免费）。
var KilocodeDefaultCost = types.ModelCost{
	Input:      0,
	Output:     0,
	CacheRead:  0,
	CacheWrite: 0,
}

// KilocodeModelCatalogEntry Kilocode 模型目录条目。
// 对应 TS: KilocodeModelCatalogEntry
type KilocodeModelCatalogEntry struct {
	// ID 模型标识符
	ID string
	// Name 模型显示名称
	Name string
	// Reasoning 是否支持推理
	Reasoning bool
	// Input 支持的输入类型
	Input []string
	// ContextWindow 上下文窗口大小（可选，0 表示未设置）
	ContextWindow int
	// MaxTokens 最大输出 token 数（可选，0 表示未设置）
	MaxTokens int
}

// KilocodeModelCatalog Kilocode 可用模型目录。
// 对应 TS: KILOCODE_MODEL_CATALOG
var KilocodeModelCatalog = []KilocodeModelCatalogEntry{
	{
		ID:            KilocodeDefaultModelID,
		Name:          KilocodeDefaultModelName,
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 1000000,
		MaxTokens:     128000,
	},
	{
		ID:            "z-ai/glm-5:free",
		Name:          "GLM-5 (Free)",
		Reasoning:     true,
		Input:         []string{"text"},
		ContextWindow: 202800,
		MaxTokens:     131072,
	},
	{
		ID:            "minimax/minimax-m2.5:free",
		Name:          "MiniMax M2.5 (Free)",
		Reasoning:     true,
		Input:         []string{"text"},
		ContextWindow: 204800,
		MaxTokens:     131072,
	},
	{
		ID:            "anthropic/claude-sonnet-4.5",
		Name:          "Claude Sonnet 4.5",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 1000000,
		MaxTokens:     64000,
	},
	{
		ID:            "openai/gpt-5.2",
		Name:          "GPT-5.2",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 400000,
		MaxTokens:     128000,
	},
	{
		ID:            "google/gemini-3-pro-preview",
		Name:          "Gemini 3 Pro Preview",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 1048576,
		MaxTokens:     65536,
	},
	{
		ID:            "google/gemini-3-flash-preview",
		Name:          "Gemini 3 Flash Preview",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 1048576,
		MaxTokens:     65535,
	},
	{
		ID:            "x-ai/grok-code-fast-1",
		Name:          "Grok Code Fast 1",
		Reasoning:     true,
		Input:         []string{"text"},
		ContextWindow: 256000,
		MaxTokens:     10000,
	},
	{
		ID:            "moonshotai/kimi-k2.5",
		Name:          "Kimi K2.5",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		ContextWindow: 262144,
		MaxTokens:     65535,
	},
}
