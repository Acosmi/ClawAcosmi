package auth

// TS 对照: src/agents/cli-credentials.ts (607L)
// 多厂商 CLI 凭证读写：Claude Code, Codex, Qwen, MiniMax

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ---------- 凭证类型 ----------

// CliCredentialType 凭证类型。
type CliCredentialType string

const (
	CliCredTypeOAuth CliCredentialType = "oauth"
	CliCredTypeToken CliCredentialType = "token"
)

// ClaudeCliCredential Claude CLI 凭证。
type ClaudeCliCredential struct {
	Type     CliCredentialType `json:"type"`
	Provider string            `json:"provider"` // "anthropic"
	Access   string            `json:"access,omitempty"`
	Refresh  string            `json:"refresh,omitempty"`
	Token    string            `json:"token,omitempty"`
	Expires  int64             `json:"expires,omitempty"`
}

// CodexCliCredential Codex CLI 凭证。
type CodexCliCredential struct {
	Type      CliCredentialType `json:"type"`
	Provider  string            `json:"provider"`
	Access    string            `json:"access"`
	Refresh   string            `json:"refresh"`
	Expires   int64             `json:"expires"`
	AccountID string            `json:"accountId,omitempty"`
}

// QwenCliCredential Qwen CLI 凭证。
type QwenCliCredential struct {
	Type     CliCredentialType `json:"type"`
	Provider string            `json:"provider"` // "qwen-portal"
	Access   string            `json:"access"`
	Refresh  string            `json:"refresh"`
	Expires  int64             `json:"expires"`
}

// MiniMaxCliCredential MiniMax CLI 凭证。
type MiniMaxCliCredential struct {
	Type     CliCredentialType `json:"type"`
	Provider string            `json:"provider"` // "minimax-portal"
	Access   string            `json:"access"`
	Refresh  string            `json:"refresh"`
	Expires  int64             `json:"expires"`
}

// ---------- 路径解析 ----------

const (
	claudeCliRelPath  = ".claude/credentials.json"
	codexCliRelPath   = ".codex/auth.json"
	qwenCliRelPath    = ".qwen/credentials.json"
	minimaxCliRelPath = ".minimax/oauth_creds.json"
	claudeKeychainSvc = "Claude Code-credentials"
	claudeKeychainAcc = "Claude Code"
)

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// ResolveClaudeCliCredentialsPath claude 凭证文件路径。
func ResolveClaudeCliCredentialsPath(home string) string {
	if home == "" {
		home = homeDir()
	}
	return filepath.Join(home, claudeCliRelPath)
}

// ResolveCodexCliAuthPath codex 凭证文件路径。
func ResolveCodexCliAuthPath() string {
	codexHome := resolveCodexHomePath()
	return filepath.Join(codexHome, "auth.json")
}

func resolveCodexHomePath() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	configDir, _ := os.UserConfigDir()
	if configDir != "" {
		return filepath.Join(configDir, "codex")
	}
	return filepath.Join(homeDir(), ".codex")
}

// ResolveQwenCliCredentialsPath qwen 凭证文件路径。
func ResolveQwenCliCredentialsPath(home string) string {
	if home == "" {
		home = homeDir()
	}
	return filepath.Join(home, qwenCliRelPath)
}

// ResolveMiniMaxCliCredentialsPath minimax 凭证文件路径。
func ResolveMiniMaxCliCredentialsPath(home string) string {
	if home == "" {
		home = homeDir()
	}
	return filepath.Join(home, minimaxCliRelPath)
}

// ---------- 凭证读取 ----------

// ReadClaudeCliCredentials 读取 Claude CLI 凭证（文件 + Keychain）。
// TS 对照: cli-credentials.ts readClaudeCliCredentials
func ReadClaudeCliCredentials(home string) *ClaudeCliCredential {
	// 优先文件
	p := ResolveClaudeCliCredentialsPath(home)
	data, err := os.ReadFile(p)
	if err == nil {
		var cred ClaudeCliCredential
		if json.Unmarshal(data, &cred) == nil && cred.Type != "" {
			return &cred
		}
	}
	// macOS Keychain
	if runtime.GOOS == "darwin" {
		if kc := readClaudeKeychainCredential(); kc != nil {
			return kc
		}
	}
	return nil
}

func readClaudeKeychainCredential() *ClaudeCliCredential {
	out, err := exec.Command(
		"security", "find-generic-password",
		"-s", claudeKeychainSvc,
		"-a", claudeKeychainAcc,
		"-w",
	).Output()
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	var cred ClaudeCliCredential
	if json.Unmarshal([]byte(raw), &cred) != nil {
		return nil
	}
	return &cred
}

// ComputeCodexKeychainAccount 计算 codex Keychain account 名。
func ComputeCodexKeychainAccount(codexHome string) string {
	h := sha256.Sum256([]byte(codexHome))
	return "codex-" + hex.EncodeToString(h[:8])
}

// ReadCodexCliCredentials 读取 Codex CLI 凭证。
func ReadCodexCliCredentials() *CodexCliCredential {
	// macOS Keychain
	if runtime.GOOS == "darwin" {
		codexHome := resolveCodexHomePath()
		account := ComputeCodexKeychainAccount(codexHome)
		out, err := exec.Command(
			"security", "find-generic-password",
			"-s", "codex-credentials",
			"-a", account,
			"-w",
		).Output()
		if err == nil {
			raw := strings.TrimSpace(string(out))
			var cred CodexCliCredential
			if json.Unmarshal([]byte(raw), &cred) == nil && cred.Access != "" {
				return &cred
			}
		}
	}
	// 文件回退
	p := ResolveCodexCliAuthPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var cred CodexCliCredential
	if json.Unmarshal(data, &cred) != nil || cred.Access == "" {
		return nil
	}
	return &cred
}

// ReadQwenCliCredentials 读取 Qwen CLI 凭证。
func ReadQwenCliCredentials(home string) *QwenCliCredential {
	p := ResolveQwenCliCredentialsPath(home)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var cred QwenCliCredential
	if json.Unmarshal(data, &cred) != nil || cred.Access == "" {
		return nil
	}
	return &cred
}

// ReadMiniMaxCliCredentials 读取 MiniMax CLI 凭证。
func ReadMiniMaxCliCredentials(home string) *MiniMaxCliCredential {
	p := ResolveMiniMaxCliCredentialsPath(home)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var cred MiniMaxCliCredential
	if json.Unmarshal(data, &cred) != nil || cred.Access == "" {
		return nil
	}
	return &cred
}

// ---------- 缓存读取 ----------

type cachedCred[T any] struct {
	mu       sync.Mutex
	value    *T
	readAt   time.Time
	cacheKey string
}

var (
	claudeCache  cachedCred[ClaudeCliCredential]
	codexCache   cachedCred[CodexCliCredential]
	qwenCache    cachedCred[QwenCliCredential]
	minimaxCache cachedCred[MiniMaxCliCredential]
)

const defaultCredTTL = 30 * time.Second

// ReadClaudeCliCredentialsCached 缓存读取 Claude 凭证。
func ReadClaudeCliCredentialsCached(home string, ttl time.Duration) *ClaudeCliCredential {
	if ttl == 0 {
		ttl = defaultCredTTL
	}
	claudeCache.mu.Lock()
	defer claudeCache.mu.Unlock()
	key := ResolveClaudeCliCredentialsPath(home)
	if claudeCache.value != nil && claudeCache.cacheKey == key && time.Since(claudeCache.readAt) < ttl {
		return claudeCache.value
	}
	cred := ReadClaudeCliCredentials(home)
	claudeCache.value = cred
	claudeCache.readAt = time.Now()
	claudeCache.cacheKey = key
	return cred
}

// ReadCodexCliCredentialsCached 缓存读取 Codex 凭证。
func ReadCodexCliCredentialsCached(ttl time.Duration) *CodexCliCredential {
	if ttl == 0 {
		ttl = defaultCredTTL
	}
	codexCache.mu.Lock()
	defer codexCache.mu.Unlock()
	if codexCache.value != nil && time.Since(codexCache.readAt) < ttl {
		return codexCache.value
	}
	cred := ReadCodexCliCredentials()
	codexCache.value = cred
	codexCache.readAt = time.Now()
	return cred
}

// ReadQwenCliCredentialsCached 缓存读取 Qwen 凭证。
func ReadQwenCliCredentialsCached(home string, ttl time.Duration) *QwenCliCredential {
	if ttl == 0 {
		ttl = defaultCredTTL
	}
	qwenCache.mu.Lock()
	defer qwenCache.mu.Unlock()
	if qwenCache.value != nil && time.Since(qwenCache.readAt) < ttl {
		return qwenCache.value
	}
	cred := ReadQwenCliCredentials(home)
	qwenCache.value = cred
	qwenCache.readAt = time.Now()
	return cred
}

// ReadMiniMaxCliCredentialsCached 缓存读取 MiniMax 凭证。
func ReadMiniMaxCliCredentialsCached(home string, ttl time.Duration) *MiniMaxCliCredential {
	if ttl == 0 {
		ttl = defaultCredTTL
	}
	minimaxCache.mu.Lock()
	defer minimaxCache.mu.Unlock()
	if minimaxCache.value != nil && time.Since(minimaxCache.readAt) < ttl {
		return minimaxCache.value
	}
	cred := ReadMiniMaxCliCredentials(home)
	minimaxCache.value = cred
	minimaxCache.readAt = time.Now()
	return cred
}

// ResetCliCredentialCachesForTest 测试用重置。
func ResetCliCredentialCachesForTest() {
	claudeCache.mu.Lock()
	claudeCache.value = nil
	claudeCache.mu.Unlock()
	codexCache.mu.Lock()
	codexCache.value = nil
	codexCache.mu.Unlock()
	qwenCache.mu.Lock()
	qwenCache.value = nil
	qwenCache.mu.Unlock()
	minimaxCache.mu.Lock()
	minimaxCache.value = nil
	minimaxCache.mu.Unlock()
}
