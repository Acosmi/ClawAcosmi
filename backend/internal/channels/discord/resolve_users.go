package discord

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Discord 用户允许列表解析 — 继承自 src/discord/resolve-users.ts (181L)

// DiscordUserResolution 用户解析结果
type DiscordUserResolution struct {
	Input     string `json:"input"`
	Resolved  bool   `json:"resolved"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	GuildID   string `json:"guildId,omitempty"`
	GuildName string `json:"guildName,omitempty"`
	Note      string `json:"note,omitempty"`
}

// parsedUserInput 解析后的用户输入
type parsedUserInput struct {
	userID    string
	guildID   string
	guildName string
	userName  string
}

var userMentionRe = regexp.MustCompile(`^<@!?(\d+)>$`)
var userPrefixRe = regexp.MustCompile(`(?i)^(?:user:|discord:)?(\d+)$`)

func parseDiscordUserInput(raw string) parsedUserInput {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsedUserInput{}
	}
	if m := userMentionRe.FindStringSubmatch(trimmed); m != nil {
		return parsedUserInput{userID: m[1]}
	}
	if m := userPrefixRe.FindStringSubmatch(trimmed); m != nil {
		return parsedUserInput{userID: m[1]}
	}

	var parts []string
	if strings.Contains(trimmed, "/") {
		parts = strings.SplitN(trimmed, "/", 2)
	} else if strings.Contains(trimmed, "#") {
		parts = strings.SplitN(trimmed, "#", 2)
	}
	if len(parts) >= 2 {
		guild := strings.TrimSpace(parts[0])
		user := strings.TrimSpace(parts[1])
		if guild != "" && discordNumericRe.MatchString(guild) {
			return parsedUserInput{guildID: guild, userName: user}
		}
		return parsedUserInput{guildName: guild, userName: user}
	}
	return parsedUserInput{userName: strings.TrimPrefix(trimmed, "@")}
}

// scoreDiscordMember 对成员匹配度打分
func scoreDiscordMember(member discordMember, query string) int {
	q := strings.ToLower(query)
	user := member.User
	var candidates []string
	if user.Username != "" {
		candidates = append(candidates, strings.ToLower(user.Username))
	}
	if user.GlobalName != "" {
		candidates = append(candidates, strings.ToLower(user.GlobalName))
	}
	if member.Nick != "" {
		candidates = append(candidates, strings.ToLower(member.Nick))
	}

	score := 0
	exactMatch := false
	containsMatch := false
	for _, c := range candidates {
		if c == q {
			exactMatch = true
		}
		if strings.Contains(c, q) {
			containsMatch = true
		}
	}
	if exactMatch {
		score += 3
	}
	if containsMatch {
		score += 1
	}
	if !user.Bot {
		score += 1
	}
	return score
}

// ResolveDiscordUserAllowlist 解析用户允许列表
// W-072: 添加 opts 参数注入 fetcher 选项
func ResolveDiscordUserAllowlist(ctx context.Context, token string, entries []string, opts *DiscordFetchOptions) ([]DiscordUserResolution, error) {
	normalizedToken := NormalizeDiscordToken(token)
	if normalizedToken == "" {
		results := make([]DiscordUserResolution, len(entries))
		for i, input := range entries {
			results[i] = DiscordUserResolution{Input: input, Resolved: false}
		}
		return results, nil
	}

	guilds, err := listGuilds(ctx, normalizedToken, opts)
	if err != nil {
		return nil, fmt.Errorf("resolve user allowlist: %w", err)
	}

	// W-064: 收集错误
	var errs []error
	var results []DiscordUserResolution

	for _, input := range entries {
		parsed := parseDiscordUserInput(input)

		// 已知用户 ID
		if parsed.userID != "" {
			results = append(results, DiscordUserResolution{Input: input, Resolved: true, ID: parsed.userID})
			continue
		}

		query := strings.TrimSpace(parsed.userName)
		if query == "" {
			results = append(results, DiscordUserResolution{Input: input, Resolved: false})
			continue
		}

		// 过滤 guild 列表
		var guildList []discordGuildSummary
		if parsed.guildID != "" {
			for _, g := range guilds {
				if g.ID == parsed.guildID {
					guildList = append(guildList, g)
				}
			}
		} else if parsed.guildName != "" {
			slug := NormalizeDiscordSlug(parsed.guildName)
			for _, g := range guilds {
				if g.Slug == slug {
					guildList = append(guildList, g)
				}
			}
		} else {
			guildList = guilds
		}

		type bestMatch struct {
			member discordMember
			guild  discordGuildSummary
			score  int
		}
		var best *bestMatch
		matches := 0

		for _, guild := range guildList {
			params := url.Values{
				"query": {query},
				"limit": {"25"},
			}
			path := fmt.Sprintf("/guilds/%s/members/search?%s", guild.ID, params.Encode())
			members, err := FetchDiscord[[]discordMember](ctx, path, normalizedToken, opts)
			// W-064: 收集 member search API 错误并 continue
			if err != nil {
				errs = append(errs, fmt.Errorf("member search guild %s: %w", guild.ID, err))
				continue
			}
			for _, member := range members {
				score := scoreDiscordMember(member, query)
				if score == 0 {
					continue
				}
				matches++
				if best == nil || score > best.score {
					best = &bestMatch{member: member, guild: guild, score: score}
				}
			}
		}

		if best != nil {
			user := best.member.User
			name := strings.TrimSpace(best.member.Nick)
			if name == "" {
				name = strings.TrimSpace(user.GlobalName)
			}
			if name == "" {
				name = strings.TrimSpace(user.Username)
			}
			note := ""
			if matches > 1 {
				note = "multiple matches; chose best"
			}
			results = append(results, DiscordUserResolution{
				Input: input, Resolved: true, ID: user.ID, Name: name,
				GuildID: best.guild.ID, GuildName: best.guild.Name, Note: note,
			})
		} else {
			results = append(results, DiscordUserResolution{Input: input, Resolved: false})
		}
	}

	return results, errors.Join(errs...)
}
