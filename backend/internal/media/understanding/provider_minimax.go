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

// TS 对照: media-understanding/providers/minimax.ts (~80L)
// MiniMax Provider — 图像描述

// NewMiniMaxProvider 创建 MiniMax Provider。
func NewMiniMaxProvider() *Provider {
	return &Provider{
		ID: "minimax",
		Capabilities: []Capability{
			{Kind: KindImageDescription, Models: []string{"abab6.5s-chat"}},
		},
		DescribeImage: minimaxDescribeImage,
	}
}

// minimaxDescribeImage MiniMax 图像描述。
// POST /v1/text/chatcompletion_v2 (JSON with image content)
func minimaxDescribeImage(req ImageDescriptionRequest) (*ImageDescriptionResult, error) {
	model := req.Model
	if model == "" {
		model = DefaultImageModels["minimax"]
	}

	apiKey := os.Getenv("MINIMAX_API_KEY")
	groupID := os.Getenv("MINIMAX_GROUP_ID")
	if apiKey == "" {
		return nil, fmt.Errorf("minimax: 缺少 MINIMAX_API_KEY 环境变量")
	}

	data, err := readAttachmentData(req.Attachment)
	if err != nil {
		return nil, fmt.Errorf("minimax: 读取图像失败: %w", err)
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
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []interface{}{
					map[string]string{"type": "text", "text": prompt},
					map[string]interface{}{
						"type": "image_url",
						"image_url": map[string]string{
							"url": fmt.Sprintf("data:%s;base64,%s", mime, b64Data),
						},
					},
				},
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("minimax: 序列化请求失败: %w", err)
	}

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	apiURL := "https://api.minimax.chat/v1/text/chatcompletion_v2"
	if groupID != "" {
		apiURL += "?GroupId=" + groupID
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("minimax: 创建请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("minimax: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("minimax: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var mmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mmResp); err != nil {
		return nil, fmt.Errorf("minimax: 解析响应失败: %w", err)
	}

	if len(mmResp.Choices) == 0 {
		return nil, fmt.Errorf("minimax: 无有效响应内容")
	}

	text := mmResp.Choices[0].Message.Content
	if req.MaxChars > 0 && len([]rune(text)) > req.MaxChars {
		text = string([]rune(text)[:req.MaxChars])
	}

	return &ImageDescriptionResult{Text: text}, nil
}
