package types

// Memory 配置类型 — 继承自 src/config/types.memory.ts (51 行)

// MemoryBackend 记忆后端类型
type MemoryBackend string

const (
	MemoryBackendBuiltin MemoryBackend = "builtin"
	MemoryBackendQmd     MemoryBackend = "qmd"
)

// MemoryCitationsMode 引用模式
type MemoryCitationsMode string

const (
	MemoryCitationsAuto MemoryCitationsMode = "auto"
	MemoryCitationsOn   MemoryCitationsMode = "on"
	MemoryCitationsOff  MemoryCitationsMode = "off"
)

// MemoryQmdIndexPath QMD 索引路径
type MemoryQmdIndexPath struct {
	Path    string `json:"path"`
	Name    string `json:"name,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// MemoryQmdSessionConfig QMD 会话配置
type MemoryQmdSessionConfig struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	ExportDir     string `json:"exportDir,omitempty"`
	RetentionDays *int   `json:"retentionDays,omitempty"`
}

// MemoryQmdUpdateConfig QMD 更新配置
type MemoryQmdUpdateConfig struct {
	Interval         string `json:"interval,omitempty"`
	DebounceMs       *int   `json:"debounceMs,omitempty"`
	OnBoot           *bool  `json:"onBoot,omitempty"`
	WaitForBootSync  *bool  `json:"waitForBootSync,omitempty"`
	EmbedInterval    string `json:"embedInterval,omitempty"`
	CommandTimeoutMs *int   `json:"commandTimeoutMs,omitempty"`
	UpdateTimeoutMs  *int   `json:"updateTimeoutMs,omitempty"`
	EmbedTimeoutMs   *int   `json:"embedTimeoutMs,omitempty"`
}

// MemoryQmdLimitsConfig QMD 限制配置
type MemoryQmdLimitsConfig struct {
	MaxResults       *int `json:"maxResults,omitempty"`
	MaxSnippetChars  *int `json:"maxSnippetChars,omitempty"`
	MaxInjectedChars *int `json:"maxInjectedChars,omitempty"`
	TimeoutMs        *int `json:"timeoutMs,omitempty"`
}

// MemoryQmdConfig QMD 记忆配置
type MemoryQmdConfig struct {
	Command              string                   `json:"command,omitempty"`
	IncludeDefaultMemory *bool                    `json:"includeDefaultMemory,omitempty"`
	Paths                []MemoryQmdIndexPath     `json:"paths,omitempty"`
	Sessions             *MemoryQmdSessionConfig  `json:"sessions,omitempty"`
	Update               *MemoryQmdUpdateConfig   `json:"update,omitempty"`
	Limits               *MemoryQmdLimitsConfig   `json:"limits,omitempty"`
	Scope                *SessionSendPolicyConfig `json:"scope,omitempty"`
}

// MemoryConfig 记忆总配置
type MemoryConfig struct {
	Backend   MemoryBackend       `json:"backend,omitempty"`
	Citations MemoryCitationsMode `json:"citations,omitempty"`
	Qmd       *MemoryQmdConfig    `json:"qmd,omitempty"`
	UHMS      *MemoryUHMSConfig   `json:"uhms,omitempty"` // UHMS 统一层级记忆系统
}

// MemoryUHMSConfig — UHMS 配置 (镜像 uhms.UHMSConfig, 避免循环导入)。
// 详细文档见 internal/memory/uhms/config.go。
type MemoryUHMSConfig struct {
	Enabled                   bool   `json:"enabled,omitempty"`
	DBPath                    string `json:"dbPath,omitempty"`                    // SQLite 路径, 默认 ~/.openacosmi/memory/uhms.db
	VFSPath                   string `json:"vfsPath,omitempty"`                   // VFS 根目录, 默认 ~/.openacosmi/memory/vfs/
	VectorMode                string `json:"vectorMode,omitempty"`                // off(默认)|builtin|ffi|segment|qdrant|hybrid
	CompressionThreshold      int    `json:"compressionThreshold,omitempty"`      // 触发压缩的 token 阈值, 默认 200000
	DecayEnabled              *bool  `json:"decayEnabled,omitempty"`              // FSRS-6 衰减, 默认 true
	DecayIntervalHours        int    `json:"decayIntervalHours,omitempty"`        // 衰减周期(小时), 默认 6
	MaxMemories               int    `json:"maxMemories,omitempty"`               // 每用户最大记忆数, 默认 100000
	TieredLoadingEnabled      *bool  `json:"tieredLoadingEnabled,omitempty"`      // L0/L1/L2 渐进加载, 默认 true
	EmbeddingProvider         string `json:"embeddingProvider,omitempty"`         // auto|ollama|openai|anthropic|cohere|local
	EmbeddingModel            string `json:"embeddingModel,omitempty"`            // 空=按 provider 默认 (e.g. text-embedding-3-small)
	EmbeddingBaseURL          string `json:"embeddingBaseUrl,omitempty"`          // 空=使用 provider 默认 URL
	QdrantEndpoint            string `json:"qdrantEndpoint,omitempty"`            // Qdrant 服务器地址, 默认 http://localhost:6334
	LLMProvider               string `json:"llmProvider,omitempty"`               // 空=跟随 agent, "anthropic"|"openai"|"ollama"|"deepseek"|...
	LLMModel                  string `json:"llmModel,omitempty"`                  // 空=按 provider 默认
	LLMBaseURL                string `json:"llmBaseUrl,omitempty"`                // 空=使用 provider 默认 URL
	LLMApiKey                 string `json:"llmApiKey,omitempty"`                 // 独立 API key, 空=从 agent providers 查找
	CompressionTriggerPercent int    `json:"compressionTriggerPercent,omitempty"` // 0-100, 0=legacy
	ObservationMaskTurns      int    `json:"observationMaskTurns,omitempty"`      // 遮蔽 N 轮前的 tool 输出, 0=关闭
	KeepRecentMessages        int    `json:"keepRecentMessages,omitempty"`        // 保留最近 N 条, 0=默认5
	BootFilePath              string `json:"bootFilePath,omitempty"`              // Boot 启动文件路径, 默认 ~/.openacosmi/memory/boot.json
}
