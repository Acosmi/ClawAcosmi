package discord

import (
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Discord 在线状态缓存 — 继承自 src/discord/monitor/presence-cache.ts (61L)
// Phase 9 实现：per-account maps + LRU 单条淘汰，与 TS 行为完全对齐。
//
// W-052 fix: eviction strategy aligned with TS — evicts the single oldest entry
// (first inserted via Map iteration order) instead of ~50% random entries.
//
// W-053 fix: cache is now per-account (keyed by accountID) instead of global,
// matching the TS per-account Map<string, Map<string, GatewayPresenceUpdate>> structure.

// presenceCacheMaxPerAccount is the maximum number of entries per account cache.
// When exceeded during an Update, the single oldest entry is evicted (LRU).
// TS ref: MAX_PRESENCE_PER_ACCOUNT = 5000
// W-056: confirmed aligned with TS value of 5000.
const presenceCacheMaxPerAccount = 5000

// PresenceData holds the full presence information for a user, aligning with
// the TS GatewayPresenceUpdate type. The Go cache previously stored only the
// status string; this struct extends it to match TS data granularity.
// W-054 fix: expanded from string-only to full presence data.
// TS ref: GatewayPresenceUpdate from discord-api-types/v10
type PresenceData struct {
	Status       string                  // "online", "idle", "dnd", "offline"
	Activities   []*discordgo.Activity   // user's current activities
	ClientStatus *discordgo.ClientStatus // per-platform (desktop/mobile/web) status
}

// presenceAccountCache is a per-account presence cache using an ordered map
// to support LRU eviction of the oldest entry.
type presenceAccountCache struct {
	entries map[string]*presenceEntry
	head    *presenceEntry // oldest (front of the doubly-linked list)
	tail    *presenceEntry // newest (back of the doubly-linked list)
}

// presenceEntry is a node in the doubly-linked list used for LRU ordering.
// W-054 fix: stores full PresenceData instead of just status string.
type presenceEntry struct {
	key  string // userID
	data PresenceData
	prev *presenceEntry
	next *presenceEntry
}

func newPresenceAccountCache() *presenceAccountCache {
	return &presenceAccountCache{
		entries: make(map[string]*presenceEntry),
	}
}

// set inserts or updates a userID → PresenceData mapping. On update, moves to tail (most recent).
// If over capacity, evicts the oldest single entry (head).
func (c *presenceAccountCache) set(userID string, data PresenceData) {
	if e, ok := c.entries[userID]; ok {
		// Update existing: update data and move to tail
		e.data = data
		c.moveToTail(e)
		return
	}
	// New entry
	e := &presenceEntry{key: userID, data: data}
	c.entries[userID] = e
	c.appendToTail(e)

	// Evict oldest if over capacity — TS evicts 1 oldest entry
	if len(c.entries) > presenceCacheMaxPerAccount {
		c.evictOldest()
	}
}

// get returns the PresenceData for a userID, or nil if not found.
func (c *presenceAccountCache) get(userID string) *PresenceData {
	if e, ok := c.entries[userID]; ok {
		return &e.data
	}
	return nil
}

// del removes a userID from the cache.
func (c *presenceAccountCache) del(userID string) {
	if e, ok := c.entries[userID]; ok {
		c.removeNode(e)
		delete(c.entries, userID)
	}
}

// size returns the number of entries in this account cache.
func (c *presenceAccountCache) size() int {
	return len(c.entries)
}

// evictOldest removes the head (oldest) entry.
func (c *presenceAccountCache) evictOldest() {
	if c.head == nil {
		return
	}
	oldest := c.head
	c.removeNode(oldest)
	delete(c.entries, oldest.key)
}

// appendToTail adds a node to the end of the linked list.
func (c *presenceAccountCache) appendToTail(e *presenceEntry) {
	e.prev = c.tail
	e.next = nil
	if c.tail != nil {
		c.tail.next = e
	}
	c.tail = e
	if c.head == nil {
		c.head = e
	}
}

// removeNode removes a node from the linked list.
func (c *presenceAccountCache) removeNode(e *presenceEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

// moveToTail moves an existing node to the tail (most recent).
func (c *presenceAccountCache) moveToTail(e *presenceEntry) {
	if c.tail == e {
		return // already at tail
	}
	c.removeNode(e)
	c.appendToTail(e)
}

// DiscordPresenceCache is a per-account presence cache.
// Each account has its own cache keyed by userID, matching the TS structure:
//
//	const presenceCache = new Map<string, Map<string, GatewayPresenceUpdate>>();
type DiscordPresenceCache struct {
	mu       sync.Mutex
	accounts map[string]*presenceAccountCache // accountID → per-account cache
}

// NewDiscordPresenceCache 创建新的在线状态缓存。
func NewDiscordPresenceCache() *DiscordPresenceCache {
	return &DiscordPresenceCache{
		accounts: make(map[string]*presenceAccountCache),
	}
}

// resolveAccountKey returns a normalized account key, using "default" when empty.
// TS ref: resolveAccountKey (presence-cache.ts L11-13)
func resolveAccountKey(accountID string) string {
	if accountID == "" {
		return "default"
	}
	return accountID
}

// Update 更新用户在线状态 (per-account)。
// W-054 fix: accepts full PresenceData instead of just status string.
// TS ref: setPresence (presence-cache.ts L16-35)
func (c *DiscordPresenceCache) Update(accountID, userID string, data PresenceData) {
	if userID == "" {
		return
	}
	key := resolveAccountKey(accountID)

	c.mu.Lock()
	defer c.mu.Unlock()

	ac, ok := c.accounts[key]
	if !ok {
		ac = newPresenceAccountCache()
		c.accounts[key] = ac
	}
	ac.set(userID, data)
}

// Get 获取用户完整在线状态 (per-account)。
// W-054 fix: returns *PresenceData (nil if not found) instead of string.
// TS ref: getPresence (presence-cache.ts L38-43)
func (c *DiscordPresenceCache) Get(accountID, userID string) *PresenceData {
	key := resolveAccountKey(accountID)

	c.mu.Lock()
	defer c.mu.Unlock()

	ac, ok := c.accounts[key]
	if !ok {
		return nil
	}
	return ac.get(userID)
}

// GetStatus 获取用户在线状态字符串 (per-account)。
// Convenience wrapper that returns the status string, or "" if not found.
func (c *DiscordPresenceCache) GetStatus(accountID, userID string) string {
	data := c.Get(accountID, userID)
	if data == nil {
		return ""
	}
	return data.Status
}

// IsOnline 检查用户是否在线 (per-account)。
func (c *DiscordPresenceCache) IsOnline(accountID, userID string) bool {
	status := c.GetStatus(accountID, userID)
	return status == "online" || status == "idle" || status == "dnd"
}

// Delete 移除用户状态 (per-account)。
func (c *DiscordPresenceCache) Delete(accountID, userID string) {
	key := resolveAccountKey(accountID)

	c.mu.Lock()
	defer c.mu.Unlock()

	ac, ok := c.accounts[key]
	if !ok {
		return
	}
	ac.del(userID)
}

// Clear clears cached presence data.
// If accountID is non-empty, clears only that account's cache.
// If empty, clears all caches.
// TS ref: clearPresences (presence-cache.ts L46-52)
func (c *DiscordPresenceCache) Clear(accountID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if accountID != "" {
		delete(c.accounts, resolveAccountKey(accountID))
		return
	}
	c.accounts = make(map[string]*presenceAccountCache)
}

// Len returns the total number of entries across all account caches.
// TS ref: presenceCacheSize (presence-cache.ts L55-61)
func (c *DiscordPresenceCache) Len() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	var total int64
	for _, ac := range c.accounts {
		total += int64(ac.size())
	}
	return total
}
