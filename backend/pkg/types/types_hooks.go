package types

// Hooks 配置类型 — 继承自 src/config/types.hooks.ts (125 行)

// ============================================================
// Hook 映射 (Webhook Handlers)
// ============================================================

// HookMappingMatch Hook 映射匹配条件
type HookMappingMatch struct {
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
}

// HookMappingTransform Hook 映射转换
type HookMappingTransform struct {
	Module string `json:"module"`
	Export string `json:"export,omitempty"`
}

// HookMappingConfig Hook 映射配置
type HookMappingConfig struct {
	ID                         string                `json:"id,omitempty"`
	Match                      *HookMappingMatch     `json:"match,omitempty"`
	Action                     string                `json:"action,omitempty"`   // "wake"|"agent"
	WakeMode                   string                `json:"wakeMode,omitempty"` // "now"|"next-heartbeat"
	Name                       string                `json:"name,omitempty"`
	SessionKey                 string                `json:"sessionKey,omitempty"`
	MessageTemplate            string                `json:"messageTemplate,omitempty"`
	TextTemplate               string                `json:"textTemplate,omitempty"`
	Deliver                    *bool                 `json:"deliver,omitempty"`
	AllowUnsafeExternalContent *bool                 `json:"allowUnsafeExternalContent,omitempty"`
	Channel                    string                `json:"channel,omitempty"` // "last"|channel name
	To                         string                `json:"to,omitempty"`
	Model                      string                `json:"model,omitempty"`
	Thinking                   string                `json:"thinking,omitempty"`
	TimeoutSeconds             *int                  `json:"timeoutSeconds,omitempty"`
	Transform                  *HookMappingTransform `json:"transform,omitempty"`
}

// ============================================================
// Gmail Hooks
// ============================================================

// HooksGmailTailscaleMode Gmail Tailscale 模式
type HooksGmailTailscaleMode string

const (
	GmailTailscaleOff    HooksGmailTailscaleMode = "off"
	GmailTailscaleServe  HooksGmailTailscaleMode = "serve"
	GmailTailscaleFunnel HooksGmailTailscaleMode = "funnel"
)

// HooksGmailServeConfig Gmail 服务配置
type HooksGmailServeConfig struct {
	Bind string `json:"bind,omitempty"`
	Port *int   `json:"port,omitempty"`
	Path string `json:"path,omitempty"`
}

// HooksGmailTailscaleConfig Gmail Tailscale 配置
type HooksGmailTailscaleConfig struct {
	Mode   HooksGmailTailscaleMode `json:"mode,omitempty"`
	Path   string                  `json:"path,omitempty"`
	Target string                  `json:"target,omitempty"`
}

// HooksGmailConfig Gmail Hook 配置
type HooksGmailConfig struct {
	Account                    string                     `json:"account,omitempty"`
	Label                      string                     `json:"label,omitempty"`
	Topic                      string                     `json:"topic,omitempty"`
	Subscription               string                     `json:"subscription,omitempty"`
	PushToken                  string                     `json:"pushToken,omitempty"`
	HookURL                    string                     `json:"hookUrl,omitempty"`
	IncludeBody                *bool                      `json:"includeBody,omitempty"`
	MaxBytes                   *int                       `json:"maxBytes,omitempty"`
	RenewEveryMinutes          *int                       `json:"renewEveryMinutes,omitempty"`
	AllowUnsafeExternalContent *bool                      `json:"allowUnsafeExternalContent,omitempty"`
	Serve                      *HooksGmailServeConfig     `json:"serve,omitempty"`
	Tailscale                  *HooksGmailTailscaleConfig `json:"tailscale,omitempty"`
	Model                      string                     `json:"model,omitempty"`
	Thinking                   string                     `json:"thinking,omitempty"`
}

// ============================================================
// 内部 Hooks
// ============================================================

// InternalHookHandlerConfig 内部 Hook 处理器配置
type InternalHookHandlerConfig struct {
	Event  string `json:"event"`
	Module string `json:"module"`
	Export string `json:"export,omitempty"`
}

// HookConfig 单个 Hook 配置
type HookConfig struct {
	Enabled *bool             `json:"enabled,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// HookInstallRecord Hook 安装记录
type HookInstallRecord struct {
	Source      string   `json:"source"` // "npm"|"archive"|"path"
	Spec        string   `json:"spec,omitempty"`
	SourcePath  string   `json:"sourcePath,omitempty"`
	InstallPath string   `json:"installPath,omitempty"`
	Version     string   `json:"version,omitempty"`
	InstalledAt string   `json:"installedAt,omitempty"`
	Hooks       []string `json:"hooks,omitempty"`
}

// InternalHooksLoadConfig 内部 Hooks 加载配置
type InternalHooksLoadConfig struct {
	ExtraDirs []string `json:"extraDirs,omitempty"`
}

// InternalHooksConfig 内部 Hooks 总配置
type InternalHooksConfig struct {
	Enabled  *bool                         `json:"enabled,omitempty"`
	Handlers []InternalHookHandlerConfig   `json:"handlers,omitempty"`
	Entries  map[string]*HookConfig        `json:"entries,omitempty"`
	Load     *InternalHooksLoadConfig      `json:"load,omitempty"`
	Installs map[string]*HookInstallRecord `json:"installs,omitempty"`
}

// HooksConfig Hooks 总配置
type HooksConfig struct {
	Enabled       *bool                `json:"enabled,omitempty"`
	Path          string               `json:"path,omitempty"`
	Token         string               `json:"token,omitempty"`
	MaxBodyBytes  *int                 `json:"maxBodyBytes,omitempty"`
	Presets       []string             `json:"presets,omitempty"`
	TransformsDir string               `json:"transformsDir,omitempty"`
	Mappings      []HookMappingConfig  `json:"mappings,omitempty"`
	Gmail         *HooksGmailConfig    `json:"gmail,omitempty"`
	Internal      *InternalHooksConfig `json:"internal,omitempty"`
}
