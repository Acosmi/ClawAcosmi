// tools/images.go — 工具结果图片处理。
// TS 参考：src/agents/tool-images.ts (223L)
package tools

import (
	"encoding/base64"
	"strings"
)

// ---------- 常量 ----------

const (
	// DefaultMaxImageDimension API 支持的最大图片尺寸。
	DefaultMaxImageDimension = 4096
	// DefaultMaxImageBytes API 支持的最大图片大小（字节）。
	DefaultMaxImageBytes = 20 * 1024 * 1024 // 20MB
)

// ---------- 图片结果清洗 ----------

// SanitizeToolResultImages 清洗工具结果中的图片内容。
// 检查尺寸和大小限制，超限时尝试降级或移除。
// TS 参考: tool-images.ts sanitizeToolResultImages
func SanitizeToolResultImages(result *AgentToolResult, maxDim, maxBytes int) *AgentToolResult {
	if result == nil || len(result.Content) == 0 {
		return result
	}
	if maxDim <= 0 {
		maxDim = DefaultMaxImageDimension
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxImageBytes
	}

	sanitized := make([]ContentBlock, 0, len(result.Content))
	for _, block := range result.Content {
		if block.Type == "image" && block.Data != "" {
			cleaned := sanitizeImageBlock(block, maxBytes)
			if cleaned != nil {
				sanitized = append(sanitized, *cleaned)
			}
		} else {
			sanitized = append(sanitized, block)
		}
	}

	return &AgentToolResult{
		Content: sanitized,
		Details: result.Details,
	}
}

// SanitizeContentBlocksImages 清洗内容块列表中的图片。
func SanitizeContentBlocksImages(blocks []ContentBlock, maxBytes int) []ContentBlock {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxImageBytes
	}
	result := make([]ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "image" && block.Data != "" {
			cleaned := sanitizeImageBlock(block, maxBytes)
			if cleaned != nil {
				result = append(result, *cleaned)
			}
		} else {
			result = append(result, block)
		}
	}
	return result
}

// sanitizeImageBlock 清洗单个图片块。
func sanitizeImageBlock(block ContentBlock, maxBytes int) *ContentBlock {
	// 估算 base64 解码后的大小
	rawSize := base64DecodedSize(block.Data)
	if rawSize > maxBytes {
		// 超限 → 用文本提示替代
		return &ContentBlock{
			Type: "text",
			Text: "[Image removed: exceeds size limit]",
		}
	}

	// 确保 MIME 类型
	mime := block.MimeType
	if mime == "" {
		mime = InferMimeTypeFromBase64(block.Data)
	}

	return &ContentBlock{
		Type:     "image",
		Data:     block.Data,
		MimeType: mime,
	}
}

// InferMimeTypeFromBase64 从 base64 数据推断 MIME 类型。
// TS 参考: tool-images.ts inferMimeType
func InferMimeTypeFromBase64(data string) string {
	// 解码前几个字节
	sample := data
	if len(sample) > 32 {
		sample = sample[:32]
	}
	raw, err := base64.StdEncoding.DecodeString(padBase64(sample))
	if err != nil || len(raw) < 4 {
		return "image/png"
	}
	return detectMimeFromBytes(raw)
}

// padBase64 补齐 base64 padding。
func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}

// base64DecodedSize 估算 base64 解码后的大小。
func base64DecodedSize(data string) int {
	// 移除 padding 之外的等号
	n := len(data)
	padding := strings.Count(data[max(0, n-3):], "=")
	return (n * 3 / 4) - padding
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
