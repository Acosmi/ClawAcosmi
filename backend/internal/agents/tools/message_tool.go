// tools/message_tool.go — 消息发送工具。
// TS 参考：src/agents/tools/message-tool.ts (481L)
package tools

import (
	"context"
	"fmt"
)

// MessageSender 消息发送接口。
type MessageSender interface {
	SendMessage(ctx context.Context, target, content string, opts MessageSendOpts) error
	ResolveTarget(ctx context.Context, target string) (string, error)
}

// MessageSendOpts 消息发送选项。
type MessageSendOpts struct {
	Format   string `json:"format,omitempty"` // text | markdown | html
	ReplyTo  string `json:"replyTo,omitempty"`
	ThreadID string `json:"threadId,omitempty"`
	Silent   bool   `json:"silent,omitempty"`
}

// CreateMessageTool 创建消息发送工具。
// TS 参考: message-tool.ts
func CreateMessageTool(sender MessageSender) *AgentTool {
	return &AgentTool{
		Name:        "message",
		Label:       "Message",
		Description: "Send a message to a specific channel target (e.g., telegram:123456789, discord:guild:channel).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"description": "Channel target in format 'channel:identifier' (e.g. 'telegram:123456789')",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Message content to send",
				},
				"format": map[string]any{
					"type":        "string",
					"enum":        []any{"text", "markdown", "html"},
					"description": "Message format (default: text)",
				},
				"reply_to": map[string]any{
					"type":        "string",
					"description": "Message ID to reply to (optional)",
				},
				"thread_id": map[string]any{
					"type":        "string",
					"description": "Thread ID to send to (optional)",
				},
				"silent": map[string]any{
					"type":        "boolean",
					"description": "Send without notification (default: false)",
				},
			},
			"required": []any{"target", "content"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			target, err := ReadStringParam(args, "target", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			content, err := ReadStringParam(args, "content", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}

			format, _ := ReadStringParam(args, "format", nil)
			replyTo, _ := ReadStringParam(args, "reply_to", nil)
			threadID, _ := ReadStringParam(args, "thread_id", nil)
			silent := false
			if v, ok := args["silent"].(bool); ok {
				silent = v
			}

			if sender == nil {
				return nil, fmt.Errorf("message sender not configured")
			}

			// 解析目标
			resolvedTarget, err := sender.ResolveTarget(ctx, target)
			if err != nil {
				return nil, fmt.Errorf("resolve target: %w", err)
			}

			opts := MessageSendOpts{
				Format:   format,
				ReplyTo:  replyTo,
				ThreadID: threadID,
				Silent:   silent,
			}

			if err := sender.SendMessage(ctx, resolvedTarget, content, opts); err != nil {
				return nil, fmt.Errorf("send message: %w", err)
			}

			return JsonResult(map[string]any{
				"status": "sent",
				"target": target,
			}), nil
		},
	}
}
