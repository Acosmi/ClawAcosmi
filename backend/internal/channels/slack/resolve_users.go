package slack

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// Slack 用户解析 — 继承自 src/slack/resolve-users.ts (193L)

// SlackUserLookup 用户查找结果
type SlackUserLookup struct {
	ID          string
	Name        string
	DisplayName string
	RealName    string
	Email       string
	Deleted     bool
	IsBot       bool
	IsAppUser   bool
}

// SlackUserResolution 用户解析结果
type SlackUserResolution struct {
	Input    string `json:"input"`
	Resolved bool   `json:"resolved"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Deleted  *bool  `json:"deleted,omitempty"`
	IsBot    *bool  `json:"isBot,omitempty"`
	Note     string `json:"note,omitempty"`
}

// parseSlackUserInput 解析用户输入格式。
func parseSlackUserInput(raw string) (id, name, email string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return
	}

	// <@U12345>
	if m := slackUserMentionRe.FindStringSubmatch(trimmed); m != nil {
		return strings.ToUpper(m[1]), "", ""
	}

	// 去前缀
	prefixed := trimmed
	lower := strings.ToLower(prefixed)
	if strings.HasPrefix(lower, "slack:") {
		prefixed = strings.TrimSpace(prefixed[6:])
	} else if strings.HasPrefix(lower, "user:") {
		prefixed = strings.TrimSpace(prefixed[5:])
	}

	// 纯字母数字 ID
	if slackAlphanumericRe.MatchString(prefixed) {
		return strings.ToUpper(prefixed), "", ""
	}

	// 邮箱（包含 @ 且不以 @ 开头）
	if strings.Contains(trimmed, "@") && !strings.HasPrefix(trimmed, "@") {
		return "", "", strings.ToLower(trimmed)
	}

	// 用户名
	cleanName := strings.TrimPrefix(trimmed, "@")
	cleanName = strings.TrimSpace(cleanName)
	return "", cleanName, ""
}

// listSlackUsers 列出所有用户（分页）。
func listSlackUsers(ctx context.Context, client *SlackWebClient) ([]SlackUserLookup, error) {
	var users []SlackUserLookup
	var cursor string

	for {
		params := map[string]interface{}{
			"limit": 200,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		raw, err := client.APICall(ctx, "users.list", params)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Members []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Deleted   bool   `json:"deleted"`
				IsBot     bool   `json:"is_bot"`
				IsAppUser bool   `json:"is_app_user"`
				RealName  string `json:"real_name"`
				Profile   struct {
					DisplayName string `json:"display_name"`
					RealName    string `json:"real_name"`
					Email       string `json:"email"`
				} `json:"profile"`
			} `json:"members"`
			ResponseMetadata struct {
				NextCursor string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}

		for _, m := range resp.Members {
			id := strings.TrimSpace(m.ID)
			name := strings.TrimSpace(m.Name)
			if id == "" || name == "" {
				continue
			}
			displayName := strings.TrimSpace(m.Profile.DisplayName)
			realName := strings.TrimSpace(m.Profile.RealName)
			if realName == "" {
				realName = strings.TrimSpace(m.RealName)
			}
			email := strings.TrimSpace(strings.ToLower(m.Profile.Email))

			users = append(users, SlackUserLookup{
				ID:          id,
				Name:        name,
				DisplayName: displayName,
				RealName:    realName,
				Email:       email,
				Deleted:     m.Deleted,
				IsBot:       m.IsBot,
				IsAppUser:   m.IsAppUser,
			})
		}

		next := strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if next == "" {
			break
		}
		cursor = next
	}

	return users, nil
}

// scoreSlackUser 评分用户匹配度。
func scoreSlackUser(user SlackUserLookup, matchName, matchEmail string) int {
	score := 0
	if !user.Deleted {
		score += 3
	}
	if !user.IsBot && !user.IsAppUser {
		score += 2
	}
	if matchEmail != "" && user.Email == matchEmail {
		score += 5
	}
	if matchName != "" {
		target := strings.ToLower(matchName)
		candidates := []string{
			strings.ToLower(user.Name),
			strings.ToLower(user.DisplayName),
			strings.ToLower(user.RealName),
		}
		for _, c := range candidates {
			if c != "" && c == target {
				score += 2
				break
			}
		}
	}
	return score
}

// ResolveSlackUserAllowlist 解析用户允许列表（名称/邮箱→ID）。
func ResolveSlackUserAllowlist(ctx context.Context, token string, entries []string) ([]SlackUserResolution, error) {
	client := NewSlackWebClient(token)
	users, err := listSlackUsers(ctx, client)
	if err != nil {
		return nil, err
	}

	var results []SlackUserResolution
	for _, input := range entries {
		id, name, email := parseSlackUserInput(input)

		if id != "" {
			// 按 ID 查找
			var match *SlackUserLookup
			for i, u := range users {
				if u.ID == id {
					match = &users[i]
					break
				}
			}
			displayName := ""
			if match != nil {
				displayName = match.DisplayName
				if displayName == "" {
					displayName = match.RealName
				}
				if displayName == "" {
					displayName = match.Name
				}
			}
			r := SlackUserResolution{Input: input, Resolved: true, ID: id, Name: displayName}
			if match != nil {
				r.Email = match.Email
				r.Deleted = &match.Deleted
				r.IsBot = &match.IsBot
			}
			results = append(results, r)
			continue
		}

		if email != "" {
			var matches []SlackUserLookup
			for _, u := range users {
				if u.Email == email {
					matches = append(matches, u)
				}
			}
			if len(matches) > 0 {
				best := bestMatch(matches, name, email)
				r := SlackUserResolution{
					Input: input, Resolved: true, ID: best.ID,
					Name: bestDisplayName(best), Email: best.Email,
					Deleted: &best.Deleted, IsBot: &best.IsBot,
				}
				if len(matches) > 1 {
					r.Note = "multiple matches; chose best"
				}
				results = append(results, r)
				continue
			}
		}

		if name != "" {
			target := strings.ToLower(name)
			var matches []SlackUserLookup
			for _, u := range users {
				candidates := []string{
					strings.ToLower(u.Name),
					strings.ToLower(u.DisplayName),
					strings.ToLower(u.RealName),
				}
				for _, c := range candidates {
					if c != "" && c == target {
						matches = append(matches, u)
						break
					}
				}
			}
			if len(matches) > 0 {
				best := bestMatch(matches, name, email)
				r := SlackUserResolution{
					Input: input, Resolved: true, ID: best.ID,
					Name: bestDisplayName(best), Email: best.Email,
					Deleted: &best.Deleted, IsBot: &best.IsBot,
				}
				if len(matches) > 1 {
					r.Note = "multiple matches; chose best"
				}
				results = append(results, r)
				continue
			}
		}

		results = append(results, SlackUserResolution{Input: input, Resolved: false})
	}

	return results, nil
}

func bestMatch(matches []SlackUserLookup, name, email string) SlackUserLookup {
	type scored struct {
		user  SlackUserLookup
		score int
	}
	var items []scored
	for _, m := range matches {
		items = append(items, scored{user: m, score: scoreSlackUser(m, name, email)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	return items[0].user
}

func bestDisplayName(user SlackUserLookup) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	if user.RealName != "" {
		return user.RealName
	}
	return user.Name
}
