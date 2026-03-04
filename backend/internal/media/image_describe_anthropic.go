package media

// image_describe_anthropic.go — Anthropic 原生图片理解实现（Phase E 新增）
// Anthropic Messages API 使用 source.type=base64 格式，与 OpenAI 不同

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// AnthropicImageDescriber Anthropic 原生图片理解实现
type AnthropicImageDescriber struct {
	apiKey    string
	model     string
	baseURL   string
	prompt    string
	maxTokens int
	client    *http.Client
}

// NewAnthropicImageDescriber 创建 Anthropic ImageDescriber
func NewAnthropicImageDescriber(cfg *types.ImageUnderstandingConfig) *AnthropicImageDescriber {
	prompt := cfg.Prompt
	if prompt == "" {
		prompt = "请详细描述这张图片的内容。"
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	return &AnthropicImageDescriber{
		apiKey:    cfg.APIKey,
		model:     cfg.Model,
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		prompt:    prompt,
		maxTokens: maxTokens,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (d *AnthropicImageDescriber) Name() string {
	return "anthropic"
}

func (d *AnthropicImageDescriber) Describe(ctx context.Context, imageData []byte, mimeType string) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("anthropic-image: empty image data")
	}
	if mimeType == "" {
		mimeType = "image/png"
	}

	b64 := base64.StdEncoding.EncodeToString(imageData)

	reqBody := map[string]interface{}{
		"model":      d.model,
		"max_tokens": d.maxTokens,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": mimeType,
							"data":       b64,
						},
					},
					map[string]string{
						"type": "text",
						"text": d.prompt,
					},
				},
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("anthropic-image: marshal request: %w", err)
	}

	url := d.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("anthropic-image: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", d.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic-image: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("anthropic-image: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var msgResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return "", fmt.Errorf("anthropic-image: decode response: %w", err)
	}

	for _, block := range msgResp.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("anthropic-image: empty response")
}

func (d *AnthropicImageDescriber) TestConnection(ctx context.Context) error {
	// 发一个 1-token 文本请求验证 API key
	reqBody := map[string]interface{}{
		"model":      d.model,
		"max_tokens": 1,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "hi",
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("anthropic-image: marshal test request: %w", err)
	}

	url := d.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("anthropic-image: create test request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", d.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("anthropic-image: test request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("anthropic-image: invalid API key (HTTP %d)", resp.StatusCode)
	}
	// 200 or 400 (model issue) both indicate key works
	return nil
}
