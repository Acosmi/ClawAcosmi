package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/pkg/markdown"
	"github.com/Acosmi/ClawAcosmi/pkg/polls"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
	"github.com/Acosmi/ClawAcosmi/pkg/utils"
)

// WhatsApp 出站消息 — 继承自 src/web/outbound.ts (177L)

// SendMessageOptions 发送消息选项
type SendMessageOptions struct {
	Verbose     bool
	MediaURL    string
	GifPlayback bool
	AccountID   string
}

// SendMessageResult 发送结果
type SendMessageResult struct {
	MessageID string
	ToJid     string
}

// ToWhatsAppJid 将 E.164 或目标转换为 WhatsApp JID
func ToWhatsAppJid(target string) string {
	trimmed := strings.TrimSpace(target)
	// 已经是 JID 格式
	if strings.Contains(trimmed, "@") {
		return trimmed
	}
	// 纯电话号码 → JID
	phone := utils.NormalizeE164(trimmed)
	if phone == "" {
		return trimmed
	}
	// 去掉 + 号
	digits := strings.TrimPrefix(phone, "+")
	return digits + "@s.whatsapp.net"
}

// SendMessageWhatsApp 通过活跃监听器发送消息
func SendMessageWhatsApp(to, body string, opts SendMessageOptions) (*SendMessageResult, error) {
	accountID, listener, err := RequireActiveWebListener(opts.AccountID)
	if err != nil {
		return nil, err
	}

	// WA-E: Markdown 表格转换
	tableMode := markdown.TableMode(ResolveWhatsAppTableMode(nil))
	text := markdown.ConvertMarkdownTables(body, tableMode)
	jid := ToWhatsAppJid(to)

	var mediaBuffer []byte
	var mediaType string
	if opts.MediaURL != "" {
		media, mediaErr := LoadWebMedia(opts.MediaURL)
		if mediaErr != nil {
			return nil, fmt.Errorf("failed to load media: %w", mediaErr)
		}
		mediaBuffer = media.Buffer
		mediaType = media.ContentType
		if media.Kind == "audio" && media.ContentType == "audio/ogg" {
			mediaType = "audio/ogg; codecs=opus"
		}
	}

	// 发送打字指示
	_ = listener.SendComposingTo(to)

	sendOpts := &ActiveWebSendOptions{
		GifPlayback: opts.GifPlayback,
	}
	if opts.AccountID != "" {
		sendOpts.AccountID = accountID
	}

	messageID, err := listener.SendMessage(to, text, mediaBuffer, mediaType, sendOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to send message to %s: %w", jid, err)
	}

	return &SendMessageResult{
		MessageID: messageID,
		ToJid:     jid,
	}, nil
}

// SendReactionOptions 发送反应选项
type SendReactionOptions struct {
	Verbose     bool
	FromMe      bool
	Participant string
	AccountID   string
}

// SendReactionWhatsApp 通过活跃监听器发送反应
func SendReactionWhatsApp(chatJid, messageID, emoji string, opts SendReactionOptions) error {
	_, listener, err := RequireActiveWebListener(opts.AccountID)
	if err != nil {
		return err
	}

	jid := ToWhatsAppJid(chatJid)
	_ = jid // 用于日志

	return listener.SendReaction(chatJid, messageID, emoji, opts.FromMe, opts.Participant)
}

// SendPollOptions 发送投票选项
type SendPollOptions struct {
	Verbose   bool
	AccountID string
}

// NormalizePollInput 规范化投票输入。
// 使用 pkg/polls 共享验证 + WhatsApp 平台特有默认值。
func NormalizePollInput(poll PollInput, maxOptions int) PollInput {
	// WhatsApp 特有：空问题默认为 "Poll"（共享层会报错）
	question := strings.TrimSpace(poll.Question)
	if question == "" {
		question = "Poll"
	}

	// 预处理：trim + 过滤空值
	var options []string
	for _, opt := range poll.Options {
		trimmed := strings.TrimSpace(opt)
		if trimmed != "" {
			options = append(options, trimmed)
		}
	}

	// WhatsApp 特有：选项不足时自动填充（共享层会报错）
	for len(options) < 2 {
		options = append(options, fmt.Sprintf("Option %d", len(options)+1))
	}

	// 调用共享验证层（此时 question 非空、options >= 2，不会报错）
	normalized, err := polls.NormalizePollInput(polls.PollInput{
		Question: question,
		Options:  options,
	}, maxOptions)
	if err != nil {
		// 共享验证失败时回退到预处理结果（仅可能在 maxOptions 超限时触发）
		if maxOptions > 0 && len(options) > maxOptions {
			options = options[:maxOptions]
		}
		return PollInput{Question: question, Options: options}
	}

	return PollInput{Question: normalized.Question, Options: normalized.Options}
}

// SendPollWhatsApp 通过活跃监听器发送投票
func SendPollWhatsApp(to string, poll PollInput, opts SendPollOptions) (*SendMessageResult, error) {
	_, listener, err := RequireActiveWebListener(opts.AccountID)
	if err != nil {
		return nil, err
	}

	jid := ToWhatsAppJid(to)
	normalized := NormalizePollInput(poll, 12) // WhatsApp 最多 12 个选项

	messageID, err := listener.SendPoll(to, normalized)
	if err != nil {
		return nil, fmt.Errorf("failed to send poll to %s: %w", jid, err)
	}

	return &SendMessageResult{
		MessageID: messageID,
		ToJid:     jid,
	}, nil
}

// sendOutboundTimestamp 生成出站日志时间戳（内部辅助）
func sendOutboundTimestamp() int64 {
	return time.Now().UnixMilli()
}

// ResolveWhatsAppTableMode 从 config 解析 WhatsApp 表格模式。
// TS 对照: config/markdown-tables.ts resolveMarkdownTableMode({cfg, channel: "whatsapp"})
// WhatsApp 默认使用 bullets，因为原生不支持 Markdown 表格。
func ResolveWhatsAppTableMode(cfg *types.OpenAcosmiConfig) types.MarkdownTableMode {
	if cfg != nil && cfg.Markdown != nil && cfg.Markdown.Tables != "" {
		return cfg.Markdown.Tables
	}
	return types.MarkdownTableBullets
}
