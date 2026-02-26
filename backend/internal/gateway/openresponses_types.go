package gateway

// openresponses_types.go — OpenResponses /v1/responses API 类型定义
// 对应 TS: src/gateway/open-responses.schema.ts (355L)
// 文档: https://www.open-responses.com/

import "encoding/json"

// ---------- 请求体 ----------

// CreateResponseBody /v1/responses 请求体。
type CreateResponseBody struct {
	Model           string           `json:"model"`
	Input           json.RawMessage  `json:"input"` // string | []ItemParam
	Instructions    string           `json:"instructions,omitempty"`
	Tools           []ToolDefinition `json:"tools,omitempty"`
	ToolChoice      json.RawMessage  `json:"tool_choice,omitempty"` // "auto"|"none"|"required"|{type,function}
	Stream          *bool            `json:"stream,omitempty"`
	MaxOutputTokens *int             `json:"max_output_tokens,omitempty"`
	User            string           `json:"user,omitempty"`
	Temperature     *float64         `json:"temperature,omitempty"`
	TopP            *float64         `json:"top_p,omitempty"`
}

// ToolDefinition OpenResponses 工具定义。
type ToolDefinition struct {
	Type     string                 `json:"type"` // "function"
	Function ToolDefinitionFunction `json:"function"`
}

// ToolDefinitionFunction 工具函数详情。
type ToolDefinitionFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ---------- Item 类型 ----------

// ItemParam 输入项（消息/函数调用/函数输出/推理）。
type ItemParam struct {
	Type string `json:"type"` // "message"|"function_call"|"function_call_output"|"reasoning"|"item_reference"

	// message fields
	Role    string          `json:"role,omitempty"`    // "system"|"developer"|"user"|"assistant"
	Content json.RawMessage `json:"content,omitempty"` // string | []ContentPart

	// function_call fields
	ID        string `json:"id,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// function_call_output fields
	Output string `json:"output,omitempty"`
}

// ContentPart 内容块。
type ContentPart struct {
	Type string `json:"type"` // "input_text"|"output_text"|"input_image"|"input_file"
	Text string `json:"text,omitempty"`
	// input_image / input_file 共用
	Source   *ContentSource `json:"source,omitempty"`
	ImageURL string         `json:"image_url,omitempty"` // input_image shorthand
	Filename string         `json:"filename,omitempty"`  // input_file
}

// ContentSource 图像/文件来源（base64 或 URL）。
type ContentSource struct {
	Type      string `json:"type"`           // "base64" | "url"
	URL       string `json:"url,omitempty"`  // type="url" 时
	Data      string `json:"data,omitempty"` // type="base64" 时
	MediaType string `json:"media_type,omitempty"`
	Filename  string `json:"filename,omitempty"` // input_file 用
}

// ---------- 响应类型 ----------

// ResponseResource /v1/responses 响应资源。
type ResponseResource struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"` // "response"
	CreatedAt int64        `json:"created_at"`
	Status    string       `json:"status"` // "in_progress"|"completed"|"failed"|"cancelled"|"incomplete"
	Model     string       `json:"model"`
	Output    []OutputItem `json:"output"`
	Usage     ORUsage      `json:"usage"`
	Error     *ORError     `json:"error,omitempty"`
}

// OutputItem 输出项。
type OutputItem struct {
	Type      string           `json:"type"` // "message"|"function_call"|"reasoning"
	ID        string           `json:"id"`
	Role      string           `json:"role,omitempty"`      // "assistant" (message)
	Content   []OutputTextPart `json:"content,omitempty"`   // message content
	Status    string           `json:"status,omitempty"`    // "in_progress"|"completed"
	CallID    string           `json:"call_id,omitempty"`   // function_call
	Name      string           `json:"name,omitempty"`      // function_call
	Arguments string           `json:"arguments,omitempty"` // function_call
}

// OutputTextPart 输出文本块。
type OutputTextPart struct {
	Type string `json:"type"` // "output_text"
	Text string `json:"text"`
}

// ORUsage token 使用统计。
type ORUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ORError 错误信息。
type ORError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---------- SSE 事件类型 ----------
// 所有事件通过 writeOpenResponsesSSE 写入,
// 使用 map[string]interface{} 构建以保持灵活性

// createResponseResource 创建 ResponseResource。
func createResponseResource(id, model, status string, output []OutputItem, usage ORUsage, respErr *ORError) ResponseResource {
	return ResponseResource{
		ID:        id,
		Object:    "response",
		CreatedAt: nowUnix(),
		Status:    status,
		Model:     model,
		Output:    output,
		Usage:     usage,
		Error:     respErr,
	}
}

// createAssistantOutputItem 创建 assistant 输出项。
func createAssistantOutputItem(id, text, status string) OutputItem {
	return OutputItem{
		Type: "message",
		ID:   id,
		Role: "assistant",
		Content: []OutputTextPart{
			{Type: "output_text", Text: text},
		},
		Status: status,
	}
}

// emptyUsage 空 usage。
func emptyUsage() ORUsage {
	return ORUsage{}
}
