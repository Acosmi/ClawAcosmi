package types

// Cron 配置类型 — 继承自 src/config/types.cron.ts (6 行)

// CronConfig 定时任务配置
type CronConfig struct {
	Enabled           *bool  `json:"enabled,omitempty"`
	Store             string `json:"store,omitempty"`
	MaxConcurrentRuns *int   `json:"maxConcurrentRuns,omitempty"`
}
