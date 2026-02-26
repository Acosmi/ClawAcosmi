package understanding

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/anthropic/open-acosmi/internal/security"
)

// TS 对照: media-understanding/providers/image.ts (~100L)
// 通用图像描述辅助函数。

// ResolveImageBase64 将图像附件解析为 Base64 编码内容。
// 支持本地文件和 URL。
func ResolveImageBase64(attachment MediaAttachment, maxBytes int) (string, string, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	if attachment.Path != "" {
		data, err := os.ReadFile(attachment.Path)
		if err != nil {
			return "", "", fmt.Errorf("读取图像文件失败: %w", err)
		}
		if len(data) > maxBytes {
			return "", "", fmt.Errorf("图像文件大小 %d 超过限制 %d", len(data), maxBytes)
		}
		mime := attachment.MIME
		if mime == "" {
			mime = "image/jpeg" // 默认假设 JPEG
		}
		return base64.StdEncoding.EncodeToString(data), mime, nil
	}

	if attachment.URL != "" {
		// 使用 SafeFetchURL 执行带 SSRF 防护的 HTTP GET
		resp, err := security.SafeFetchURL(attachment.URL, nil)
		if err != nil {
			return "", "", fmt.Errorf("下载图像失败: %w", err)
		}
		defer resp.Body.Close()

		// 限制读取大小
		limited := io.LimitReader(resp.Body, int64(maxBytes)+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return "", "", fmt.Errorf("读取图像数据失败: %w", err)
		}
		if len(data) > maxBytes {
			return "", "", fmt.Errorf("图像 URL 数据大小 %d 超过限制 %d", len(data), maxBytes)
		}

		// 从 Content-Type 或附件元数据获取 MIME
		mime := attachment.MIME
		if mime == "" {
			mime = resp.Header.Get("Content-Type")
		}
		if mime == "" {
			mime = "image/jpeg"
		}
		return base64.StdEncoding.EncodeToString(data), mime, nil
	}

	return "", "", fmt.Errorf("图像附件缺少路径和 URL")
}

// BuildImagePrompt 构建图像描述 prompt。
func BuildImagePrompt(userPrompt string, maxChars int) string {
	prompt := userPrompt
	if prompt == "" {
		prompt = DefaultPrompt
	}
	if maxChars > 0 {
		prompt += fmt.Sprintf(" Please keep your response under %d characters.", maxChars)
	}
	return prompt
}
