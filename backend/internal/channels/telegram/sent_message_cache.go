package telegram

import (
	"fmt"
	"sync"
	"time"
)

const sentMessageTTL = 24 * time.Hour

type sentCacheEntry struct {
	messageIDs map[int]time.Time // messageID → recordTime
}

// SentMessageCache 已发送消息 ID 的内存缓存。
// 用于标识 Bot 自身发送的消息（反应过滤 "own" 模式）。
// 继承自 sent-message-cache.ts。
type SentMessageCache struct {
	mu      sync.RWMutex
	entries map[string]*sentCacheEntry // chatKey → entry
}

// NewSentMessageCache 创建新的已发送消息缓存。
func NewSentMessageCache() *SentMessageCache {
	return &SentMessageCache{
		entries: make(map[string]*sentCacheEntry),
	}
}

func chatKey(chatID interface{}) string {
	return fmt.Sprintf("%v", chatID)
}

func (c *SentMessageCache) cleanupExpired(entry *sentCacheEntry) {
	now := time.Now()
	for msgID, ts := range entry.messageIDs {
		if now.Sub(ts) > sentMessageTTL {
			delete(entry.messageIDs, msgID)
		}
	}
}

// RecordSentMessage 记录 Bot 发送的消息 ID。
func (c *SentMessageCache) RecordSentMessage(chatID interface{}, messageID int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := chatKey(chatID)
	entry, ok := c.entries[key]
	if !ok {
		entry = &sentCacheEntry{messageIDs: make(map[int]time.Time)}
		c.entries[key] = entry
	}
	entry.messageIDs[messageID] = time.Now()
	// 周期性清理
	if len(entry.messageIDs) > 100 {
		c.cleanupExpired(entry)
	}
}

// WasSentByBot 检查消息是否为 Bot 发送。
func (c *SentMessageCache) WasSentByBot(chatID interface{}, messageID int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := chatKey(chatID)
	entry, ok := c.entries[key]
	if !ok {
		return false
	}
	c.cleanupExpired(entry)
	_, found := entry.messageIDs[messageID]
	return found
}

// Clear 清除所有缓存（用于测试）。
func (c *SentMessageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*sentCacheEntry)
}
