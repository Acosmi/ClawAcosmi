package media

// image_describe_openai.go — OpenAI 兼容图片理解实现（Phase E 新增）
// 覆盖 qwen-vl / openai / ollama / google 等 OpenAI Chat Completions 兼容 API
// POST /chat/completions with image_url (data:mime;base64,...)

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

// OpenAIImageDescriber OpenAI 兼容图片理解实现
type OpenAIImageDescriber struct {
	provider  string
	apiKey    string
	model     string
	baseURL   string
	prompt    string
	maxTokens int
	client    *http.Client
}

// NewOpenAIImageDescriber 创建 OpenAI 兼容 ImageDescriber
func NewOpenAIImageDescriber(cfg *types.ImageUnderstandingConfig) *OpenAIImageDescriber {
	prompt := cfg.Prompt
	if prompt == "" {
		prompt = "请详细描述这张图片的内容。"
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	return &OpenAIImageDescriber{
		provider:  cfg.Provider,
		apiKey:    cfg.APIKey,
		model:     cfg.Model,
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		prompt:    prompt,
		maxTokens: maxTokens,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (d *OpenAIImageDescriber) Name() string {
	return d.provider
}

func (d *OpenAIImageDescriber) Describe(ctx context.Context, imageData []byte, mimeType string) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("image: empty image data")
	}
	if mimeType == "" {
		mimeType = "image/png"
	}

	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	reqBody := map[string]interface{}{
		"model": d.model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []interface{}{
					map[string]string{"type": "text", "text": d.prompt},
					map[string]interface{}{
						"type": "image_url",
						"image_url": map[string]string{
							"url": dataURL,
						},
					},
				},
			},
		},
		"max_tokens": d.maxTokens,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("image: marshal request: %w", err)
	}

	url := d.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("image: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if d.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("image: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("image: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("image: decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("image: empty response from %s", d.provider)
	}

	text := chatResp.Choices[0].Message.Content
	return text, nil
}

func (d *OpenAIImageDescriber) TestConnection(ctx context.Context) error {
	// Ollama 无 key — 检查端点可达
	if d.apiKey == "" {
		url := strings.TrimSuffix(d.baseURL, "/v1")
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url+"/api/tags", nil)
		if err != nil {
			return fmt.Errorf("image: create probe request: %w", err)
		}
		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Errorf("image: endpoint unreachable: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("image: endpoint returned HTTP %d", resp.StatusCode)
		}
		return nil
	}

	// 有 key — GET /models 验证
	url := d.baseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("image: create test request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("image: test request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("image: invalid API key (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image: test endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}
