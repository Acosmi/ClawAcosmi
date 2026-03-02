package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Slack 消息发送 — 继承自 src/slack/send.ts (207L)

const slackTextLimit = 4000

// SlackSendResult 发送结果
type SlackSendResult struct {
	MessageID string
	ChannelID string
}

// SendMessageSlack 发送 Slack 消息（支持文本分块 + 线程回复）。
func SendMessageSlack(ctx context.Context, cfg *types.OpenAcosmiConfig, to, message string, opts SendSlackOpts) (SlackSendResult, error) {
	trimmedMessage := strings.TrimSpace(message)
	if trimmedMessage == "" && opts.MediaURL == "" {
		return SlackSendResult{}, fmt.Errorf("slack send requires text or media")
	}

	account := ResolveSlackAccount(cfg, opts.AccountID)

	// 解析 token
	token, err := resolveSlackSendToken(opts.Token, account)
	if err != nil {
		return SlackSendResult{}, err
	}
	client := opts.Client
	if client == nil {
		client = NewSlackWebClient(token)
	}

	// 解析收件人
	target := ParseSlackTarget(to, "")
	if target == nil {
		return SlackSendResult{}, fmt.Errorf("recipient is required for Slack sends")
	}

	// 解析频道 ID
	channelID, err := resolveSlackSendChannelID(ctx, client, target)
	if err != nil {
		return SlackSendResult{}, err
	}

	// 分块
	chunkLimit := slackTextLimit
	if account.TextChunkLimit != nil && *account.TextChunkLimit < chunkLimit {
		chunkLimit = *account.TextChunkLimit
	}

	chunks := MarkdownToSlackMrkdwnChunks(trimmedMessage, chunkLimit)
	if len(chunks) == 0 && trimmedMessage != "" {
		chunks = []string{trimmedMessage}
	}

	// 发送
	var lastMessageID string
	for _, chunk := range chunks {
		resp, err := client.PostMessage(ctx, PostMessageParams{
			Channel:  channelID,
			Text:     chunk,
			ThreadTs: opts.ThreadTs,
		})
		if err != nil {
			return SlackSendResult{}, err
		}
		if resp != nil && resp.Ts != "" {
			lastMessageID = resp.Ts
		}
	}

	if lastMessageID == "" {
		lastMessageID = "unknown"
	}

	return SlackSendResult{
		MessageID: lastMessageID,
		ChannelID: channelID,
	}, nil
}

// SendSlackOpts 发送消息选项
type SendSlackOpts struct {
	Token     string
	AccountID string
	MediaURL  string
	Client    *SlackWebClient
	ThreadTs  string
}

// resolveSlackSendToken 解析发送消息的 token。
func resolveSlackSendToken(explicit string, account ResolvedSlackAccount) (string, error) {
	if t := ResolveSlackBotToken(explicit); t != "" {
		return t, nil
	}
	if t := ResolveSlackBotToken(account.BotToken); t != "" {
		return t, nil
	}
	return "", fmt.Errorf("slack bot token missing for account %q", account.AccountID)
}

// resolveSlackSendChannelID 解析目标频道 ID（用户类型需通过 conversations.open 获取 DM 频道）。
func resolveSlackSendChannelID(ctx context.Context, client *SlackWebClient, target *SlackTarget) (string, error) {
	if target.Kind == SlackTargetKindChannel {
		return target.ID, nil
	}
	// 用户 → 打开 DM
	raw, err := client.APICall(ctx, "conversations.open", map[string]interface{}{
		"users": target.ID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to open Slack DM channel: %w", err)
	}

	var resp struct {
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("failed to parse conversations.open response: %w", err)
	}
	if resp.Channel.ID == "" {
		return "", fmt.Errorf("failed to open Slack DM channel")
	}
	return resp.Channel.ID, nil
}
