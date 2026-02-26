package discord

import (
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/anthropic/open-acosmi/internal/routing"
)

// Discord 线程工具 — 继承自 src/discord/monitor/threading.ts (347L)
// 仅移植类型和纯逻辑函数；Client 依赖延迟到 Phase 7

// DiscordThreadChannel 线程频道
// TS ref: includes parent channel context (name, type, ID) for thread resolution
type DiscordThreadChannel struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	ParentID         string `json:"parentId,omitempty"`
	OwnerID          string `json:"ownerId,omitempty"`
	ThreadParentID   string `json:"threadParentId,omitempty"`
	ThreadParentName string `json:"threadParentName,omitempty"`
	ThreadParentType *int   `json:"threadParentType,omitempty"`
}

// DiscordThreadStarter 线程起始消息
type DiscordThreadStarter struct {
	Text      string `json:"text"`
	Author    string `json:"author"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// DiscordThreadParentInfo 线程父级信息
type DiscordThreadParentInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type *int   `json:"type,omitempty"`
}

// IsDiscordThreadType 判断是否为线程类型
func IsDiscordThreadType(channelType *int) bool {
	if channelType == nil {
		return false
	}
	t := *channelType
	return t == channelTypeGuildNewsThread ||
		t == channelTypeGuildPublicThread ||
		t == channelTypeGuildPrivateThread
}

// ResolveDiscordReplyTarget 解析回复目标
// W-031 fix: 对齐 TS 枚举值 "off"/"all"，同时兼容 Go 原有 "never"/"always"
func ResolveDiscordReplyTarget(replyToMode, replyToID string, hasReplied bool) string {
	switch replyToMode {
	case "all", "always":
		return replyToID
	case "first":
		if !hasReplied {
			return replyToID
		}
		return ""
	case "off", "never":
		return ""
	default:
		return replyToID
	}
}

// thread name mention 清理正则（全局替换，不锚定）
var (
	threadUserMentionRe    = regexp.MustCompile(`<@!?\d+>`)
	threadRoleMentionRe    = regexp.MustCompile(`<@&\d+>`)
	threadChannelMentionRe = regexp.MustCompile(`<#\d+>`)
	threadMultiSpaceRe     = regexp.MustCompile(`\s+`)
)

// SanitizeDiscordThreadName 清理线程名称
// W-032 fix: 对齐 TS — 剥离 mention、先截断80再100、fallback 添加 "Thread " 前缀
func SanitizeDiscordThreadName(rawName, fallbackID string) string {
	cleaned := rawName
	cleaned = threadUserMentionRe.ReplaceAllString(cleaned, "")
	cleaned = threadRoleMentionRe.ReplaceAllString(cleaned, "")
	cleaned = threadChannelMentionRe.ReplaceAllString(cleaned, "")
	cleaned = threadMultiSpaceRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	fallback := "Thread " + fallbackID
	if cleaned == "" {
		cleaned = fallback
	}
	base := TruncateUtf16Safe(cleaned, 80)
	result := TruncateUtf16Safe(base, 100)
	if result == "" {
		return fallback
	}
	return result
}

// TruncateUtf16Safe 按 UTF-16 长度截断字符串
func TruncateUtf16Safe(s string, maxLen int) string {
	runes := []rune(s)
	var count int
	var end int
	for i, r := range runes {
		units := len(utf16.Encode([]rune{r}))
		if count+units > maxLen {
			break
		}
		count += units
		end = i + 1
	}
	if end >= len(runes) {
		return s
	}
	return string(runes[:end])
}

// ReplyReferencePlanner 有状态的回复引用规划器
// TS ref: src/auto-reply/reply/reply-reference.ts — createReplyReferencePlanner
// 首次回复时带 reference，后续根据 replyToMode 决定
type ReplyReferencePlanner struct {
	replyToMode    string
	existingID     string // thread 内已有的 reference ID
	startID        string // 用于新引用的起始消息 ID
	allowReference bool
	replied        bool
}

// NewReplyReferencePlanner 创建 ReplyReferencePlanner
func NewReplyReferencePlanner(replyToMode, existingID, startID string, allowReference bool) *ReplyReferencePlanner {
	return &ReplyReferencePlanner{
		replyToMode:    replyToMode,
		existingID:     strings.TrimSpace(existingID),
		startID:        strings.TrimSpace(startID),
		allowReference: allowReference,
		replied:        false,
	}
}

// Use 返回本次发送应使用的 reply reference ID 并更新内部状态
// TS ref: use() in createReplyReferencePlanner
func (p *ReplyReferencePlanner) Use() string {
	if !p.allowReference {
		return ""
	}
	if p.existingID != "" {
		p.replied = true
		return p.existingID
	}
	if p.startID == "" {
		return ""
	}
	switch p.replyToMode {
	case "off", "never":
		return ""
	case "all", "always":
		p.replied = true
		return p.startID
	default:
		// "first" 或其他默认模式：仅首次回复带 reference
		if !p.replied {
			p.replied = true
			return p.startID
		}
		return ""
	}
}

// MarkSent 标记已发送一条回复（不带 reference 时调用）
func (p *ReplyReferencePlanner) MarkSent() {
	p.replied = true
}

// HasReplied 返回是否已发送过回复
func (p *ReplyReferencePlanner) HasReplied() bool {
	return p.replied
}

// DiscordReplyDeliveryPlan 回复交付计划
type DiscordReplyDeliveryPlan struct {
	DeliverTarget  string                 `json:"deliverTarget"`
	ReplyTarget    string                 `json:"replyTarget"`
	ReplyReference *ReplyReferencePlanner `json:"-"`
}

// DiscordAutoThreadContext 自动线程上下文
type DiscordAutoThreadContext struct {
	CreatedThreadID  string `json:"createdThreadId"`
	From             string `json:"from"`
	To               string `json:"to"`
	OriginatingTo    string `json:"originatingTo"`
	SessionKey       string `json:"sessionKey"`
	ParentSessionKey string `json:"parentSessionKey"`
}

// ResolveDiscordAutoThreadContext 解析自动线程上下文
// W-033 fix: 对齐 TS From/To/SessionKey 格式
// TS: From = `${channel}:channel:${threadId}`, To = `channel:${threadId}`
// TS: SessionKey = buildAgentPeerSessionKey({ agentId, channel, peer: { kind: "channel", id } })
func ResolveDiscordAutoThreadContext(agentID, channel, messageChannelID, createdThreadID string) *DiscordAutoThreadContext {
	createdThreadID = strings.TrimSpace(createdThreadID)
	if createdThreadID == "" {
		return nil
	}
	messageChannelID = strings.TrimSpace(messageChannelID)
	if messageChannelID == "" {
		return nil
	}

	threadSessionKey := routing.BuildAgentPeerSessionKey(routing.PeerSessionKeyParams{
		AgentID:  agentID,
		Channel:  channel,
		PeerKind: "channel",
		PeerID:   createdThreadID,
	})
	parentSessionKey := routing.BuildAgentPeerSessionKey(routing.PeerSessionKeyParams{
		AgentID:  agentID,
		Channel:  channel,
		PeerKind: "channel",
		PeerID:   messageChannelID,
	})

	lowerChannel := strings.ToLower(strings.TrimSpace(channel))
	if lowerChannel == "" {
		lowerChannel = "unknown"
	}
	lowerThread := strings.ToLower(createdThreadID)

	return &DiscordAutoThreadContext{
		CreatedThreadID:  createdThreadID,
		From:             lowerChannel + ":channel:" + lowerThread,
		To:               "channel:" + lowerThread,
		OriginatingTo:    "channel:" + lowerThread,
		SessionKey:       threadSessionKey,
		ParentSessionKey: parentSessionKey,
	}
}

// ResolveDiscordReplyDeliveryPlan 解析回复交付计划
// W-034 fix: 对齐 TS — replyTarget 在创建线程时更新，引入 ReplyReferencePlanner，
// allowReference 机制确保进入新线程后不带 reply reference
func ResolveDiscordReplyDeliveryPlan(replyTarget, replyToMode, messageID string, threadChannel *DiscordThreadChannel, createdThreadID string) DiscordReplyDeliveryPlan {
	originalReplyTarget := replyTarget
	deliverTarget := originalReplyTarget
	rt := originalReplyTarget

	// TS line 334-337: 创建了新线程时，deliverTarget 和 replyTarget 都指向新线程
	if createdThreadID != "" {
		deliverTarget = "channel:" + createdThreadID
		rt = deliverTarget
	}

	// TS line 338: allowReference = deliverTarget === originalReplyTarget
	// 如果 deliverTarget 被改成了新线程，说明我们在新线程内发送，不应带 reply reference
	allowReference := deliverTarget == originalReplyTarget

	// TS line 339-344: 构建 ReplyReferencePlanner
	// existingId: 如果当前已在线程内(threadChannel != nil)，使用 messageId 作为已有引用
	// startId: 始终为 messageId
	// replyToMode: 如果 !allowReference 则强制 "off"
	effectiveMode := replyToMode
	if !allowReference {
		effectiveMode = "off"
	}

	var existingID string
	if threadChannel != nil {
		existingID = messageID
	}

	planner := NewReplyReferencePlanner(effectiveMode, existingID, messageID, allowReference)

	return DiscordReplyDeliveryPlan{
		DeliverTarget:  deliverTarget,
		ReplyTarget:    rt,
		ReplyReference: planner,
	}
}
