package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/retry"
)

// maxParallelReactionRemovals 限制并行移除反应的最大并发数，以避免触发 429。
// TS 使用 Promise.allSettled 一次性并行所有移除；这里限制并发数
// 以降低 Discord per-route rate limit (1/0.25s) 触发的风险。
const maxParallelReactionRemovals = 3

// Discord 反应操作 — 继承自 src/discord/send.reactions.ts (123L)

// reactionRetryConfig 反应操作的重试配置：最多 3 次尝试，500ms 起步指数退避，仅重试 429。
var reactionRetryConfig = retry.Config{
	MaxAttempts:  3,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	JitterFactor: 0.1,
	ShouldRetry: func(err error, _ int) bool {
		if apiErr, ok := err.(*DiscordAPIError); ok {
			return apiErr.StatusCode == http.StatusTooManyRequests
		}
		return false
	},
	RetryAfterHint: func(err error) time.Duration {
		if apiErr, ok := err.(*DiscordAPIError); ok && apiErr.RetryAfter != nil {
			return time.Duration(*apiErr.RetryAfter * float64(time.Second))
		}
		return 0
	},
}

// ReactMessageDiscord 添加反应（带 429 限速重试）
func ReactMessageDiscord(ctx context.Context, channelID, messageID, emoji, token string) error {
	encoded, err := NormalizeReactionEmoji(emoji)
	if err != nil {
		return fmt.Errorf("normalize emoji: %w", err)
	}
	path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/@me", channelID, messageID, encoded)
	return retry.Do(ctx, reactionRetryConfig, func(_ int) error {
		_, err := discordPUT(ctx, path, token, nil)
		return err
	})
}

// RemoveReactionDiscord 移除自己的反应（带 429 限速重试）
func RemoveReactionDiscord(ctx context.Context, channelID, messageID, emoji, token string) error {
	encoded, err := NormalizeReactionEmoji(emoji)
	if err != nil {
		return fmt.Errorf("normalize emoji: %w", err)
	}
	path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/@me", channelID, messageID, encoded)
	return retry.Do(ctx, reactionRetryConfig, func(_ int) error {
		return discordDELETE(ctx, path, token)
	})
}

// RemoveOwnReactionsDiscord 移除自己在消息上的所有反应
func RemoveOwnReactionsDiscord(ctx context.Context, channelID, messageID, token string) ([]string, error) {
	// 获取消息
	msgData, err := discordGET(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), token)
	if err != nil {
		return nil, err
	}

	var msg struct {
		Reactions []struct {
			Emoji struct {
				ID   *string `json:"id"`
				Name *string `json:"name"`
			} `json:"emoji"`
		} `json:"reactions"`
	}
	if err := json.Unmarshal(msgData, &msg); err != nil {
		return nil, fmt.Errorf("parse message reactions: %w", err)
	}

	seen := make(map[string]bool)
	var identifiers []string
	for _, r := range msg.Reactions {
		emojiID := ""
		if r.Emoji.ID != nil {
			emojiID = *r.Emoji.ID
		}
		emojiName := ""
		if r.Emoji.Name != nil {
			emojiName = *r.Emoji.Name
		}
		id := BuildReactionIdentifier(emojiID, emojiName)
		if id != "" && !seen[id] {
			seen[id] = true
			identifiers = append(identifiers, id)
		}
	}

	if len(identifiers) == 0 {
		return nil, nil
	}

	// DY-006 fix: 并行移除反应，对齐 TS Promise.allSettled 行为。
	// 使用带限流的 semaphore 防止 429 rate-limit。
	type removeResult struct {
		idx int
		ok  bool
	}
	results := make([]removeResult, len(identifiers))

	sem := make(chan struct{}, maxParallelReactionRemovals)
	var wg sync.WaitGroup
	for i, id := range identifiers {
		wg.Add(1)
		go func(idx int, ident string) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			encoded := url.PathEscape(ident)
			path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/@me", channelID, messageID, encoded)
			if err := discordDELETE(ctx, path, token); err != nil {
				results[idx] = removeResult{idx: idx, ok: false}
				return
			}
			results[idx] = removeResult{idx: idx, ok: true}
		}(i, id)
	}
	wg.Wait()

	// 按原始顺序收集成功移除的标识符（对齐 TS 提前 push 行为，
	// TS 的 Promise.allSettled 不区分成功/失败都 push 到 removed，
	// 但 Go 版保守一些，仅返回实际删除成功的）。
	var removed []string
	for i, r := range results {
		if r.ok {
			removed = append(removed, identifiers[i])
		}
	}
	return removed, nil
}

// FetchReactionsDiscord 获取消息上的反应详情
func FetchReactionsDiscord(ctx context.Context, channelID, messageID, token string, limit int) ([]DiscordReactionSummary, error) {
	msgData, err := discordGET(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), token)
	if err != nil {
		return nil, err
	}

	var msg struct {
		Reactions []struct {
			Count int `json:"count"`
			Emoji struct {
				ID   *string `json:"id"`
				Name *string `json:"name"`
			} `json:"emoji"`
		} `json:"reactions"`
	}
	if err := json.Unmarshal(msgData, &msg); err != nil {
		return nil, fmt.Errorf("parse message reactions: %w", err)
	}

	if len(msg.Reactions) == 0 {
		return nil, nil
	}

	fetchLimit := 100
	if limit > 0 {
		fetchLimit = int(math.Min(math.Max(float64(limit), 1), 100))
	}

	var summaries []DiscordReactionSummary
	for _, r := range msg.Reactions {
		emojiID := ""
		if r.Emoji.ID != nil {
			emojiID = *r.Emoji.ID
		}
		emojiName := ""
		if r.Emoji.Name != nil {
			emojiName = *r.Emoji.Name
		}
		identifier := BuildReactionIdentifier(emojiID, emojiName)
		if identifier == "" {
			continue
		}

		encoded := url.PathEscape(identifier)
		path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s?limit=%d", channelID, messageID, encoded, fetchLimit)
		userData, err := discordGET(ctx, path, token)
		if err != nil {
			continue
		}

		var users []struct {
			ID            string  `json:"id"`
			Username      *string `json:"username"`
			Discriminator *string `json:"discriminator"`
		}
		if err := json.Unmarshal(userData, &users); err != nil {
			continue
		}

		var reactionUsers []DiscordReactionUser
		for _, u := range users {
			ru := DiscordReactionUser{ID: u.ID}
			if u.Username != nil {
				ru.Username = *u.Username
			}
			if u.Username != nil && u.Discriminator != nil {
				ru.Tag = *u.Username + "#" + *u.Discriminator
			} else if u.Username != nil {
				ru.Tag = *u.Username
			}
			reactionUsers = append(reactionUsers, ru)
		}

		summaries = append(summaries, DiscordReactionSummary{
			Emoji: DiscordEmojiRef{ID: emojiID, Name: emojiName, Raw: identifier},
			Count: r.Count,
			Users: reactionUsers,
		})
	}
	return summaries, nil
}
