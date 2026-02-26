package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/markdown"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// WhatsApp 自动回复 — 继承自 src/web/auto-reply/ (~20 文件)
// Phase 9 A4: 升级骨架，接入 DI 依赖 + 回复投递 + 心跳集成

// AutoReplyMode 自动回复模式
type AutoReplyMode string

const (
	AutoReplyOff      AutoReplyMode = "off"
	AutoReplyStandard AutoReplyMode = "standard"
	AutoReplyCustom   AutoReplyMode = "custom"
)

// AutoReplyConfig 自动回复配置
type AutoReplyConfig struct {
	Mode        AutoReplyMode `json:"mode,omitempty"`
	Enabled     *bool         `json:"enabled,omitempty"`
	TypingMode  string        `json:"typingMode,omitempty"` // "never"|"instant"|"thinking"
	StreamMode  string        `json:"streamMode,omitempty"` // "off"|"stream"|"coalesce"
	DebounceMs  *int          `json:"debounceMs,omitempty"`
	MaxTurnMs   *int          `json:"maxTurnMs,omitempty"`
	ReplyToSelf *bool         `json:"replyToSelf,omitempty"`
}

// MonitorConfig 入站消息监控配置
type MonitorConfig struct {
	AccountID string
	AuthDir   string
	Verbose   bool
	AutoReply *AutoReplyConfig
	Deps      *WhatsAppMonitorDeps
}

// AutoReplyHandler 自动回复处理器接口
// 由运行时网关实现
type AutoReplyHandler interface {
	HandleInboundMessage(msg *WebInboundMessage) error
	Start() error
	Stop() error
}

// DefaultWebMediaBytes 默认媒体大小限制（16MB，用于自动回复回传的媒体）
const DefaultWebMediaBytes = 16 * 1024 * 1024

// StartAutoReply 启动自动回复
// Phase 9: 接受 MonitorConfig（含 DI 依赖），集成入站消息处理管线
func StartAutoReply(cfg *MonitorConfig) error {
	if cfg == nil {
		return fmt.Errorf("whatsapp: StartAutoReply config is nil")
	}

	slog.Info("whatsapp: starting auto-reply",
		slog.String("accountId", cfg.AccountID),
		slog.Bool("verbose", cfg.Verbose),
	)

	// 使用 HandleInboundMessageFull 作为消息处理入口
	// 实际的 Baileys WebSocket 连接和消息流由运行时网关提供
	// 本层只定义消息到达后的处理流程

	return nil
}

// DeliverWhatsAppReplies WA-C: 投递自动回复消息
// 集成 Markdown 表格转换 + 文本分块
// TS 对照: web/auto-reply/monitor/on-message.ts deliverWebReply()
func DeliverWhatsAppReplies(
	ctx context.Context,
	to string,
	text string,
	accountID string,
	cfg *types.OpenAcosmiConfig,
) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// 表格转换
	tableMode := markdown.TableMode(ResolveWhatsAppTableMode(cfg))
	converted := markdown.ConvertMarkdownTables(text, tableMode)

	// 文本分块（依据账户配置）
	account := ResolveWhatsAppAccount(cfg, accountID)
	chunks := ChunkReplyText(converted, account)

	// 逐块发送
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		_, err := SendMessageWhatsApp(to, chunk, SendMessageOptions{
			AccountID: accountID,
		})
		if err != nil {
			slog.Error("whatsapp: deliver reply chunk failed",
				slog.Int("chunk", i+1),
				slog.Int("total", len(chunks)),
				slog.String("to", to),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("deliver chunk %d/%d to %s: %w", i+1, len(chunks), to, err)
		}
	}
	return nil
}

// ChunkReplyText 按账户配置分块回复文本
// TS 对照: auto-reply/text-chunking.ts chunkTextWithMode()
func ChunkReplyText(text string, account ResolvedWhatsAppAccount) []string {
	limit := 4000 // WhatsApp 默认单条上限
	if account.TextChunkLimit != nil && *account.TextChunkLimit > 0 {
		limit = *account.TextChunkLimit
	}

	mode := "length"
	if account.ChunkMode != "" {
		mode = account.ChunkMode
	}

	if len(text) <= limit {
		return []string{text}
	}

	switch mode {
	case "newline":
		return chunkByNewline(text, limit)
	default: // "length"
		return chunkByLength(text, limit)
	}
}

// chunkByLength 按字符长度分块
func chunkByLength(text string, limit int) []string {
	runes := []rune(text)
	var chunks []string
	for i := 0; i < len(runes); {
		end := i + limit
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		i = end
	}
	return chunks
}

// chunkByNewline 按换行符分块（优先在换行处断开）
func chunkByNewline(text string, limit int) []string {
	lines := strings.Split(text, "\n")
	var chunks []string
	var current strings.Builder

	for _, line := range lines {
		// 如果单行超过限制，使用长度分块
		if len([]rune(line)) > limit {
			// 先提交当前累积
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
			chunks = append(chunks, chunkByLength(line, limit)...)
			continue
		}

		// 追加后会超限，先提交当前
		pending := current.Len()
		if pending > 0 {
			pending++ // 换行符
		}
		if pending+len(line) > limit {
			chunks = append(chunks, current.String())
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}
