package slack

import (
	"context"
	"encoding/json"
	"strings"
)

// Slack 频道解析 — 继承自 src/slack/resolve-channels.ts (132L)

// SlackChannelLookup 频道查找结果
type SlackChannelLookup struct {
	ID        string
	Name      string
	Archived  bool
	IsPrivate bool
}

// SlackChannelResolution 频道解析结果
type SlackChannelResolution struct {
	Input    string `json:"input"`
	Resolved bool   `json:"resolved"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Archived *bool  `json:"archived,omitempty"`
}

// parseSlackChannelMention 解析 Slack 频道提及格式。
func parseSlackChannelMention(raw string) (id, name string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	// <#C12345|channel-name>
	if strings.HasPrefix(trimmed, "<#") && strings.HasSuffix(trimmed, ">") {
		inner := trimmed[2 : len(trimmed)-1]
		parts := strings.SplitN(inner, "|", 2)
		id = strings.ToUpper(strings.TrimSpace(parts[0]))
		if len(parts) > 1 {
			name = strings.TrimSpace(parts[1])
		}
		return id, name
	}

	// 去除前缀
	prefixed := trimmed
	lower := strings.ToLower(prefixed)
	if strings.HasPrefix(lower, "slack:") {
		prefixed = strings.TrimSpace(prefixed[6:])
	} else if strings.HasPrefix(lower, "channel:") {
		prefixed = strings.TrimSpace(prefixed[8:])
	}

	// C/G 开头的 ID
	if len(prefixed) > 0 && (prefixed[0] == 'C' || prefixed[0] == 'c' || prefixed[0] == 'G' || prefixed[0] == 'g') {
		if slackAlphanumericRe.MatchString(prefixed) {
			return strings.ToUpper(prefixed), ""
		}
	}

	// 频道名
	cleanName := strings.TrimPrefix(prefixed, "#")
	cleanName = strings.TrimSpace(cleanName)
	if cleanName != "" {
		return "", cleanName
	}
	return "", ""
}

// listSlackChannels 列出所有频道（分页）。
func listSlackChannels(ctx context.Context, client *SlackWebClient) ([]SlackChannelLookup, error) {
	var channels []SlackChannelLookup
	var cursor string

	for {
		params := map[string]interface{}{
			"types":            "public_channel,private_channel",
			"exclude_archived": false,
			"limit":            1000,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		raw, err := client.APICall(ctx, "conversations.list", params)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Channels []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				IsArchived bool   `json:"is_archived"`
				IsPrivate  bool   `json:"is_private"`
			} `json:"channels"`
			ResponseMetadata struct {
				NextCursor string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}

		for _, ch := range resp.Channels {
			id := strings.TrimSpace(ch.ID)
			name := strings.TrimSpace(ch.Name)
			if id == "" || name == "" {
				continue
			}
			channels = append(channels, SlackChannelLookup{
				ID:        id,
				Name:      name,
				Archived:  ch.IsArchived,
				IsPrivate: ch.IsPrivate,
			})
		}

		next := strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if next == "" {
			break
		}
		cursor = next
	}
	return channels, nil
}

// resolveByName 按名称查找频道（优先非归档）。
func resolveChannelByName(name string, channels []SlackChannelLookup) *SlackChannelLookup {
	target := strings.TrimSpace(strings.ToLower(name))
	if target == "" {
		return nil
	}
	var matches []SlackChannelLookup
	for _, ch := range channels {
		if strings.ToLower(ch.Name) == target {
			matches = append(matches, ch)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	for _, m := range matches {
		if !m.Archived {
			return &m
		}
	}
	return &matches[0]
}

// ResolveSlackChannelAllowlist 解析频道允许列表（名称→ID）。
func ResolveSlackChannelAllowlist(ctx context.Context, token string, entries []string) ([]SlackChannelResolution, error) {
	client := NewSlackWebClient(token)
	channels, err := listSlackChannels(ctx, client)
	if err != nil {
		return nil, err
	}

	var results []SlackChannelResolution
	for _, input := range entries {
		id, name := parseSlackChannelMention(input)

		if id != "" {
			// 按 ID 查找
			var match *SlackChannelLookup
			for i, ch := range channels {
				if ch.ID == id {
					match = &channels[i]
					break
				}
			}
			matchName := name
			if match != nil {
				matchName = match.Name
			}
			archived := false
			if match != nil {
				archived = match.Archived
			}
			results = append(results, SlackChannelResolution{
				Input:    input,
				Resolved: true,
				ID:       id,
				Name:     matchName,
				Archived: &archived,
			})
			continue
		}

		if name != "" {
			match := resolveChannelByName(name, channels)
			if match != nil {
				results = append(results, SlackChannelResolution{
					Input:    input,
					Resolved: true,
					ID:       match.ID,
					Name:     match.Name,
					Archived: &match.Archived,
				})
				continue
			}
		}

		results = append(results, SlackChannelResolution{
			Input:    input,
			Resolved: false,
		})
	}

	return results, nil
}
