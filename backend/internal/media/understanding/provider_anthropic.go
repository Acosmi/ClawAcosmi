package understanding

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TS 对照: media-understanding/providers/anthropic.ts (~100L)
// Anthropic Provider — 图像描述 (Claude Vision)

// NewAnthropicProvider 创建 Anthropic Provider。
func NewAnthropicProvider() *Provider {
	return &Provider{
		ID: "anthropic",
		Capabilities: []Capability{
			{Kind: KindImageDescription, Models: []string{"claude-3-haiku-20240307", "claude-3-sonnet-20240229", "claude-3-opus-20240229"}},
		},
		DescribeImage: anthropicDescribeImage,
	}
}

// anthropicDescribeImage Anthropic 图像描述（Claude Vision API）。
// POST /v1/messages with source type=base64
func anthropicDescribeImage(req ImageDescriptionRequest) (*ImageDescriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultImageModels["anthropic"]
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic: 缺少 ANTHROPIC_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("anthropic: 读取图像失败: %w", err)
	}

	mime := req.Attachment.MIME
	if mime == "" {
		mime = "image/jpeg"
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	b64Data := base64.StdEncoding.EncodeToString(data)

	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": mime,
							"data":       b64Data,
						},
					},
					map[string]string{"type": "text", "text": prompt},
				},
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("anthropic: 序列化请求失败: %w", err)
	}

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("anthropic: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("anthropic: 解析响应失败: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("anthropic: 无有效响应内容")
	}

	text := anthropicResp.Content[0].Text
	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &ImageDescriptionResult{Text: text}, nil
}
