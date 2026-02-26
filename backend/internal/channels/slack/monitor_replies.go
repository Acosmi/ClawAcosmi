package slack

// Slack 回复发送 — 继承自 src/slack/monitor/replies.ts (166L)
// Phase 9 实现：状态反应 + 分块发送。

import (
	"context"
	"log"
)

// SendSlackReply 发送 Slack 回复消息。
// 支持状态反应生命周期（⏳→✅/❌）和分块发送。
func SendSlackReply(ctx context.Context, client *SlackWebClient, channelID, text, threadTs string) (*PostMessageResponse, error) {
	if text == "" {
		return nil, nil
	}

	// 分块
	chunks := MarkdownToSlackMrkdwnChunks(text, 4000)
	if len(chunks) == 0 {
		chunks = []string{text}
	}

	var lastResp *PostMessageResponse
	for _, chunk := range chunks {
		resp, err := client.PostMessage(ctx, PostMessageParams{
			Channel:  channelID,
			Text:     chunk,
			ThreadTs: threadTs,
		})
		if err != nil {
			return lastResp, err
		}
		lastResp = resp
	}

	return lastResp, nil
}

// SendSlackReplyWithReactions 发送带反应状态的回复。
// 发送前添加 ⏳ 反应，成功后替换为 ✅，失败时替换为 ❌。
func SendSlackReplyWithReactions(
	ctx context.Context,
	client *SlackWebClient,
	channelID, text, threadTs, triggerTs string,
	removeAckAfterReply bool,
) (*PostMessageResponse, error) {
	// 添加 ⏳ 反应
	if triggerTs != "" {
		addReaction(ctx, client, channelID, "hourglass_flowing_sand", triggerTs)
	}

	resp, err := SendSlackReply(ctx, client, channelID, text, threadTs)

	if triggerTs != "" {
		// 移除 ⏳
		removeReaction(ctx, client, channelID, "hourglass_flowing_sand", triggerTs)
		if err != nil {
			addReaction(ctx, client, channelID, "x", triggerTs)
		} else if !removeAckAfterReply {
			addReaction(ctx, client, channelID, "white_check_mark", triggerTs)
		}
	}

	return resp, err
}

// addReaction 添加反应（忽略错误）。
func addReaction(ctx context.Context, client *SlackWebClient, channelID, reaction, ts string) {
	_, err := client.APICall(ctx, "reactions.add", map[string]interface{}{
		"channel":   channelID,
		"name":      reaction,
		"timestamp": ts,
	})
	if err != nil {
		log.Printf("[slack] add reaction %s failed: %v", reaction, err)
	}
}

// removeReaction 移除反应（忽略错误）。
func removeReaction(ctx context.Context, client *SlackWebClient, channelID, reaction, ts string) {
	_, err := client.APICall(ctx, "reactions.remove", map[string]interface{}{
		"channel":   channelID,
		"name":      reaction,
		"timestamp": ts,
	})
	if err != nil {
		// 忽略 — 可能反应已被移除
	}
}
