// Package config provides application configuration loaded from environment variables.
// Mirrors the Python core/config.py Settings class for full compatibility.
package config

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Config holds all application configuration fields.
// Field names use Go conventions; env var mapping is handled by Viper.
type Config struct {
	// Application
	AppName    string `mapstructure:"APP_NAME"`
	Debug      bool   `mapstructure:"DEBUG"`
	APIPrefix  string `mapstructure:"API_PREFIX"`
	ServerPort int    `mapstructure:"SERVER_PORT"`

	// Security - JWT & Auth
	JWTSecretKey         string `mapstructure:"JWT_SECRET_KEY"`
	JWTAlgorithm         string `mapstructure:"JWT_ALGORITHM"`
	AccessTokenExpireMin int    `mapstructure:"ACCESS_TOKEN_EXPIRE_MINUTES"`
	AllowDevAuthBypass   bool   `mapstructure:"ALLOW_DEV_AUTH_BYPASS"`

	// CORS
	CORSOrigins []string `mapstructure:"CORS_ORIGINS"`

	// Database - PostgreSQL
	PostgresHost     string `mapstructure:"POSTGRES_HOST"`
	PostgresPort     int    `mapstructure:"POSTGRES_PORT"`
	PostgresUser     string `mapstructure:"POSTGRES_USER"`
	PostgresPassword string `mapstructure:"POSTGRES_PASSWORD"`
	PostgresDB       string `mapstructure:"POSTGRES_DB"`

	// Vector Store — Qdrant (DEPRECATED: Phase 4 replaced gRPC with in-process segment FFI)
	// Fields retained for backward compatibility; no longer used at runtime.
	QdrantHost       string `mapstructure:"QDRANT_HOST"`
	QdrantPort       int    `mapstructure:"QDRANT_PORT"`
	QdrantCollection string `mapstructure:"QDRANT_COLLECTION"`

	// Redis
	RedisHost string `mapstructure:"REDIS_HOST"`
	RedisPort int    `mapstructure:"REDIS_PORT"`
	RedisDB   int    `mapstructure:"REDIS_DB"`

	// LLM Provider
	LLMProvider string `mapstructure:"LLM_PROVIDER"`

	// OpenAI
	OpenAIAPIKey  string `mapstructure:"OPENAI_API_KEY"`
	OpenAIModel   string `mapstructure:"OPENAI_MODEL"`
	OpenAIBaseURL string `mapstructure:"OPENAI_BASE_URL"`

	// DeepSeek
	DeepSeekAPIKey  string `mapstructure:"DEEPSEEK_API_KEY"`
	DeepSeekModel   string `mapstructure:"DEEPSEEK_MODEL"`
	DeepSeekBaseURL string `mapstructure:"DEEPSEEK_BASE_URL"`

	// Anthropic
	AnthropicAPIKey string `mapstructure:"ANTHROPIC_API_KEY"`
	AnthropicModel  string `mapstructure:"ANTHROPIC_MODEL"`

	// Google Gemini
	GeminiAPIKey string `mapstructure:"GEMINI_API_KEY"`
	GeminiModel  string `mapstructure:"GEMINI_MODEL"`

	// Alibaba Qwen
	QwenAPIKey string `mapstructure:"QWEN_API_KEY"`
	QwenModel  string `mapstructure:"QWEN_MODEL"`

	// Volcano Doubao
	DoubaoAPIKey   string `mapstructure:"DOUBAO_API_KEY"`
	DoubaoModel    string `mapstructure:"DOUBAO_MODEL"`
	DoubaoEndpoint string `mapstructure:"DOUBAO_ENDPOINT_ID"`

	// Embedding
	EmbeddingProvider  string `mapstructure:"EMBEDDING_PROVIDER"`
	EmbeddingAPIKey    string `mapstructure:"EMBEDDING_API_KEY"`
	EmbeddingBaseURL   string `mapstructure:"EMBEDDING_BASE_URL"`
	EmbeddingModelName string `mapstructure:"EMBEDDING_MODEL_NAME"`
	EmbeddingDimension int    `mapstructure:"EMBEDDING_DIMENSION"`

	// Rerank
	EnableRerank   bool   `mapstructure:"ENABLE_RERANK"`
	RerankProvider string `mapstructure:"RERANK_PROVIDER"`
	RerankAPIKey   string `mapstructure:"RERANK_API_KEY"`
	RerankModel    string `mapstructure:"RERANK_MODEL_NAME"`
	RerankTopN     int    `mapstructure:"RERANK_TOP_N"`

	// Memory Settings
	ImportanceThreshold float64 `mapstructure:"IMPORTANCE_THRESHOLD"`
	ReflectionThreshold int     `mapstructure:"REFLECTION_THRESHOLD"`
	MaxContextMemories  int     `mapstructure:"MAX_CONTEXT_MEMORIES"`

	// MemFS — 文件系统记忆模式（nexus-memfs）
	// 为空时不启动 FSStoreService，仅使用 vector 模式。
	MemFSRootPath string `mapstructure:"MEMFS_ROOT_PATH"`

	// Web Search — 想象记忆外部搜索 API
	WebSearchProvider string `mapstructure:"WEB_SEARCH_PROVIDER"` // deepseek | gemini | auto
	WebSearchAPIKey   string `mapstructure:"WEB_SEARCH_API_KEY"`  // 独立搜索 key（可选，默认复用 LLM key）

	// AGFS — 分布式文件系统服务 (队列/KV/共享存储)
	AGFSServerURL string `mapstructure:"AGFS_URL"`

	// Tiered Loading — 渐进式加载 Token 预算控制 (Phase 3)
	TieredLoadingTokenBudget int `mapstructure:"TIERED_LOADING_TOKEN_BUDGET"`
	TieredLoadingTopK        int `mapstructure:"TIERED_LOADING_TOP_K"`

	// Proxy
	HTTPProxy  string `mapstructure:"HTTP_PROXY"`
	HTTPSProxy string `mapstructure:"HTTPS_PROXY"`

	// Algorithm API Keys — comma-separated keys for /algo/* endpoint protection
	AlgoAPIKeysRaw string `mapstructure:"ALGO_API_KEYS"`
}

// LLMAPIKey returns the API key for the active LLM provider.
func (c *Config) LLMAPIKey() string {
	switch c.LLMProvider {
	case "openai":
		return c.OpenAIAPIKey
	case "deepseek":
		return c.DeepSeekAPIKey
	case "anthropic":
		return c.AnthropicAPIKey
	case "gemini":
		return c.GeminiAPIKey
	case "qwen":
		return c.QwenAPIKey
	case "doubao":
		return c.DoubaoAPIKey
	default:
		return c.OpenAIAPIKey
	}
}

// LLMBaseURL returns the base URL for the active LLM provider.
func (c *Config) LLMBaseURL() string {
	switch c.LLMProvider {
	case "openai":
		return c.OpenAIBaseURL
	case "deepseek":
		return c.DeepSeekBaseURL
	default:
		return c.OpenAIBaseURL
	}
}

// LLMModel returns the model name for the active LLM provider.
func (c *Config) LLMModel() string {
	switch c.LLMProvider {
	case "openai":
		return c.OpenAIModel
	case "deepseek":
		return c.DeepSeekModel
	case "anthropic":
		return c.AnthropicModel
	case "gemini":
		return c.GeminiModel
	case "qwen":
		return c.QwenModel
	case "doubao":
		return c.DoubaoModel
	default:
		return c.OpenAIModel
	}
}

// RerankEnabled returns whether reranking is enabled.
func (c *Config) RerankEnabled() bool { return c.EnableRerank }

// RerankModelName returns the rerank model name.
func (c *Config) RerankModelName() string { return c.RerankModel }

// DatabaseURL returns the PostgreSQL connection DSN.
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.PostgresHost, c.PostgresPort, c.PostgresUser, c.PostgresPassword, c.PostgresDB,
	)
}

// RedisURL returns the Redis connection URL.
func (c *Config) RedisURL() string {
	return fmt.Sprintf("redis://%s:%d/%d", c.RedisHost, c.RedisPort, c.RedisDB)
}

// AlgoAPIKeys returns the list of valid API keys for /algo/* endpoints.
// Returns nil if no keys configured (dev mode — permissive).
func (c *Config) AlgoAPIKeys() []string {
	if c.AlgoAPIKeysRaw == "" {
		return nil
	}
	keys := strings.Split(c.AlgoAPIKeysRaw, ",")
	var result []string
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			result = append(result, k)
		}
	}
	return result
}

// --- Singleton ---

var (
	cfg  *Config
	once sync.Once
)

// Load reads the configuration from .env file and environment variables.
// It is safe to call multiple times; only the first call performs the actual load.
func Load(envPath ...string) *Config {
	once.Do(func() {
		v := viper.New()

		// Defaults — mirrors Python Settings class defaults
		v.SetDefault("APP_NAME", "UHMS API")
		v.SetDefault("DEBUG", false)
		v.SetDefault("API_PREFIX", "/api/v1")
		v.SetDefault("SERVER_PORT", 8080)

		v.SetDefault("JWT_ALGORITHM", "HS256")
		v.SetDefault("ACCESS_TOKEN_EXPIRE_MINUTES", 1440)
		v.SetDefault("ALLOW_DEV_AUTH_BYPASS", false)

		v.SetDefault("CORS_ORIGINS", "http://localhost:3006,http://127.0.0.1:3006,http://localhost:3000,http://127.0.0.1:3000,http://localhost:5173")

		v.SetDefault("POSTGRES_HOST", "localhost")
		v.SetDefault("POSTGRES_PORT", 5432)
		v.SetDefault("POSTGRES_USER", "uhms")
		// 安全策略：不设置默认密码，强制从环境变量或 .env 读取
		v.SetDefault("POSTGRES_DB", "uhms_db")

		// Qdrant defaults removed — no longer needed (Phase 4: in-process segment FFI)

		v.SetDefault("REDIS_HOST", "localhost")
		v.SetDefault("REDIS_PORT", 6379)
		v.SetDefault("REDIS_DB", 0)

		v.SetDefault("LLM_PROVIDER", "openai")
		v.SetDefault("OPENAI_MODEL", "gpt-4")
		v.SetDefault("DEEPSEEK_MODEL", "deepseek-chat")
		v.SetDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")
		v.SetDefault("ANTHROPIC_MODEL", "claude-3-opus-20240229")
		v.SetDefault("GEMINI_MODEL", "gemini-1.5-pro")
		v.SetDefault("QWEN_MODEL", "qwen-max")
		v.SetDefault("DOUBAO_MODEL", "doubao-pro-32k")

		v.SetDefault("EMBEDDING_PROVIDER", "local")
		v.SetDefault("EMBEDDING_MODEL_NAME", "BAAI/bge-small-zh-v1.5")
		v.SetDefault("EMBEDDING_DIMENSION", 512)

		v.SetDefault("ENABLE_RERANK", false)
		v.SetDefault("RERANK_PROVIDER", "cohere")
		v.SetDefault("RERANK_MODEL_NAME", "rerank-english-v3.0")
		v.SetDefault("RERANK_TOP_N", 5)

		v.SetDefault("AGFS_URL", "") // empty = disabled; set explicitly for production

		v.SetDefault("TIERED_LOADING_TOKEN_BUDGET", 20000)
		v.SetDefault("TIERED_LOADING_TOP_K", 8)

		v.SetDefault("IMPORTANCE_THRESHOLD", 0.5)
		v.SetDefault("REFLECTION_THRESHOLD", 10)
		v.SetDefault("MAX_CONTEXT_MEMORIES", 5)

		// Read .env file
		v.SetConfigType("env")
		if len(envPath) > 0 && envPath[0] != "" {
			v.SetConfigFile(envPath[0])
		} else {
			v.SetConfigFile(".env")
		}
		_ = v.ReadInConfig() // ignore error if .env not found

		// Override with environment variables
		v.AutomaticEnv()

		// Explicit BindEnv for all config fields — required for Unmarshal
		// to correctly read Docker-injected env vars when .env file is absent.
		for _, key := range []string{
			"APP_NAME", "DEBUG", "API_PREFIX", "SERVER_PORT",
			"JWT_SECRET_KEY", "JWT_ALGORITHM", "ACCESS_TOKEN_EXPIRE_MINUTES", "ALLOW_DEV_AUTH_BYPASS",
			"CORS_ORIGINS",
			"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB",
			// "QDRANT_HOST", "QDRANT_PORT", "QDRANT_COLLECTION", // deprecated
			"REDIS_HOST", "REDIS_PORT", "REDIS_DB",
			"LLM_PROVIDER",
			"OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_BASE_URL",
			"DEEPSEEK_API_KEY", "DEEPSEEK_MODEL", "DEEPSEEK_BASE_URL",
			"ANTHROPIC_API_KEY", "ANTHROPIC_MODEL",
			"GEMINI_API_KEY", "GEMINI_MODEL",
			"QWEN_API_KEY", "QWEN_MODEL",
			"DOUBAO_API_KEY", "DOUBAO_MODEL", "DOUBAO_ENDPOINT_ID",
			"EMBEDDING_PROVIDER", "EMBEDDING_API_KEY", "EMBEDDING_BASE_URL", "EMBEDDING_MODEL_NAME", "EMBEDDING_DIMENSION",
			"ENABLE_RERANK", "RERANK_PROVIDER", "RERANK_API_KEY", "RERANK_MODEL_NAME", "RERANK_TOP_N",
			"IMPORTANCE_THRESHOLD", "REFLECTION_THRESHOLD", "MAX_CONTEXT_MEMORIES",
			"MEMFS_ROOT_PATH",
			"WEB_SEARCH_PROVIDER", "WEB_SEARCH_API_KEY",
			"AGFS_URL",
			"TIERED_LOADING_TOKEN_BUDGET", "TIERED_LOADING_TOP_K",
			"HTTP_PROXY", "HTTPS_PROXY",
			"ALGO_API_KEYS",
		} {
			_ = v.BindEnv(key)
		}

		cfg = &Config{}
		if err := v.Unmarshal(cfg); err != nil {
			panic(fmt.Sprintf("failed to unmarshal config: %v", err))
		}

		// --- 关键安全校验 ---
		// JWT 密钥未设置时拒绝启动
		if cfg.JWTSecretKey == "" {
			panic("FATAL: JWT_SECRET_KEY is not set. Refusing to start without a signing key.")
		}
		// 数据库密码未设置时拒绝启动
		if cfg.PostgresPassword == "" {
			panic("FATAL: POSTGRES_PASSWORD is not set. Refusing to start without a database password.")
		}
		// 生产环境下 Dev Auth Bypass 告警
		if cfg.AllowDevAuthBypass && !cfg.Debug {
			slog.Warn("[SECURITY] ALLOW_DEV_AUTH_BYPASS is enabled in non-debug mode! This is a security risk.")
		}

		// Handle CORS_ORIGINS as comma-separated string → slice
		if corsStr := v.GetString("CORS_ORIGINS"); corsStr != "" {
			cfg.CORSOrigins = strings.Split(corsStr, ",")
			for i := range cfg.CORSOrigins {
				cfg.CORSOrigins[i] = strings.TrimSpace(cfg.CORSOrigins[i])
			}
		}
	})

	return cfg
}

// Get returns the loaded configuration singleton.
// Panics if Load has not been called yet.
func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

// SetForTest allows tests to inject a Config instance without loading from env.
// Only use in test code.
func SetForTest(c *Config) {
	cfg = c
}
