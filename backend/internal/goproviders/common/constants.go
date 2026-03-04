// common/constants.go — Auth Profile 常量定义
// 对应 TS 文件: src/agents/auth-profiles/constants.ts
package common

// AuthStoreVersion 当前 Auth 存储版本号。
const AuthStoreVersion = 1

// AuthProfileFilename Auth Profile 存储文件名。
const AuthProfileFilename = "auth-profiles.json"

// LegacyAuthFilename 旧版 Auth 存储文件名。
const LegacyAuthFilename = "auth.json"

// CLI Profile ID 常量
const (
	// ClaudeCLIProfileID Anthropic Claude CLI 的 Profile ID。
	ClaudeCLIProfileID = "anthropic:claude-cli"
	// CodexCLIProfileID OpenAI Codex CLI 的 Profile ID。
	CodexCLIProfileID = "openai-codex:codex-cli"
	// QwenCLIProfileID 通义千问 CLI 的 Profile ID。
	QwenCLIProfileID = "qwen-portal:qwen-cli"
	// MinimaxCLIProfileID MiniMax CLI 的 Profile ID。
	MinimaxCLIProfileID = "minimax-portal:minimax-cli"
)

// AuthStoreLockOptions Auth 存储文件锁配置。
// 对应 TS: AUTH_STORE_LOCK_OPTIONS
type AuthStoreLockOptions struct {
	// Retries 重试配置
	Retries LockRetryOptions
	// StaleMs 过期阈值（毫秒）
	StaleMs int
}

// LockRetryOptions 文件锁重试配置。
type LockRetryOptions struct {
	Retries    int  // 最大重试次数
	Factor     int  // 退避因子
	MinTimeout int  // 最小超时（毫秒）
	MaxTimeout int  // 最大超时（毫秒）
	Randomize  bool // 是否随机化
}

// DefaultAuthStoreLockOptions 默认文件锁配置。
var DefaultAuthStoreLockOptions = AuthStoreLockOptions{
	Retries: LockRetryOptions{
		Retries:    10,
		Factor:     2,
		MinTimeout: 100,
		MaxTimeout: 10_000,
		Randomize:  true,
	},
	StaleMs: 30_000,
}

// ExternalCLISyncTTLMs 外部 CLI 同步 TTL（毫秒）：15 分钟。
const ExternalCLISyncTTLMs = 15 * 60 * 1000

// ExternalCLINearExpiryMs 外部 CLI 接近过期阈值（毫秒）：10 分钟。
const ExternalCLINearExpiryMs = 10 * 60 * 1000
