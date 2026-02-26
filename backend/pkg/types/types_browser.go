package types

// 浏览器配置类型 — 继承自 src/config/types.browser.ts (42 行)

// BrowserProfileConfig 浏览器配置文件
type BrowserProfileConfig struct {
	CdpPort *int   `json:"cdpPort,omitempty"`
	CdpURL  string `json:"cdpUrl,omitempty"`
	Driver  string `json:"driver,omitempty"` // "openacosmi"|"extension"
	Color   string `json:"color"`
}

// BrowserSnapshotDefaults 浏览器快照默认值
type BrowserSnapshotDefaults struct {
	Mode string `json:"mode,omitempty"` // "efficient"
}

// BrowserConfig 浏览器总配置
type BrowserConfig struct {
	Enabled                     *bool                            `json:"enabled,omitempty"`
	EvaluateEnabled             *bool                            `json:"evaluateEnabled,omitempty"`
	CdpURL                      string                           `json:"cdpUrl,omitempty"`
	RemoteCdpTimeoutMs          *int                             `json:"remoteCdpTimeoutMs,omitempty"`
	RemoteCdpHandshakeTimeoutMs *int                             `json:"remoteCdpHandshakeTimeoutMs,omitempty"`
	Color                       string                           `json:"color,omitempty"`
	ExecutablePath              string                           `json:"executablePath,omitempty"`
	Headless                    *bool                            `json:"headless,omitempty"`
	NoSandbox                   *bool                            `json:"noSandbox,omitempty"`
	AttachOnly                  *bool                            `json:"attachOnly,omitempty"`
	DefaultProfile              string                           `json:"defaultProfile,omitempty"`
	Profiles                    map[string]*BrowserProfileConfig `json:"profiles,omitempty"`
	SnapshotDefaults            *BrowserSnapshotDefaults         `json:"snapshotDefaults,omitempty"`
}
