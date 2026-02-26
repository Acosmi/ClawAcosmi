package pairing

// 渠道配对存储 — 对齐 src/pairing/pairing-store.ts (498L)
//
// 管理渠道（Discord/Telegram/Signal 等）的配对请求和白名单。
// 每个渠道有两个 JSON 文件：
//   - {channel}-pairing.json — 待处理配对请求
//   - {channel}-allowFrom.json — 已批准白名单

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/internal/channels"
	"github.com/anthropic/open-acosmi/internal/config"
)

// ---------- 常量 ----------

const (
	pairingCodeLength   = 8
	pairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 无 0O1I，人类友好
	pairingPendingTTLMs = 60 * 60 * 1000                     // 1 小时
	pairingPendingMax   = 3
)

// ---------- 类型 ----------

// PairingRequest 配对请求。
type PairingRequest struct {
	ID         string            `json:"id"`
	Code       string            `json:"code"`
	CreatedAt  string            `json:"createdAt"`
	LastSeenAt string            `json:"lastSeenAt"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// pairingStore 配对存储文件结构。
type pairingStore struct {
	Version  int              `json:"version"`
	Requests []PairingRequest `json:"requests"`
}

// allowFromStore 白名单存储文件结构。
type allowFromStore struct {
	Version   int      `json:"version"`
	AllowFrom []string `json:"allowFrom"`
}

// ---------- 全局锁 ----------

// Go 单进程，用 sync.Mutex 替代 TS 的 proper-lockfile。
// 按渠道分锁避免不同渠道互相阻塞。
var (
	storeMu    sync.Mutex
	storeLocks = make(map[string]*sync.Mutex)
)

func channelLock(channel string) *sync.Mutex {
	storeMu.Lock()
	defer storeMu.Unlock()
	mu, ok := storeLocks[channel]
	if !ok {
		mu = &sync.Mutex{}
		storeLocks[channel] = mu
	}
	return mu
}

// ---------- 路径解析 ----------

// safeChannelKey 文件名安全化（防路径遍历）。
// 对齐 TS safeChannelKey()。
func safeChannelKey(channel string) (string, error) {
	raw := strings.TrimSpace(strings.ToLower(channel))
	if raw == "" {
		return "", fmt.Errorf("invalid pairing channel")
	}
	re := regexp.MustCompile(`[\\/:*?"<>|]`)
	safe := re.ReplaceAllString(raw, "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	if safe == "" || safe == "_" {
		return "", fmt.Errorf("invalid pairing channel")
	}
	return safe, nil
}

func resolvePairingPath(channel string) (string, error) {
	key, err := safeChannelKey(channel)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.ResolveOAuthDir(), key+"-pairing.json"), nil
}

func resolveAllowFromPath(channel string) (string, error) {
	key, err := safeChannelKey(channel)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.ResolveOAuthDir(), key+"-allowFrom.json"), nil
}

// ---------- JSON 文件 I/O ----------

func readJSONFile(filePath string, v interface{}) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSONAtomic(filePath string, v interface{}) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	data = append(data, '\n')
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ---------- 时间与清理 ----------

func parseTimestamp(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return 0, false
		}
	}
	return t.UnixMilli(), true
}

func isExpired(entry PairingRequest, nowMs int64) bool {
	createdAt, ok := parseTimestamp(entry.CreatedAt)
	if !ok {
		return true
	}
	return nowMs-createdAt > pairingPendingTTLMs
}

// pruneExpiredRequests 移除过期请求。
func pruneExpiredRequests(reqs []PairingRequest, nowMs int64) (kept []PairingRequest, removed bool) {
	kept = make([]PairingRequest, 0, len(reqs))
	for _, req := range reqs {
		if isExpired(req, nowMs) {
			removed = true
			continue
		}
		kept = append(kept, req)
	}
	return
}

func resolveLastSeenAt(entry PairingRequest) int64 {
	if ms, ok := parseTimestamp(entry.LastSeenAt); ok {
		return ms
	}
	if ms, ok := parseTimestamp(entry.CreatedAt); ok {
		return ms
	}
	return 0
}

// pruneExcessRequests 裁剪到最大容量（保留最近访问的）。
func pruneExcessRequests(reqs []PairingRequest, maxPending int) (kept []PairingRequest, removed bool) {
	if maxPending <= 0 || len(reqs) <= maxPending {
		return reqs, false
	}
	sorted := make([]PairingRequest, len(reqs))
	copy(sorted, reqs)
	slices.SortFunc(sorted, func(a, b PairingRequest) int {
		return int(resolveLastSeenAt(a) - resolveLastSeenAt(b))
	})
	return sorted[len(sorted)-maxPending:], true
}

// ---------- 配对码生成 ----------

// randomCode 生成人类友好的随机码（8 位，无歧义字符）。
func randomCode() (string, error) {
	alphabetLen := big.NewInt(int64(len(pairingCodeAlphabet)))
	out := make([]byte, pairingCodeLength)
	for i := 0; i < pairingCodeLength; i++ {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("crypto/rand: %w", err)
		}
		out[i] = pairingCodeAlphabet[idx.Int64()]
	}
	return string(out), nil
}

// generateUniqueCode 生成不与已有码碰撞的唯一码。
func generateUniqueCode(existing map[string]struct{}) (string, error) {
	for attempt := 0; attempt < 500; attempt++ {
		code, err := randomCode()
		if err != nil {
			return "", err
		}
		if _, exists := existing[code]; !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique pairing code")
}

// ---------- 规范化 ----------

func normalizeID(value string) string {
	return strings.TrimSpace(value)
}

// normalizeAllowEntry 渠道特定的白名单条目规范化。
// 对齐 TS normalizeAllowEntry()，调用 adapter.NormalizeAllowEntry。
func normalizeAllowEntry(channel string, entry string) string {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" || trimmed == "*" {
		return ""
	}
	adapter := channels.GetPairingAdapter(channels.ChannelID(channel))
	if adapter != nil {
		normalized := adapter.NormalizeAllowEntry(trimmed)
		return strings.TrimSpace(normalized)
	}
	return trimmed
}

// ---------- 读取白名单 ----------

func loadAllowFromStore(filePath string) allowFromStore {
	var store allowFromStore
	_ = readJSONFile(filePath, &store)
	if store.Version == 0 {
		store.Version = 1
	}
	return store
}

func loadPairingStore(filePath string) pairingStore {
	var store pairingStore
	_ = readJSONFile(filePath, &store)
	if store.Version == 0 {
		store.Version = 1
	}
	return store
}

// ---------- 导出函数 ----------

// ReadChannelAllowFromStore 读取渠道白名单。
// 对齐 TS readChannelAllowFromStore()。
func ReadChannelAllowFromStore(channel string) ([]string, error) {
	filePath, err := resolveAllowFromPath(channel)
	if err != nil {
		return nil, err
	}
	store := loadAllowFromStore(filePath)
	var result []string
	for _, v := range store.AllowFrom {
		normalized := normalizeAllowEntry(channel, v)
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return result, nil
}

// AddChannelAllowFromStoreEntry 添加白名单条目（带去重）。
// 对齐 TS addChannelAllowFromStoreEntry()。
func AddChannelAllowFromStoreEntry(channel string, entry string) (changed bool, allowFrom []string, err error) {
	filePath, err := resolveAllowFromPath(channel)
	if err != nil {
		return false, nil, err
	}

	mu := channelLock(channel + ":allow")
	mu.Lock()
	defer mu.Unlock()

	store := loadAllowFromStore(filePath)
	current := make([]string, 0, len(store.AllowFrom))
	for _, v := range store.AllowFrom {
		if n := normalizeAllowEntry(channel, v); n != "" {
			current = append(current, n)
		}
	}

	normalized := normalizeAllowEntry(channel, normalizeID(entry))
	if normalized == "" {
		return false, current, nil
	}
	for _, c := range current {
		if c == normalized {
			return false, current, nil
		}
	}

	next := append(current, normalized)
	if err := writeJSONAtomic(filePath, allowFromStore{Version: 1, AllowFrom: next}); err != nil {
		return false, current, err
	}
	return true, next, nil
}

// RemoveChannelAllowFromStoreEntry 移除白名单条目。
// 对齐 TS removeChannelAllowFromStoreEntry()。
func RemoveChannelAllowFromStoreEntry(channel string, entry string) (changed bool, allowFrom []string, err error) {
	filePath, err := resolveAllowFromPath(channel)
	if err != nil {
		return false, nil, err
	}

	mu := channelLock(channel + ":allow")
	mu.Lock()
	defer mu.Unlock()

	store := loadAllowFromStore(filePath)
	current := make([]string, 0, len(store.AllowFrom))
	for _, v := range store.AllowFrom {
		if n := normalizeAllowEntry(channel, v); n != "" {
			current = append(current, n)
		}
	}

	normalized := normalizeAllowEntry(channel, normalizeID(entry))
	if normalized == "" {
		return false, current, nil
	}

	next := make([]string, 0, len(current))
	for _, c := range current {
		if c != normalized {
			next = append(next, c)
		}
	}
	if len(next) == len(current) {
		return false, current, nil
	}

	if err := writeJSONAtomic(filePath, allowFromStore{Version: 1, AllowFrom: next}); err != nil {
		return false, current, err
	}
	return true, next, nil
}

// ListChannelPairingRequests 列出待处理配对请求（带自动清理）。
// 对齐 TS listChannelPairingRequests()。
func ListChannelPairingRequests(channel string) ([]PairingRequest, error) {
	filePath, err := resolvePairingPath(channel)
	if err != nil {
		return nil, err
	}

	mu := channelLock(channel + ":pairing")
	mu.Lock()
	defer mu.Unlock()

	store := loadPairingStore(filePath)
	nowMs := time.Now().UnixMilli()

	prunedExpired, expiredRemoved := pruneExpiredRequests(store.Requests, nowMs)
	pruned, cappedRemoved := pruneExcessRequests(prunedExpired, pairingPendingMax)

	if expiredRemoved || cappedRemoved {
		_ = writeJSONAtomic(filePath, pairingStore{Version: 1, Requests: pruned})
	}

	// 过滤无效条目
	var result []PairingRequest
	for _, r := range pruned {
		if r.ID != "" && r.Code != "" && r.CreatedAt != "" {
			result = append(result, r)
		}
	}

	// 按 createdAt 排序
	slices.SortFunc(result, func(a, b PairingRequest) int {
		return strings.Compare(a.CreatedAt, b.CreatedAt)
	})

	return result, nil
}

// UpsertChannelPairingRequest 创建或更新配对请求。
// 对齐 TS upsertChannelPairingRequest()。
func UpsertChannelPairingRequest(channel string, id string, meta map[string]string) (code string, created bool, err error) {
	filePath, err := resolvePairingPath(channel)
	if err != nil {
		return "", false, err
	}

	mu := channelLock(channel + ":pairing")
	mu.Lock()
	defer mu.Unlock()

	store := loadPairingStore(filePath)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	nowMs := time.Now().UnixMilli()
	normalizedID := normalizeID(id)

	// 清理 meta
	var cleanMeta map[string]string
	if meta != nil {
		cleanMeta = make(map[string]string)
		for k, v := range meta {
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				cleanMeta[k] = trimmed
			}
		}
		if len(cleanMeta) == 0 {
			cleanMeta = nil
		}
	}

	// 过期清理
	reqs := store.Requests
	prunedExpired, expiredRemoved := pruneExpiredRequests(reqs, nowMs)
	reqs = prunedExpired

	// 收集已有码
	existingCodes := make(map[string]struct{})
	for _, req := range reqs {
		c := strings.TrimSpace(strings.ToUpper(req.Code))
		if c != "" {
			existingCodes[c] = struct{}{}
		}
	}

	// 查找已有请求
	existingIdx := -1
	for i, r := range reqs {
		if r.ID == normalizedID {
			existingIdx = i
			break
		}
	}

	if existingIdx >= 0 {
		existing := reqs[existingIdx]
		existingCode := strings.TrimSpace(existing.Code)
		if existingCode == "" {
			existingCode, err = generateUniqueCode(existingCodes)
			if err != nil {
				return "", false, err
			}
		}
		next := PairingRequest{
			ID:         normalizedID,
			Code:       existingCode,
			CreatedAt:  existing.CreatedAt,
			LastSeenAt: now,
			Meta:       cleanMeta,
		}
		if next.Meta == nil {
			next.Meta = existing.Meta
		}
		reqs[existingIdx] = next
		capped, _ := pruneExcessRequests(reqs, pairingPendingMax)
		if wErr := writeJSONAtomic(filePath, pairingStore{Version: 1, Requests: capped}); wErr != nil {
			return "", false, wErr
		}
		return existingCode, false, nil
	}

	// 新建请求 — 先检查容量
	capped, cappedRemoved := pruneExcessRequests(reqs, pairingPendingMax)
	reqs = capped
	if pairingPendingMax > 0 && len(reqs) >= pairingPendingMax {
		if expiredRemoved || cappedRemoved {
			_ = writeJSONAtomic(filePath, pairingStore{Version: 1, Requests: reqs})
		}
		return "", false, nil
	}

	newCode, err := generateUniqueCode(existingCodes)
	if err != nil {
		return "", false, err
	}
	next := PairingRequest{
		ID:         normalizedID,
		Code:       newCode,
		CreatedAt:  now,
		LastSeenAt: now,
		Meta:       cleanMeta,
	}
	reqs = append(reqs, next)
	if wErr := writeJSONAtomic(filePath, pairingStore{Version: 1, Requests: reqs}); wErr != nil {
		return "", false, wErr
	}
	return newCode, true, nil
}

// ApproveChannelPairingCode 按验证码批准配对请求。
// 找到匹配的请求后删除它并自动添加到白名单。
// 对齐 TS approveChannelPairingCode()。
func ApproveChannelPairingCode(channel string, code string) (id string, entry *PairingRequest, err error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return "", nil, nil
	}

	filePath, err := resolvePairingPath(channel)
	if err != nil {
		return "", nil, err
	}

	mu := channelLock(channel + ":pairing")
	mu.Lock()
	defer mu.Unlock()

	store := loadPairingStore(filePath)
	nowMs := time.Now().UnixMilli()
	pruned, removed := pruneExpiredRequests(store.Requests, nowMs)

	idx := -1
	for i, r := range pruned {
		if strings.ToUpper(strings.TrimSpace(r.Code)) == code {
			idx = i
			break
		}
	}
	if idx < 0 {
		if removed {
			_ = writeJSONAtomic(filePath, pairingStore{Version: 1, Requests: pruned})
		}
		return "", nil, nil
	}

	matched := pruned[idx]
	pruned = append(pruned[:idx], pruned[idx+1:]...)
	if wErr := writeJSONAtomic(filePath, pairingStore{Version: 1, Requests: pruned}); wErr != nil {
		return "", nil, wErr
	}

	// 自动添加到白名单（对齐 TS：错误向上传播）
	if _, _, addErr := AddChannelAllowFromStoreEntry(channel, matched.ID); addErr != nil {
		return matched.ID, &matched, fmt.Errorf("approved but failed to add allow entry: %w", addErr)
	}

	return matched.ID, &matched, nil
}
