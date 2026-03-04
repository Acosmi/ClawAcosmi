// onboard/models.go — 模型常量和构建函数
// 对应 TS 文件: src/commands/onboard-auth.models.ts
// 包含所有 Provider 的模型常量（URL、模型 ID、上下文窗口、费用）和构建函数。
package onboard

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ──────────────────────────────────────────────
// MiniMax 常量
// ──────────────────────────────────────────────

// DefaultMinimaxBaseURL MiniMax 默认 API 基础 URL。
const DefaultMinimaxBaseURL = "https://api.minimax.io/v1"

// MinimaxApiBaseURL MiniMax Anthropic 兼容 API URL。
const MinimaxApiBaseURL = "https://api.minimax.io/anthropic"

// MinimaxCnApiBaseURL MiniMax 中国区 API URL。
const MinimaxCnApiBaseURL = "https://api.minimaxi.com/anthropic"

// MinimaxHostedModelID MiniMax 托管模型 ID。
const MinimaxHostedModelID = "MiniMax-M2.5"

// MinimaxHostedModelRef MiniMax 托管模型引用。
var MinimaxHostedModelRef = "minimax/" + MinimaxHostedModelID

// DefaultMinimaxContextWindow MiniMax 默认上下文窗口大小。
const DefaultMinimaxContextWindow = 200000

// DefaultMinimaxMaxTokens MiniMax 默认最大输出 token 数。
const DefaultMinimaxMaxTokens = 8192

// ──────────────────────────────────────────────
// Moonshot 常量
// ──────────────────────────────────────────────

// MoonshotBaseURL Moonshot API 基础 URL。
const MoonshotBaseURL = "https://api.moonshot.ai/v1"

// MoonshotCnBaseURL Moonshot 中国区 API URL。
const MoonshotCnBaseURL = "https://api.moonshot.cn/v1"

// MoonshotDefaultModelID Moonshot 默认模型 ID。
const MoonshotDefaultModelID = "kimi-k2.5"

// MoonshotDefaultModelRef Moonshot 默认模型引用。
var MoonshotDefaultModelRef = "moonshot/" + MoonshotDefaultModelID

// MoonshotDefaultContextWindow Moonshot 默认上下文窗口。
const MoonshotDefaultContextWindow = 256000

// MoonshotDefaultMaxTokens Moonshot 默认最大 token 数。
const MoonshotDefaultMaxTokens = 8192

// KimiCodingModelID Kimi Coding 模型 ID。
const KimiCodingModelID = "k2p5"

// KimiCodingModelRef Kimi Coding 模型引用。
var KimiCodingModelRef = "kimi-coding/" + KimiCodingModelID

// ──────────────────────────────────────────────
// 千帆 常量
// ──────────────────────────────────────────────

// QianfanBaseURL 千帆 API 基础 URL（从外部导入，此处重新声明）。
const QianfanBaseURL = "https://qianfan.baidubce.com/v2"

// QianfanDefaultModelID 千帆默认模型 ID。
const QianfanDefaultModelID = "ernie-x1-turbo-32k"

// QianfanDefaultModelRef 千帆默认模型引用。
var QianfanDefaultModelRef = "qianfan/" + QianfanDefaultModelID

// ──────────────────────────────────────────────
// Z.AI (智谱) 常量
// ──────────────────────────────────────────────

// ZaiCodingGlobalBaseURL Z.AI Coding 全球 API URL。
const ZaiCodingGlobalBaseURL = "https://api.z.ai/api/coding/paas/v4"

// ZaiCodingCnBaseURL Z.AI Coding 中国区 API URL。
const ZaiCodingCnBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"

// ZaiGlobalBaseURL Z.AI 全球 API URL。
const ZaiGlobalBaseURL = "https://api.z.ai/api/paas/v4"

// ZaiCnBaseURL Z.AI 中国区 API URL。
const ZaiCnBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// ZaiDefaultModelID Z.AI 默认模型 ID。
const ZaiDefaultModelID = "glm-5"

// ResolveZaiBaseURL 根据 endpoint 名称解析 Z.AI 基础 URL。
func ResolveZaiBaseURL(endpoint string) string {
	switch endpoint {
	case "coding-cn":
		return ZaiCodingCnBaseURL
	case "global":
		return ZaiGlobalBaseURL
	case "cn":
		return ZaiCnBaseURL
	case "coding-global":
		return ZaiCodingGlobalBaseURL
	default:
		return ZaiGlobalBaseURL
	}
}

// ──────────────────────────────────────────────
// Mistral 常量
// ──────────────────────────────────────────────

// MistralBaseURL Mistral API 基础 URL。
const MistralBaseURL = "https://api.mistral.ai/v1"

// MistralDefaultModelID Mistral 默认模型 ID。
const MistralDefaultModelID = "mistral-large-latest"

// MistralDefaultModelRef Mistral 默认模型引用。
var MistralDefaultModelRef = "mistral/" + MistralDefaultModelID

// MistralDefaultContextWindow Mistral 默认上下文窗口。
const MistralDefaultContextWindow = 262144

// MistralDefaultMaxTokens Mistral 默认最大 token。
const MistralDefaultMaxTokens = 262144

// ──────────────────────────────────────────────
// xAI 常量
// ──────────────────────────────────────────────

// XaiBaseURL xAI API 基础 URL。
const XaiBaseURL = "https://api.x.ai/v1"

// XaiDefaultModelID xAI 默认模型 ID。
const XaiDefaultModelID = "grok-4"

// XaiDefaultModelRef xAI 默认模型引用。
var XaiDefaultModelRef = "xai/" + XaiDefaultModelID

// XaiDefaultContextWindow xAI 默认上下文窗口。
const XaiDefaultContextWindow = 131072

// XaiDefaultMaxTokens xAI 默认最大 token。
const XaiDefaultMaxTokens = 8192

// ──────────────────────────────────────────────
// 费用定义
// ──────────────────────────────────────────────

// MinimaxApiCost MiniMax API 费用（美元/百万 token）。
var MinimaxApiCost = types.ModelCost{Input: 0.3, Output: 1.2, CacheRead: 0.03, CacheWrite: 0.12}

// MinimaxHostedCost MiniMax 托管版费用（免费）。
var MinimaxHostedCost = types.ModelCost{}

// MinimaxLmStudioCost MiniMax LM Studio 费用（免费）。
var MinimaxLmStudioCost = types.ModelCost{}

// MoonshotDefaultCost Moonshot 默认费用（免费）。
var MoonshotDefaultCost = types.ModelCost{}

// ZaiDefaultCost Z.AI 默认费用（免费）。
var ZaiDefaultCost = types.ModelCost{}

// MistralDefaultCost Mistral 默认费用（免费）。
var MistralDefaultCost = types.ModelCost{}

// XaiDefaultCost xAI 默认费用（免费）。
var XaiDefaultCost = types.ModelCost{}

// ──────────────────────────────────────────────
// 模型目录
// ──────────────────────────────────────────────

type minimaxCatalogEntry struct {
	Name      string
	Reasoning bool
}

var minimaxModelCatalog = map[string]minimaxCatalogEntry{
	"MiniMax-M2.5":           {Name: "MiniMax M2.5", Reasoning: true},
	"MiniMax-M2.5-highspeed": {Name: "MiniMax M2.5 Highspeed", Reasoning: true},
	"MiniMax-M2.5-Lightning": {Name: "MiniMax M2.5 Lightning", Reasoning: true},
}

type zaiCatalogEntry struct {
	Name      string
	Reasoning bool
}

var zaiModelCatalog = map[string]zaiCatalogEntry{
	"glm-5":          {Name: "GLM-5", Reasoning: true},
	"glm-4.7":        {Name: "GLM-4.7", Reasoning: true},
	"glm-4.7-flash":  {Name: "GLM-4.7 Flash", Reasoning: true},
	"glm-4.7-flashx": {Name: "GLM-4.7 FlashX", Reasoning: true},
}

// ──────────────────────────────────────────────
// 模型构建函数
// ──────────────────────────────────────────────

// BuildMinimaxModelDefinitionParams MiniMax 模型构建参数。
type BuildMinimaxModelDefinitionParams struct {
	ID            string
	Name          string
	Reasoning     *bool
	Cost          types.ModelCost
	ContextWindow int
	MaxTokens     int
}

// BuildMinimaxModelDefinition 构建 MiniMax 模型定义。
func BuildMinimaxModelDefinition(params BuildMinimaxModelDefinitionParams) types.ModelDefinitionConfig {
	name := params.Name
	if name == "" {
		if entry, ok := minimaxModelCatalog[params.ID]; ok {
			name = entry.Name
		} else {
			name = "MiniMax " + params.ID
		}
	}
	reasoning := false
	if params.Reasoning != nil {
		reasoning = *params.Reasoning
	} else if entry, ok := minimaxModelCatalog[params.ID]; ok {
		reasoning = entry.Reasoning
	}
	return types.ModelDefinitionConfig{
		ID:            params.ID,
		Name:          name,
		Reasoning:     reasoning,
		Input:         []string{"text"},
		Cost:          params.Cost,
		ContextWindow: params.ContextWindow,
		MaxTokens:     params.MaxTokens,
	}
}

// BuildMinimaxApiModelDefinition 构建 MiniMax API 模型定义（使用 API 费用和默认窗口）。
func BuildMinimaxApiModelDefinition(modelID string) types.ModelDefinitionConfig {
	return BuildMinimaxModelDefinition(BuildMinimaxModelDefinitionParams{
		ID:            modelID,
		Cost:          MinimaxApiCost,
		ContextWindow: DefaultMinimaxContextWindow,
		MaxTokens:     DefaultMinimaxMaxTokens,
	})
}

// BuildMoonshotModelDefinition 构建 Moonshot 模型定义。
func BuildMoonshotModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            MoonshotDefaultModelID,
		Name:          "Kimi K2.5",
		Reasoning:     false,
		Input:         []string{"text", "image"},
		Cost:          MoonshotDefaultCost,
		ContextWindow: MoonshotDefaultContextWindow,
		MaxTokens:     MoonshotDefaultMaxTokens,
	}
}

// BuildMistralModelDefinition 构建 Mistral 模型定义。
func BuildMistralModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            MistralDefaultModelID,
		Name:          "Mistral Large",
		Reasoning:     false,
		Input:         []string{"text", "image"},
		Cost:          MistralDefaultCost,
		ContextWindow: MistralDefaultContextWindow,
		MaxTokens:     MistralDefaultMaxTokens,
	}
}

// BuildZaiModelDefinitionParams Z.AI 模型构建参数。
type BuildZaiModelDefinitionParams struct {
	ID            string
	Name          string
	Reasoning     *bool
	Cost          *types.ModelCost
	ContextWindow *int
	MaxTokens     *int
}

// BuildZaiModelDefinition 构建 Z.AI 模型定义。
func BuildZaiModelDefinition(params BuildZaiModelDefinitionParams) types.ModelDefinitionConfig {
	name := params.Name
	if name == "" {
		if entry, ok := zaiModelCatalog[params.ID]; ok {
			name = entry.Name
		} else {
			name = "GLM " + params.ID
		}
	}
	reasoning := true
	if params.Reasoning != nil {
		reasoning = *params.Reasoning
	} else if entry, ok := zaiModelCatalog[params.ID]; ok {
		reasoning = entry.Reasoning
	}
	cost := ZaiDefaultCost
	if params.Cost != nil {
		cost = *params.Cost
	}
	contextWindow := 204800
	if params.ContextWindow != nil {
		contextWindow = *params.ContextWindow
	}
	maxTokens := 131072
	if params.MaxTokens != nil {
		maxTokens = *params.MaxTokens
	}
	return types.ModelDefinitionConfig{
		ID:            params.ID,
		Name:          name,
		Reasoning:     reasoning,
		Input:         []string{"text"},
		Cost:          cost,
		ContextWindow: contextWindow,
		MaxTokens:     maxTokens,
	}
}

// BuildXaiModelDefinition 构建 xAI 模型定义。
func BuildXaiModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            XaiDefaultModelID,
		Name:          "Grok 4",
		Reasoning:     false,
		Input:         []string{"text"},
		Cost:          XaiDefaultCost,
		ContextWindow: XaiDefaultContextWindow,
		MaxTokens:     XaiDefaultMaxTokens,
	}
}

// ──────────────────────────────────────────────
// Kilocode 常量（从 providers/kilocode-shared 导入）
// ──────────────────────────────────────────────

// KilocodeDefaultModelID Kilocode 默认模型 ID。
const KilocodeDefaultModelID = "anthropic/claude-opus-4.6"

// KilocodeDefaultModelName Kilocode 默认模型名称。
const KilocodeDefaultModelName = "Claude Opus 4.6"

// KilocodeDefaultContextWindow Kilocode 默认上下文窗口。
const KilocodeDefaultContextWindow = 200000

// KilocodeDefaultMaxTokens Kilocode 默认最大 token。
const KilocodeDefaultMaxTokens = 16384

// KilocodeDefaultCost Kilocode 默认费用。
var KilocodeDefaultCost = types.ModelCost{}

// KilocodeBaseURL Kilocode 基础 URL。
const KilocodeBaseURL = "https://gateway.kilo.codes/v1"

// BuildKilocodeModelDefinition 构建 Kilocode 模型定义。
func BuildKilocodeModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            KilocodeDefaultModelID,
		Name:          KilocodeDefaultModelName,
		Reasoning:     true,
		Input:         []string{"text", "image"},
		Cost:          KilocodeDefaultCost,
		ContextWindow: KilocodeDefaultContextWindow,
		MaxTokens:     KilocodeDefaultMaxTokens,
	}
}
