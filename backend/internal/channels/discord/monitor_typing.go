package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Discord typing 指示器 — 继承自 src/discord/monitor/typing.ts (11L)

// SendTypingIndicator 发送 typing 指示器。
// W-035 fix: 对齐 TS sendTyping — 发送前检查 channel 是否存在。
// TS ref: const channel = await params.client.fetchChannel(params.channelId);
//
//	if (!channel) { return; }
func SendTypingIndicator(ctx context.Context, session *discordgo.Session, channelID string) error {
	if strings.TrimSpace(channelID) == "" {
		return fmt.Errorf("typing: channelID is empty")
	}

	// 对齐 TS: 先 fetchChannel 确认 channel 存在
	ch, err := session.Channel(channelID)
	if err != nil || ch == nil {
		// channel 不存在或无法获取，静默返回（与 TS 行为一致: if (!channel) return）
		return nil
	}

	return session.ChannelTyping(channelID)
}
