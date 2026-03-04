// types/secrets_providers.go — 密钥提供者配置类型
// 对应 TS 文件: src/config/types.secrets.ts (171-220行)
// SecretProvider 配置和 SecretsConfig 顶层配置
package types

// FileSecretProviderMode 文件密钥提供者模式。
type FileSecretProviderMode string

const (
	FileSecretProviderModeSingleValue FileSecretProviderMode = "singleValue"
	FileSecretProviderModeJSON        FileSecretProviderMode = "json"
)

// EnvSecretProviderConfig 环境变量密钥提供者配置。
type EnvSecretProviderConfig struct {
	Source SecretRefSource `json:"source"` // 固定为 "env"
	// Allowlist 可选的环境变量白名单（精确名称）
	Allowlist []string `json:"allowlist,omitempty"`
}

// FileSecretProviderConfig 文件密钥提供者配置。
type FileSecretProviderConfig struct {
	Source SecretRefSource `json:"source"` // 固定为 "file"
	// Path 文件路径
	Path string `json:"path"`
	// Mode 文件模式：singleValue 或 json
	Mode FileSecretProviderMode `json:"mode,omitempty"`
	// TimeoutMs 读取超时（毫秒）
	TimeoutMs *int `json:"timeoutMs,omitempty"`
	// MaxBytes 最大读取字节数
	MaxBytes *int `json:"maxBytes,omitempty"`
}

// ExecSecretProviderConfig 命令执行密钥提供者配置。
type ExecSecretProviderConfig struct {
	Source SecretRefSource `json:"source"` // 固定为 "exec"
	// Command 要执行的命令
	Command string `json:"command"`
	// Args 命令参数
	Args []string `json:"args,omitempty"`
	// TimeoutMs 执行超时（毫秒）
	TimeoutMs *int `json:"timeoutMs,omitempty"`
	// NoOutputTimeoutMs 无输出超时（毫秒）
	NoOutputTimeoutMs *int `json:"noOutputTimeoutMs,omitempty"`
	// MaxOutputBytes 最大输出字节数
	MaxOutputBytes *int `json:"maxOutputBytes,omitempty"`
	// JSONOnly 是否仅接受 JSON 输出
	JSONOnly *bool `json:"jsonOnly,omitempty"`
	// Env 执行时注入的环境变量
	Env map[string]string `json:"env,omitempty"`
	// PassEnv 从父进程传递的环境变量名
	PassEnv []string `json:"passEnv,omitempty"`
	// TrustedDirs 受信任的目录列表
	TrustedDirs []string `json:"trustedDirs,omitempty"`
	// AllowInsecurePath 是否允许不安全的 PATH
	AllowInsecurePath *bool `json:"allowInsecurePath,omitempty"`
	// AllowSymlinkCommand 是否允许符号链接命令
	AllowSymlinkCommand *bool `json:"allowSymlinkCommand,omitempty"`
}

// SecretProviderConfig 密钥提供者配置联合类型。
// 在 Go 中使用 struct 带判别字段表示。
type SecretProviderConfig struct {
	// Source 判别字段：env / file / exec
	Source SecretRefSource `json:"source"`
	// 以下字段按 source 类型选择性使用

	// Env 提供者专用字段
	Allowlist []string `json:"allowlist,omitempty"`

	// File 提供者专用字段
	Path     string                 `json:"path,omitempty"`
	Mode     FileSecretProviderMode `json:"mode,omitempty"`
	MaxBytes *int                   `json:"maxBytes,omitempty"`

	// Exec 提供者专用字段
	Command             string            `json:"command,omitempty"`
	Args                []string          `json:"args,omitempty"`
	NoOutputTimeoutMs   *int              `json:"noOutputTimeoutMs,omitempty"`
	MaxOutputBytes      *int              `json:"maxOutputBytes,omitempty"`
	JSONOnly            *bool             `json:"jsonOnly,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	PassEnv             []string          `json:"passEnv,omitempty"`
	TrustedDirs         []string          `json:"trustedDirs,omitempty"`
	AllowInsecurePath   *bool             `json:"allowInsecurePath,omitempty"`
	AllowSymlinkCommand *bool             `json:"allowSymlinkCommand,omitempty"`

	// File/Exec 共用字段
	TimeoutMs *int `json:"timeoutMs,omitempty"`
}

// SecretsConfig 密钥系统顶层配置。
// 对应 TS: SecretsConfig
type SecretsConfig struct {
	// Providers 密钥提供者映射
	Providers map[string]SecretProviderConfig `json:"providers,omitempty"`
	// Defaults 默认提供者别名
	Defaults *SecretDefaults `json:"defaults,omitempty"`
	// Resolution 解析配置
	Resolution *SecretsResolutionConfig `json:"resolution,omitempty"`
}

// SecretsResolutionConfig 密钥解析配置。
type SecretsResolutionConfig struct {
	// MaxProviderConcurrency 最大提供者并发数
	MaxProviderConcurrency *int `json:"maxProviderConcurrency,omitempty"`
	// MaxRefsPerProvider 每个提供者最大引用数
	MaxRefsPerProvider *int `json:"maxRefsPerProvider,omitempty"`
	// MaxBatchBytes 最大批次字节数
	MaxBatchBytes *int `json:"maxBatchBytes,omitempty"`
}
