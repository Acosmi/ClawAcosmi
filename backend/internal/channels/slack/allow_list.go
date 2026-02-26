package slack

import (
	"fmt"
	"regexp"
	"strings"
)

// Slack 监控允许列表 — 继承自 src/slack/monitor/allow-list.ts (81L)

var slugWhitespaceRe = regexp.MustCompile(`\s+`)
var slugInvalidRe = regexp.MustCompile(`[^a-z0-9#@._+\-]+`)
var slugDashRepeatRe = regexp.MustCompile(`-{2,}`)
var slugDashTrimRe = regexp.MustCompile(`^[.\-]+|[.\-]+$`)

// NormalizeSlackSlug 规范化 Slack slug（用户名/频道名）。
func NormalizeSlackSlug(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	dashed := slugWhitespaceRe.ReplaceAllString(trimmed, "-")
	cleaned := slugInvalidRe.ReplaceAllString(dashed, "-")
	result := slugDashRepeatRe.ReplaceAllString(cleaned, "-")
	return slugDashTrimRe.ReplaceAllString(result, "")
}

// NormalizeAllowList 规范化字符串允许列表。
func NormalizeAllowList(list []interface{}) []string {
	var result []string
	for _, entry := range list {
		s := strings.TrimSpace(fmt.Sprintf("%v", entry))
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// NormalizeAllowListLower 规范化并小写化允许列表。
func NormalizeAllowListLower(list []interface{}) []string {
	raw := NormalizeAllowList(list)
	for i, s := range raw {
		raw[i] = strings.ToLower(s)
	}
	return raw
}

// SlackAllowListMatchSource 匹配来源
type SlackAllowListMatchSource string

const (
	MatchSourceWildcard     SlackAllowListMatchSource = "wildcard"
	MatchSourceID           SlackAllowListMatchSource = "id"
	MatchSourcePrefixedID   SlackAllowListMatchSource = "prefixed-id"
	MatchSourcePrefixedUser SlackAllowListMatchSource = "prefixed-user"
	MatchSourceName         SlackAllowListMatchSource = "name"
	MatchSourcePrefixedName SlackAllowListMatchSource = "prefixed-name"
	MatchSourceSlug         SlackAllowListMatchSource = "slug"
)

// SlackAllowListMatch 允许列表匹配结果
type SlackAllowListMatch struct {
	Allowed     bool
	MatchKey    string
	MatchSource SlackAllowListMatchSource
}

// ResolveSlackAllowListMatch 检查 ID/名称是否在允许列表中。
func ResolveSlackAllowListMatch(allowList []string, id, name string) SlackAllowListMatch {
	if len(allowList) == 0 {
		return SlackAllowListMatch{Allowed: false}
	}

	for _, entry := range allowList {
		if entry == "*" {
			return SlackAllowListMatch{Allowed: true, MatchKey: "*", MatchSource: MatchSourceWildcard}
		}
	}

	lowerID := strings.ToLower(id)
	lowerName := strings.ToLower(name)
	slug := NormalizeSlackSlug(name)

	type candidate struct {
		value  string
		source SlackAllowListMatchSource
	}
	candidates := []candidate{
		{lowerID, MatchSourceID},
	}
	if lowerID != "" {
		candidates = append(candidates,
			candidate{"slack:" + lowerID, MatchSourcePrefixedID},
			candidate{"user:" + lowerID, MatchSourcePrefixedUser},
		)
	}
	candidates = append(candidates, candidate{lowerName, MatchSourceName})
	if lowerName != "" {
		candidates = append(candidates, candidate{"slack:" + lowerName, MatchSourcePrefixedName})
	}
	candidates = append(candidates, candidate{slug, MatchSourceSlug})

	for _, c := range candidates {
		if c.value == "" {
			continue
		}
		for _, entry := range allowList {
			if entry == c.value {
				return SlackAllowListMatch{Allowed: true, MatchKey: c.value, MatchSource: c.source}
			}
		}
	}

	return SlackAllowListMatch{Allowed: false}
}

// AllowListMatches 快速检查是否匹配。
func AllowListMatches(allowList []string, id, name string) bool {
	return ResolveSlackAllowListMatch(allowList, id, name).Allowed
}

// ResolveSlackUserAllowed 检查用户是否被允许。
func ResolveSlackUserAllowed(allowList []interface{}, userID, userName string) bool {
	normalized := NormalizeAllowListLower(allowList)
	if len(normalized) == 0 {
		return true
	}
	return AllowListMatches(normalized, userID, userName)
}
