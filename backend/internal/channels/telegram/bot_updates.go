package telegram

import (
	"fmt"
	"sync"
	"time"
)

// Telegram 更新去重与媒体分组 — 继承自 src/telegram/bot-updates.ts (57L)

const (
	// MediaGroupTimeoutMs 媒体组聚合超时(ms)。
	// 同一 media_group_id 的消息在此超时后触发聚合处理。
	// TS 对照: MEDIA_GROUP_TIMEOUT_MS = 500
	MediaGroupTimeoutMs = 500

	// recentUpdateTTL 去重缓存条目 TTL。
	// TS 对照: RECENT_TELEGRAM_UPDATE_TTL_MS = 5 * 60_000
	recentUpdateTTL = 5 * time.Minute

	// recentUpdateMax 去重缓存最大条目数。
	// TS 对照: RECENT_TELEGRAM_UPDATE_MAX = 2000
	recentUpdateMax = 2000
)

// TelegramUpdateKeyContext 用于提取 update 去重 key 的上下文。
// DY-013: 扩展以支持嵌套 update 结构（对齐 TS TelegramUpdateKeyContext）。
type TelegramUpdateKeyContext struct {
	UpdateID    int
	Message     *TelegramMessage
	EditedMsg   *TelegramMessage
	CallbackID  string
	CallbackMsg *TelegramMessage

	// DY-013: 支持嵌套 update 对象（TS ctx.update?.update_id）
	NestedUpdateID *int
}

// ResolveTelegramUpdateID 从上下文中提取 update_id。
// DY-013: 完善实现，优先使用嵌套 update_id，与 TS 对齐:
//
//	ctx.update?.update_id ?? ctx.update_id
func ResolveTelegramUpdateID(ctx *TelegramUpdateKeyContext) *int {
	if ctx == nil {
		return nil
	}
	// 优先使用嵌套 update 对象中的 ID（对齐 TS ctx.update?.update_id）
	if ctx.NestedUpdateID != nil {
		return ctx.NestedUpdateID
	}
	// 回退到顶层 update ID
	if ctx.UpdateID > 0 {
		return &ctx.UpdateID
	}
	return nil
}

// BuildTelegramUpdateKey 构建更新去重 key。
// DY-013: 使用 ResolveTelegramUpdateID 统一 ID 解析。
func BuildTelegramUpdateKey(ctx *TelegramUpdateKeyContext) string {
	if ctx == nil {
		return ""
	}

	// 优先使用统一的 update ID 解析
	if updateID := ResolveTelegramUpdateID(ctx); updateID != nil {
		return fmt.Sprintf("update:%d", *updateID)
	}
	if ctx.CallbackID != "" {
		return fmt.Sprintf("callback:%s", ctx.CallbackID)
	}
	msg := ctx.Message
	if msg == nil {
		msg = ctx.EditedMsg
	}
	if msg == nil {
		msg = ctx.CallbackMsg
	}
	if msg != nil && msg.Chat.ID != 0 {
		return fmt.Sprintf("message:%d:%d", msg.Chat.ID, msg.MessageID)
	}
	return ""
}

// dedupeEntry 去重缓存条目，包含插入时间戳
type dedupeEntry struct {
	timestamp time.Time
}

// TelegramUpdateDedupe 更新去重缓存。
// DY-013: 实现完整的 TTL 过期 + maxSize 清理机制，对齐 TS createDedupeCache。
// TS 版本在每次 check() 时先执行 TTL prune，再执行 maxSize eviction。
type TelegramUpdateDedupe struct {
	mu      sync.Mutex
	entries map[string]*dedupeEntry
	// order 维护插入顺序，用于 LRU eviction（对齐 TS Map 的插入顺序）
	order []string
}

// NewTelegramUpdateDedupe 创建去重缓存
func NewTelegramUpdateDedupe() *TelegramUpdateDedupe {
	return &TelegramUpdateDedupe{
		entries: make(map[string]*dedupeEntry),
	}
}

// IsDuplicate 检查是否为重复更新。
// DY-013: 完整实现对齐 TS createDedupeCache.check():
//  1. 空 key 返回 false
//  2. 检查已有条目（未过期则 touch 并返回 true）
//  3. 插入新条目
//  4. TTL prune（删除过期条目）
//  5. maxSize eviction（删除最旧条目直到 <= maxSize）
func (d *TelegramUpdateDedupe) IsDuplicate(key string) bool {
	if key == "" {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	// 检查已有条目
	if existing, ok := d.entries[key]; ok {
		// 检查 TTL
		if now.Sub(existing.timestamp) < recentUpdateTTL {
			// 未过期：touch（更新时间戳并移至末尾）
			existing.timestamp = now
			d.touchOrder(key)
			return true
		}
		// 已过期：删除旧条目，按新条目处理
		delete(d.entries, key)
		d.removeFromOrder(key)
	}

	// 插入新条目
	d.entries[key] = &dedupeEntry{timestamp: now}
	d.order = append(d.order, key)

	// TTL prune
	d.pruneTTL(now)

	// maxSize eviction
	d.pruneMaxSize()

	return false
}

// Size 返回缓存当前条目数
func (d *TelegramUpdateDedupe) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}

// Clear 清空缓存
func (d *TelegramUpdateDedupe) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = make(map[string]*dedupeEntry)
	d.order = nil
}

// pruneTTL 删除过期条目（需持有锁）
func (d *TelegramUpdateDedupe) pruneTTL(now time.Time) {
	cutoff := now.Add(-recentUpdateTTL)
	// 遍历 order 从头开始，删除过期的
	newOrder := d.order[:0]
	for _, key := range d.order {
		entry, ok := d.entries[key]
		if !ok {
			continue
		}
		if entry.timestamp.Before(cutoff) {
			delete(d.entries, key)
		} else {
			newOrder = append(newOrder, key)
		}
	}
	d.order = newOrder
}

// pruneMaxSize 当条目数超过 maxSize 时删除最旧的（需持有锁）
func (d *TelegramUpdateDedupe) pruneMaxSize() {
	for len(d.entries) > recentUpdateMax && len(d.order) > 0 {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.entries, oldest)
	}
}

// touchOrder 将 key 移动到 order 末尾（需持有锁）
func (d *TelegramUpdateDedupe) touchOrder(key string) {
	d.removeFromOrder(key)
	d.order = append(d.order, key)
}

// removeFromOrder 从 order 中移除 key（需持有锁）
func (d *TelegramUpdateDedupe) removeFromOrder(key string) {
	for i, k := range d.order {
		if k == key {
			d.order = append(d.order[:i], d.order[i+1:]...)
			return
		}
	}
}
