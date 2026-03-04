package slack

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Slack 动态目录查询 — 继承自 src/slack/directory-live.ts (184L)

// DirectoryConfigParams 目录配置参数（与 TS ChannelDirectoryConfigParams 对应）
type DirectoryConfigParams struct {
	Cfg       *types.OpenAcosmiConfig
	AccountID string
	Query     string
	Limit     int
}

// ChannelDirectoryEntry 频道目录条目
type ChannelDirectoryEntry struct {
	Kind   string      `json:"kind"`
	ID     string      `json:"id"`
	Name   string      `json:"name,omitempty"`
	Handle string      `json:"handle,omitempty"`
	Rank   int         `json:"rank,omitempty"`
	Raw    interface{} `json:"raw,omitempty"`
}

// resolveReadToken 解析用于只读查询的 token。
func resolveReadToken(params DirectoryConfigParams) string {
	account := ResolveSlackAccount(params.Cfg, params.AccountID)
	userToken := strings.TrimSpace(account.Config.UserToken)
	if userToken != "" {
		return userToken
	}
	return strings.TrimSpace(account.BotToken)
}

// ListSlackDirectoryPeersLive 列出 Slack 用户目录（实时）。
func ListSlackDirectoryPeersLive(ctx context.Context, params DirectoryConfigParams) ([]ChannelDirectoryEntry, error) {
	token := resolveReadToken(params)
	if token == "" {
		return nil, nil
	}
	client := NewSlackWebClient(token)
	query := strings.TrimSpace(strings.ToLower(params.Query))

	users, err := listSlackUsers(ctx, client)
	if err != nil {
		return nil, err
	}

	var rows []ChannelDirectoryEntry
	for _, user := range users {
		if query != "" {
			name := strings.ToLower(bestDisplayName(user))
			handle := strings.ToLower(user.Name)
			email := strings.ToLower(user.Email)
			if !strings.Contains(name, query) && !strings.Contains(handle, query) && !strings.Contains(email, query) {
				continue
			}
		}

		display := bestDisplayName(user)
		rank := 0
		if !user.Deleted {
			rank += 2
		}
		if !user.IsBot && !user.IsAppUser {
			rank++
		}

		var handle string
		if user.Name != "" {
			handle = "@" + user.Name
		}

		rows = append(rows, ChannelDirectoryEntry{
			Kind:   "user",
			ID:     "user:" + user.ID,
			Name:   display,
			Handle: handle,
			Rank:   rank,
		})
	}

	if params.Limit > 0 && len(rows) > params.Limit {
		rows = rows[:params.Limit]
	}
	return rows, nil
}

// ListSlackDirectoryGroupsLive 列出 Slack 频道目录（实时）。
func ListSlackDirectoryGroupsLive(ctx context.Context, params DirectoryConfigParams) ([]ChannelDirectoryEntry, error) {
	token := resolveReadToken(params)
	if token == "" {
		return nil, nil
	}
	client := NewSlackWebClient(token)
	query := strings.TrimSpace(strings.ToLower(params.Query))

	// 列出频道
	var channels []struct {
		ID         string
		Name       string
		IsArchived bool
	}
	var cursor string

	for {
		callParams := map[string]interface{}{
			"types":            "public_channel,private_channel",
			"exclude_archived": false,
			"limit":            1000,
		}
		if cursor != "" {
			callParams["cursor"] = cursor
		}

		raw, err := client.APICall(ctx, "conversations.list", callParams)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Channels []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				IsArchived bool   `json:"is_archived"`
			} `json:"channels"`
			ResponseMetadata struct {
				NextCursor string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}

		for _, ch := range resp.Channels {
			channels = append(channels, struct {
				ID         string
				Name       string
				IsArchived bool
			}{
				ID:         strings.TrimSpace(ch.ID),
				Name:       strings.TrimSpace(ch.Name),
				IsArchived: ch.IsArchived,
			})
		}

		next := strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if next == "" {
			break
		}
		cursor = next
	}

	var rows []ChannelDirectoryEntry
	for _, ch := range channels {
		if ch.ID == "" || ch.Name == "" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(ch.Name), query) {
			continue
		}

		rank := 1
		if ch.IsArchived {
			rank = 0
		}

		rows = append(rows, ChannelDirectoryEntry{
			Kind:   "group",
			ID:     "channel:" + ch.ID,
			Name:   ch.Name,
			Handle: "#" + ch.Name,
			Rank:   rank,
		})
	}

	if params.Limit > 0 && len(rows) > params.Limit {
		rows = rows[:params.Limit]
	}
	return rows, nil
}
