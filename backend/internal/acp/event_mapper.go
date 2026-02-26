package acp

import (
	"fmt"
	"strings"
)

// ExtractTextFromPrompt 从 ContentBlock 列表中提取文本。
// 合并 text、resource.text、resource_link 类型内容。
// 对应 TS: acp/event-mapper.ts extractTextFromPrompt()
func ExtractTextFromPrompt(blocks []ContentBlock) string {
	var parts []string
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "resource":
			if block.Resource != nil && block.Resource.Text != "" {
				parts = append(parts, block.Resource.Text)
			}
		case "resource_link":
			title := block.Title
			if title == "" {
				title = "link"
			}
			parts = append(parts, fmt.Sprintf("[Resource link (%s)] %s", title, block.URI))
		}
	}
	return strings.Join(parts, "\n")
}

// ExtractAttachmentsFromPrompt 从 ContentBlock 列表中提取图片附件。
// 对应 TS: acp/event-mapper.ts extractAttachmentsFromPrompt()
func ExtractAttachmentsFromPrompt(blocks []ContentBlock) []GatewayAttachment {
	var attachments []GatewayAttachment
	for _, block := range blocks {
		if block.Type != "image" {
			continue
		}
		if block.Data == "" {
			continue
		}
		attachments = append(attachments, GatewayAttachment{
			Type:     "image",
			MimeType: block.MimeType,
			Content:  block.Data,
		})
	}
	return attachments
}

// FormatToolTitle 格式化工具标题。
// 对应 TS: acp/event-mapper.ts formatToolTitle()
func FormatToolTitle(name string, args map[string]interface{}) string {
	if name == "" {
		return "tool"
	}
	return name
}

// InferToolKind 从工具名推断工具类型。
// 对应 TS: acp/event-mapper.ts inferToolKind()
func InferToolKind(name string) ToolKind {
	lower := strings.ToLower(name)

	if strings.Contains(lower, "read") || strings.Contains(lower, "view") ||
		strings.Contains(lower, "get") || strings.Contains(lower, "list") ||
		strings.Contains(lower, "cat") || strings.Contains(lower, "head") {
		return ToolKindRead
	}
	if strings.Contains(lower, "write") || strings.Contains(lower, "edit") ||
		strings.Contains(lower, "create") || strings.Contains(lower, "update") ||
		strings.Contains(lower, "patch") || strings.Contains(lower, "replace") {
		return ToolKindEdit
	}
	if strings.Contains(lower, "delete") || strings.Contains(lower, "remove") ||
		strings.Contains(lower, "rm") {
		return ToolKindDelete
	}
	if strings.Contains(lower, "move") || strings.Contains(lower, "rename") ||
		strings.Contains(lower, "mv") {
		return ToolKindMove
	}
	if strings.Contains(lower, "search") || strings.Contains(lower, "find") ||
		strings.Contains(lower, "grep") || strings.Contains(lower, "query") {
		return ToolKindSearch
	}
	if strings.Contains(lower, "exec") || strings.Contains(lower, "run") ||
		strings.Contains(lower, "bash") || strings.Contains(lower, "shell") ||
		strings.Contains(lower, "command") {
		return ToolKindExecute
	}
	if strings.Contains(lower, "fetch") || strings.Contains(lower, "download") ||
		strings.Contains(lower, "curl") || strings.Contains(lower, "http") ||
		strings.Contains(lower, "url") {
		return ToolKindFetch
	}
	return ToolKindOther
}
