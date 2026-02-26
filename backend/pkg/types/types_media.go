package types

// types_media.go — 多媒体处理配置类型（Phase C/D 新增）
// STT（语音转文本）和 DocConv（文档转换）的配置定义

// STTConfig 语音转文本（Speech-to-Text）配置。
// 支持多种 provider，可在 UI 向导中独立配置和切换。
type STTConfig struct {
	// Provider 提供商标识: "openai" | "groq" | "azure" | "local-whisper" | ""(禁用)
	Provider string `json:"provider" label:"STT Provider"`

	// APIKey API 密钥（openai/groq/azure）
	APIKey string `json:"apiKey,omitempty" label:"API Key" sensitive:"true"`

	// Model 模型标识: "whisper-1" | "gpt-4o-transcribe" | "whisper-large-v3" 等
	Model string `json:"model,omitempty" label:"Model"`

	// BaseURL 自定义 API 端点（用于 Groq/Azure/私有部署）
	BaseURL string `json:"baseUrl,omitempty" label:"Base URL"`

	// BinaryPath whisper.cpp 二进制路径（local-whisper 模式）
	BinaryPath string `json:"binaryPath,omitempty" label:"Whisper Binary Path"`

	// ModelPath whisper ggml 模型文件路径（local-whisper 模式）
	ModelPath string `json:"modelPath,omitempty" label:"Whisper Model Path"`

	// Language 优先语言代码（如 "zh", "en"），空则自动检测
	Language string `json:"language,omitempty" label:"Language"`
}

// DocConvConfig 文档转换配置。
// 支持 MCP 工具协议或内置 pandoc 等方式，可在 UI 向导中独立配置和切换。
type DocConvConfig struct {
	// Provider 模式: "mcp" | "builtin" | ""(禁用)
	Provider string `json:"provider" label:"DocConv Provider"`

	// MCPServer MCP 文档转换服务器配置（provider="mcp" 时使用）
	MCPServerName string `json:"mcpServerName,omitempty" label:"MCP Server Name"`
	MCPTransport  string `json:"mcpTransport,omitempty" label:"MCP Transport"` // "stdio" | "sse"
	MCPCommand    string `json:"mcpCommand,omitempty" label:"MCP Command"`     // stdio 模式的启动命令
	MCPURL        string `json:"mcpUrl,omitempty" label:"MCP URL"`             // sse 模式的端点 URL

	// PandocPath pandoc CLI 路径（provider="builtin" 时可选）
	PandocPath string `json:"pandocPath,omitempty" label:"Pandoc Path"`

	// UseSandbox 是否通过沙箱处理文件（默认 true）
	UseSandbox *bool `json:"useSandbox,omitempty" label:"Use Sandbox"`
}

// IsSTTEnabled 判断 STT 是否已启用
func (c *STTConfig) IsSTTEnabled() bool {
	return c != nil && c.Provider != ""
}

// IsDocConvEnabled 判断文档转换是否已启用
func (c *DocConvConfig) IsDocConvEnabled() bool {
	return c != nil && c.Provider != ""
}
