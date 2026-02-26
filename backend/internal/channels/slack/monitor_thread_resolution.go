package slack

// Slack 线程解析 — 继承自 src/slack/monitor/thread-resolution.ts (143L)
// Phase 9 实现：conversations.replies API + 历史裁剪。

import (
	"context"
	"encoding/json"
)

// SlackThreadResolution 线程解析结果
type SlackThreadResolution struct {
	ThreadTs        string
	IsReply         bool
	ParentTs        string
	HistoryMessages []SlackMessageSummary
}

// ResolveSlackThreadHistory 解析线程历史消息。
// 调用 conversations.replies API 获取线程消息，并裁剪到 limit 条。
func ResolveSlackThreadHistory(ctx context.Context, client *SlackWebClient, channelID, threadTs string, limit int) (*SlackThreadResolution, error) {
	if threadTs == "" {
		return &SlackThreadResolution{IsReply: false}, nil
	}

	if limit <= 0 {
		limit = 20
	}

	raw, err := client.APICall(ctx, "conversations.replies", map[string]interface{}{
		"channel": channelID,
		"ts":      threadTs,
		"limit":   limit + 1, // +1 因为父消息也会返回
	})
	if err != nil {
		// API 失败时返回空历史
		return &SlackThreadResolution{
			ThreadTs: threadTs,
			IsReply:  true,
		}, nil
	}

	var resp struct {
		Messages []SlackMessageSummary `json:"messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return &SlackThreadResolution{
			ThreadTs: threadTs,
			IsReply:  true,
		}, nil
	}

	// 裁剪：去除父消息，只保留最近 limit 条回复
	messages := resp.Messages
	var parentTs string
	if len(messages) > 0 && messages[0].Ts == threadTs {
		parentTs = messages[0].Ts
		messages = messages[1:] // 去掉父消息
	}
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	return &SlackThreadResolution{
		ThreadTs:        threadTs,
		IsReply:         true,
		ParentTs:        parentTs,
		HistoryMessages: messages,
	}, nil
}
