package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Telegram 群组成员审计 — 继承自 src/telegram/audit.ts (163L)

// GroupMembershipAuditEntry 单个群组审计结果
type GroupMembershipAuditEntry struct {
	ChatID      string `json:"chatId"`
	OK          bool   `json:"ok"`
	Status      string `json:"status,omitempty"`
	Error       string `json:"error,omitempty"`
	MatchKey    string `json:"matchKey,omitempty"`
	MatchSource string `json:"matchSource,omitempty"`
}

// GroupMembershipAudit 完整审计结果
type GroupMembershipAudit struct {
	OK                           bool                        `json:"ok"`
	CheckedGroups                int                         `json:"checkedGroups"`
	UnresolvedGroups             int                         `json:"unresolvedGroups"`
	HasWildcardUnmentionedGroups bool                        `json:"hasWildcardUnmentionedGroups"`
	Groups                       []GroupMembershipAuditEntry `json:"groups"`
	ElapsedMs                    int64                       `json:"elapsedMs"`
}

// CollectTelegramUnmentionedGroupIDs 收集不需要 @mention 的群组 ID
func CollectTelegramUnmentionedGroupIDs(groups map[string]*types.TelegramGroupConfig) (groupIDs []string, unresolved int, hasWildcard bool) {
	if groups == nil {
		return nil, 0, false
	}
	if g, ok := groups["*"]; ok && g != nil && g.RequireMention != nil && !*g.RequireMention && (g.Enabled == nil || *g.Enabled) {
		hasWildcard = true
	}
	for key, val := range groups {
		if key == "*" || val == nil {
			continue
		}
		if val.Enabled != nil && !*val.Enabled {
			continue
		}
		if val.RequireMention == nil || *val.RequireMention {
			continue
		}
		id := strings.TrimSpace(key)
		if id == "" {
			continue
		}
		if isNumericChatID(id) {
			groupIDs = append(groupIDs, id)
		} else {
			unresolved++
		}
	}
	sort.Strings(groupIDs)
	return
}

func isNumericChatID(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' {
		start = 1
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return start < len(s)
}

// AuditTelegramGroupMembership 检查 bot 是否在指定群组中
func AuditTelegramGroupMembership(ctx context.Context, client *http.Client, token string, botID int64, groupIDs []string, timeoutMs int) *GroupMembershipAudit {
	started := time.Now()
	if strings.TrimSpace(token) == "" || len(groupIDs) == 0 {
		return &GroupMembershipAudit{OK: true, ElapsedMs: time.Since(started).Milliseconds()}
	}
	base := fmt.Sprintf("%s/bot%s", TelegramAPIBaseURL, token)
	var groups []GroupMembershipAuditEntry

	for _, chatID := range groupIDs {
		entry := GroupMembershipAuditEntry{ChatID: chatID, MatchKey: chatID, MatchSource: "id"}
		apiURL := fmt.Sprintf("%s/getChatMember?chat_id=%s&user_id=%d",
			base, url.QueryEscape(chatID), botID)

		tCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		req, err := http.NewRequestWithContext(tCtx, http.MethodGet, apiURL, nil)
		if err != nil {
			cancel()
			entry.Error = err.Error()
			groups = append(groups, entry)
			continue
		}
		resp, err := client.Do(req)
		cancel()
		if err != nil {
			entry.Error = err.Error()
			groups = append(groups, entry)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var apiResp struct {
			OK     bool   `json:"ok"`
			Desc   string `json:"description"`
			Result *struct {
				Status string `json:"status"`
			} `json:"result"`
		}
		_ = json.Unmarshal(body, &apiResp)

		// 对齐 TS: !res.ok || !json.ok 同时检查 HTTP 状态码和 API ok 字段
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || !apiResp.OK {
			entry.Error = apiResp.Desc
			if entry.Error == "" {
				entry.Error = fmt.Sprintf("getChatMember failed (%d)", resp.StatusCode)
			}
			groups = append(groups, entry)
			continue
		}

		if apiResp.Result != nil {
			entry.Status = apiResp.Result.Status
			entry.OK = entry.Status == "creator" || entry.Status == "administrator" || entry.Status == "member"
		}
		if !entry.OK {
			entry.Error = "bot not in group"
		}
		groups = append(groups, entry)
	}

	allOK := true
	for _, g := range groups {
		if !g.OK {
			allOK = false
			break
		}
	}
	return &GroupMembershipAudit{
		OK:            allOK,
		CheckedGroups: len(groups),
		Groups:        groups,
		ElapsedMs:     time.Since(started).Milliseconds(),
	}
}
