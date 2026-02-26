package whatsapp

import (
	"sync"
	"time"
)

// WhatsApp 入站消息处理 — 继承自 src/web/inbound/ (7 文件)
// 提取类型定义、去重、文本提取等核心逻辑

// ── 类型定义 (types.ts) ──

// WebListenerCloseReason 监听器关闭原因
type WebListenerCloseReason struct {
	Status      int    `json:"status,omitempty"`
	IsLoggedOut bool   `json:"isLoggedOut"`
	Error       string `json:"error,omitempty"`
}

// NormalizedLocation 规范化的地理位置
type NormalizedLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
	URL       string  `json:"url,omitempty"`
}

// WebInboundMessage 入站消息结构
type WebInboundMessage struct {
	ID                string              `json:"id,omitempty"`
	From              string              `json:"from"`
	ConversationID    string              `json:"conversationId"`
	To                string              `json:"to"`
	AccountID         string              `json:"accountId"`
	Body              string              `json:"body"`
	PushName          string              `json:"pushName,omitempty"`
	Timestamp         int64               `json:"timestamp,omitempty"`
	ChatType          string              `json:"chatType"` // "direct"|"group"
	ChatID            string              `json:"chatId"`
	SenderJid         string              `json:"senderJid,omitempty"`
	SenderE164        string              `json:"senderE164,omitempty"`
	SenderName        string              `json:"senderName,omitempty"`
	ReplyToID         string              `json:"replyToId,omitempty"`
	ReplyToBody       string              `json:"replyToBody,omitempty"`
	ReplyToSender     string              `json:"replyToSender,omitempty"`
	ReplyToSenderJid  string              `json:"replyToSenderJid,omitempty"`
	ReplyToSenderE164 string              `json:"replyToSenderE164,omitempty"`
	GroupSubject      string              `json:"groupSubject,omitempty"`
	GroupParticipants []string            `json:"groupParticipants,omitempty"`
	MentionedJids     []string            `json:"mentionedJids,omitempty"`
	SelfJid           string              `json:"selfJid,omitempty"`
	SelfE164          string              `json:"selfE164,omitempty"`
	Location          *NormalizedLocation `json:"location,omitempty"`
	MediaPath         string              `json:"mediaPath,omitempty"`
	MediaType         string              `json:"mediaType,omitempty"`
	MediaURL          string              `json:"mediaUrl,omitempty"`
	WasMentioned      bool                `json:"wasMentioned,omitempty"`
}

// ── 去重 (dedupe.ts) ──

const (
	recentWebMessageTTLMs   = 20 * 60 * 1000 // 20 分钟
	recentWebMessageMaxSize = 5000
)

// dedupeEntry 去重缓存条目
type dedupeEntry struct {
	key       string
	createdAt time.Time
}

// dedupeCache 消息去重缓存
type dedupeCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	order   []dedupeEntry
	ttl     time.Duration
	maxSize int
}

func newDedupeCache(ttl time.Duration, maxSize int) *dedupeCache {
	return &dedupeCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Check 检查是否已见过此消息（如未见过则标记为已见）
func (c *dedupeCache) Check(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evict()

	if _, ok := c.entries[key]; ok {
		return true
	}

	now := time.Now()
	c.entries[key] = now
	c.order = append(c.order, dedupeEntry{key: key, createdAt: now})
	return false
}

// Clear 清空去重缓存
func (c *dedupeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]time.Time)
	c.order = nil
}

// evict 驱逐过期和超容量条目（需要在持锁状态下调用）
func (c *dedupeCache) evict() {
	now := time.Now()
	// 过期驱逐
	for len(c.order) > 0 {
		oldest := c.order[0]
		if now.Sub(oldest.createdAt) < c.ttl {
			break
		}
		delete(c.entries, oldest.key)
		c.order = c.order[1:]
	}
	// 容量驱逐
	for len(c.order) > c.maxSize {
		oldest := c.order[0]
		delete(c.entries, oldest.key)
		c.order = c.order[1:]
	}
}

var recentInboundMessages = newDedupeCache(
	time.Duration(recentWebMessageTTLMs)*time.Millisecond,
	recentWebMessageMaxSize,
)

// ResetWebInboundDedupe 重置入站消息去重缓存
func ResetWebInboundDedupe() {
	recentInboundMessages.Clear()
}

// IsRecentInboundMessage 检查是否为近期已处理的入站消息
func IsRecentInboundMessage(key string) bool {
	return recentInboundMessages.Check(key)
}

// ── 文本提取辅助 (extract.ts 部分逻辑，Baileys proto 特定逻辑延迟) ──

// FormatLocationText 格式化位置信息文本
func FormatLocationText(loc *NormalizedLocation) string {
	if loc == nil {
		return ""
	}
	parts := []string{}
	if loc.Name != "" {
		parts = append(parts, loc.Name)
	}
	if loc.Address != "" {
		parts = append(parts, loc.Address)
	}
	if loc.URL != "" {
		parts = append(parts, loc.URL)
	}
	if len(parts) == 0 {
		return ""
	}
	return "[Location] " + joinNonEmpty(parts, " • ")
}

func joinNonEmpty(parts []string, sep string) string {
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	result := ""
	for i, p := range filtered {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

// FormatContactPlaceholder 格式化联系人占位符
func FormatContactPlaceholder(name string, phones []string) string {
	if name == "" && len(phones) == 0 {
		return "[Contact]"
	}
	label := name
	if label == "" && len(phones) > 0 {
		label = phones[0]
	}
	if len(phones) > 0 {
		return "[Contact: " + label + " " + phones[0] + "]"
	}
	return "[Contact: " + label + "]"
}
