package discord

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Discord 频道解析 — 继承自 src/discord/resolve-channels.ts (324L)

// NormalizeDiscordSlug 规范化 Discord slug（小写+替换非字母数字为连字符）
// 注：此函数也被 monitor/allow-list.ts 使用
var slugNonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func NormalizeDiscordSlug(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	return strings.Trim(slugNonAlphanumRe.ReplaceAllString(lower, "-"), "-")
}

// discordGuildSummary 服务器摘要
type discordGuildSummary struct {
	ID   string
	Name string
	Slug string
}

// discordChannelSummary 频道摘要
type discordChannelSummary struct {
	ID       string
	Name     string
	GuildID  string
	Type     *int
	Archived bool
}

// discordChannelPayload Discord API 频道响应
type discordChannelPayload struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	Type           *int   `json:"type,omitempty"`
	GuildID        string `json:"guild_id,omitempty"`
	ThreadMetadata *struct {
		Archived bool `json:"archived,omitempty"`
	} `json:"thread_metadata,omitempty"`
}

// DiscordChannelResolution 频道解析结果
type DiscordChannelResolution struct {
	Input       string `json:"input"`
	Resolved    bool   `json:"resolved"`
	GuildID     string `json:"guildId,omitempty"`
	GuildName   string `json:"guildName,omitempty"`
	ChannelID   string `json:"channelId,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	Note        string `json:"note,omitempty"`
}

// parseDiscordChannelInput 解析频道输入字符串
type parsedChannelInput struct {
	guild     string
	channel   string
	channelID string
	guildID   string
	guildOnly bool
}

var channelMentionRe = regexp.MustCompile(`^<#(\d+)>$`)
var channelPrefixRe = regexp.MustCompile(`(?i)^(?:channel:|discord:)?(\d+)$`)
var guildPrefixRe = regexp.MustCompile(`(?i)^(?:guild:|server:)?(\d+)$`)

func parseDiscordChannelInput(raw string) parsedChannelInput {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsedChannelInput{}
	}

	if m := channelMentionRe.FindStringSubmatch(trimmed); m != nil {
		return parsedChannelInput{channelID: m[1]}
	}
	if m := channelPrefixRe.FindStringSubmatch(trimmed); m != nil {
		return parsedChannelInput{channelID: m[1]}
	}
	if m := guildPrefixRe.FindStringSubmatch(trimmed); m != nil {
		if !strings.Contains(trimmed, "/") && !strings.Contains(trimmed, "#") {
			return parsedChannelInput{guildID: m[1], guildOnly: true}
		}
	}

	var parts []string
	if strings.Contains(trimmed, "/") {
		parts = strings.SplitN(trimmed, "/", 2)
	} else if strings.Contains(trimmed, "#") {
		parts = strings.SplitN(trimmed, "#", 2)
	}
	if len(parts) >= 2 {
		guild := strings.TrimSpace(parts[0])
		channel := strings.TrimSpace(parts[1])
		if channel == "" {
			if guild != "" {
				return parsedChannelInput{guild: guild, guildOnly: true}
			}
			return parsedChannelInput{}
		}
		if guild != "" && discordNumericRe.MatchString(guild) {
			return parsedChannelInput{guildID: guild, channel: channel}
		}
		return parsedChannelInput{guild: guild, channel: channel}
	}

	return parsedChannelInput{guild: trimmed, guildOnly: true}
}

// listGuilds 获取 Bot 加入的服务器列表
func listGuilds(ctx context.Context, token string, opts *DiscordFetchOptions) ([]discordGuildSummary, error) {
	raw, err := FetchDiscord[[]discordGuild](ctx, "/users/@me/guilds", token, opts)
	if err != nil {
		return nil, err
	}
	result := make([]discordGuildSummary, len(raw))
	for i, g := range raw {
		result[i] = discordGuildSummary{ID: g.ID, Name: g.Name, Slug: NormalizeDiscordSlug(g.Name)}
	}
	return result, nil
}

// listGuildChannels 获取服务器的频道列表
func listGuildChannels(ctx context.Context, token string, guildID string, opts *DiscordFetchOptions) ([]discordChannelSummary, error) {
	raw, err := FetchDiscord[[]discordChannelPayload](ctx, fmt.Sprintf("/guilds/%s/channels", guildID), token, opts)
	if err != nil {
		return nil, err
	}
	var result []discordChannelSummary
	for _, ch := range raw {
		if ch.ID == "" || ch.Name == "" {
			continue
		}
		var archived bool
		if ch.ThreadMetadata != nil {
			archived = ch.ThreadMetadata.Archived
		}
		result = append(result, discordChannelSummary{
			ID: ch.ID, Name: ch.Name, GuildID: guildID, Type: ch.Type, Archived: archived,
		})
	}
	return result, nil
}

// fetchSingleChannel 获取单个频道信息
func fetchSingleChannel(ctx context.Context, token string, channelID string, opts *DiscordFetchOptions) (*discordChannelSummary, error) {
	raw, err := FetchDiscord[discordChannelPayload](ctx, fmt.Sprintf("/channels/%s", channelID), token, opts)
	if err != nil {
		return nil, err
	}
	if raw.GuildID == "" || raw.ID == "" {
		return nil, nil
	}
	// W-063: 从 ThreadMetadata 提取 Archived 字段
	var archived bool
	if raw.ThreadMetadata != nil {
		archived = raw.ThreadMetadata.Archived
	}
	return &discordChannelSummary{
		ID: raw.ID, Name: raw.Name, GuildID: raw.GuildID, Type: raw.Type, Archived: archived,
	}, nil
}

// preferActiveMatch 优先选择活跃频道
func preferActiveMatch(candidates []discordChannelSummary) *discordChannelSummary {
	if len(candidates) == 0 {
		return nil
	}
	type scored struct {
		channel discordChannelSummary
		score   int
	}
	items := make([]scored, len(candidates))
	for i, ch := range candidates {
		score := 0
		if !ch.Archived {
			score += 2
		}
		isThread := ch.Type != nil && (*ch.Type == 11 || *ch.Type == 12)
		if !isThread {
			score += 1
		}
		items[i] = scored{channel: ch, score: score}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})
	return &items[0].channel
}

// resolveGuildByName 按名称查找服务器
func resolveGuildByName(guilds []discordGuildSummary, input string) *discordGuildSummary {
	slug := NormalizeDiscordSlug(input)
	if slug == "" {
		return nil
	}
	for i := range guilds {
		if guilds[i].Slug == slug {
			return &guilds[i]
		}
	}
	return nil
}

// ResolveDiscordChannelAllowlist 解析频道白名单
// W-071: 添加 opts 参数注入 fetcher 选项
func ResolveDiscordChannelAllowlist(ctx context.Context, token string, entries []string, opts *DiscordFetchOptions) ([]DiscordChannelResolution, error) {
	normalizedToken := NormalizeDiscordToken(token)
	if normalizedToken == "" {
		results := make([]DiscordChannelResolution, len(entries))
		for i, input := range entries {
			results[i] = DiscordChannelResolution{Input: input, Resolved: false}
		}
		return results, nil
	}

	guilds, err := listGuilds(ctx, normalizedToken, opts)
	if err != nil {
		return nil, fmt.Errorf("resolve channel allowlist: %w", err)
	}

	// W-062: 收集错误
	var errs []error

	// 频道缓存
	channelCache := make(map[string][]discordChannelSummary)
	getChannels := func(guildID string) ([]discordChannelSummary, error) {
		if cached, ok := channelCache[guildID]; ok {
			return cached, nil
		}
		channels, err := listGuildChannels(ctx, normalizedToken, guildID, opts)
		if err != nil {
			return nil, err
		}
		channelCache[guildID] = channels
		return channels, nil
	}

	var results []DiscordChannelResolution

	for _, input := range entries {
		parsed := parseDiscordChannelInput(input)

		// 仅 guild
		if parsed.guildOnly {
			var guild *discordGuildSummary
			if parsed.guildID != "" {
				for i := range guilds {
					if guilds[i].ID == parsed.guildID {
						guild = &guilds[i]
						break
					}
				}
			} else if parsed.guild != "" {
				guild = resolveGuildByName(guilds, parsed.guild)
			}
			if guild != nil {
				results = append(results, DiscordChannelResolution{Input: input, Resolved: true, GuildID: guild.ID, GuildName: guild.Name})
			} else {
				results = append(results, DiscordChannelResolution{Input: input, Resolved: false, GuildID: parsed.guildID, GuildName: parsed.guild})
			}
			continue
		}

		// 精确频道 ID
		if parsed.channelID != "" {
			ch, err := fetchSingleChannel(ctx, normalizedToken, parsed.channelID, opts)
			if err == nil && ch != nil {
				var guildName string
				for _, g := range guilds {
					if g.ID == ch.GuildID {
						guildName = g.Name
						break
					}
				}
				results = append(results, DiscordChannelResolution{Input: input, Resolved: true, GuildID: ch.GuildID, GuildName: guildName, ChannelID: ch.ID, ChannelName: ch.Name, Archived: ch.Archived})
			} else {
				results = append(results, DiscordChannelResolution{Input: input, Resolved: false, ChannelID: parsed.channelID})
			}
			continue
		}

		// guild + channel 名称
		if parsed.guildID != "" || parsed.guild != "" {
			var guild *discordGuildSummary
			if parsed.guildID != "" {
				for i := range guilds {
					if guilds[i].ID == parsed.guildID {
						guild = &guilds[i]
						break
					}
				}
			} else if parsed.guild != "" {
				guild = resolveGuildByName(guilds, parsed.guild)
			}
			channelQuery := strings.TrimSpace(parsed.channel)
			if guild == nil || channelQuery == "" {
				results = append(results, DiscordChannelResolution{Input: input, Resolved: false, GuildID: parsed.guildID, GuildName: parsed.guild, ChannelName: parsed.channel})
				continue
			}
			// W-062: getChannels 错误收集 — 标记 resolved=false 并 continue
			channels, err := getChannels(guild.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("getChannels guild %s: %w", guild.ID, err))
				results = append(results, DiscordChannelResolution{Input: input, Resolved: false, GuildID: guild.ID, GuildName: guild.Name, ChannelName: parsed.channel, Note: fmt.Sprintf("failed to list channels: %v", err)})
				continue
			}
			var matches []discordChannelSummary
			for _, ch := range channels {
				if NormalizeDiscordSlug(ch.Name) == NormalizeDiscordSlug(channelQuery) {
					matches = append(matches, ch)
				}
			}
			match := preferActiveMatch(matches)
			if match != nil {
				results = append(results, DiscordChannelResolution{Input: input, Resolved: true, GuildID: guild.ID, GuildName: guild.Name, ChannelID: match.ID, ChannelName: match.Name, Archived: match.Archived})
			} else {
				results = append(results, DiscordChannelResolution{Input: input, Resolved: false, GuildID: guild.ID, GuildName: guild.Name, ChannelName: parsed.channel, Note: fmt.Sprintf("channel not found in guild %s", guild.Name)})
			}
			continue
		}

		// 仅频道名
		channelName := strings.TrimPrefix(strings.TrimSpace(input), "#")
		if channelName == "" {
			results = append(results, DiscordChannelResolution{Input: input, Resolved: false, ChannelName: channelName})
			continue
		}
		var candidates []discordChannelSummary
		for _, guild := range guilds {
			// W-062: 跨 guild 搜索时 getChannels 错误直接 continue（跳过无法访问的 guild）
			channels, err := getChannels(guild.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("getChannels guild %s: %w", guild.ID, err))
				continue
			}
			for _, ch := range channels {
				if NormalizeDiscordSlug(ch.Name) == NormalizeDiscordSlug(channelName) {
					candidates = append(candidates, ch)
				}
			}
		}
		match := preferActiveMatch(candidates)
		if match != nil {
			var guildName string
			for _, g := range guilds {
				if g.ID == match.GuildID {
					guildName = g.Name
					break
				}
			}
			note := ""
			if len(candidates) > 1 && guildName != "" {
				note = fmt.Sprintf("matched multiple; chose %s", guildName)
			}
			results = append(results, DiscordChannelResolution{Input: input, Resolved: true, GuildID: match.GuildID, GuildName: guildName, ChannelID: match.ID, ChannelName: match.Name, Archived: match.Archived, Note: note})
		} else {
			results = append(results, DiscordChannelResolution{Input: input, Resolved: false, ChannelName: channelName})
		}
	}

	return results, errors.Join(errs...)
}
