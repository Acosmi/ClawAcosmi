package discord

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Discord 目录实时查找 — 继承自 src/discord/directory-live.ts (107L)

// ChannelDirectoryEntry 频道目录条目
type ChannelDirectoryEntry struct {
	Kind   string      `json:"kind"`
	ID     string      `json:"id"`
	Name   string      `json:"name,omitempty"`
	Handle string      `json:"handle,omitempty"`
	Rank   int         `json:"rank,omitempty"`
	Raw    interface{} `json:"raw,omitempty"`
}

// DirectoryConfigParams 目录查找参数
type DirectoryConfigParams struct {
	Cfg       *types.OpenAcosmiConfig
	AccountID string
	Query     string
	Limit     int
}

// discordGuild Discord 服务器（API 响应）
type discordGuild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// discordUser Discord 用户（API 响应）
type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name,omitempty"`
	Bot        bool   `json:"bot,omitempty"`
}

// discordMember Discord 成员（API 响应）
type discordMember struct {
	User discordUser `json:"user"`
	Nick string      `json:"nick,omitempty"`
}

// discordChannel Discord 频道（API 响应）
type discordChannel struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

func normalizeQuery(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func buildUserRank(user discordUser) int {
	if user.Bot {
		return 0
	}
	return 1
}

// ListDiscordDirectoryGroupsLive 实时查找 Discord 频道目录
func ListDiscordDirectoryGroupsLive(ctx context.Context, params DirectoryConfigParams) ([]ChannelDirectoryEntry, error) {
	account := ResolveDiscordAccount(params.Cfg, params.AccountID)
	token := NormalizeDiscordToken(account.Token)
	if token == "" {
		return nil, nil
	}

	query := normalizeQuery(params.Query)

	guilds, err := FetchDiscord[[]discordGuild](ctx, "/users/@me/guilds", token, nil)
	if err != nil {
		return nil, fmt.Errorf("discord directory: list guilds: %w", err)
	}

	// W-065: 收集错误
	var errs []error
	var rows []ChannelDirectoryEntry
	for _, guild := range guilds {
		channels, err := FetchDiscord[[]discordChannel](ctx, fmt.Sprintf("/guilds/%s/channels", guild.ID), token, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("list channels guild %s: %w", guild.ID, err))
			continue
		}
		for _, ch := range channels {
			name := strings.TrimSpace(ch.Name)
			if name == "" {
				continue
			}
			if query != "" && !strings.Contains(NormalizeDiscordSlug(name), NormalizeDiscordSlug(query)) {
				continue
			}
			rows = append(rows, ChannelDirectoryEntry{
				Kind:   "group",
				ID:     fmt.Sprintf("channel:%s", ch.ID),
				Name:   name,
				Handle: "#" + name,
				Raw:    ch,
			})
			if params.Limit > 0 && len(rows) >= params.Limit {
				return rows, errors.Join(errs...)
			}
		}
	}
	return rows, errors.Join(errs...)
}

// ListDiscordDirectoryPeersLive 实时查找 Discord 用户目录
func ListDiscordDirectoryPeersLive(ctx context.Context, params DirectoryConfigParams) ([]ChannelDirectoryEntry, error) {
	account := ResolveDiscordAccount(params.Cfg, params.AccountID)
	token := NormalizeDiscordToken(account.Token)
	if token == "" {
		return nil, nil
	}

	query := normalizeQuery(params.Query)
	if query == "" {
		return nil, nil
	}

	guilds, err := FetchDiscord[[]discordGuild](ctx, "/users/@me/guilds", token, nil)
	if err != nil {
		return nil, fmt.Errorf("discord directory: list guilds: %w", err)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}

	// W-065: 收集错误
	var errs []error
	var rows []ChannelDirectoryEntry
	for _, guild := range guilds {
		searchLimit := limit
		if searchLimit > 100 {
			searchLimit = 100
		}
		params := url.Values{
			"query": {query},
			"limit": {fmt.Sprintf("%d", searchLimit)},
		}
		path := fmt.Sprintf("/guilds/%s/members/search?%s", guild.ID, params.Encode())
		members, err := FetchDiscord[[]discordMember](ctx, path, token, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("member search guild %s: %w", guild.ID, err))
			continue
		}
		for _, member := range members {
			user := member.User
			if user.ID == "" {
				continue
			}
			name := strings.TrimSpace(member.Nick)
			if name == "" {
				name = strings.TrimSpace(user.GlobalName)
			}
			if name == "" {
				name = strings.TrimSpace(user.Username)
			}
			var handle string
			if user.Username != "" {
				handle = "@" + user.Username
			}
			rows = append(rows, ChannelDirectoryEntry{
				Kind:   "user",
				ID:     fmt.Sprintf("user:%s", user.ID),
				Name:   name,
				Handle: handle,
				Rank:   buildUserRank(user),
				Raw:    member,
			})
			if len(rows) >= limit {
				return rows, errors.Join(errs...)
			}
		}
	}
	return rows, errors.Join(errs...)
}
