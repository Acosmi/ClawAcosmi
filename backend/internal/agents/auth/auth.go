package auth

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------- 认证凭据类型 ----------

// TS 参考: src/agents/auth-profiles/types.ts (75 行)

// CredentialType 凭据类型。
type CredentialType string

const (
	CredentialAPIKey CredentialType = "api_key"
	CredentialToken  CredentialType = "token"
	CredentialOAuth  CredentialType = "oauth"
)

// AuthProfileCredential 认证配置文件凭据。
type AuthProfileCredential struct {
	Type     CredentialType    `json:"type"`
	Provider string            `json:"provider"`
	Key      string            `json:"key,omitempty"`      // api_key only
	Token    string            `json:"token,omitempty"`    // token only
	Expires  *int64            `json:"expires,omitempty"`  // token expiry (ms)
	ClientID string            `json:"clientId,omitempty"` // oauth only
	Email    string            `json:"email,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"` // provider-specific
}

// FailureReason 认证失败原因。
type FailureReason string

const (
	FailureAuth      FailureReason = "auth"
	FailureFormat    FailureReason = "format"
	FailureRateLimit FailureReason = "rate_limit"
	FailureBilling   FailureReason = "billing"
	FailureTimeout   FailureReason = "timeout"
	FailureUnknown   FailureReason = "unknown"
)

// ProfileUsageStats 配置文件使用统计。
type ProfileUsageStats struct {
	LastUsed       *int64                `json:"lastUsed,omitempty"`
	CooldownUntil  *int64                `json:"cooldownUntil,omitempty"`
	DisabledUntil  *int64                `json:"disabledUntil,omitempty"`
	DisabledReason FailureReason         `json:"disabledReason,omitempty"`
	ErrorCount     *int                  `json:"errorCount,omitempty"`
	FailureCounts  map[FailureReason]int `json:"failureCounts,omitempty"`
	LastFailureAt  *int64                `json:"lastFailureAt,omitempty"`
}

// AuthProfileStore 认证配置文件存储。
type AuthProfileStore struct {
	Version    int                               `json:"version"`
	Profiles   map[string]*AuthProfileCredential `json:"profiles"`
	Order      map[string][]string               `json:"order,omitempty"`
	LastGood   map[string]string                 `json:"lastGood,omitempty"`
	UsageStats map[string]*ProfileUsageStats     `json:"usageStats,omitempty"`
}

// ---------- 常量 ----------

const (
	AuthStoreVersion   = 1
	ClaudeCliProfileID = "anthropic:claude-cli"
	CodexCliProfileID  = "openai-codex:codex-cli"
)

// ---------- 存储管理 ----------

// AuthStore 认证存储管理器。
type AuthStore struct {
	mu        sync.RWMutex
	storePath string
	data      *AuthProfileStore
}

// NewAuthStore 创建认证存储实例。
func NewAuthStore(storePath string) *AuthStore {
	return &AuthStore{storePath: storePath}
}

// Load 从磁盘加载认证存储。
// TS 参考: auth-profiles/store.ts → loadAuthProfileStore()
func (s *AuthStore) Load() (*AuthProfileStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = newEmptyStore()
			return s.cloneData(), nil
		}
		return nil, fmt.Errorf("读取认证存储失败: %w", err)
	}

	store := &AuthProfileStore{}
	if err := json.Unmarshal(raw, store); err != nil {
		s.data = newEmptyStore()
		return s.cloneData(), nil
	}

	// 校验 + 迁移
	if store.Profiles == nil {
		store.Profiles = make(map[string]*AuthProfileCredential)
	}
	if store.UsageStats == nil {
		store.UsageStats = make(map[string]*ProfileUsageStats)
	}

	s.data = store
	return s.cloneData(), nil
}

// Save 保存认证存储到磁盘。
// TS 参考: auth-profiles/store.ts → saveAuthProfileStore()
// S-01: 增加跨进程文件锁，等价于 TS proper-lockfile 行为。
func (s *AuthStore) Save(store *AuthProfileStore) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// S-01: 跨进程排他锁 — 防止多实例并发写入撕裂数据
	fl := NewFileLock(s.storePath + ".lock")
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer fl.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.storePath), 0o700); err != nil {
		return fmt.Errorf("创建认证存储目录失败: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化认证存储失败: %w", err)
	}

	if err := os.WriteFile(s.storePath, data, 0o600); err != nil {
		return fmt.Errorf("写入认证存储失败: %w", err)
	}

	s.data = store
	return nil
}

// Update 原子读-改-写。
// S-01: 增加跨进程文件锁保护读-改-写全过程。
func (s *AuthStore) Update(updater func(store *AuthProfileStore) bool) (*AuthProfileStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// S-01: 跨进程排他锁 — 覆盖完整的 read-modify-write 周期
	fl := NewFileLock(s.storePath + ".lock")
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("acquire file lock: %w", err)
	}
	defer fl.Unlock()

	// 重新从磁盘读取确保一致性
	raw, err := os.ReadFile(s.storePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取认证存储失败: %w", err)
	}

	store := newEmptyStore()
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, store); err != nil {
			store = newEmptyStore()
		}
	}
	if store.Profiles == nil {
		store.Profiles = make(map[string]*AuthProfileCredential)
	}
	if store.UsageStats == nil {
		store.UsageStats = make(map[string]*ProfileUsageStats)
	}

	if !updater(store) {
		return store, nil
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.storePath), 0o700); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(s.storePath, data, 0o600); err != nil {
		return nil, fmt.Errorf("写入失败: %w", err)
	}

	s.data = store
	return store, nil
}

// GetProfile 获取指定配置文件。
func (s *AuthStore) GetProfile(profileID string) *AuthProfileCredential {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return nil
	}
	return s.data.Profiles[profileID]
}

func newEmptyStore() *AuthProfileStore {
	return &AuthProfileStore{
		Version:    AuthStoreVersion,
		Profiles:   make(map[string]*AuthProfileCredential),
		UsageStats: make(map[string]*ProfileUsageStats),
	}
}

func (s *AuthStore) cloneData() *AuthProfileStore {
	if s.data == nil {
		return newEmptyStore()
	}
	data, _ := json.Marshal(s.data)
	result := &AuthProfileStore{}
	_ = json.Unmarshal(data, result)
	return result
}

// ---------- 使用统计 & 冷却 ----------

// TS 参考: auth-profiles/usage.ts (323 行)

// IsProfileInCooldown 检查配置文件是否在冷却中。
func IsProfileInCooldown(store *AuthProfileStore, profileID string) bool {
	if store == nil || store.UsageStats == nil {
		return false
	}
	stats := store.UsageStats[profileID]
	if stats == nil {
		return false
	}
	now := time.Now().UnixMilli()
	if stats.CooldownUntil != nil && *stats.CooldownUntil > now {
		return true
	}
	if stats.DisabledUntil != nil && *stats.DisabledUntil > now {
		return true
	}
	return false
}

// CalculateCooldownMs 计算冷却时间（指数退避）。
// 冷却时间: 1min, 5min, 25min, 最长 1 小时。
func CalculateCooldownMs(errorCount int) int64 {
	if errorCount <= 0 {
		return 60_000 // 1 分钟
	}
	ms := int64(60_000 * math.Pow(5, float64(errorCount-1)))
	const maxMs = 3_600_000 // 1 小时
	if ms > maxMs {
		return maxMs
	}
	return ms
}

// MarkProfileUsed 标记配置文件已使用。重置错误计数。
func MarkProfileUsed(store *AuthProfileStore, profileID string) {
	if store.UsageStats == nil {
		store.UsageStats = make(map[string]*ProfileUsageStats)
	}
	now := time.Now().UnixMilli()
	stats := store.UsageStats[profileID]
	if stats == nil {
		stats = &ProfileUsageStats{}
		store.UsageStats[profileID] = stats
	}
	stats.LastUsed = &now
	zero := 0
	stats.ErrorCount = &zero
	stats.CooldownUntil = nil
}

// MarkProfileFailure 标记配置文件失败。
func MarkProfileFailure(store *AuthProfileStore, profileID string, reason FailureReason) {
	if store.UsageStats == nil {
		store.UsageStats = make(map[string]*ProfileUsageStats)
	}
	now := time.Now().UnixMilli()
	stats := store.UsageStats[profileID]
	if stats == nil {
		stats = &ProfileUsageStats{}
		store.UsageStats[profileID] = stats
	}

	count := 0
	if stats.ErrorCount != nil {
		count = *stats.ErrorCount
	}
	count++
	stats.ErrorCount = &count
	stats.LastFailureAt = &now

	if stats.FailureCounts == nil {
		stats.FailureCounts = make(map[FailureReason]int)
	}
	stats.FailureCounts[reason]++

	if reason == FailureBilling {
		// Billing：较长的禁用期
		disableMs := calculateBillingDisableMs(count)
		disabledUntil := now + disableMs
		stats.DisabledUntil = &disabledUntil
		stats.DisabledReason = FailureBilling
	} else {
		// 常规冷却
		cooldownMs := CalculateCooldownMs(count)
		cooldownUntil := now + cooldownMs
		stats.CooldownUntil = &cooldownUntil
	}
}

// MarkProfileCooldown 标记配置文件为冷却（rate limit）。
func MarkProfileCooldown(store *AuthProfileStore, profileID string) {
	MarkProfileFailure(store, profileID, FailureRateLimit)
}

// ClearProfileCooldown 清除配置文件冷却。
func ClearProfileCooldown(store *AuthProfileStore, profileID string) {
	if store.UsageStats == nil {
		return
	}
	stats := store.UsageStats[profileID]
	if stats == nil {
		return
	}
	zero := 0
	stats.ErrorCount = &zero
	stats.CooldownUntil = nil
	stats.DisabledUntil = nil
	stats.DisabledReason = ""
}

func calculateBillingDisableMs(errorCount int) int64 {
	// 默认 1h base，24h max
	baseMs := int64(3_600_000) // 1 小时
	maxMs := int64(86_400_000) // 24 小时

	ms := baseMs * int64(math.Pow(2, float64(errorCount-1)))
	if ms > maxMs {
		return maxMs
	}
	return ms
}
