package discord

// Discord 回复上下文跟踪 — 继承自 src/discord/monitor/reply-context.ts (45L)
// W-028: 补全 resolveReplyContext 核心回复链解析逻辑
// W-029: 补全 buildDirectLabel / buildGuildLabel 标签构造函数

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// DiscordReplyCtx 回复上下文
type DiscordReplyCtx struct {
	ChannelID string
	MessageID string
	ThreadID  string
	GuildID   string
}

// DiscordReplyContextMap 回复上下文映射（channelID → 最近回复目标）
type DiscordReplyContextMap struct {
	m sync.Map
}

// NewDiscordReplyContextMap 创建新的回复上下文映射。
func NewDiscordReplyContextMap() *DiscordReplyContextMap {
	return &DiscordReplyContextMap{}
}

// Set 设置频道的回复上下文。
func (r *DiscordReplyContextMap) Set(channelID string, ctx *DiscordReplyCtx) {
	r.m.Store(channelID, ctx)
}

// Get 获取频道的回复上下文。
func (r *DiscordReplyContextMap) Get(channelID string) *DiscordReplyCtx {
	if v, ok := r.m.Load(channelID); ok {
		return v.(*DiscordReplyCtx)
	}
	return nil
}

// Delete 移除频道的回复上下文。
func (r *DiscordReplyContextMap) Delete(channelID string) {
	r.m.Delete(channelID)
}

// ---------------------------------------------------------------------------
// ResolveReplyContext — 核心回复链解析
// TS ref: resolveReplyContext (reply-context.ts L6-34)
//
// 接收一个 discordgo 消息, 解析其 ReferencedMessage, 提取被引用消息的文本,
// 解析 sender 身份, 构建 envelope 格式化输出。
// 如果没有被引用消息或文本为空, 返回空字符串。
// ---------------------------------------------------------------------------

// ReplyContextOptions 回复上下文解析选项。
// TS ref: options?: { envelope?: EnvelopeFormatOptions }
type ReplyContextOptions struct {
	// IncludeTimestamp 是否在 envelope 头中包含时间戳 (default: true)
	IncludeTimestamp bool
}

// DefaultReplyContextOptions 返回默认的回复上下文选项。
func DefaultReplyContextOptions() ReplyContextOptions {
	return ReplyContextOptions{
		IncludeTimestamp: true,
	}
}

// ResolveReplyContext 解析 Discord 消息的回复链上下文。
// 返回格式化后的 envelope 字符串, 或空字符串（无回复 / 被引用消息已删除）。
//
// TS ref: resolveReplyContext (reply-context.ts L6-34)
//   - 从 message.referencedMessage 获取被引用消息
//   - 解析 sender 身份（调用 resolveDiscordSenderIdentity）
//   - 构建 fromLabel 和 body（含 discord message id / channel / from / user id 元数据）
//   - 使用 formatAgentEnvelope 格式化输出
func ResolveReplyContext(msg *discordgo.Message, opts *ReplyContextOptions) string {
	if msg == nil {
		return ""
	}

	referenced := msg.ReferencedMessage
	if referenced == nil || referenced.Author == nil {
		return ""
	}

	// 解析被引用消息的文本（含转发内容）
	// TS ref: resolveDiscordMessageText(referenced, { includeForwarded: true })
	referencedText := ResolveReferencedMessageText(referenced)
	if referencedText == "" {
		return ""
	}

	// 解析 sender 身份
	// TS ref: resolveDiscordSenderIdentity({ author: referenced.author, pluralkitInfo: null })
	author := DiscordAuthorInfo{
		ID:            referenced.Author.ID,
		Username:      referenced.Author.Username,
		GlobalName:    referenced.Author.GlobalName,
		Discriminator: referenced.Author.Discriminator,
	}
	sender := ResolveDiscordSenderIdentity(author, nil, nil)

	// 构建 fromLabel
	// TS ref: const fromLabel = referenced.author ? buildDirectLabel(referenced.author, sender.tag) : "Unknown"
	fromLabel := "Unknown"
	if referenced.Author != nil {
		fromLabel = BuildDirectLabel(referenced.Author.ID, sender.Tag)
	}

	// 构建 body（含 discord 元数据标签）
	// TS ref: `${referencedText}\n[discord message id: ${referenced.id} channel: ${referenced.channelId} from: ${sender.tag ?? sender.label} user id:${sender.id}]`
	senderIdentifier := sender.Tag
	if senderIdentifier == "" {
		senderIdentifier = sender.Label
	}
	body := fmt.Sprintf("%s\n[discord message id: %s channel: %s from: %s user id:%s]",
		referencedText,
		referenced.ID,
		referenced.ChannelID,
		senderIdentifier,
		sender.ID,
	)

	// 构建 envelope
	// TS ref: formatAgentEnvelope({ channel: "Discord", from: fromLabel, timestamp: resolveTimestampMs(referenced.timestamp), body, envelope: options?.envelope })
	effectiveOpts := DefaultReplyContextOptions()
	if opts != nil {
		effectiveOpts = *opts
	}

	var timestamp int64
	if !referenced.Timestamp.IsZero() {
		timestamp = referenced.Timestamp.UnixMilli()
	}

	return FormatReplyEnvelope(FormatReplyEnvelopeParams{
		Channel:          "Discord",
		From:             fromLabel,
		Timestamp:        timestamp,
		Body:             body,
		IncludeTimestamp: effectiveOpts.IncludeTimestamp,
	})
}

// ResolveReplyContextFromCreate 是 ResolveReplyContext 的便捷包装,
// 接受 *discordgo.MessageCreate。
func ResolveReplyContextFromCreate(msg *discordgo.MessageCreate, opts *ReplyContextOptions) string {
	if msg == nil {
		return ""
	}
	return ResolveReplyContext(msg.Message, opts)
}

// ---------------------------------------------------------------------------
// ResolveReferencedMessageText 解析被引用消息的文本内容。
// 这是 TS resolveDiscordMessageText(referenced, { includeForwarded: true }) 的等价物。
// ---------------------------------------------------------------------------

// ResolveReferencedMessageText 从被引用消息中提取文本。
// 依次尝试: content, embed descriptions, attachment placeholders。
// TS ref: resolveDiscordMessageText (message-utils.ts L169-190)
func ResolveReferencedMessageText(msg *discordgo.Message) string {
	if msg == nil {
		return ""
	}

	// 1. 主文本
	text := strings.TrimSpace(msg.Content)

	// 2. 如果主文本为空, 尝试附件占位符
	if text == "" && len(msg.Attachments) > 0 {
		var placeholders []string
		for _, a := range msg.Attachments {
			placeholders = append(placeholders, InferAttachmentPlaceholder(a.Filename, a.ContentType))
		}
		text = strings.Join(placeholders, "\n")
	}

	// 3. 如果仍为空, 尝试 embed description
	if text == "" && len(msg.Embeds) > 0 {
		for _, embed := range msg.Embeds {
			if embed.Description != "" {
				text = embed.Description
				break
			}
		}
	}

	// 4. 包含转发消息内容 (includeForwarded: true)
	// discordgo 的 MessageSnapshot 处理
	forwardedText := resolveForwardedMessageText(msg)
	if forwardedText != "" {
		if text == "" {
			return forwardedText
		}
		return text + "\n" + forwardedText
	}

	return text
}

// resolveForwardedMessageText 解析转发消息的文本。
// TS ref: resolveDiscordForwardedMessagesText (message-utils.ts L192-222)
func resolveForwardedMessageText(msg *discordgo.Message) string {
	if msg == nil || len(msg.MessageSnapshots) == 0 {
		return ""
	}

	var blocks []string
	for _, snapshot := range msg.MessageSnapshots {
		if snapshot.Message == nil {
			continue
		}
		snapMsg := snapshot.Message
		text := strings.TrimSpace(snapMsg.Content)
		if text == "" && len(snapMsg.Embeds) > 0 {
			for _, embed := range snapMsg.Embeds {
				if embed.Description != "" {
					text = embed.Description
					break
				}
			}
		}
		if text == "" {
			continue
		}

		// 格式化转发作者
		authorLabel := ""
		if snapMsg.Author != nil {
			authorLabel = snapMsg.Author.GlobalName
			if authorLabel == "" {
				authorLabel = snapMsg.Author.Username
			}
		}
		if authorLabel != "" {
			blocks = append(blocks, fmt.Sprintf("[Forwarded message from %s]\n%s", authorLabel, text))
		} else {
			blocks = append(blocks, fmt.Sprintf("[Forwarded message]\n%s", text))
		}
	}

	return strings.Join(blocks, "\n")
}

// ---------------------------------------------------------------------------
// BuildDirectLabel / BuildGuildLabel — 标签构造函数
// TS ref: reply-context.ts L36-45 (W-029)
// ---------------------------------------------------------------------------

// BuildDirectLabel 构建直接消息标签。
// 格式: "<username> user id:<authorID>"
// TS ref: buildDirectLabel (reply-context.ts L36-40)
//
//	const username = tagOverride?.trim() || resolveDiscordSenderIdentity({ author, pluralkitInfo: null }).tag;
//	return `${username ?? "unknown"} user id:${author.id}`;
func BuildDirectLabel(authorID string, tagOverride string) string {
	username := strings.TrimSpace(tagOverride)
	if username == "" {
		username = "unknown"
	}
	return fmt.Sprintf("%s user id:%s", username, authorID)
}

// BuildGuildLabel 构建 Guild 频道标签。
// 格式: "<guildName> #<channelName> channel id:<channelID>"
// TS ref: buildGuildLabel (reply-context.ts L42-45)
//
//	return `${guild?.name ?? "Guild"} #${channelName} channel id:${channelId}`;
func BuildGuildLabel(guildName, channelName, channelID string) string {
	name := strings.TrimSpace(guildName)
	if name == "" {
		name = "Guild"
	}
	return fmt.Sprintf("%s #%s channel id:%s", name, channelName, channelID)
}

// ---------------------------------------------------------------------------
// FormatReplyEnvelope — 回复上下文信封格式化
// TS ref: formatAgentEnvelope (auto-reply/envelope.ts L119-155)
//
// 生成格式: [Channel from timestamp] body
// ---------------------------------------------------------------------------

// FormatReplyEnvelopeParams 回复信封格式化参数。
type FormatReplyEnvelopeParams struct {
	Channel          string
	From             string
	Timestamp        int64 // milliseconds since epoch; 0 = omit
	Body             string
	IncludeTimestamp bool
}

// FormatReplyEnvelope 格式化回复上下文信封。
// 生成 "[Discord from timestamp] body" 格式。
// TS ref: formatAgentEnvelope (auto-reply/envelope.ts L119-155)
func FormatReplyEnvelope(p FormatReplyEnvelopeParams) string {
	channel := strings.TrimSpace(p.Channel)
	if channel == "" {
		channel = "Channel"
	}

	parts := []string{channel}

	if from := strings.TrimSpace(p.From); from != "" {
		parts = append(parts, from)
	}

	if p.IncludeTimestamp && p.Timestamp > 0 {
		t := time.UnixMilli(p.Timestamp)
		parts = append(parts, t.UTC().Format("2006-01-02 15:04 UTC"))
	}

	header := "[" + strings.Join(parts, " ") + "]"
	return header + " " + p.Body
}
