package types

// 插件配置类型 — 继承自 src/config/types.plugins.ts (37 行)

// PluginEntryConfig 单个插件配置
type PluginEntryConfig struct {
	Enabled *bool                  `json:"enabled,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// PluginSlotsConfig 插件槽位配置
type PluginSlotsConfig struct {
	Memory string `json:"memory,omitempty"` // "none" 禁用记忆插件
}

// PluginsLoadConfig 插件加载配置
type PluginsLoadConfig struct {
	Paths []string `json:"paths,omitempty"`
}

// PluginInstallRecord 插件安装记录
type PluginInstallRecord struct {
	Source      string `json:"source"` // "npm"|"archive"|"path"
	Spec        string `json:"spec,omitempty"`
	SourcePath  string `json:"sourcePath,omitempty"`
	InstallPath string `json:"installPath,omitempty"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
}

// PluginsConfig 插件总配置
type PluginsConfig struct {
	Enabled  *bool                           `json:"enabled,omitempty"`
	Allow    []string                        `json:"allow,omitempty"`
	Deny     []string                        `json:"deny,omitempty"`
	Load     *PluginsLoadConfig              `json:"load,omitempty"`
	Slots    *PluginSlotsConfig              `json:"slots,omitempty"`
	Entries  map[string]*PluginEntryConfig   `json:"entries,omitempty"`
	Installs map[string]*PluginInstallRecord `json:"installs,omitempty"`
}
