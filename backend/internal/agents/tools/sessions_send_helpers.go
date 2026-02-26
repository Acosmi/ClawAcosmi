// tools/sessions_send_helpers.go — 会话发送辅助函数。
// TS 参考：sessions-send-helpers.ts (167L) + sessions-announce-target.ts (59L)
//   - sessions-send-tool.a2a.ts (143L)
//
// 全量移植：AnnounceTarget 解析 + A2A 消息上下文 + ping-pong 限制
package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ---------- 常量 ----------

const (
	AnnounceSkipToken    = "ANNOUNCE_SKIP"
	ReplySkipToken       = "REPLY_SKIP"
	DefaultPingPongTurns = 5
	MaxPingPongTurns     = 5
)

// ---------- AnnounceTarget ----------

// AnnounceTarget 消息投递目标。
type AnnounceTarget struct {
	Channel   string `json:"channel"`
	To        string `json:"to"`
	AccountID string `json:"accountId,omitempty"`
	ThreadID  string `json:"threadId,omitempty"`
}

// topicOrThreadRE 匹配 :topic:N 或 :thread:N 后缀。
var topicOrThreadRE = regexp.MustCompile(`:(topic|thread):(\d+)$`)

// ResolveAnnounceTargetFromKey 从 session key 解析投递目标。
// 对齐 TS: resolveAnnounceTargetFromKey()
func ResolveAnnounceTargetFromKey(sessionKey string) *AnnounceTarget {
	rawParts := splitFilterEmpty(sessionKey, ":")
	parts := rawParts
	if len(rawParts) >= 3 && rawParts[0] == "agent" {
		parts = rawParts[2:]
	}
	if len(parts) < 3 {
		return nil
	}
	channelRaw := parts[0]
	kind := parts[1]
	if kind != "group" && kind != "channel" {
		return nil
	}
	rest := strings.Join(parts[2:], ":")

	// 提取 thread/topic ID
	var threadID string
	match := topicOrThreadRE.FindStringSubmatch(rest)
	if match != nil {
		threadID = match[2]
		rest = topicOrThreadRE.ReplaceAllString(rest, "")
	}
	id := strings.TrimSpace(rest)
	if id == "" || channelRaw == "" {
		return nil
	}

	channel := strings.ToLower(channelRaw)
	// 对 discord/slack 使用 channel: 前缀
	kindTarget := id
	if channel == "discord" || channel == "slack" {
		kindTarget = "channel:" + id
	} else if kind == "channel" {
		kindTarget = "channel:" + id
	} else {
		kindTarget = "group:" + id
	}

	return &AnnounceTarget{
		Channel:  channel,
		To:       kindTarget,
		ThreadID: threadID,
	}
}

// splitFilterEmpty 按分隔符分割并过滤空段。
func splitFilterEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ---------- A2A 消息上下文构建 ----------

// BuildAgentToAgentMessageContext 构建 A2A 消息上下文文本。
// 对齐 TS: buildAgentToAgentMessageContext()
func BuildAgentToAgentMessageContext(requesterSessionKey, requesterChannel, targetSessionKey string) string {
	var lines []string
	lines = append(lines, "Agent-to-agent message context:")
	if requesterSessionKey != "" {
		lines = append(lines, "Agent 1 (requester) session: "+requesterSessionKey+".")
	}
	if requesterChannel != "" {
		lines = append(lines, "Agent 1 (requester) channel: "+requesterChannel+".")
	}
	lines = append(lines, "Agent 2 (target) session: "+targetSessionKey+".")
	return strings.Join(lines, "\n")
}

// BuildAgentToAgentReplyContext 构建 A2A ping-pong 回复上下文。
// 对齐 TS: buildAgentToAgentReplyContext()
func BuildAgentToAgentReplyContext(
	requesterSessionKey, requesterChannel string,
	targetSessionKey, targetChannel string,
	currentRole string, turn, maxTurns int,
) string {
	currentLabel := "Agent 2 (target)"
	if currentRole == "requester" {
		currentLabel = "Agent 1 (requester)"
	}
	lines := []string{
		"Agent-to-agent reply step:",
		"Current agent: " + currentLabel + ".",
		fmt.Sprintf("Turn %d of %d.", turn, maxTurns),
	}
	if requesterSessionKey != "" {
		lines = append(lines, "Agent 1 (requester) session: "+requesterSessionKey+".")
	}
	if requesterChannel != "" {
		lines = append(lines, "Agent 1 (requester) channel: "+requesterChannel+".")
	}
	lines = append(lines, "Agent 2 (target) session: "+targetSessionKey+".")
	if targetChannel != "" {
		lines = append(lines, "Agent 2 (target) channel: "+targetChannel+".")
	}
	lines = append(lines, `If you want to stop the ping-pong, reply exactly "`+ReplySkipToken+`".`)
	return strings.Join(lines, "\n")
}

// BuildAgentToAgentAnnounceContext 构建 A2A 公告上下文。
// 对齐 TS: buildAgentToAgentAnnounceContext()
func BuildAgentToAgentAnnounceContext(
	requesterSessionKey, requesterChannel string,
	targetSessionKey, targetChannel string,
	originalMessage, roundOneReply, latestReply string,
) string {
	lines := []string{"Agent-to-agent announce step:"}
	if requesterSessionKey != "" {
		lines = append(lines, "Agent 1 (requester) session: "+requesterSessionKey+".")
	}
	if requesterChannel != "" {
		lines = append(lines, "Agent 1 (requester) channel: "+requesterChannel+".")
	}
	lines = append(lines, "Agent 2 (target) session: "+targetSessionKey+".")
	if targetChannel != "" {
		lines = append(lines, "Agent 2 (target) channel: "+targetChannel+".")
	}
	lines = append(lines, "Original request: "+originalMessage)
	if roundOneReply != "" {
		lines = append(lines, "Round 1 reply: "+roundOneReply)
	} else {
		lines = append(lines, "Round 1 reply: (not available).")
	}
	if latestReply != "" {
		lines = append(lines, "Latest reply: "+latestReply)
	} else {
		lines = append(lines, "Latest reply: (not available).")
	}
	lines = append(lines, `If you want to remain silent, reply exactly "`+AnnounceSkipToken+`".`)
	lines = append(lines, "Any other reply will be posted to the target channel.")
	lines = append(lines, "After this reply, the agent-to-agent conversation is over.")
	return strings.Join(lines, "\n")
}

// ---------- Skip 判定 ----------

// IsAnnounceSkip 检查回复是否为公告跳过标记。
func IsAnnounceSkip(text string) bool {
	return strings.TrimSpace(text) == AnnounceSkipToken
}

// IsReplySkip 检查回复是否为回复跳过标记。
func IsReplySkip(text string) bool {
	return strings.TrimSpace(text) == ReplySkipToken
}

// ---------- Ping-Pong 限制 ----------

// ResolvePingPongTurns 解析 A2A ping-pong 最大回合数。
// 对齐 TS: resolvePingPongTurns()
func ResolvePingPongTurns(maxTurns *int) int {
	if maxTurns == nil {
		return DefaultPingPongTurns
	}
	v := *maxTurns
	if v < 0 {
		v = 0
	}
	if v > MaxPingPongTurns {
		v = MaxPingPongTurns
	}
	return v
}

// ---------- Announce Target 解析（Gateway 版） ----------

// AnnounceTargetResolver 公告目标解析器接口。
type AnnounceTargetResolver interface {
	ListSessionRows(ctx context.Context) ([]SessionListRow, error)
}

// ResolveAnnounceTarget 解析公告投递目标（优先从 key 解析，必要时查 gateway）。
// 对齐 TS: resolveAnnounceTarget() (sessions-announce-target.ts)
func ResolveAnnounceTarget(ctx context.Context, resolver AnnounceTargetResolver, sessionKey, displayKey string) *AnnounceTarget {
	// 优先从 key 静态解析
	parsed := ResolveAnnounceTargetFromKey(sessionKey)
	parsedDisplay := ResolveAnnounceTargetFromKey(displayKey)
	fallback := parsed
	if fallback == nil {
		fallback = parsedDisplay
	}

	// 如果有 fallback 且不需要 session lookup 的频道，直接返回
	if fallback != nil {
		return fallback
	}

	// 尝试通过 gateway 查询
	if resolver != nil {
		sessions, err := resolver.ListSessionRows(ctx)
		if err == nil {
			var match *SessionListRow
			for i := range sessions {
				if sessions[i].Key == sessionKey {
					match = &sessions[i]
					break
				}
			}
			if match == nil {
				for i := range sessions {
					if sessions[i].Key == displayKey {
						match = &sessions[i]
						break
					}
				}
			}
			if match != nil {
				var channel, to, accountID string
				if match.DeliveryContext != nil {
					channel = match.DeliveryContext.Channel
					to = match.DeliveryContext.To
					accountID = match.DeliveryContext.AccountID
				}
				if channel == "" {
					channel = match.LastChannel
				}
				if to == "" {
					to = match.LastTo
				}
				if accountID == "" {
					accountID = match.LastAccountID
				}
				if channel != "" && to != "" {
					return &AnnounceTarget{
						Channel:   channel,
						To:        to,
						AccountID: accountID,
					}
				}
			}
		}
	}

	return fallback
}
