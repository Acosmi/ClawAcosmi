package slack

import (
	"context"
	"encoding/json"
	"fmt"
)

// Slack 动作 API — 继承自 src/slack/actions.ts (263L)

// SlackMessageSummary 消息摘要
type SlackMessageSummary struct {
	Ts         string `json:"ts,omitempty"`
	Text       string `json:"text,omitempty"`
	User       string `json:"user,omitempty"`
	ThreadTs   string `json:"thread_ts,omitempty"`
	ReplyCount int    `json:"reply_count,omitempty"`
	Reactions  []struct {
		Name  string   `json:"name,omitempty"`
		Count int      `json:"count,omitempty"`
		Users []string `json:"users,omitempty"`
	} `json:"reactions,omitempty"`
}

// SlackPin 固定消息
type SlackPin struct {
	Type    string `json:"type,omitempty"`
	Message *struct {
		Ts   string `json:"ts,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"message,omitempty"`
	File *struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"file,omitempty"`
}

// normalizeEmoji 去除 emoji 两端的冒号。
func normalizeEmoji(raw string) (string, error) {
	trimmed := raw
	if trimmed == "" {
		return "", fmt.Errorf("emoji is required for Slack reactions")
	}
	// 去除首尾冒号
	for len(trimmed) > 0 && trimmed[0] == ':' {
		trimmed = trimmed[1:]
	}
	for len(trimmed) > 0 && trimmed[len(trimmed)-1] == ':' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if trimmed == "" {
		return "", fmt.Errorf("emoji is required for Slack reactions")
	}
	return trimmed, nil
}

// ReactSlackMessage 给消息添加 emoji 反应。
func ReactSlackMessage(ctx context.Context, client *SlackWebClient, channelID, messageID, emoji string) error {
	name, err := normalizeEmoji(emoji)
	if err != nil {
		return err
	}
	_, err = client.APICall(ctx, "reactions.add", map[string]interface{}{
		"channel":   channelID,
		"timestamp": messageID,
		"name":      name,
	})
	return err
}

// RemoveSlackReaction 移除消息上的 emoji 反应。
func RemoveSlackReaction(ctx context.Context, client *SlackWebClient, channelID, messageID, emoji string) error {
	name, err := normalizeEmoji(emoji)
	if err != nil {
		return err
	}
	_, err = client.APICall(ctx, "reactions.remove", map[string]interface{}{
		"channel":   channelID,
		"timestamp": messageID,
		"name":      name,
	})
	return err
}

// ListSlackReactions 列出消息的所有反应。
func ListSlackReactions(ctx context.Context, client *SlackWebClient, channelID, messageID string) ([]struct {
	Name  string   `json:"name,omitempty"`
	Count int      `json:"count,omitempty"`
	Users []string `json:"users,omitempty"`
}, error) {
	raw, err := client.APICall(ctx, "reactions.get", map[string]interface{}{
		"channel":   channelID,
		"timestamp": messageID,
		"full":      true,
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Message *SlackMessageSummary `json:"message"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Message == nil {
		return nil, nil
	}
	return resp.Message.Reactions, nil
}

// RemoveOwnSlackReactions 移除 bot 自身在消息上的所有反应。
func RemoveOwnSlackReactions(ctx context.Context, client *SlackWebClient, channelID, messageID string) ([]string, error) {
	// 获取 bot 用户 ID
	auth, err := client.AuthTest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve bot user id: %w", err)
	}
	botUserID := auth.UserID

	reactions, err := ListSlackReactions(ctx, client, channelID, messageID)
	if err != nil {
		return nil, err
	}

	var toRemove []string
	for _, r := range reactions {
		if r.Name == "" {
			continue
		}
		for _, u := range r.Users {
			if u == botUserID {
				toRemove = append(toRemove, r.Name)
				break
			}
		}
	}

	if len(toRemove) == 0 {
		return nil, nil
	}

	for _, name := range toRemove {
		_, _ = client.APICall(ctx, "reactions.remove", map[string]interface{}{
			"channel":   channelID,
			"timestamp": messageID,
			"name":      name,
		})
	}

	return toRemove, nil
}

// SendSlackMessage 发送 Slack 消息（简易版，转发到 send.go 的 SendMessageSlack）。
func SendSlackMessage(ctx context.Context, client *SlackWebClient, channelID, text string, threadTs string) (*PostMessageResponse, error) {
	return client.PostMessage(ctx, PostMessageParams{
		Channel:  channelID,
		Text:     text,
		ThreadTs: threadTs,
	})
}

// EditSlackMessage 编辑已发送的消息。
func EditSlackMessage(ctx context.Context, client *SlackWebClient, channelID, messageID, content string) error {
	_, err := client.APICall(ctx, "chat.update", map[string]interface{}{
		"channel": channelID,
		"ts":      messageID,
		"text":    content,
	})
	return err
}

// DeleteSlackMessage 删除消息。
func DeleteSlackMessage(ctx context.Context, client *SlackWebClient, channelID, messageID string) error {
	_, err := client.APICall(ctx, "chat.delete", map[string]interface{}{
		"channel": channelID,
		"ts":      messageID,
	})
	return err
}

// ReadSlackMessagesOpts 读取消息选项
type ReadSlackMessagesOpts struct {
	Limit    int
	Before   string
	After    string
	ThreadID string
}

// ReadSlackMessagesResult 读取消息结果
type ReadSlackMessagesResult struct {
	Messages []SlackMessageSummary `json:"messages"`
	HasMore  bool                  `json:"has_more"`
}

// ReadSlackMessages 读取频道/线程消息。
func ReadSlackMessages(ctx context.Context, client *SlackWebClient, channelID string, opts ReadSlackMessagesOpts) (ReadSlackMessagesResult, error) {
	params := map[string]interface{}{
		"channel": channelID,
	}
	if opts.Limit > 0 {
		params["limit"] = opts.Limit
	}
	if opts.Before != "" {
		params["latest"] = opts.Before
	}
	if opts.After != "" {
		params["oldest"] = opts.After
	}

	var method string
	if opts.ThreadID != "" {
		method = "conversations.replies"
		params["ts"] = opts.ThreadID
	} else {
		method = "conversations.history"
	}

	raw, err := client.APICall(ctx, method, params)
	if err != nil {
		return ReadSlackMessagesResult{}, err
	}

	var resp struct {
		Messages []SlackMessageSummary `json:"messages"`
		HasMore  bool                  `json:"has_more"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ReadSlackMessagesResult{}, err
	}

	messages := resp.Messages
	// conversations.replies 包含父消息，过滤掉
	if opts.ThreadID != "" {
		var filtered []SlackMessageSummary
		for _, m := range messages {
			if m.Ts != opts.ThreadID {
				filtered = append(filtered, m)
			}
		}
		messages = filtered
	}

	return ReadSlackMessagesResult{Messages: messages, HasMore: resp.HasMore}, nil
}

// GetSlackMemberInfo 获取用户信息。
func GetSlackMemberInfo(ctx context.Context, client *SlackWebClient, userID string) (json.RawMessage, error) {
	return client.APICall(ctx, "users.info", map[string]interface{}{
		"user": userID,
	})
}

// ListSlackEmojis 列出工作区自定义 emoji。
func ListSlackEmojis(ctx context.Context, client *SlackWebClient) (json.RawMessage, error) {
	return client.APICall(ctx, "emoji.list", nil)
}

// PinSlackMessage 固定消息。
func PinSlackMessage(ctx context.Context, client *SlackWebClient, channelID, messageID string) error {
	_, err := client.APICall(ctx, "pins.add", map[string]interface{}{
		"channel":   channelID,
		"timestamp": messageID,
	})
	return err
}

// UnpinSlackMessage 取消固定消息。
func UnpinSlackMessage(ctx context.Context, client *SlackWebClient, channelID, messageID string) error {
	_, err := client.APICall(ctx, "pins.remove", map[string]interface{}{
		"channel":   channelID,
		"timestamp": messageID,
	})
	return err
}

// ListSlackPins 列出频道固定消息。
func ListSlackPins(ctx context.Context, client *SlackWebClient, channelID string) ([]SlackPin, error) {
	raw, err := client.APICall(ctx, "pins.list", map[string]interface{}{
		"channel": channelID,
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Items []SlackPin `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}
