package channels

import (
	"regexp"
	"strings"
)

// 通讯录配置查询 — 继承自 src/channels/plugins/directory-config.ts (241L)
// 从频道配置中提取 peer/group 列表

// DirectoryEntry 通讯录条目
type DirectoryEntry struct {
	Kind string `json:"kind"` // "user" | "group"
	ID   string `json:"id"`
}

// DirectoryQueryParams 查询参数
type DirectoryQueryParams struct {
	AllowFrom []string
	DMs       map[string]interface{}
	Channels  map[string]interface{}
	Guilds    map[string]interface{}
	Groups    map[string]interface{}
	Query     string
	Limit     int
}

// ── Slack ──

var slackUserMentionExtract = regexp.MustCompile(`(?i)^<@([A-Z0-9]+)>$`)

// ListSlackDirectoryPeers Slack 用户通讯录
func ListSlackDirectoryPeers(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	ids := make(map[string]bool)
	for _, entry := range p.AllowFrom {
		raw := strings.TrimSpace(entry)
		if raw != "" && raw != "*" {
			ids[raw] = true
		}
	}
	for id := range p.DMs {
		if t := strings.TrimSpace(id); t != "" {
			ids[t] = true
		}
	}
	var result []DirectoryEntry
	for raw := range ids {
		m := slackUserMentionExtract.FindStringSubmatch(raw)
		var uid string
		if len(m) > 1 {
			uid = m[1]
		} else {
			uid = regexp.MustCompile(`(?i)^(slack|user):`).ReplaceAllString(raw, "")
			uid = strings.TrimSpace(uid)
		}
		if uid == "" {
			continue
		}
		target := "user:" + uid
		if q != "" && !strings.Contains(strings.ToLower(target), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "user", ID: target})
	}
	return applyLimit(result, p.Limit)
}

// ListSlackDirectoryGroups Slack 频道通讯录
func ListSlackDirectoryGroups(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	var result []DirectoryEntry
	for raw := range p.Channels {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		normalized := NormalizeSlackMessagingTarget(t)
		if normalized == "" {
			normalized = strings.ToLower(t)
		}
		if !strings.HasPrefix(normalized, "channel:") {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(normalized), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "group", ID: normalized})
	}
	return applyLimit(result, p.Limit)
}

// ── Discord ──

var discordUserMentionExtract = regexp.MustCompile(`^<@!?(\d+)>$`)
var discordChannelMentionExtract = regexp.MustCompile(`^<#(\d+)>$`)
var discordNumericOnly = regexp.MustCompile(`^\d+$`)

// ListDiscordDirectoryPeers Discord 用户通讯录
func ListDiscordDirectoryPeers(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	ids := make(map[string]bool)
	for _, entry := range p.AllowFrom {
		raw := strings.TrimSpace(entry)
		if raw != "" && raw != "*" {
			ids[raw] = true
		}
	}
	for id := range p.DMs {
		if t := strings.TrimSpace(id); t != "" {
			ids[t] = true
		}
	}
	var result []DirectoryEntry
	for raw := range ids {
		m := discordUserMentionExtract.FindStringSubmatch(raw)
		var cleaned string
		if len(m) > 1 {
			cleaned = m[1]
		} else {
			cleaned = regexp.MustCompile(`(?i)^(discord|user):`).ReplaceAllString(raw, "")
			cleaned = strings.TrimSpace(cleaned)
		}
		if !discordNumericOnly.MatchString(cleaned) {
			continue
		}
		target := "user:" + cleaned
		if q != "" && !strings.Contains(strings.ToLower(target), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "user", ID: target})
	}
	return applyLimit(result, p.Limit)
}

// ListDiscordDirectoryGroups Discord 频道通讯录
func ListDiscordDirectoryGroups(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	ids := make(map[string]bool)
	for _, guild := range p.Guilds {
		gm, ok := guild.(map[string]interface{})
		if !ok {
			continue
		}
		chs, ok := gm["channels"].(map[string]interface{})
		if !ok {
			continue
		}
		for chID := range chs {
			if t := strings.TrimSpace(chID); t != "" {
				ids[t] = true
			}
		}
	}
	var result []DirectoryEntry
	for raw := range ids {
		m := discordChannelMentionExtract.FindStringSubmatch(raw)
		var cleaned string
		if len(m) > 1 {
			cleaned = m[1]
		} else {
			cleaned = regexp.MustCompile(`(?i)^(discord|channel|group):`).ReplaceAllString(raw, "")
			cleaned = strings.TrimSpace(cleaned)
		}
		if !discordNumericOnly.MatchString(cleaned) {
			continue
		}
		target := "channel:" + cleaned
		if q != "" && !strings.Contains(strings.ToLower(target), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "group", ID: target})
	}
	return applyLimit(result, p.Limit)
}

// ── Telegram ──

// ListTelegramDirectoryPeers Telegram 用户通讯录
func ListTelegramDirectoryPeers(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	seen := make(map[string]bool)
	var raw []string
	for _, entry := range p.AllowFrom {
		raw = append(raw, entry)
	}
	for id := range p.DMs {
		raw = append(raw, id)
	}
	var result []DirectoryEntry
	for _, entry := range raw {
		t := strings.TrimSpace(entry)
		if t == "" {
			continue
		}
		t = regexp.MustCompile(`(?i)^(telegram|tg):`).ReplaceAllString(t, "")
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		id := t
		if regexp.MustCompile(`^-?\d+$`).MatchString(id) {
			// 数字 ID 直接用
		} else if !strings.HasPrefix(id, "@") {
			id = "@" + id
		}
		if q != "" && !strings.Contains(strings.ToLower(id), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "user", ID: id})
	}
	return applyLimit(result, p.Limit)
}

// ListTelegramDirectoryGroups Telegram 群组通讯录
func ListTelegramDirectoryGroups(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	var result []DirectoryEntry
	for id := range p.Groups {
		t := strings.TrimSpace(id)
		if t == "" || t == "*" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "group", ID: t})
	}
	return applyLimit(result, p.Limit)
}

// ── WhatsApp ──

// ListWhatsAppDirectoryPeers WhatsApp 用户通讯录
func ListWhatsAppDirectoryPeers(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	var result []DirectoryEntry
	for _, entry := range p.AllowFrom {
		t := strings.TrimSpace(entry)
		if t == "" || t == "*" {
			continue
		}
		normalized := NormalizeWhatsAppMessagingTarget(t)
		if normalized == "" || strings.Contains(normalized, "@g.us") {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(normalized), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "user", ID: normalized})
	}
	return applyLimit(result, p.Limit)
}

// ListWhatsAppDirectoryGroups WhatsApp 群组通讯录
func ListWhatsAppDirectoryGroups(p DirectoryQueryParams) []DirectoryEntry {
	q := strings.ToLower(strings.TrimSpace(p.Query))
	var result []DirectoryEntry
	for id := range p.Groups {
		t := strings.TrimSpace(id)
		if t == "" || t == "*" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t), q) {
			continue
		}
		result = append(result, DirectoryEntry{Kind: "group", ID: t})
	}
	return applyLimit(result, p.Limit)
}

// applyLimit 截断结果
func applyLimit(entries []DirectoryEntry, limit int) []DirectoryEntry {
	if limit > 0 && len(entries) > limit {
		return entries[:limit]
	}
	return entries
}
